package nodes

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
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

func TestNodeLifecycle(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, Node{
		Name:    "node-1",
		Address: "10.0.0.5",
		Tags:    []string{"eu", "fast"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Port != 62050 || created.SSHPort != 22 || created.SSHUser != "root" {
		t.Fatalf("unexpected defaults: %+v", created)
	}
	if created.Status != StatusConnecting {
		t.Fatalf("status = %q", created.Status)
	}

	if _, err := store.Create(ctx, Node{Name: "node-1", Address: "10.0.0.6"}); err != ErrDuplicateName {
		t.Fatalf("duplicate error = %v, want ErrDuplicateName", err)
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "node-1" || len(got.Tags) != 2 {
		t.Fatalf("unexpected node %+v", got)
	}

	got.Message = "all good"
	got.Status = StatusConnected
	if err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ = store.Get(ctx, created.ID)
	if got.Status != StatusConnected || got.Message != "all good" {
		t.Fatalf("update did not stick: %+v", got)
	}

	if err := store.SetStatus(ctx, created.ID, StatusError, "unreachable"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	got, _ = store.Get(ctx, created.ID)
	if got.Status != StatusError {
		t.Fatalf("status = %q", got.Status)
	}

	if err := store.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, created.ID); err != ErrNotFound {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestHealthSamples(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	node, err := store.Create(ctx, Node{Name: "node-1", Address: "10.0.0.5"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	sample := HealthSample{
		NodeID:    node.ID,
		DiskTotal: 1000,
		DiskUsed:  250,
		MemTotal:  2000,
		MemUsed:   500,
		Load1:     0.5,
	}
	if err := store.RecordHealth(ctx, sample); err != nil {
		t.Fatalf("RecordHealth: %v", err)
	}

	samples, err := store.Health(ctx, node.ID, 10)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("samples = %d", len(samples))
	}
	if got := samples[0].DiskUsage(); got != 25 {
		t.Fatalf("disk usage = %v, want 25", got)
	}
	if got := samples[0].MemoryUsage(); got != 25 {
		t.Fatalf("memory usage = %v, want 25", got)
	}
}

func TestUsageIsAccumulated(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	node, _ := store.Create(ctx, Node{Name: "node-1", Address: "10.0.0.5"})
	if err := store.RecordUsage(ctx, Usage{NodeID: node.ID, DateTime: 100, Up: 10, Down: 20}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	if err := store.RecordUsage(ctx, Usage{NodeID: node.ID, DateTime: 100, Up: 30, Down: 40}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}

	usage, err := store.Usage(ctx, node.ID, 0)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if len(usage) != 1 || usage[0].Up != 30 || usage[0].Down != 40 {
		t.Fatalf("usage = %+v", usage)
	}
}
