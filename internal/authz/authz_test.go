package authz

import (
	"context"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	handle, err := sqlOpen(t)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := NewStore(q)
	if err := store.EnsureBuiltins(context.Background()); err != nil {
		t.Fatalf("ensure builtins: %v", err)
	}
	return store
}

func TestEnsureBuiltinsIsIdempotent(t *testing.T) {
	store := newStore(t)
	if err := store.EnsureBuiltins(context.Background()); err != nil {
		t.Fatalf("EnsureBuiltins: %v", err)
	}
	roles, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(roles) != 3 {
		t.Fatalf("roles = %d, want 3", len(roles))
	}
}

func TestOwnerAllowsEverything(t *testing.T) {
	store := newStore(t)
	owner, err := store.GetByName(context.Background(), RoleOwner)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	allowed, err := store.Allows(context.Background(), owner, ScopeOperatorsWrite)
	if err != nil {
		t.Fatalf("Allows: %v", err)
	}
	if !allowed {
		t.Fatal("owner must be allowed to write operators")
	}
}

func TestViewerCannotWriteOperators(t *testing.T) {
	store := newStore(t)
	viewer, err := store.GetByName(context.Background(), RoleViewer)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	allowed, err := store.Allows(context.Background(), viewer, ScopeOperatorsWrite)
	if err != nil {
		t.Fatalf("Allows: %v", err)
	}
	if allowed {
		t.Fatal("viewer must not be allowed to write operators")
	}
	read, err := store.Allows(context.Background(), viewer, ScopeSettingsRead)
	if err != nil {
		t.Fatalf("Allows: %v", err)
	}
	if !read {
		t.Fatal("viewer must be allowed to read settings")
	}
}

func TestCustomRoleInheritsBaseScopes(t *testing.T) {
	store := newStore(t)
	created, err := store.Create(context.Background(), Role{
		Name:        "auditor",
		Base:        RoleViewer,
		Description: "reads audit only",
		Scopes:      []string{string(ScopeAuditRead)},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(created.Scopes) != 1 {
		t.Fatalf("scopes = %v, want one", created.Scopes)
	}

	effective, err := store.Effective(context.Background(), created)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if !Matches(effective, ScopeSettingsRead) {
		t.Fatalf("effective scopes %v missing inherited settings:read", effective)
	}
	if !Matches(effective, ScopeAuditRead) {
		t.Fatalf("effective scopes %v missing audit:read", effective)
	}
}

func TestCreateRejectsUnknownScope(t *testing.T) {
	store := newStore(t)
	_, err := store.Create(context.Background(), Role{Name: "bad", Scopes: []string{"users:write"}})
	if err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestBuiltinRoleCannotBeDeleted(t *testing.T) {
	store := newStore(t)
	owner, err := store.GetByName(context.Background(), RoleOwner)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if err := store.Delete(context.Background(), owner.ID); err == nil {
		t.Fatal("expected error deleting a builtin role")
	}
}

func TestMatchesWildcardGroup(t *testing.T) {
	if !Matches([]string{"users:*"}, Scope("users:read")) {
		t.Fatal("users:* must cover users:read")
	}
	if Matches([]string{"users:*"}, Scope("settings:read")) {
		t.Fatal("users:* must not cover settings:read")
	}
	if !Matches([]string{"*"}, Scope("anything:at:all")) {
		t.Fatal("* must cover everything")
	}
}
