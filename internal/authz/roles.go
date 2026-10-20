package authz

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// ErrNotFound is returned when a role does not exist.
var ErrNotFound = errors.New("authz: role not found")

// Role is a named set of scopes.
type Role struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Base           string   `json:"base"`
	Builtin        bool     `json:"builtin"`
	Description    string   `json:"description"`
	Scopes         []string `json:"scopes"`
	UserLimit      int      `json:"userLimit"`
	MaxDeviceLimit int      `json:"maxDeviceLimit"`
	MaxDuration    int      `json:"maxDuration"`
	CreatedAt      int64    `json:"createdAt"`
	UpdatedAt      int64    `json:"updatedAt"`
}

// Builtin role names.
const (
	RoleOwner    = "owner"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// builtinRoles are created by the migrations and cannot be edited.
var builtinRoles = []Role{
	{
		Name:        RoleOwner,
		Base:        RoleOwner,
		Builtin:     true,
		Description: "Unrestricted access to every part of the panel.",
		Scopes:      []string{"*"},
	},
	{
		Name:        RoleOperator,
		Base:        RoleOperator,
		Builtin:     true,
		Description: "Runs the panel day to day without managing operators.",
		Scopes: []string{
			string(ScopeOperatorsRead),
			string(ScopeRolesRead),
			string(ScopeSettingsRead),
			string(ScopeSettingsWrite),
			string(ScopeAuditRead),
			string(ScopeTokensRead),
			string(ScopeTokensWrite),
			string(ScopeNodesRead),
			string(ScopeNodesWrite),
			string(ScopeNodesInstall),
		},
	},
	{
		Name:        RoleViewer,
		Base:        RoleViewer,
		Builtin:     true,
		Description: "Read-only access to every screen.",
		Scopes: []string{
			string(ScopeOperatorsRead),
			string(ScopeRolesRead),
			string(ScopeSettingsRead),
			string(ScopeAuditRead),
			string(ScopeTokensRead),
			string(ScopeNodesRead),
		},
	},
}

// Store reads and writes roles.
type Store struct {
	db db.Querier
}

// NewStore returns a role store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// EnsureBuiltins inserts the builtin roles if they are missing and keeps
// their scopes in sync with the catalogue. It is safe to call on every
// start.
func (s *Store) EnsureBuiltins(ctx context.Context) error {
	now := db.Now()
	for _, role := range builtinRoles {
		var (
			roleID  string
			scopes  string
			builtin int
		)
		err := s.db.QueryRowContext(ctx, "SELECT id, scopes, builtin FROM roles WHERE name = ?", role.Name).
			Scan(&roleID, &scopes, &builtin)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO roles (id, name, base, builtin, description, scopes, user_limit, max_device_limit, max_duration, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id.New(), role.Name, role.Base, 1, role.Description, strings.Join(role.Scopes, ","),
				role.UserLimit, role.MaxDeviceLimit, role.MaxDuration, now, now)
			if err != nil {
				return fmt.Errorf("authz: insert builtin role %s: %w", role.Name, err)
			}
			continue
		case err != nil:
			return fmt.Errorf("authz: read builtin role %s: %w", role.Name, err)
		}

		// The role exists: keep its scopes and description in step with the
		// catalogue so a new release adds its scopes.
		if builtin == 0 {
			continue
		}
		if scopes == strings.Join(role.Scopes, ",") {
			continue
		}
		_, err = s.db.ExecContext(ctx,
			"UPDATE roles SET scopes = ?, description = ?, updated_at = ? WHERE id = ?",
			strings.Join(role.Scopes, ","), role.Description, now, roleID)
		if err != nil {
			return fmt.Errorf("authz: update builtin role %s: %w", role.Name, err)
		}
	}
	return nil
}

// List returns every role ordered by name.
func (s *Store) List(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, base, builtin, description, scopes,
		user_limit, max_device_limit, max_duration, created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("authz: list roles: %w", err)
	}
	defer rows.Close()

	var out []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authz: iterate roles: %w", err)
	}
	return out, nil
}

// Get returns a role by identifier.
func (s *Store) Get(ctx context.Context, roleID string) (Role, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, base, builtin, description, scopes,
		user_limit, max_device_limit, max_duration, created_at, updated_at FROM roles WHERE id = ?`, roleID)
	role, err := scanRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

// GetByName returns a role by name.
func (s *Store) GetByName(ctx context.Context, name string) (Role, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, base, builtin, description, scopes,
		user_limit, max_device_limit, max_duration, created_at, updated_at FROM roles WHERE name = ?`, name)
	role, err := scanRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

