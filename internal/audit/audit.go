// Package audit records every change an operator makes to the panel.
//
// Entries are written inside the same transaction as the change they
// describe, so the log can never disagree with the data.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// Entry is one audited change.
type Entry struct {
	ID        string `json:"id"`
	ActorID   string `json:"actorId"`
	ActorName string `json:"actorName"`
	Action    string `json:"action"`
	Entity    string `json:"entity"`
	EntityID  string `json:"entityId"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	RequestID string `json:"requestId"`
	CreatedAt int64  `json:"createdAt"`
}

// Store reads and writes audit entries.
type Store struct {
	db db.Querier
}

// NewStore returns an audit store.
func NewStore(q db.Querier) *Store { return &Store{db: q} }

// Record appends an entry. The caller supplies the transaction so the entry
// is committed together with the change.
func (s *Store) Record(ctx context.Context, q db.Querier, entry Entry) error {
	before, err := marshal(entry.Before)
	if err != nil {
		return fmt.Errorf("audit: encode before: %w", err)
	}
	after, err := marshal(entry.After)
	if err != nil {
		return fmt.Errorf("audit: encode after: %w", err)
	}
	if entry.ID == "" {
		entry.ID = id.New()
	}
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().UTC().Unix()
	}

	_, err = q.ExecContext(ctx,
		`INSERT INTO audit_log (id, actor_id, actor_name, action, entity, entity_id, before_json, after_json, ip, user_agent, request_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ActorID, entry.ActorName, entry.Action, entry.Entity, entry.EntityID,
		before, after, entry.IP, entry.UserAgent, entry.RequestID, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("audit: insert entry: %w", err)
	}
	return nil
}

// Filter narrows an audit query.
type Filter struct {
	ActorID  string
	Entity   string
	EntityID string
	Action   string
	Since    int64
	Until    int64
	Limit    int
	Offset   int
}

// List returns audit entries matching the filter, newest first.
func (s *Store) List(ctx context.Context, filter Filter) ([]Entry, int64, error) {
	where := []string{"1 = 1"}
	args := []any{}
	add := func(clause string, value any) {
		where = append(where, clause)
		args = append(args, value)
	}
	if filter.ActorID != "" {
		add("actor_id = ?", filter.ActorID)
	}
	if filter.Entity != "" {
		add("entity = ?", filter.Entity)
	}
	if filter.EntityID != "" {
		add("entity_id = ?", filter.EntityID)
	}
	if filter.Action != "" {
		add("action = ?", filter.Action)
	}
	if filter.Since > 0 {
		add("created_at >= ?", filter.Since)
	}
	if filter.Until > 0 {
		add("created_at <= ?", filter.Until)
	}

	clause := strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: count: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, actor_id, actor_name, action, entity, entity_id, before_json, after_json, ip, user_agent, request_id, created_at
		FROM audit_log WHERE ` + clause + ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, append(args, limit, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var (
			entry  Entry
			before string
			after  string
		)
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.ActorName, &entry.Action, &entry.Entity,
			&entry.EntityID, &before, &after, &entry.IP, &entry.UserAgent, &entry.RequestID, &entry.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("audit: scan: %w", err)
		}
		entry.Before = decode(before)
		entry.After = decode(after)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit: iterate: %w", err)
	}
	return out, total, nil
}

// Facets counts the distinct actors, entities and actions in the log.
func (s *Store) Facets(ctx context.Context) (map[string][]string, error) {
	out := map[string][]string{}
	for _, facet := range []string{"actor_name", "entity", "action"} {
		rows, err := s.db.QueryContext(ctx,
			"SELECT DISTINCT "+facet+" FROM audit_log WHERE "+facet+" <> '' ORDER BY "+facet)
		if err != nil {
			return nil, fmt.Errorf("audit: facets %s: %w", facet, err)
		}
		var values []string
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return nil, fmt.Errorf("audit: facets scan: %w", err)
			}
			values = append(values, value)
		}
		rows.Close()
		out[facet] = values
	}
	return out, nil
}

func marshal(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decode(raw string) any {
	if raw == "" {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}
