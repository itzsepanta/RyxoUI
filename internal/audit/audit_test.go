package audit

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
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

func TestRecordAndList(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	if err := store.Record(ctx, store.db, Entry{
		ActorID:   "op-1",
		ActorName: "owner",
		Action:    "operator.create",
		Entity:    "operator",
		EntityID:  "op-2",
		After:     map[string]any{"username": "watcher"},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, total, err := store.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("entries = %d, total = %d", len(entries), total)
	}
	entry := entries[0]
	if entry.Action != "operator.create" || entry.ActorName != "owner" {
		t.Fatalf("unexpected entry %+v", entry)
	}
	if entry.After == nil {
		t.Fatal("expected the after payload")
	}
}

func TestListFilters(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	for _, action := range []string{"operator.create", "operator.update", "role.create"} {
		if err := store.Record(ctx, store.db, Entry{
			ActorID: "op-1", ActorName: "owner", Action: action, Entity: "operator", EntityID: "op-2",
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	entries, total, err := store.List(ctx, Filter{Action: "role.create"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(entries) != 1 || entries[0].Action != "role.create" {
		t.Fatalf("filter failed: %d entries, total %d", len(entries), total)
	}
}

func TestFacets(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	for _, action := range []string{"operator.create", "operator.create", "role.create"} {
		if err := store.Record(ctx, store.db, Entry{
			ActorID: "op-1", ActorName: "owner", Action: action, Entity: "operator",
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	facets, err := store.Facets(ctx)
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if len(facets["action"]) != 2 {
		t.Fatalf("actions = %v", facets["action"])
	}
}
