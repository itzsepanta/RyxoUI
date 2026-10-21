package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/events"
	_ "modernc.org/sqlite"
)

func testStore(t *testing.T) *Store {
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
	return NewStore(q)
}

func TestTheSubscriptionIsStored(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, Subscription{
		Name:    "the receiver",
		URL:     "https://example.invalid/hook",
		Secret:  "the secret",
		Events:  []string{"node.created"},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected an identifier")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("subscriptions = %d, want 1", len(list))
	}
	if list[0].Secret != "the secret" {
		t.Fatalf("the secret was lost: %+v", list[0])
	}
	if len(list[0].Events) != 1 || list[0].Events[0] != "node.created" {
		t.Fatalf("the events were lost: %+v", list[0])
	}

	// The same name twice is refused.
	if _, err := store.Create(ctx, Subscription{Name: "the receiver", URL: "https://example.invalid/hook"}); err != ErrDuplicateName {
		t.Fatalf("error = %v, want ErrDuplicateName", err)
	}

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("subscriptions = %d, want 0", len(list))
	}
}

func TestTheSubscriptionAsksForItsEvents(t *testing.T) {
	all := Subscription{}
	if !all.Wants("node.created") {
		t.Fatal("a subscription without events wants every event")
	}

	some := Subscription{Events: []string{"node.created", "node.*"}}
	if !some.Wants("node.created") {
		t.Fatal("the event was asked for")
	}
	if !some.Wants("node.deleted") {
		t.Fatal("the family was asked for")
	}
	if some.Wants("operator.login") {
		t.Fatal("the event was not asked for")
	}
}

func TestTheEventIsSentWithItsSignature(t *testing.T) {
	var (
		body      []byte
		signature string
		event     string
		delivery  string
	)

	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		signature = r.Header.Get(SignatureHeader)
		event = r.Header.Get(EventHeader)
		delivery = r.Header.Get(DeliveryHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	store := testStore(t)
	if _, err := store.Create(context.Background(), Subscription{
		Name:    "the receiver",
		URL:     receiver.URL,
		Secret:  "the secret",
		Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	subscriber := NewSubscriber(store, receiver.Client(), nil)
	err := subscriber.Deliver(context.Background(), events.Message{
		ID:        "event-1",
		Event:     "node.created",
		Payload:   map[string]any{"name": "eu-1"},
		CreatedAt: 1700000000,
	})
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}

	if event != "node.created" {
		t.Fatalf("the event header is %q", event)
	}
	if delivery != "event-1" {
		t.Fatalf("the delivery header is %q", delivery)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode the body: %v", err)
	}
	if payload["event"] != "node.created" {
		t.Fatalf("the body is %v", payload)
	}
	if !Verify(body, "the secret", signature) {
		t.Fatalf("the signature %q does not match the body", signature)
	}
	if Verify(body, "another secret", signature) {
		t.Fatal("the signature must not match another secret")
	}
}

func TestTheSubscriptionThatIsOffGetsNothing(t *testing.T) {
	called := false
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	store := testStore(t)
	if _, err := store.Create(context.Background(), Subscription{
		Name:    "the receiver",
		URL:     receiver.URL,
		Enabled: false,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	subscriber := NewSubscriber(store, receiver.Client(), nil)
	if err := subscriber.Deliver(context.Background(), events.Message{Event: "node.created"}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if called {
		t.Fatal("a subscription that is off must not receive anything")
	}
}

func TestTheEventThatIsNotAskedForIsNotSent(t *testing.T) {
	called := false
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	store := testStore(t)
	if _, err := store.Create(context.Background(), Subscription{
		Name:    "the receiver",
		URL:     receiver.URL,
		Events:  []string{"operator.login"},
		Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	subscriber := NewSubscriber(store, receiver.Client(), nil)
	if err := subscriber.Deliver(context.Background(), events.Message{Event: "node.created"}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if called {
		t.Fatal("the event was not asked for")
	}
}
