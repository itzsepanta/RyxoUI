package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// Delivery is one attempt to hand an event to a subscriber.
type Delivery struct {
	ID         string `json:"id"`
	OutboxID   string `json:"outboxId"`
	Subscriber string `json:"subscriber"`
	Attempt    int    `json:"attempt"`
	Status     string `json:"status"`
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
	NextAt     int64  `json:"nextAt"`
	Delivered  int64  `json:"deliveredAt"`
	CreatedAt  int64  `json:"createdAt"`
}

// Delivery status values.
const (
	StatusPending   = "pending"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"
	StatusDead      = "dead"
)

// Subscriber receives events.
type Subscriber interface {
	// Name identifies the subscriber in delivery records.
	Name() string
	// Deliver hands one event to the subscriber.
	Deliver(ctx context.Context, msg Message) error
}

// Dispatcher moves events from the outbox to the subscribers.
type Dispatcher struct {
	outbox      *Outbox
	db          *db.DB
	subscribers []Subscriber
	log         *slog.Logger
	interval    time.Duration
	batch       int
	maxTries    int

	mu      sync.Mutex
	stopped bool
}

// DispatcherOptions configures a dispatcher.
type DispatcherOptions struct {
	Interval time.Duration
	Batch    int
	MaxTries int
	Logger   *slog.Logger
}

// NewDispatcher returns a dispatcher.
func NewDispatcher(outbox *Outbox, handle *db.DB, subscribers []Subscriber, opts DispatcherOptions) *Dispatcher {
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	if opts.Batch <= 0 {
		opts.Batch = 50
	}
	if opts.MaxTries <= 0 {
		opts.MaxTries = 5
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		outbox:      outbox,
		db:          handle,
		subscribers: subscribers,
		log:         logger,
		interval:    opts.Interval,
		batch:       opts.Batch,
		maxTries:    opts.MaxTries,
	}
}

// Run polls the outbox until the context is cancelled.
func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(intervalOf(d))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.DispatchOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				d.log.Error("event dispatch failed", "error", err)
			}
		}
	}
}

// DispatchOnce delivers one batch. Every message is attempted even when an
// earlier one fails; the failures are reported together.
func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	messages, err := d.outbox.Due(ctx, batchOf(d))
	if err != nil {
		return err
	}
	var failures []error
	for _, msg := range messages {
		if err := d.deliver(ctx, msg); err != nil {
			d.log.Error("event delivery failed", "event", msg.Event, "error", err)
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (d *Dispatcher) deliver(ctx context.Context, msg Message) error {
	if len(d.subscribers) == 0 {
		// Nobody is listening yet: keep the message for later subscribers.
		return nil
	}

	var lastErr error
	for _, subscriber := range d.subscribers {
		attempt := d.attemptCount(ctx, msg.ID, subscriber.Name())
		delivery := Delivery{
			ID:         id.New(),
			OutboxID:   msg.ID,
			Subscriber: subscriber.Name(),
			Attempt:    attempt + 1,
			Status:     StatusDelivered,
			CreatedAt:  db.Now(),
		}

		err := subscriber.Deliver(ctx, msg)
		switch {
		case err == nil:
			delivery.Delivered = db.Now()
		case errors.Is(err, ErrPermanent):
			delivery.Status = StatusDead
			delivery.Error = err.Error()
			lastErr = err
		default:
			delivery.Status = StatusFailed
			delivery.Error = err.Error()
			delivery.NextAt = time.Now().UTC().Add(backoff(attempt + 1)).Unix()
			lastErr = err
		}

		if err := d.recordDelivery(ctx, delivery); err != nil {
			return err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return d.outbox.Remove(ctx, d.db, msg.ID)
}

func (d *Dispatcher) attemptCount(ctx context.Context, outboxID, subscriber string) int {
	var attempt int
	err := d.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(attempt), 0) FROM event_deliveries WHERE outbox_id = ? AND subscriber = ?",
		outboxID, subscriber).Scan(&attempt)
	if err != nil {
		return 0
	}
	return attempt
}

func (d *Dispatcher) recordDelivery(ctx context.Context, delivery Delivery) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO event_deliveries (id, outbox_id, subscriber, attempt, status, status_code, error, next_at, delivered_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.ID, delivery.OutboxID, delivery.Subscriber, delivery.Attempt, delivery.Status,
		delivery.StatusCode, delivery.Error, delivery.NextAt, delivery.Delivered, delivery.CreatedAt)
	if err != nil {
		return fmt.Errorf("events: record delivery: %w", err)
	}
	return nil
}

// ErrPermanent marks a failure that must not be retried.
var ErrPermanent = errors.New("events: permanent delivery failure")

// backoff returns the delay before the next attempt.
func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 10 {
		attempt = 10
	}
	delay := time.Duration(1<<uint(attempt-1)) * time.Second
	if delay > 10*time.Minute {
		delay = 10 * time.Minute
	}
	return delay
}

// intervalOf and batchOf expose the dispatcher tuning to the tests.
func intervalOf(d *Dispatcher) time.Duration { return d.interval }

func batchOf(d *Dispatcher) int { return d.batch }
