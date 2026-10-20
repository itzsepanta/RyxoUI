package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func openMemory(t *testing.T) *DB {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(1)
	t.Cleanup(func() { handle.Close() })
	return New(handle, SQLite)
}

func TestMigrateAppliesOnce(t *testing.T) {
	db := openMemory(t)
	migrator := NewMigrator(db)

	ran, err := migrator.Migrate()
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(ran) == 0 {
		t.Fatal("expected at least one migration to be applied")
	}
	if ran[0] != "0001" {
		t.Fatalf("first applied migration = %q, want 0001", ran[0])
	}

	again, err := migrator.Migrate()
	if err != nil {
		t.Fatalf("Migrate (second): %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second run applied %v, want none", again)
	}
}

func TestMigrateCreatesTables(t *testing.T) {
	db := openMemory(t)
	if _, err := NewMigrator(db).Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for _, table := range []string{
		"roles", "operators", "operator_sessions", "api_tokens",
		"login_attempts", "settings", "audit_log",
		"event_types", "event_outbox", "event_deliveries", "schema_migrations",
	} {
		var name string
		err := db.QueryRowContext(context.Background(),
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
}

func TestMigrateDetectsDrift(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS a (id TEXT);")},
	}
	if _, err := NewMigratorFS(db, fsys, "migrations").Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	fsys2 := fstest.MapFS{
		"migrations/0001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE IF NOT EXISTS a (id TEXT, x TEXT);")},
	}
	_, err := NewMigratorFS(db, fsys2, "migrations").Migrate()
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}
}

func TestRebindPostgres(t *testing.T) {
	got := Rebind(Postgres, "SELECT * FROM t WHERE a = ? AND b = ?")
	want := "SELECT * FROM t WHERE a = $1 AND b = $2"
	if got != want {
		t.Fatalf("Rebind = %q, want %q", got, want)
	}
	if got := Rebind(SQLite, "SELECT ?"); got != "SELECT ?" {
		t.Fatalf("Rebind(sqlite) = %q, want unchanged", got)
	}
}

func TestInTxRollsBack(t *testing.T) {
	db := openMemory(t)
	if _, err := NewMigrator(db).Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	err := db.InTx(context.Background(), func(tx *Tx) error {
		if _, err := tx.ExecContext(context.Background(),
			"INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)", "a", "b", 1); err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM settings").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after rollback", count)
	}
}

func TestOpenFileDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel.db")

	db, err := Open(context.Background(), Options{
		Driver:  "sqlite",
		DSN:     path,
		Dialect: SQLite,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := NewMigrator(db).Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
}
