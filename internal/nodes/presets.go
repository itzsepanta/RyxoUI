package nodes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// The kinds of preset the panel knows.
const (
	// PresetEntrypoint is a ready made listener.
	PresetEntrypoint = "entrypoint"
	// PresetEgress is a ready made outgoing connection.
	PresetEgress = "egress"
)

// ErrDuplicatePreset is returned when a preset name is already taken.
var ErrDuplicatePreset = errors.New("nodes: the preset name is already taken")

// Preset is a ready made configuration of a protocol.
type Preset struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	Type      string         `json:"type"`
	Remark    string         `json:"remark"`
	Config    map[string]any `json:"config"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// PresetStore reads and writes the presets.
type PresetStore struct {
	db db.Querier
}

// NewPresetStore returns a preset store.
func NewPresetStore(q db.Querier) *PresetStore { return &PresetStore{db: q} }

// Presets returns the presets of a kind, or all of them when the kind is
// empty.
func (s *PresetStore) Presets(ctx context.Context, kind string) ([]Preset, error) {
	query := "SELECT id, name, kind, type, remark, config, created_at, updated_at FROM presets"
	args := []any{}
	if kind != "" {
		query += " WHERE kind = ?"
		args = append(args, kind)
	}
	query += " ORDER BY name"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: list the presets: %w", err)
	}
	defer rows.Close()

	out := []Preset{}
	for rows.Next() {
		var (
			item   Preset
			config string
		)
		if err := rows.Scan(&item.ID, &item.Name, &item.Kind, &item.Type, &item.Remark, &config, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan the preset: %w", err)
		}
		item.Config = decodeConfig(config)
		out = append(out, item)
	}
	return out, rows.Err()
}

// Get reads one preset.
func (s *PresetStore) Get(ctx context.Context, presetID string) (Preset, error) {
	var (
		item   Preset
		config string
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, kind, type, remark, config, created_at, updated_at FROM presets WHERE id = ?", presetID).
		Scan(&item.ID, &item.Name, &item.Kind, &item.Type, &item.Remark, &config, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Preset{}, ErrKeyNotFound
	}
	if err != nil {
		return Preset{}, fmt.Errorf("nodes: read the preset: %w", err)
	}
	item.Config = decodeConfig(config)
	return item, nil
}

// Create stores a preset.
func (s *PresetStore) Create(ctx context.Context, item Preset) (Preset, error) {
	if strings.TrimSpace(item.Name) == "" {
		return Preset{}, errors.New("nodes: the name of the preset must not be empty")
	}
	if item.Kind == "" {
		item.Kind = PresetEntrypoint
	}

	encoded, err := encodeConfig(item.Config)
	if err != nil {
		return Preset{}, err
	}

	now := db.Now()
	item.ID = id.New()
	item.CreatedAt = now
	item.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO presets (id, name, kind, type, remark, config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		item.ID, item.Name, item.Kind, item.Type, item.Remark, encoded, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return Preset{}, ErrDuplicatePreset
		}
		return Preset{}, fmt.Errorf("nodes: insert the preset: %w", err)
	}
	return item, nil
}

// Delete removes a preset.
func (s *PresetStore) Delete(ctx context.Context, presetID string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM presets WHERE id = ?", presetID); err != nil {
		return fmt.Errorf("nodes: delete the preset: %w", err)
	}
	return nil
}

// BuiltinPresets are the presets the panel ships with.
//
// They are written into the database the first time the panel starts, so an
// operator can change them like any other preset.
func BuiltinPresets() []Preset {
	return []Preset{
		{
			Name:   "vless-reality-tcp",
			Kind:   PresetEntrypoint,
			Type:   "vless",
			Remark: "VLESS over Reality and TCP, the quiet listener.",
			Config: map[string]any{
				"port":           443,
				"protocol":       "vless",
				"security":       "reality",
				"flow":           "xtls-rprx-vision",
				"streamSettings": map[string]any{"network": "tcp"},
				"realitySettings": map[string]any{
					"show":        false,
					"dest":        "www.cloudflare.com:443",
					"serverNames": []any{"www.microsoft.com", "www.apple.com"},
					"privateKey":  "",
					"shortIds":    []any{""},
				},
				"clients": []any{},
			},
		},
		{
			Name:   "vless-ws-tls",
			Kind:   PresetEntrypoint,
			Type:   "vless",
			Remark: "VLESS over WebSocket and TLS, for a domain with a certificate.",
			Config: map[string]any{
				"port":     443,
				"protocol": "vless",
				"security": "tls",
				"streamSettings": map[string]any{
					"network":  "ws",
					"security": "tls",
					"wsSettings": map[string]any{
						"path": "/ray",
					},
				},
				"clients": []any{},
			},
		},
		{
			Name:   "trojan-tls",
			Kind:   PresetEntrypoint,
			Type:   "trojan",
			Remark: "Trojan over TLS, the plain listener of a domain with a certificate.",
			Config: map[string]any{
				"port":           443,
				"protocol":       "trojan",
				"security":       "tls",
				"streamSettings": map[string]any{"network": "tcp"},
				"clients":        []any{},
			},
		},
		{
			Name:   "shadowsocks-2022",
			Kind:   PresetEntrypoint,
			Type:   "shadowsocks",
			Remark: "Shadowsocks with the method of the year 2022.",
			Config: map[string]any{
				"port":     8388,
				"protocol": "shadowsocks",
				"method":   "2022-blake3-aes-128-gcm",
				"password": "",
				"network":  "tcp",
				"clients":  []any{},
			},
		},
		{
			Name:   "freedom-out",
			Kind:   PresetEgress,
			Type:   "freedom",
			Remark: "The way out to the internet.",
			Config: map[string]any{
				"protocol": "freedom",
			},
		},
		{
			Name:   "blackhole-out",
			Kind:   PresetEgress,
			Type:   "blackhole",
			Remark: "The way out to nowhere, used to block a domain.",
			Config: map[string]any{
				"protocol": "blackhole",
			},
		},
	}
}
