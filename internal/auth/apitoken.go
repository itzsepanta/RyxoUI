package auth

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

// APIToken is a long lived credential for programmatic access.
type APIToken struct {
	ID         string   `json:"id"`
	OperatorID string   `json:"operatorId"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	RateLimit  int      `json:"rateLimit"`
	LastUsedAt int64    `json:"lastUsedAt"`
	LastUsedIP string   `json:"lastUsedIp"`
	ExpiresAt  int64    `json:"expiresAt"`
	CreatedAt  int64    `json:"createdAt"`
	Plaintext  string   `json:"plaintext,omitempty"`
}

// APITokenStore reads and writes API tokens.
type APITokenStore struct {
	db db.Querier
}

// NewAPITokenStore returns an API token store.
func NewAPITokenStore(q db.Querier) *APITokenStore { return &APITokenStore{db: q} }

// Create stores a new API token. The plaintext is returned once and never
// stored.
func (s *APITokenStore) Create(ctx context.Context, operatorID, name string, scopes []string, expiresAt int64, rateLimit int) (APIToken, error) {
	token, hash, err := GenerateAPIToken()
	if err != nil {
		return APIToken{}, err
	}
	now := db.Now()
	record := APIToken{
		ID:         id.New(),
		OperatorID: operatorID,
		Name:       name,
		Scopes:     scopes,
		RateLimit:  rateLimit,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, operator_id, name, token_hash, scopes, rate_limit, last_used_at, last_used_ip, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.OperatorID, record.Name, hash, strings.Join(scopes, ","), record.RateLimit,
		0, "", record.ExpiresAt, now)
	if err != nil {
		return APIToken{}, fmt.Errorf("auth: insert api token: %w", err)
	}
	record.Plaintext = token
	return record, nil
}

// Resolve returns the token record behind a plaintext token.
func (s *APITokenStore) Resolve(ctx context.Context, plaintext string) (APIToken, error) {
	if !strings.HasPrefix(plaintext, APITokenPrefix) {
		return APIToken{}, ErrInvalidToken
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, operator_id, name, scopes, rate_limit, last_used_at, last_used_ip, expires_at, created_at
		 FROM api_tokens WHERE token_hash = ?`, HashToken(plaintext))

	var (
		token  APIToken
		scopes string
	)
	err := row.Scan(&token.ID, &token.OperatorID, &token.Name, &scopes, &token.RateLimit,
		&token.LastUsedAt, &token.LastUsedIP, &token.ExpiresAt, &token.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return APIToken{}, ErrInvalidToken
	}
	if err != nil {
		return APIToken{}, fmt.Errorf("auth: resolve api token: %w", err)
	}
	token.Scopes = splitList(scopes)

	if token.ExpiresAt > 0 && token.ExpiresAt < time.Now().UTC().Unix() {
		return APIToken{}, ErrInvalidToken
	}
	return token, nil
}

// List returns the tokens of an operator.
func (s *APITokenStore) List(ctx context.Context, operatorID string) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, operator_id, name, scopes, rate_limit, last_used_at, last_used_ip, expires_at, created_at
		 FROM api_tokens WHERE operator_id = ? ORDER BY created_at DESC`, operatorID)
	if err != nil {
		return nil, fmt.Errorf("auth: list api tokens: %w", err)
	}
	defer rows.Close()

	var out []APIToken
	for rows.Next() {
		var (
			token  APIToken
			scopes string
		)
		if err := rows.Scan(&token.ID, &token.OperatorID, &token.Name, &scopes, &token.RateLimit,
			&token.LastUsedAt, &token.LastUsedIP, &token.ExpiresAt, &token.CreatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan api token: %w", err)
		}
		token.Scopes = splitList(scopes)
		out = append(out, token)
	}
	return out, rows.Err()
}

// MarkUsed records the usage of a token.
func (s *APITokenStore) MarkUsed(ctx context.Context, tokenID, ip string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE api_tokens SET last_used_at = ?, last_used_ip = ? WHERE id = ?", db.Now(), ip, tokenID)
	if err != nil {
		return fmt.Errorf("auth: mark api token used: %w", err)
	}
	return nil
}

// Delete revokes a token.
func (s *APITokenStore) Delete(ctx context.Context, tokenID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM api_tokens WHERE id = ?", tokenID)
	if err != nil {
		return fmt.Errorf("auth: delete api token: %w", err)
	}
	return nil
}

// ErrInvalidToken is returned for an unknown or expired API token.
var ErrInvalidToken = errors.New("auth: invalid api token")

func splitList(raw string) []string {
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
