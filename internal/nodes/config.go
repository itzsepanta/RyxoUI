package nodes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// ErrDuplicateTag is returned when a tag is already taken.
var ErrDuplicateTag = errors.New("nodes: the tag is already taken")

// EntryPoint is a listener that a node exposes.
type EntryPoint struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Tag       string         `json:"tag"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	NodeID    string         `json:"nodeId"`
	Enabled   bool           `json:"enabled"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// Egress is an outgoing connection of a node.
type Egress struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Tag       string         `json:"tag"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	NodeID    string         `json:"nodeId"`
	Enabled   bool           `json:"enabled"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// Endpoint is a named destination of a node.
type Endpoint struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	NodeID    string         `json:"nodeId"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// Template is a named set of routes and engines that is applied to nodes.
type Template struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Remark    string         `json:"remark"`
	Route     map[string]any `json:"route"`
	Core      map[string]any `json:"core"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// ConfigStore reads and writes the configuration objects of the panel.
type ConfigStore struct {
	db db.Querier
}

// NewConfigStore returns a configuration store.
func NewConfigStore(q db.Querier) *ConfigStore { return &ConfigStore{db: q} }

// CreateEntryPoint stores a listener.
func (s *ConfigStore) CreateEntryPoint(ctx context.Context, item EntryPoint) (EntryPoint, error) {
	if strings.TrimSpace(item.Tag) == "" {
		return EntryPoint{}, errors.New("nodes: the tag must not be empty")
	}
	if strings.TrimSpace(item.Type) == "" {
		return EntryPoint{}, errors.New("nodes: the type must not be empty")
	}
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return EntryPoint{}, err
	}
	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO entrypoints (id, name, tag, type, config, node_id, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.Tag, item.Type, encoded, item.NodeID, boolToInt(item.Enabled), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return EntryPoint{}, ErrDuplicateTag
		}
		return EntryPoint{}, fmt.Errorf("nodes: insert entry point: %w", err)
	}
	return item, nil
}

// EntryPoints returns the listeners of a node, or all of them when the node
// identifier is empty.
func (s *ConfigStore) EntryPoints(ctx context.Context, nodeID string) ([]EntryPoint, error) {
	query := `SELECT id, name, tag, type, config, node_id, enabled, created_at, updated_at FROM entrypoints`
	args := []any{}
	if nodeID != "" {
		query += " WHERE node_id = ?"
		args = append(args, nodeID)
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: list entry points: %w", err)
	}
	defer rows.Close()

	var out []EntryPoint
	for rows.Next() {
		var (
			item    EntryPoint
			config  string
			enabled int
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Tag, &item.Type, &config, &item.NodeID,
			&enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan entry point: %w", err)
		}
		item.Enabled = enabled != 0
		item.Config = decodeConfig(config)
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateEntryPoint writes a listener.
func (s *ConfigStore) UpdateEntryPoint(ctx context.Context, item EntryPoint) error {
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return err
	}
	now := db.Now()
	_, err = s.db.ExecContext(ctx,
		"UPDATE entrypoints SET name = ?, tag = ?, type = ?, config = ?, node_id = ?, enabled = ?, updated_at = ? WHERE id = ?",
		item.Name, item.Tag, item.Type, encoded, item.NodeID, boolToInt(item.Enabled), now, item.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateTag
		}
		return fmt.Errorf("nodes: update entry point: %w", err)
	}
	return nil
}

// DeleteEntryPoint removes a listener.
func (s *ConfigStore) DeleteEntryPoint(ctx context.Context, entryID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM entrypoints WHERE id = ?", entryID); err != nil {
		return fmt.Errorf("nodes: delete entry point: %w", err)
	}
	return nil
}

// CreateEgress stores an outgoing connection.
func (s *ConfigStore) CreateEgress(ctx context.Context, item Egress) (Egress, error) {
	if strings.TrimSpace(item.Tag) == "" {
		return Egress{}, errors.New("nodes: the tag must not be empty")
	}
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return Egress{}, err
	}
	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO egresses (id, name, tag, type, config, node_id, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.Tag, item.Type, encoded, item.NodeID, boolToInt(item.Enabled), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return Egress{}, ErrDuplicateTag
		}
		return Egress{}, fmt.Errorf("nodes: insert egress: %w", err)
	}
	return item, nil
}

// Egresses returns the outgoing connections of a node.
func (s *ConfigStore) Egresses(ctx context.Context, nodeID string) ([]Egress, error) {
	query := `SELECT id, name, tag, type, config, node_id, enabled, created_at, updated_at FROM egresses`
	args := []any{}
	if nodeID != "" {
		query += " WHERE node_id = ?"
		args = append(args, nodeID)
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: list egresses: %w", err)
	}
	defer rows.Close()

	var out []Egress
	for rows.Next() {
		var (
			item    Egress
			config  string
			enabled int
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Tag, &item.Type, &config, &item.NodeID,
			&enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan egress: %w", err)
		}
		item.Enabled = enabled != 0
		item.Config = decodeConfig(config)
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateEgress writes an outgoing connection.
func (s *ConfigStore) UpdateEgress(ctx context.Context, item Egress) error {
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return err
	}
	now := db.Now()
	_, err = s.db.ExecContext(ctx,
		"UPDATE egresses SET name = ?, tag = ?, type = ?, config = ?, node_id = ?, enabled = ?, updated_at = ? WHERE id = ?",
		item.Name, item.Tag, item.Type, encoded, item.NodeID, boolToInt(item.Enabled), now, item.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateTag
		}
		return fmt.Errorf("nodes: update egress: %w", err)
	}
	return nil
}

