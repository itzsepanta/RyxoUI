// Package authz implements the permission model of the panel.
//
// Permissions are named scopes such as "users:read". Roles are named sets of
// scopes. Every handler declares the scope it needs, and the check is
// fail-closed: a request without a matching scope is rejected.
package authz

import (
	"fmt"
	"sort"
	"strings"
)

// Scope is a permission name.
type Scope string

// The scopes understood by the panel. The list grows with every milestone;
// see docs/superpowers/plans/2026-10-20-ryxoui-panel-capability-todo.md.
const (
	ScopeOperatorsRead  Scope = "operators:read"
	ScopeOperatorsWrite Scope = "operators:write"
	ScopeRolesRead      Scope = "roles:read"
	ScopeRolesWrite     Scope = "roles:write"
	ScopeSettingsRead   Scope = "settings:read"
	ScopeSettingsWrite  Scope = "settings:write"
	ScopeAuditRead      Scope = "audit:read"
	ScopeTokensRead     Scope = "tokens:read"
	ScopeTokensWrite    Scope = "tokens:write"
)

// ScopeInfo describes a scope for the user interface.
type ScopeInfo struct {
	Name        Scope  `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

// scopeRegistry is the single source of truth for known scopes.
var scopeRegistry = map[Scope]ScopeInfo{
	ScopeOperatorsRead:  {"operators:read", "View panel operators and their roles.", "operators"},
	ScopeOperatorsWrite: {"operators:write", "Create, edit and remove panel operators.", "operators"},
	ScopeRolesRead:      {"roles:read", "View roles and the scope catalogue.", "access"},
	ScopeRolesWrite:     {"roles:write", "Create, edit and remove roles.", "access"},
	ScopeSettingsRead:   {"settings:read", "View panel settings.", "settings"},
	ScopeSettingsWrite:  {"settings:write", "Change panel settings.", "settings"},
	ScopeAuditRead:      {"audit:read", "Read the audit log.", "security"},
	ScopeTokensRead:     {"tokens:read", "View API tokens.", "security"},
	ScopeTokensWrite:    {"tokens:write", "Create and revoke API tokens.", "security"},
}

// Scopes returns every known scope sorted by name.
func Scopes() []ScopeInfo {
	out := make([]ScopeInfo, 0, len(scopeRegistry))
	for _, info := range scopeRegistry {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Known reports whether scope exists in the registry.
func Known(scope Scope) bool {
	_, ok := scopeRegistry[scope]
	return ok
}

// ValidateScopes checks that every scope exists and normalises the list by
// removing duplicates and empty entries.
func ValidateScopes(scopes []string) ([]string, error) {
	seen := make(map[string]bool, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, raw := range scopes {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if s == "*" {
			seen[s] = true
			out = append(out, s)
			continue
		}
		if !Known(Scope(s)) {
			return nil, fmt.Errorf("authz: unknown scope %q", s)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// Matches reports whether granted covers wanted.
//
// Matching rules:
//   - "*" covers everything;
//   - an exact name covers itself;
//   - a trailing ":*" group covers every scope in that group.
func Matches(granted []string, wanted Scope) bool {
	for _, g := range granted {
		switch {
		case g == "*":
			return true
		case g == string(wanted):
			return true
		case strings.HasSuffix(g, ":*"):
			group := strings.TrimSuffix(g, ":*")
			if strings.HasPrefix(string(wanted), group+":") {
				return true
			}
		}
	}
	return false
}
