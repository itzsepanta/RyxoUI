package nodes

import (
	"context"
	"log/slog"
	"time"
)

// CollectorOptions configures the collector.
type CollectorOptions struct {
	// Interval is the delay between two rounds. Zero means five minutes.
	Interval time.Duration
	// Timeout bounds the probe of a single node. Zero means thirty seconds.
	Timeout time.Duration
	// Logger receives the reports.
	Logger *slog.Logger
}

// Collector keeps the state of the nodes fresh.
//
// Every round it asks each enabled node about itself, records the answer as
// a health sample and moves the node to the status the answer deserves. A
// node that cannot be reached is reported, never silently ignored.
type Collector struct {
	store  *Store
	keys   *KeyStore
	probe  func(ctx context.Context, node Node, privateKey string) (ProbeResult, error)
	opts   CollectorOptions
	logger *slog.Logger
}

// NewCollector returns a collector.
func NewCollector(store *Store, keys *KeyStore, probe func(ctx context.Context, node Node, privateKey string) (ProbeResult, error), opts CollectorOptions) *Collector {
	if opts.Interval <= 0 {
		opts.Interval = 5 * time.Minute
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if probe == nil {
		probe = Probe
	}
	return &Collector{store: store, keys: keys, probe: probe, opts: opts, logger: opts.Logger}
}

// Run probes the nodes until the context is cancelled.
func (c *Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.opts.Interval)
	defer ticker.Stop()

	c.Round(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.Round(ctx)
		}
	}
}

// Round probes every enabled node once.
func (c *Collector) Round(ctx context.Context) {
	list, err := c.store.List(ctx)
	if err != nil {
		c.logger.Error("list the nodes", "error", err)
		return
	}

	for _, node := range list {
		if !node.Enabled {
			continue
		}
		c.probeNode(ctx, node)
	}
}

// probeNode asks one node about itself and records the answer.
func (c *Collector) probeNode(ctx context.Context, node Node) {
	if !node.KeyInstalled {
		return
	}

	key, err := c.keys.Get(ctx, node.SSHKeyID)
	if err != nil {
		c.logger.Error("read the key of the node", "node", node.Name, "error", err)
		return
	}

	timeout, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	result, err := c.probe(timeout, node, key.PrivateKey)
	if err != nil {
		c.logger.Warn("the node did not answer", "node", node.Name, "error", err)
		if setErr := c.store.SetStatus(ctx, node.ID, StatusError, err.Error()); setErr != nil {
			c.logger.Error("set the status of the node", "node", node.Name, "error", setErr)
		}
		return
	}

	if err := c.store.ApplyProbe(ctx, node.ID, result, StatusConnected, ""); err != nil {
		c.logger.Error("store the answer of the node", "node", node.Name, "error", err)
		return
	}
	if err := c.store.RecordHealth(ctx, result.HealthSample(node.ID, time.Now().UTC().Unix())); err != nil {
		c.logger.Error("record the health of the node", "node", node.Name, "error", err)
	}
}
