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

// The kinds of warning the panel knows.
const (
	// AlertUnreachable is raised when a node does not answer.
	AlertUnreachable = "unreachable"
	// AlertDisk is raised when the disk of a node is almost full.
	AlertDisk = "disk"
	// AlertMemory is raised when the memory of a node is almost full.
	AlertMemory = "memory"
	// AlertLoad is raised when the load of a node is too high.
	AlertLoad = "load"
)

// ErrAlertNotFound is returned when a warning does not exist.
var ErrAlertNotFound = errors.New("nodes: warning not found")

// The severity of a warning.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Alert is a warning about a node.
type Alert struct {
	ID        string `json:"id"`
	NodeID    string `json:"nodeId"`
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Details   string `json:"details"`
	RaisedAt  int64  `json:"raisedAt"`
	ClearedAt int64  `json:"clearedAt"`
}

// Active reports whether the warning is still standing.
func (a Alert) Active() bool { return a.ClearedAt == 0 }

// AlertStore reads and writes the warnings of the nodes.
type AlertStore struct {
	db db.Querier
}

// NewAlertStore returns a store for the warnings.
func NewAlertStore(q db.Querier) *AlertStore { return &AlertStore{db: q} }

// Alerts returns the warnings of a node. The active ones come first, the
// newest ones before the older ones.
func (s *AlertStore) Alerts(ctx context.Context, nodeID string, activeOnly bool) ([]Alert, error) {
	query := "SELECT id, node_id, kind, severity, message, details, raised_at, cleared_at FROM node_alerts"
	args := []any{}
	if nodeID != "" {
		query += " WHERE node_id = ?"
		args = append(args, nodeID)
	}
	if activeOnly {
		if nodeID == "" {
			query += " AND cleared_at = 0"
		} else {
			query += " AND cleared_at = 0"
		}
	}
	query += " ORDER BY cleared_at ASC, raised_at DESC LIMIT 200"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("nodes: list the warnings: %w", err)
	}
	defer rows.Close()

	out := []Alert{}
	for rows.Next() {
		var alert Alert
		if err := rows.Scan(&alert.ID, &alert.NodeID, &alert.Kind, &alert.Severity, &alert.Message, &alert.Details, &alert.RaisedAt, &alert.ClearedAt); err != nil {
			return nil, fmt.Errorf("nodes: scan the warning: %w", err)
		}
		out = append(out, alert)
	}
	return out, rows.Err()
}

// Raise records a warning about a node.
//
// A warning that stands already is not raised twice, only its text is
// refreshed.
func (s *AlertStore) Raise(ctx context.Context, nodeID, kind, severity, message string) error {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return errors.New("nodes: the kind of the warning must not be empty")
	}
	if severity == "" {
		severity = SeverityWarning
	}

	now := db.Now()

	// A warning that stands already only gets a new text.
	result, err := s.db.ExecContext(ctx,
		"UPDATE node_alerts SET message = ?, severity = ? WHERE node_id = ? AND kind = ? AND cleared_at = 0",
		message, severity, nodeID, kind)
	if err != nil {
		return fmt.Errorf("nodes: refresh the warning: %w", err)
	}
	if affected, err := result.RowsAffected(); err == nil && affected > 0 {
		return nil
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO node_alerts (id, node_id, kind, severity, message, details, raised_at, cleared_at) VALUES (?, ?, ?, ?, ?, '', ?, 0)",
		id.New(), nodeID, kind, severity, message, now)
	if err != nil {
		return fmt.Errorf("nodes: raise the warning: %w", err)
	}
	return nil
}

// Clear ends the warnings of a kind for a node.
func (s *AlertStore) Clear(ctx context.Context, nodeID, kind string) error {
	now := db.Now()
	_, err := s.db.ExecContext(ctx,
		"UPDATE node_alerts SET cleared_at = ? WHERE node_id = ? AND kind = ? AND cleared_at = 0",
		now, nodeID, kind)
	if err != nil {
		return fmt.Errorf("nodes: clear the warning: %w", err)
	}
	return nil
}

// ClearAll ends every warning of a node.
func (s *AlertStore) ClearAll(ctx context.Context, nodeID string) error {
	now := db.Now()
	_, err := s.db.ExecContext(ctx,
		"UPDATE node_alerts SET cleared_at = ? WHERE node_id = ? AND cleared_at = 0",
		now, nodeID)
	if err != nil {
		return fmt.Errorf("nodes: clear the warnings: %w", err)
	}
	return nil
}

// Get reads one warning.
func (s *AlertStore) Get(ctx context.Context, alertID string) (Alert, error) {
	var alert Alert
	err := s.db.QueryRowContext(ctx,
		"SELECT id, node_id, kind, severity, message, details, raised_at, cleared_at FROM node_alerts WHERE id = ?", alertID).
		Scan(&alert.ID, &alert.NodeID, &alert.Kind, &alert.Severity, &alert.Message, &alert.Details, &alert.RaisedAt, &alert.ClearedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Alert{}, ErrAlertNotFound
	}
	if err != nil {
		return Alert{}, fmt.Errorf("nodes: read the warning: %w", err)
	}
	return alert, nil
}
