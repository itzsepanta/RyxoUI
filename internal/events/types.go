// Package events implements the panel event catalogue and the delivery
// pipeline.
//
// Events are published to an outbox table inside the transaction that caused
// them. A dispatcher later hands each event to the subscribers and records
// every attempt, so a failing subscriber never loses an event.
package events

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Field describes one member of an event payload.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Type describes a kind of event.
type Type struct {
	Name        string  `json:"name"`
	Family      string  `json:"family"`
	Scope       string  `json:"scope"`
	Description string  `json:"description"`
	Payload     []Field `json:"payload"`
}

// Event families.
const (
	FamilyPanel    = "panel"
	FamilyOperator = "operator"
	FamilyRole     = "role"
	FamilySettings = "settings"
	FamilySession  = "session"
)

// Registry holds the known event types.
type Registry struct {
	mu    sync.RWMutex
	types map[string]Type
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{types: map[string]Type{}}
}

// Register adds an event type. It fails on duplicates and on unknown scopes
// being impossible to detect here, so only the format is validated.
func (r *Registry) Register(event Type) error {
	if strings.TrimSpace(event.Name) == "" {
		return fmt.Errorf("events: event name must not be empty")
	}
	if strings.TrimSpace(event.Family) == "" {
		return fmt.Errorf("events: event %s needs a family", event.Name)
	}
	for _, field := range event.Payload {
		if strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Type) == "" {
			return fmt.Errorf("events: event %s has an incomplete payload field", event.Name)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.types[event.Name]; exists {
		return fmt.Errorf("events: event %s is already registered", event.Name)
	}
	r.types[event.Name] = event
	return nil
}

// MustRegister adds an event type and panics on error. It is used for the
// built-in catalogue at start up.
func (r *Registry) MustRegister(event Type) {
	if err := r.Register(event); err != nil {
		panic(err)
	}
}

// Get returns an event type.
func (r *Registry) Get(name string) (Type, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	event, ok := r.types[name]
	return event, ok
}

// List returns every event type sorted by name.
func (r *Registry) List() []Type {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Type, 0, len(r.types))
	for _, event := range r.types {
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// EncodePayload renders a payload map as JSON.
func EncodePayload(payload map[string]any) (string, error) {
	if payload == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("events: encode payload: %w", err)
	}
	return string(raw), nil
}

// DecodePayload parses a payload into a map.
func DecodePayload(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("events: decode payload: %w", err)
	}
	return payload, nil
}

// DefaultRegistry returns the catalogue of events implemented so far.
func DefaultRegistry() *Registry {
	registry := NewRegistry()
	for _, event := range []Type{
		{
			Name: "operator.created", Family: FamilyOperator, Scope: "operators:read",
			Description: "An operator account was created.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
		{
			Name: "operator.updated", Family: FamilyOperator, Scope: "operators:read",
			Description: "An operator account was changed.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
		{
			Name: "operator.deleted", Family: FamilyOperator, Scope: "operators:read",
			Description: "An operator account was removed.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
		{
			Name: "operator.login", Family: FamilySession, Scope: "operators:read",
			Description: "An operator signed in successfully.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
		{
			Name: "operator.login_failed", Family: FamilySession, Scope: "operators:read",
			Description: "A sign in attempt was rejected.",
			Payload:     []Field{{Name: "username", Type: "string"}},
		},
		{
			Name: "role.created", Family: FamilyRole, Scope: "roles:read",
			Description: "A role was created.",
			Payload:     []Field{{Name: "roleId", Type: "string"}, {Name: "name", Type: "string"}},
		},
		{
			Name: "role.updated", Family: FamilyRole, Scope: "roles:read",
			Description: "A role was changed.",
			Payload:     []Field{{Name: "roleId", Type: "string"}, {Name: "name", Type: "string"}},
		},
		{
			Name: "role.deleted", Family: FamilyRole, Scope: "roles:read",
			Description: "A role was removed.",
			Payload:     []Field{{Name: "roleId", Type: "string"}, {Name: "name", Type: "string"}},
		},
		{
			Name: "settings.updated", Family: FamilySettings, Scope: "settings:read",
			Description: "A panel setting was changed.",
			Payload:     []Field{{Name: "key", Type: "string"}},
		},
		{
			Name: "operator.2fa_enabled", Family: FamilyOperator, Scope: "operators:read",
			Description: "An operator turned a second factor on.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
		{
			Name: "operator.2fa_disabled", Family: FamilyOperator, Scope: "operators:read",
			Description: "An operator turned a second factor off.",
			Payload:     []Field{{Name: "operatorId", Type: "string"}, {Name: "username", Type: "string"}},
		},
	} {
		registry.MustRegister(event)
	}
	return registry
}
