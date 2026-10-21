// Package webhooks sends the events of the panel to the outside world.
//
// A subscription names an address and a secret. Every event the
// subscription asks for is sent to that address, with the body signed, so
// the receiver can tell that the body really comes from the panel and that
// nobody changed it on the way.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/events"
	"github.com/ryxoui/ryxoui/internal/id"
)

// SignatureHeader carries the signature of the body.
const SignatureHeader = "X-RyxoUI-Signature"

// EventHeader carries the name of the event.
const EventHeader = "X-RyxoUI-Event"

// DeliveryHeader carries the identifier of the event.
const DeliveryHeader = "X-RyxoUI-Delivery"

// ErrNotFound is returned when a subscription does not exist.
var ErrNotFound = errors.New("webhooks: subscription not found")

// ErrDuplicateName is returned when the name of a subscription is taken.
var ErrDuplicateName = errors.New("webhooks: the name of the subscription is already taken")

// Subscription is a place the events of the panel are sent to.
type Subscription struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Secret    string   `json:"-"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
}

// Wants reports whether the subscription asks for an event.
func (s Subscription) Wants(event string) bool {
	if len(s.Events) == 0 {
		return true
	}
	for _, item := range s.Events {
		if item == event {
			return true
		}
		if strings.HasSuffix(item, "*") && strings.HasPrefix(event, strings.TrimSuffix(item, "*")) {
			return true
		}
	}
	return false
}

// Store reads and writes the subscriptions.
type Store struct {
	db db.Querier
}

// NewStore returns a subscription store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// List returns every subscription.
func (s *Store) List(ctx context.Context) ([]Subscription, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, url, secret, events, enabled, created_at, updated_at FROM webhook_subscriptions ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("webhooks: list the subscriptions: %w", err)
	}
	defer rows.Close()

	out := []Subscription{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Get reads one subscription.
func (s *Store) Get(ctx context.Context, subscriptionID string) (Subscription, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, name, url, secret, events, enabled, created_at, updated_at FROM webhook_subscriptions WHERE id = ?", subscriptionID)
	item, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	if err != nil {
		return Subscription{}, err
	}
	return item, nil
}

// Create stores a subscription.
func (s *Store) Create(ctx context.Context, item Subscription) (Subscription, error) {
	if strings.TrimSpace(item.Name) == "" {
		return Subscription{}, errors.New("webhooks: the name of the subscription must not be empty")
	}
	if strings.TrimSpace(item.URL) == "" {
		return Subscription{}, errors.New("webhooks: the address of the subscription must not be empty")
	}

	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO webhook_subscriptions (id, name, url, secret, events, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		item.ID, item.Name, item.URL, item.Secret, strings.Join(item.Events, ","), boolToInt(item.Enabled), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return Subscription{}, ErrDuplicateName
		}
		return Subscription{}, fmt.Errorf("webhooks: insert the subscription: %w", err)
	}
	return item, nil
}

// Update writes a subscription.
func (s *Store) Update(ctx context.Context, item Subscription) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE webhook_subscriptions SET name = ?, url = ?, secret = ?, events = ?, enabled = ?, updated_at = ? WHERE id = ?",
		item.Name, item.URL, item.Secret, strings.Join(item.Events, ","), boolToInt(item.Enabled), db.Now(), item.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("webhooks: update the subscription: %w", err)
	}
	return nil
}

// Delete removes a subscription.
func (s *Store) Delete(ctx context.Context, subscriptionID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM webhook_subscriptions WHERE id = ?", subscriptionID); err != nil {
		return fmt.Errorf("webhooks: delete the subscription: %w", err)
	}
	return nil
}

// Subscriber sends the events of the panel to the subscriptions.
type Subscriber struct {
	store  *Store
	client *http.Client
	logger *slog.Logger
}

// NewSubscriber returns a subscriber.
func NewSubscriber(store *Store, client *http.Client, logger *slog.Logger) *Subscriber {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Subscriber{store: store, client: client, logger: logger}
}

// Name identifies the subscriber in the delivery records.
func (s *Subscriber) Name() string { return "webhooks" }

// Deliver sends an event to every subscription that asks for it.
func (s *Subscriber) Deliver(ctx context.Context, msg events.Message) error {
	subscriptions, err := s.store.List(ctx)
	if err != nil {
		return err
	}

	failures := []error{}
	for _, subscription := range subscriptions {
		if !subscription.Enabled || !subscription.Wants(msg.Event) {
			continue
		}
		if err := s.send(ctx, subscription, msg); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", subscription.Name, err))
		}
	}
	return errors.Join(failures...)
}

// send delivers one event to one subscription.
func (s *Subscriber) send(ctx context.Context, subscription Subscription, msg events.Message) error {
	body, err := json.Marshal(map[string]any{
		"id":        msg.ID,
		"event":     msg.Event,
		"payload":   msg.Payload,
		"createdAt": msg.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("webhooks: encode the event: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhooks: build the request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(EventHeader, msg.Event)
	request.Header.Set(DeliveryHeader, msg.ID)
	if subscription.Secret != "" {
		request.Header.Set(SignatureHeader, "sha256="+Sign(body, subscription.Secret))
	}

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("webhooks: send the event: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("webhooks: the receiver answered with %d", response.StatusCode)
	}
	return nil
}

// Sign is the signature of a body with a secret.
func Sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether a body carries the signature of a secret.
func Verify(body []byte, secret, signature string) bool {
	expected := "sha256=" + Sign(body, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// scanner reads one subscription.
type scanner interface {
	Scan(dest ...any) error
}

// scan reads a subscription.
func scan(row scanner) (Subscription, error) {
	var (
		item    Subscription
		events  string
		enabled int
	)
	if err := row.Scan(&item.ID, &item.Name, &item.URL, &item.Secret, &events, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Subscription{}, err
		}
		return Subscription{}, fmt.Errorf("webhooks: scan the subscription: %w", err)
	}
	item.Enabled = enabled != 0
	item.Events = splitList(events)
	return item, nil
}

// splitList reads a list of names.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// boolToInt turns a flag into the number the database holds.
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// isUniqueViolation reports whether the database refused a duplicate.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "UNIQUE constraint failed") || strings.Contains(text, "duplicate key")
}
