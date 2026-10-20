package events

import (
	"context"
	"fmt"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// Message is one queued event.
type Message struct {
	ID        string         `json:"id"`
	Event     string         `json:"event"`
	Payload   map[string]any `json:"payload"`
	CreatedAt int64          `json:"createdAt"`
}

// Outbox is the durable queue of pending events.
type Outbox struct {
	db db.Querier
}

// NewOutbox returns an outbox.
func NewOutbox(q db.Querier) *Outbox { return &Outbox{db: q} }

// Publish appends an event. It must be called with the transaction that
// performed the change so the event is delivered exactly once the change
// is committed.
func (o *Outbox) Publish(ctx context.Context, q db.Querier, event string, payload map[string]any) error {
	encoded, err := EncodePayload(payload)
	if err != nil {
		return err
	}
	now := db.Now()
	_, err = q.ExecContext(ctx,
		"INSERT INTO event_outbox (id, event, payload, created_at, available_at) VALUES (?, ?, ?, ?, ?)",
		id.New(), event, encoded, now, now)
	if err != nil {
		return fmt.Errorf("events: publish %s: %w", event, err)
	}
	return nil
}

// Due returns the messages that are ready for delivery.
func (o *Outbox) Due(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := o.db.QueryContext(ctx,
		`SELECT id, event, payload, created_at FROM event_outbox
		 WHERE available_at <= ? ORDER BY created_at ASC LIMIT ?`, db.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("events: due: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var (
			msg     Message
			payload string
		)
		if err := rows.Scan(&msg.ID, &msg.Event, &payload, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("events: scan: %w", err)
		}
		decoded, err := DecodePayload(payload)
		if err != nil {
			return nil, err
		}
		msg.Payload = decoded
		out = append(out, msg)
	}
	return out, rows.Err()
}

// Remove deletes a delivered message from the outbox.
func (o *Outbox) Remove(ctx context.Context, q db.Querier, messageID string) error {
	if _, err := q.ExecContext(ctx, "DELETE FROM event_outbox WHERE id = ?", messageID); err != nil {
		return fmt.Errorf("events: remove %s: %w", messageID, err)
	}
	return nil
}

// Defer pushes a message back into the queue for a later attempt.
func (o *Outbox) Defer(ctx context.Context, q db.Querier, messageID string, at time.Time) error {
	if _, err := q.ExecContext(ctx, "UPDATE event_outbox SET available_at = ? WHERE id = ?",
		at.UTC().Unix(), messageID); err != nil {
		return fmt.Errorf("events: defer %s: %w", messageID, err)
	}
	return nil
}
