package nodes

import (
	"context"
	"fmt"
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
	alerts *AlertStore
	probe  func(ctx context.Context, node Node, privateKey string) (ProbeResult, error)
	opts   CollectorOptions
	logger *slog.Logger
}

// NewCollector returns a collector.
func NewCollector(store *Store, keys *KeyStore, alerts *AlertStore, probe func(ctx context.Context, node Node, privateKey string) (ProbeResult, error), opts CollectorOptions) *Collector {
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
	return &Collector{store: store, keys: keys, alerts: alerts, probe: probe, opts: opts, logger: opts.Logger}
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
		c.checkAlerts(ctx, node, nil)
		return
	}

	if err := c.store.ApplyProbe(ctx, node.ID, result, StatusConnected, ""); err != nil {
		c.logger.Error("store the answer of the node", "node", node.Name, "error", err)
		return
	}
	if err := c.store.RecordHealth(ctx, result.HealthSample(node.ID, time.Now().UTC().Unix())); err != nil {
		c.logger.Error("record the health of the node", "node", node.Name, "error", err)
	}
	c.checkAlerts(ctx, node, &result)
}

// The shares of the resources that raise a warning.
const (
	diskWarning  = 90.0
	diskCritical = 98.0
	memWarning   = 90.0
	memCritical  = 98.0
)

// checkAlerts raises the warnings of a node and ends the ones that do not
// stand anymore.
func (c *Collector) checkAlerts(ctx context.Context, node Node, result *ProbeResult) {
	if c.alerts == nil {
		return
	}

	if result == nil {
		c.raise(ctx, node, AlertUnreachable, SeverityCritical, "the node does not answer")
		c.clear(ctx, node, AlertDisk)
		c.clear(ctx, node, AlertMemory)
		return
	}

	c.clear(ctx, node, AlertUnreachable)

	disk := share(result.DiskUsed, result.DiskTotal)
	switch {
	case disk >= diskCritical:
		c.raise(ctx, node, AlertDisk, SeverityCritical, fmt.Sprintf("the disk of the node is %.0f%% full", disk))
	case disk >= diskWarning:
		c.raise(ctx, node, AlertDisk, SeverityWarning, fmt.Sprintf("the disk of the node is %.0f%% full", disk))
	default:
		c.clear(ctx, node, AlertDisk)
	}

	memory := share(result.MemUsed, result.MemTotal)
	switch {
	case memory >= memCritical:
		c.raise(ctx, node, AlertMemory, SeverityCritical, fmt.Sprintf("the memory of the node is %.0f%% used", memory))
	case memory >= memWarning:
		c.raise(ctx, node, AlertMemory, SeverityWarning, fmt.Sprintf("the memory of the node is %.0f%% used", memory))
	default:
		c.clear(ctx, node, AlertMemory)
	}
}

// raise records a warning.
func (c *Collector) raise(ctx context.Context, node Node, kind, severity, message string) {
	if err := c.alerts.Raise(ctx, node.ID, kind, severity, message); err != nil {
		c.logger.Error("raise the warning", "node", node.Name, "kind", kind, "error", err)
	}
}

// clear ends a warning.
func (c *Collector) clear(ctx context.Context, node Node, kind string) {
	if err := c.alerts.Clear(ctx, node.ID, kind); err != nil {
		c.logger.Error("clear the warning", "node", node.Name, "kind", kind, "error", err)
	}
}

// share is the used part of a resource in percent.
func share(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}
