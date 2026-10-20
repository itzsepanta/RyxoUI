// Package nodes owns the registry of the machines the panel drives.
//
// A node is a server that runs a protocol engine. The panel reaches it over
// SSH, installs the engine, renders its configuration and keeps an eye on
// its health and traffic.
package nodes

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

// Node status values.
const (
	StatusConnecting   = "connecting"
	StatusConnected    = "connected"
	StatusDisconnected = "disconnected"
	StatusError        = "error"
	StatusInstalling   = "installing"
)

// ErrNotFound is returned when a node does not exist.
var ErrNotFound = errors.New("nodes: node not found")

// ErrDuplicateName is returned when a node name is already taken.
var ErrDuplicateName = errors.New("nodes: node name is already taken")

// Node is a machine the panel drives.
type Node struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Address          string   `json:"address"`
	Port             int      `json:"port"`
	Status           string   `json:"status"`
	Message          string   `json:"message"`
	Remark           string   `json:"remark"`
	Tags             []string `json:"tags"`
	Arch             string   `json:"arch"`
	OS               string   `json:"os"`
	CoreVersion      string   `json:"coreVersion"`
	NodeVersion      string   `json:"nodeVersion"`
	Multiplier       float64  `json:"multiplier"`
	Enabled          bool     `json:"enabled"`
	LimitBytes       int64    `json:"limitBytes"`
	LimitPeriod      string   `json:"limitPeriod"`
	LimitStartDay    int      `json:"limitStartDay"`
	LimitStartMonth  int      `json:"limitStartMonth"`
	LimitDisabled    bool     `json:"limitDisabled"`
	TemplateID       string   `json:"templateId"`
	SSHHost          string   `json:"sshHost"`
	SSHPort          int      `json:"sshPort"`
	SSHUser          string   `json:"sshUser"`
	SSHKeyID         string   `json:"sshKeyId"`
	HostKey          string   `json:"-"`
	KeyInstalled     bool     `json:"keyInstalled"`
	LastSeenAt       int64    `json:"lastSeenAt"`
	LastStatusChange int64    `json:"lastStatusChange"`
	CreatedAt        int64    `json:"createdAt"`
	UpdatedAt        int64    `json:"updatedAt"`
}

// SSHAddress returns the address used for the SSH connection.
func (n Node) SSHAddress() string {
	return fmt.Sprintf("%s:%d", n.SSHHostName(), n.SSHPort)
}

// SSHHostName returns the host of the node without the port. The address of
// the node is used when no SSH host was given.
func (n Node) SSHHostName() string {
	if n.SSHHost != "" {
		return n.SSHHost
	}
	return n.Address
}

// Store reads and writes nodes.
type Store struct {
	db db.Querier
}

// NewStore returns a node store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// Create stores a new node.
func (s *Store) Create(ctx context.Context, node Node) (Node, error) {
	node.Name = strings.TrimSpace(node.Name)
	if node.Name == "" {
		return Node{}, errors.New("nodes: name must not be empty")
	}
	if strings.TrimSpace(node.Address) == "" {
		return Node{}, errors.New("nodes: address must not be empty")
	}
	if node.Port == 0 {
		node.Port = 62050
	}
	if node.SSHPort == 0 {
		node.SSHPort = 22
	}
	if node.SSHUser == "" {
		node.SSHUser = "root"
	}
	if node.Status == "" {
		node.Status = StatusConnecting
	}
	if node.Multiplier == 0 {
		node.Multiplier = 1
	}

	now := db.Now()
	node.ID = id.New()
	node.CreatedAt = now
	node.UpdatedAt = now
	node.LastStatusChange = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes (id, name, address, port, status, message, remark, tags, arch, os,
		 core_version, node_version, multiplier, enabled, limit_bytes, limit_period, limit_start_day,
		 limit_start_month, limit_disabled, template_id, ssh_host, ssh_port, ssh_user, ssh_key_id,
		 host_key, key_installed, last_seen_at, last_status_change, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Name, node.Address, node.Port, node.Status, node.Message, node.remark(),
		strings.Join(node.Tags, ","), node.Arch, node.OS, node.CoreVersion, node.NodeVersion,
		node.Multiplier, boolToInt(node.Enabled), node.LimitBytes, node.LimitPeriod, node.LimitStartDay,
		node.LimitStartMonth, boolToInt(node.LimitDisabled), node.TemplateID, node.SSHHost, node.SSHPort,
		node.SSHUser, node.SSHKeyID, node.HostKey, boolToInt(node.KeyInstalled), node.LastSeenAt,
		node.LastStatusChange, node.CreatedAt, node.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return Node{}, ErrDuplicateName
		}
		return Node{}, fmt.Errorf("nodes: insert node %s: %w", node.Name, err)
	}
	return node, nil
}

// List returns every node ordered by name.
func (s *Store) List(ctx context.Context) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, nodeColumns+" ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("nodes: list: %w", err)
	}
	defer rows.Close()

	var out []Node
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nodes: iterate: %w", err)
	}
	return out, nil
}

