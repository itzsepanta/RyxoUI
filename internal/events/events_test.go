package events

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func newDB(t *testing.T) *db.DB {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(4)
	t.Cleanup(func() { handle.Close() })

	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return q
}

func TestPublishAndDue(t *testing.T) {
	handle := newDB(t)
	ctx := context.Background()
	outbox := NewOutbox(handle)

	if err := outbox.Publish(ctx, handle, "operator.created", map[string]any{"username": "owner"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	messages, err := outbox.Due(ctx, 10)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	if messages[0].Event != "operator.created" {
		t.Fatalf("event = %q", messages[0].Event)
	}
	if messages[0].Payload["username"] != "owner" {
		t.Fatalf("payload = %v", messages[0].Payload)
	}
}

// recordingSubscriber counts the deliveries it receives.
type recordingSubscriber struct {
	name    string
	calls   int
	failure error
}

func (s *recordingSubscriber) Name() string { return s.name }

func (s *recordingSubscriber) Deliver(ctx context.Context, msg Message) error {
	s.calls++
	return s.failure
}

func TestDispatcherDeliversAndRemoves(t *testing.T) {
	handle := newDB(t)
	ctx := context.Background()
	outbox := NewOutbox(handle)

	if err := outbox.Publish(ctx, handle, "operator.created", map[string]any{"username": "owner"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	subscriber := &recordingSubscriber{name: "test"}
	dispatcher := NewDispatcher(outbox, handle, []Subscriber{subscriber}, DispatcherOptions{})

	if err := dispatcher.DispatchOnce(ctx); err != nil {
		t.Fatalf("DispatchOnce: %v", err)
	}
	if subscriber.calls != 1 {
		t.Fatalf("calls = %d, want 1", subscriber.calls)
	}

	messages, err := outbox.Due(ctx, 10)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("outbox not drained: %d messages", len(messages))
	}

	var deliveries int
	if err := handle.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_deliveries").Scan(&deliveries); err != nil {
		t.Fatalf("count: %v", err)
	}
	if deliveries != 1 {
		t.Fatalf("deliveries = %d, want 1", deliveries)
	}
}

func TestDispatcherRecordsFailures(t *testing.T) {
	handle := newDB(t)
	ctx := context.Background()
	outbox := NewOutbox(handle)

	if err := outbox.Publish(ctx, handle, "operator.created", nil); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	subscriber := &recordingSubscriber{name: "failing", failure: errors.New("boom")}
	dispatcher := NewDispatcher(outbox, handle, []Subscriber{subscriber}, DispatcherOptions{})

	if err := dispatcher.DispatchOnce(ctx); err == nil {
		t.Fatal("expected the dispatch to report the failure")
	}

	var status string
	if err := handle.QueryRowContext(ctx, "SELECT status FROM event_deliveries").Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != StatusFailed {
		t.Fatalf("status = %q, want %q", status, StatusFailed)
	}

	// The message must stay in the outbox for a later attempt.
	messages, err := outbox.Due(ctx, 10)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	registry := NewRegistry()
	entry := Type{Name: "user.created", Family: FamilyPanel, Payload: []Field{{Name: "id", Type: "string"}}}
	if err := registry.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := registry.Register(entry); err == nil {
		t.Fatal("expected a duplicate error")
	}
	if _, ok := registry.Get("user.created"); !ok {
		t.Fatal("expected the event to be registered")
	}
	if len(registry.List()) != 1 {
		t.Fatalf("list = %d", len(registry.List()))
	}
}

func TestDefaultRegistryCoversCoreEvents(t *testing.T) {
	registry := DefaultRegistry()
	for _, name := range []string{"operator.created", "operator.login", "role.updated", "settings.updated"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("event %s is missing", name)
		}
	}
}