// Create stores a new custom role.
func (s *Store) Create(ctx context.Context, role Role) (Role, error) {
	scopes, err := ValidateScopes(role.Scopes)
	if err != nil {
		return Role{}, err
	}
	base := role.Base
	if base == "" {
		base = RoleOperator
	}
	if _, err := s.GetByName(ctx, base); err != nil {
		return Role{}, fmt.Errorf("authz: base role %q: %w", base, err)
	}

	now := db.Now()
	role.ID = id.New()
	role.Builtin = false
	role.Scopes = scopes
	role.CreatedAt = now
	role.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO roles (id, name, base, builtin, description, scopes, user_limit, max_device_limit, max_duration, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		role.ID, role.Name, role.Base, 0, role.Description, strings.Join(role.Scopes, ","),
		role.UserLimit, role.MaxDeviceLimit, role.MaxDuration, now, now)
	if err != nil {
		return Role{}, fmt.Errorf("authz: insert role %s: %w", role.Name, err)
	}
	return role, nil
}

// Update replaces the mutable fields of a custom role.
func (s *Store) Update(ctx context.Context, role Role) (Role, error) {
	current, err := s.Get(ctx, role.ID)
	if err != nil {
		return Role{}, err
	}
	if current.Builtin {
		return Role{}, fmt.Errorf("authz: builtin role %s cannot be edited", current.Name)
	}
	scopes, err := ValidateScopes(role.Scopes)
	if err != nil {
		return Role{}, err
	}
	base := role.Base
	if base == "" {
		base = current.Base
	}
	if _, err := s.GetByName(ctx, base); err != nil {
		return Role{}, fmt.Errorf("authz: base role %q: %w", base, err)
	}

	now := db.Now()
	role.Builtin = false
	role.Scopes = scopes
	role.Base = base
	role.CreatedAt = current.CreatedAt
	role.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`UPDATE roles SET name = ?, base = ?, description = ?, scopes = ?, user_limit = ?, max_device_limit = ?,
		 max_duration = ?, updated_at = ? WHERE id = ?`,
		role.Name, role.Base, role.Description, strings.Join(role.Scopes, ","),
		role.UserLimit, role.MaxDeviceLimit, role.MaxDuration, now, role.ID)
	if err != nil {
		return Role{}, fmt.Errorf("authz: update role %s: %w", role.Name, err)
	}
	return role, nil
}

// Delete removes a custom role.
func (s *Store) Delete(ctx context.Context, roleID string) error {
	current, err := s.Get(ctx, roleID)
	if err != nil {
		return err
	}
	if current.Builtin {
		return fmt.Errorf("authz: builtin role %s cannot be deleted", current.Name)
	}
	var used int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operators WHERE role_id = ?", roleID).Scan(&used); err != nil {
		return fmt.Errorf("authz: count operators: %w", err)
	}
	if used > 0 {
		return fmt.Errorf("authz: role %s is still assigned to %d operator(s)", current.Name, used)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM roles WHERE id = ?", roleID); err != nil {
		return fmt.Errorf("authz: delete role %s: %w", current.Name, err)
	}
	return nil
}

// Effective returns the scopes a role grants, including inherited scopes
// from its base role.
func (s *Store) Effective(ctx context.Context, role Role) ([]string, error) {
	seen := map[string]bool{}
	var out []string

	add := func(scopes []string) {
		for _, scope := range scopes {
			if !seen[scope] {
				seen[scope] = true
				out = append(out, scope)
			}
		}
	}

	add(role.Scopes)
	if role.Base != "" && role.Base != role.Name {
		base, err := s.GetByName(ctx, role.Base)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return out, nil
			}
			return nil, err
		}
		add(base.Scopes)
	}
	return out, nil
}

// Allows reports whether the role grants the wanted scope.
func (s *Store) Allows(ctx context.Context, role Role, wanted Scope) (bool, error) {
	scopes, err := s.Effective(ctx, role)
	if err != nil {
		return false, err
	}
	return Matches(scopes, wanted), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRole(row scanner) (Role, error) {
	var (
		role    Role
		scopes  string
		builtin int
	)
	err := row.Scan(&role.ID, &role.Name, &role.Base, &builtin, &role.Description, &scopes,
		&role.UserLimit, &role.MaxDeviceLimit, &role.MaxDuration, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Role{}, err
		}
		return Role{}, fmt.Errorf("authz: scan role: %w", err)
	}
	role.Builtin = builtin != 0
	role.Scopes = splitScopes(scopes)
	return role, nil
}

func splitScopes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Touch updates the modification timestamp of a role.
func (s *Store) Touch(ctx context.Context, roleID string) error {
	if _, err := s.db.ExecContext(ctx, "UPDATE roles SET updated_at = ? WHERE id = ?", time.Now().Unix(), roleID); err != nil {
		return fmt.Errorf("authz: touch role %s: %w", roleID, err)
	}
	return nil
}
