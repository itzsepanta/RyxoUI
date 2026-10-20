package auth

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(1)
	t.Cleanup(func() { handle.Close() })
	return handle
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("password should verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
	if VerifyPassword("not-a-hash", "correct horse battery staple") {
		t.Fatal("malformed hash must not verify")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestOperatorStoreLifecycle(t *testing.T) {
	handle := testDB(t)
	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewOperatorStore(q)
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	op, err := store.Create(context.Background(), Operator{
		Username: "admin", PasswordHash: hash, RoleID: "role-1", IsOwner: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.Create(context.Background(), Operator{
		Username: "admin", PasswordHash: hash, RoleID: "role-1",
	}); err != ErrDuplicateUsername {
		t.Fatalf("duplicate error = %v, want ErrDuplicateUsername", err)
	}

	got, err := store.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if got.ID != op.ID || !got.IsOwner {
		t.Fatalf("unexpected operator %+v", got)
	}

	if err := store.MarkLogin(context.Background(), op.ID, "127.0.0.1"); err != nil {
		t.Fatalf("MarkLogin: %v", err)
	}
	got, _ = store.Get(context.Background(), op.ID)
	if got.LastLoginAt == 0 || got.LastLoginIP != "127.0.0.1" {
		t.Fatalf("login not recorded: %+v", got)
	}

	if err := store.Delete(context.Background(), op.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(context.Background(), op.ID); err != ErrNotFound {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
}

func TestSessionIssueResolveRevoke(t *testing.T) {
	handle := testDB(t)
	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewSessionService(q)
	token, session, err := service.Issue(context.Background(), "op-1", "10.0.0.1", "agent", true)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" || session.ExpiresAt <= session.CreatedAt {
		t.Fatalf("bad session %+v", session)
	}

	resolved, err := service.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != session.ID || !resolved.Remember {
		t.Fatalf("resolved %+v", resolved)
	}

	if _, err := service.Resolve(context.Background(), "nope"); err != ErrInvalidSession {
		t.Fatalf("Resolve(bad) = %v, want ErrInvalidSession", err)
	}

	if err := service.Revoke(context.Background(), session.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := service.Resolve(context.Background(), token); err != ErrInvalidSession {
		t.Fatalf("Resolve(revoked) = %v, want ErrInvalidSession", err)
	}
}

func TestAPITokenRoundTrip(t *testing.T) {
	handle := testDB(t)
	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewAPITokenStore(q)
	created, err := store.Create(context.Background(), "op-1", "ci", []string{"users:read"}, 0, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Plaintext == "" {
		t.Fatal("plaintext must be returned once")
	}

	resolved, err := store.Resolve(context.Background(), created.Plaintext)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != created.ID || len(resolved.Scopes) != 1 {
		t.Fatalf("resolved %+v", resolved)
	}

	if _, err := store.Resolve(context.Background(), "ryx_nope"); err != ErrInvalidToken {
		t.Fatalf("Resolve(bad) = %v, want ErrInvalidToken", err)
	}
}