// DeleteEgress removes an outgoing connection.
func (s *ConfigStore) DeleteEgress(ctx context.Context, egressID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM egresses WHERE id = ?", egressID); err != nil {
		return fmt.Errorf("nodes: delete egress: %w", err)
	}
	return nil
}

// CreateEndpoint stores a destination.
func (s *ConfigStore) CreateEndpoint(ctx context.Context, item Endpoint) (Endpoint, error) {
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return Endpoint{}, err
	}
	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO endpoints (id, name, node_id, type, config, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, item.NodeID, item.Type, encoded, now, now)
	if err != nil {
		return Endpoint{}, fmt.Errorf("nodes: insert endpoint: %w", err)
	}
	return item, nil
}

// Endpoints returns the destinations of a node.
func (s *ConfigStore) Endpoints(ctx context.Context, nodeID string) ([]Endpoint, error) {
	query := `SELECT id, name, node_id, type, config, created_at, updated_at FROM endpoints`
	args := []any{}
	if nodeID != "" {
		query += " WHERE node_id = ?"
		args = append(args, nodeID)
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: list endpoints: %w", err)
	}
	defer rows.Close()

	var out []Endpoint
	for rows.Next() {
		var (
			item   Endpoint
			config string
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.NodeID, &item.Type, &config,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan endpoint: %w", err)
		}
		item.Config = decodeConfig(config)
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateEndpoint writes a destination.
func (s *ConfigStore) UpdateEndpoint(ctx context.Context, item Endpoint) error {
	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return err
	}
	now := db.Now()
	_, err = s.db.ExecContext(ctx,
		"UPDATE endpoints SET name = ?, node_id = ?, type = ?, config = ?, updated_at = ? WHERE id = ?",
		item.Name, item.NodeID, item.Type, encoded, now, item.ID)
	if err != nil {
		return fmt.Errorf("nodes: update endpoint: %w", err)
	}
	return nil
}

// DeleteEndpoint removes a destination.
func (s *ConfigStore) DeleteEndpoint(ctx context.Context, endpointID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM endpoints WHERE id = ?", endpointID); err != nil {
		return fmt.Errorf("nodes: delete endpoint: %w", err)
	}
	return nil
}

// Template reads one template.
func (s *ConfigStore) Template(ctx context.Context, templateID string) (Template, error) {
	var (
		item  Template
		route string
		core  string
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, remark, route, core, created_at, updated_at FROM templates WHERE id = ?", templateID).
		Scan(&item.ID, &item.Name, &item.Remark, &route, &core, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrKeyNotFound
	}
	if err != nil {
		return Template{}, fmt.Errorf("nodes: read the template: %w", err)
	}
	item.Route = decodeConfig(route)
	item.Core = decodeConfig(core)
	return item, nil
}

// CreateTemplate stores a template.
func (s *ConfigStore) CreateTemplate(ctx context.Context, item Template) (Template, error) {
	if strings.TrimSpace(item.Name) == "" {
		return Template{}, errors.New("nodes: the name must not be empty")
	}
	route, err := encodeConfig(item.Route)
	if err != nil {
		return Template{}, err
	}
	core, err := encodeConfig(item.Core)
	if err != nil {
		return Template{}, err
	}
	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO templates (id, name, remark, route, core, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		item.ID, item.Name, item.Remark, route, core, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return Template{}, ErrDuplicateTag
		}
		return Template{}, fmt.Errorf("nodes: insert template: %w", err)
	}
	return item, nil
}

// Templates returns every template.
func (s *ConfigStore) Templates(ctx context.Context) ([]Template, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, remark, route, core, created_at, updated_at FROM templates ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("nodes: list templates: %w", err)
	}
	defer rows.Close()

	var out []Template
	for rows.Next() {
		var (
			item  Template
			route string
			core  string
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Remark, &route, &core,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan template: %w", err)
		}
		item.Route = decodeConfig(route)
		item.Core = decodeConfig(core)
		out = append(out, item)
	}
	return out, rows.Err()
}

// DeleteTemplate removes a template.
func (s *ConfigStore) DeleteTemplate(ctx context.Context, templateID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM templates WHERE id = ?", templateID); err != nil {
		return fmt.Errorf("nodes: delete template: %w", err)
	}
	return nil
}

func encodeConfig(config map[string]any) (string, error) {
	if config == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("nodes: encode the configuration: %w", err)
	}
	return string(raw), nil
}

func decodeConfig(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return map[string]any{}
	}
	if config == nil {
		return map[string]any{}
	}
	return config
}

// rowExists reports whether a row with the given identifier exists.
func rowExists(q db.Querier, table, id string) (bool, error) {
	var count int
	err := q.QueryRowContext(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", table), id).Scan(&count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return count > 0, nil
}
