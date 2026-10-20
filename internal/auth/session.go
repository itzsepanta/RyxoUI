package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	"github.com/ryxoui/ryxoui/internal/id"
)

// Session is an authenticated browser session.
type Session struct {
	ID         string `json:"id"`
	OperatorID string `json:"operatorId"`
	TokenHash  string `json:"-"`
	IP         string `json:"ip"`
	UserAgent  string `json:"userAgent"`
	Remember   bool   `json:"remember"`
	CreatedAt  int64  `json:"createdAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
	ExpiresAt  int64  `json:"expiresAt"`
}

// SessionService issues and validates sessions.
type SessionService struct {
	db db.Querier
}

// NewSessionService returns a session service.
func NewSessionService(q db.Querier) *SessionService { return &SessionService{db: q} }

// Issue creates a session and returns the plaintext token. Only the hash of
// the token is stored.
func (s *SessionService) Issue(ctx context.Context, operatorID, ip, userAgent string, remember bool) (string, Session, error) {
	token, err := RandomToken(32)
	if err != nil {
		return "", Session{}, err
	}

	ttl := SessionTTL
	if remember {
		ttl = RememberTTL
	}
	now := time.Now().UTC()
	session := Session{
		ID:         id.New(),
		OperatorID: operatorID,
		TokenHash:  HashToken(token),
		IP:         ip,
		UserAgent:  userAgent,
		Remember:   remember,
		CreatedAt:  now.Unix(),
		LastSeenAt: now.Unix(),
		ExpiresAt:  now.Add(ttl).Unix(),
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO operator_sessions (id, operator_id, token_hash, ip, user_agent, remember, created_at, last_seen_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.OperatorID, session.TokenHash, session.IP, session.UserAgent,
		boolToInt(session.Remember), session.CreatedAt, session.LastSeenAt, session.ExpiresAt)
	if err != nil {
		return "", Session{}, fmt.Errorf("auth: insert session: %w", err)
	}
	return token, session, nil
}

// Resolve returns the session behind a plaintext token.
func (s *SessionService) Resolve(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrInvalidSession
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, operator_id, token_hash, ip, user_agent, remember, created_at, last_seen_at, expires_at
		 FROM operator_sessions WHERE token_hash = ?`, HashToken(token))

	var (
		session  Session
		remember int
	)
	err := row.Scan(&session.ID, &session.OperatorID, &session.TokenHash, &session.IP, &session.UserAgent,
		&remember, &session.CreatedAt, &session.LastSeenAt, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrInvalidSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("auth: resolve session: %w", err)
	}
	session.Remember = remember != 0

	if session.ExpiresAt > 0 && session.ExpiresAt < time.Now().UTC().Unix() {
		_ = s.Revoke(ctx, session.ID)
		return Session{}, ErrInvalidSession
	}
	return session, nil
}

// Touch records activity on a session.
func (s *SessionService) Touch(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE operator_sessions SET last_seen_at = ? WHERE id = ?", time.Now().UTC().Unix(), sessionID)
	if err != nil {
		return fmt.Errorf("auth: touch session: %w", err)
	}
	return nil
}

// Revoke removes a single session.
func (s *SessionService) Revoke(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM operator_sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("auth: revoke session: %w", err)
	}
	return nil
}

// RevokeAll removes every session of an operator.
func (s *SessionService) RevokeAll(ctx context.Context, operatorID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM operator_sessions WHERE operator_id = ?", operatorID)
	if err != nil {
		return fmt.Errorf("auth: revoke sessions: %w", err)
	}
	return nil
}

// List returns the sessions of an operator, newest first.
func (s *SessionService) List(ctx context.Context, operatorID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, operator_id, token_hash, ip, user_agent, remember, created_at, last_seen_at, expires_at
		 FROM operator_sessions WHERE operator_id = ? ORDER BY created_at DESC`, operatorID)
	if err != nil {
		return nil, fmt.Errorf("auth: list sessions: %w", err)
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var (
			session  Session
			remember int
		)
		if err := rows.Scan(&session.ID, &session.OperatorID, &session.TokenHash, &session.IP, &session.UserAgent,
			&remember, &session.CreatedAt, &session.LastSeenAt, &session.ExpiresAt); err != nil {
			return nil, fmt.Errorf("auth: scan session: %w", err)
		}
		session.Remember = remember != 0
		out = append(out, session)
	}
	return out, rows.Err()
}

// ErrInvalidSession is returned for an unknown or expired session.
var ErrInvalidSession = errors.New("auth: invalid session")

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