// Get returns a node by identifier.
func (s *Store) Get(ctx context.Context, nodeID string) (Node, error) {
	row := s.db.QueryRowContext(ctx, nodeColumns+" WHERE id = ?", nodeID)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	return node, err
}

// GetByName returns a node by name.
func (s *Store) GetByName(ctx context.Context, name string) (Node, error) {
	row := s.db.QueryRowContext(ctx, nodeColumns+" WHERE name = ?", strings.TrimSpace(name))
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	return node, err
}

// Update writes the mutable fields of a node.
func (s *Store) Update(ctx context.Context, node Node) error {
	current, err := s.Get(ctx, node.ID)
	if err != nil {
		return err
	}
	if node.Name == "" {
		node.Name = current.Name
	}
	if node.Address == "" {
		node.Address = current.Address
	}
	if node.Port == 0 {
		node.Port = current.Port
	}
	if node.SSHPort == 0 {
		node.SSHPort = current.SSHPort
	}
	if node.SSHUser == "" {
		node.SSHUser = current.SSHUser
	}
	if node.Status == "" {
		node.Status = current.Status
	}

	now := db.Now()
	statusChanged := ""
	if node.Status != current.Status {
		statusChanged = fmt.Sprintf("%d", now)
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE nodes SET name = ?, address = ?, port = ?, status = ?, message = ?, remark = ?, tags = ?,
		 arch = ?, os = ?, core_version = ?, node_version = ?, multiplier = ?, enabled = ?, limit_bytes = ?,
		 limit_period = ?, limit_start_day = ?, limit_start_month = ?, limit_disabled = ?, template_id = ?,
		 ssh_host = ?, ssh_port = ?, ssh_user = ?, ssh_key_id = ?, host_key = ?, key_installed = ?,
		 last_seen_at = ?, last_status_change = ?, updated_at = ? WHERE id = ?`,
		node.Name, node.Address, node.Port, node.Status, node.Message, node.Remark,
		strings.Join(node.Tags, ","), node.Arch, node.OS, node.CoreVersion, node.NodeVersion,
		node.Multiplier, boolToInt(node.Enabled), node.LimitBytes, node.LimitPeriod, node.LimitStartDay,
		node.LimitStartMonth, boolToInt(node.LimitDisabled), node.TemplateID, node.SSHHost, node.SSHPort,
		node.SSHUser, node.SSHKeyID, node.HostKey, boolToInt(node.KeyInstalled), node.LastSeenAt,
		statusChange(statusChanged, current.LastStatusChange), now, node.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("nodes: update node %s: %w", node.Name, err)
	}
	return nil
}

// SetStatus records a new status of a node.
func (s *Store) SetStatus(ctx context.Context, nodeID, status, message string) error {
	now := db.Now()
	_, err := s.db.ExecContext(ctx,
		"UPDATE nodes SET status = ?, message = ?, last_status_change = ?, updated_at = ? WHERE id = ?",
		status, message, now, now, nodeID)
	if err != nil {
		return fmt.Errorf("nodes: set status: %w", err)
	}
	return nil
}

// Touch records that a node was seen.
func (s *Store) Touch(ctx context.Context, nodeID string) error {
	now := db.Now()
	_, err := s.db.ExecContext(ctx,
		"UPDATE nodes SET last_seen_at = ?, updated_at = ? WHERE id = ?", now, now, nodeID)
	if err != nil {
		return fmt.Errorf("nodes: touch: %w", err)
	}
	return nil
}

// ApplyProbe stores what a probe reported about a node. The host key is only
// replaced when the probe brought one, so a pinned key is never lost.
func (s *Store) ApplyProbe(ctx context.Context, nodeID string, result ProbeResult, status, message string) error {
	now := db.Now()
	hostKey := result.HostKey
	if hostKey == "" {
		current, err := s.Get(ctx, nodeID)
		if err != nil {
			return err
		}
		hostKey = current.HostKey
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET arch = ?, os = ?, core_version = ?, node_version = ?, host_key = ?,
		 status = ?, message = ?, last_seen_at = ?, last_status_change = ?, updated_at = ? WHERE id = ?`,
		result.Arch, result.OS, result.CoreVersion, result.NodeVersion, hostKey,
		status, message, now, now, now, nodeID)
	if err != nil {
		return fmt.Errorf("nodes: apply the probe: %w", err)
	}
	return nil
}

// Delete removes a node.
func (s *Store) Delete(ctx context.Context, nodeID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", nodeID)
	if err != nil {
		return fmt.Errorf("nodes: delete: %w", err)
	}
	return nil
}

// HealthSample is one reading of the resources of a node.
type HealthSample struct {
	ID        string  `json:"id"`
	NodeID    string  `json:"nodeId"`
	DiskTotal int64   `json:"diskTotal"`
	DiskUsed  int64   `json:"diskUsed"`
	MemTotal  int64   `json:"memTotal"`
	MemUsed   int64   `json:"memUsed"`
	Load1     float64 `json:"load1"`
	SampledAt int64   `json:"sampledAt"`
}

// DiskUsage returns the used share of the disk.
func (h HealthSample) DiskUsage() float64 {
	if h.DiskTotal == 0 {
		return 0
	}
	return float64(h.DiskUsed) / float64(h.DiskTotal) * 100
}

// MemoryUsage returns the used share of the memory.
func (h HealthSample) MemoryUsage() float64 {
	if h.MemTotal == 0 {
		return 0
	}
	return float64(h.MemUsed) / float64(h.MemTotal) * 100
}

// RecordHealth stores a health reading.
func (s *Store) RecordHealth(ctx context.Context, sample HealthSample) error {
	if sample.SampledAt == 0 {
		sample.SampledAt = db.Now()
	}
	sample.ID = id.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_health_samples (id, node_id, disk_total, disk_used, mem_total, mem_used, load1, sampled_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ID, sample.NodeID, sample.DiskTotal, sample.DiskUsed, sample.MemTotal, sample.MemUsed,
		sample.Load1, sample.SampledAt)
	if err != nil {
		return fmt.Errorf("nodes: record health: %w", err)
	}
	return nil
}

// Health returns the newest health readings of a node.
func (s *Store) Health(ctx context.Context, nodeID string, limit int) ([]HealthSample, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, node_id, disk_total, disk_used, mem_total, mem_used, load1, sampled_at
		 FROM node_health_samples WHERE node_id = ? ORDER BY sampled_at DESC LIMIT ?`, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("nodes: health: %w", err)
	}
	defer rows.Close()

	var out []HealthSample
	for rows.Next() {
		var sample HealthSample
		if err := rows.Scan(&sample.ID, &sample.NodeID, &sample.DiskTotal, &sample.DiskUsed,
			&sample.MemTotal, &sample.MemUsed, &sample.Load1, &sample.SampledAt); err != nil {
			return nil, fmt.Errorf("nodes: scan health: %w", err)
		}
		out = append(out, sample)
	}
	return out, rows.Err()
}

// Usage is the traffic of a node in one period.
type Usage struct {
	ID       string `json:"id"`
	NodeID   string `json:"nodeId"`
	DateTime int64  `json:"dateTime"`
	Up       int64  `json:"up"`
	Down     int64  `json:"down"`
}

// RecordUsage stores the traffic of a node.
func (s *Store) RecordUsage(ctx context.Context, usage Usage) error {
	usage.ID = id.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_usage (id, node_id, date_time, up, down) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (node_id, date_time) DO UPDATE SET up = excluded.up, down = excluded.down`,
		usage.ID, usage.NodeID, usage.DateTime, usage.Up, usage.Down)
	if err != nil {
		return fmt.Errorf("nodes: record usage: %w", err)
	}
	return nil
}

// Usage returns the traffic of a node.
func (s *Store) Usage(ctx context.Context, nodeID string, since int64) ([]Usage, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, node_id, date_time, up, down FROM node_usage WHERE node_id = ? AND date_time >= ? ORDER BY date_time",
		nodeID, since)
	if err != nil {
		return nil, fmt.Errorf("nodes: usage: %w", err)
	}
	defer rows.Close()

	var out []Usage
	for rows.Next() {
		var usage Usage
		if err := rows.Scan(&usage.ID, &usage.NodeID, &usage.DateTime, &usage.Up, &usage.Down); err != nil {
			return nil, fmt.Errorf("nodes: scan usage: %w", err)
		}
		out = append(out, usage)
	}
	return out, rows.Err()
}

const nodeColumns = `SELECT id, name, address, port, status, message, remark, tags, arch, os,
	core_version, node_version, multiplier, enabled, limit_bytes, limit_period, limit_start_day,
	limit_start_month, limit_disabled, template_id, ssh_host, ssh_port, ssh_user, ssh_key_id,
	host_key, key_installed, last_seen_at, last_status_change, created_at, updated_at FROM nodes`

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(row scanner) (Node, error) {
	var (
		node     Node
		tags     string
		enabled  int
		limitOff int
		keyOK    int
	)
	err := row.Scan(&node.ID, &node.Name, &node.Address, &node.Port, &node.Status, &node.Message,
		&node.Remark, &tags, &node.Arch, &node.OS, &node.CoreVersion, &node.NodeVersion,
		&node.Multiplier, &enabled, &node.LimitBytes, &node.LimitPeriod, &node.LimitStartDay,
		&node.LimitStartMonth, &limitOff, &node.TemplateID, &node.SSHHost, &node.SSHPort,
		&node.SSHUser, &node.SSHKeyID, &node.HostKey, &keyOK, &node.LastSeenAt,
		&node.LastStatusChange, &node.CreatedAt, &node.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, err
		}
		return Node{}, fmt.Errorf("nodes: scan: %w", err)
	}
	node.Enabled = enabled != 0
	node.LimitDisabled = limitOff != 0
	node.KeyInstalled = keyOK != 0
	node.Tags = splitList(tags)
	return node, nil
}

func (n Node) remark() string { return n.Remark }

func statusChange(value string, fallback int64) int64 {
	if value == "" {
		return fallback
	}
	var parsed int64
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func splitList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique violation")
}

// Now returns the current time in the schema format.
func Now() int64 { return time.Now().UTC().Unix() }
