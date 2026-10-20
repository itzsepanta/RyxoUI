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

// Operator is a panel account.
type Operator struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	RoleID       string `json:"roleId"`
	IsOwner      bool   `json:"isOwner"`
	Status       string `json:"status"`
	LastLoginAt  int64  `json:"lastLoginAt"`
	LastLoginIP  string `json:"lastLoginIp"`
	TOTPSecret   string `json:"-"`
	TOTPEnabled  bool   `json:"totpEnabled"`
	TOTPLastUsed int64  `json:"-"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// Operator status values.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// ErrNotFound is returned when an operator does not exist.
var ErrNotFound = errors.New("auth: operator not found")

// ErrDuplicateUsername is returned when a username is already taken.
var ErrDuplicateUsername = errors.New("auth: username already exists")

// OperatorStore reads and writes panel accounts.
type OperatorStore struct {
	db db.Querier
}

// NewOperatorStore returns an operator store.
func NewOperatorStore(q db.Querier) *OperatorStore { return &OperatorStore{db: q} }

// Create stores a new operator.
func (s *OperatorStore) Create(ctx context.Context, op Operator) (Operator, error) {
	username := strings.TrimSpace(op.Username)
	if username == "" {
		return Operator{}, errors.New("auth: username must not be empty")
	}
	if op.PasswordHash == "" {
		return Operator{}, errors.New("auth: password hash must not be empty")
	}
	if op.Status == "" {
		op.Status = StatusActive
	}

	now := db.Now()
	op.ID = id.New()
	op.Username = username
	op.CreatedAt = now
	op.UpdatedAt = now

	owner := 0
	if op.IsOwner {
		owner = 1
	}
	totpEnabledAt := int64(0)
	if op.TOTPEnabled {
		totpEnabledAt = now
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO operators (id, username, password_hash, role_id, is_owner, status,
		 last_login_at, last_login_ip, totp_secret, totp_enabled_at, totp_last_used_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.ID, op.Username, op.PasswordHash, op.RoleID, owner, op.Status,
		op.LastLoginAt, op.LastLoginIP, op.TOTPSecret, totpEnabledAt, op.TOTPLastUsed, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return Operator{}, ErrDuplicateUsername
		}
		return Operator{}, fmt.Errorf("auth: insert operator %s: %w", op.Username, err)
	}
	return op, nil
}

// Get returns an operator by identifier.
func (s *OperatorStore) Get(ctx context.Context, operatorID string) (Operator, error) {
	row := s.db.QueryRowContext(ctx, operatorColumns+" WHERE id = ?", operatorID)
	op, err := scanOperator(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Operator{}, ErrNotFound
	}
	return op, err
}

// GetByUsername returns an operator by username.
func (s *OperatorStore) GetByUsername(ctx context.Context, username string) (Operator, error) {
	row := s.db.QueryRowContext(ctx, operatorColumns+" WHERE username = ?", strings.TrimSpace(username))
	op, err := scanOperator(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Operator{}, ErrNotFound
	}
	return op, err
}

// List returns every operator ordered by username.
func (s *OperatorStore) List(ctx context.Context) ([]Operator, error) {
	rows, err := s.db.QueryContext(ctx, operatorColumns+" ORDER BY username")
	if err != nil {
		return nil, fmt.Errorf("auth: list operators: %w", err)
	}
	defer rows.Close()

	var out []Operator
	for rows.Next() {
		op, err := scanOperator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: iterate operators: %w", err)
	}
	return out, nil
}

// Count returns the number of operators.
func (s *OperatorStore) Count(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operators").Scan(&count); err != nil {
		return 0, fmt.Errorf("auth: count operators: %w", err)
	}
	return count, nil
}

// Update writes the mutable fields of an operator.
func (s *OperatorStore) Update(ctx context.Context, op Operator) error {
	owner := 0
	if op.IsOwner {
		owner = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE operators SET username = ?, role_id = ?, is_owner = ?, status = ?, updated_at = ? WHERE id = ?`,
		op.Username, op.RoleID, owner, op.Status, db.Now(), op.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateUsername
		}
		return fmt.Errorf("auth: update operator %s: %w", op.Username, err)
	}
	return nil
}

// SetPassword replaces the stored password hash.
func (s *OperatorStore) SetPassword(ctx context.Context, operatorID, hash string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE operators SET password_hash = ?, updated_at = ? WHERE id = ?", hash, db.Now(), operatorID)
	if err != nil {
		return fmt.Errorf("auth: set password: %w", err)
	}
	return nil
}

// SetTOTP stores the TOTP secret and enables or disables the factor.
func (s *OperatorStore) SetTOTP(ctx context.Context, operatorID, secret string, enabled bool) error {
	enabledAt := int64(0)
	if enabled {
		enabledAt = db.Now()
	}
	_, err := s.db.ExecContext(ctx,
		"UPDATE operators SET totp_secret = ?, totp_enabled_at = ?, updated_at = ? WHERE id = ?",
		secret, enabledAt, db.Now(), operatorID)
	if err != nil {
		return fmt.Errorf("auth: set totp: %w", err)
	}
	return nil
}

// MarkLogin records a successful sign in.
func (s *OperatorStore) MarkLogin(ctx context.Context, operatorID, ip string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE operators SET last_login_at = ?, last_login_ip = ?, updated_at = ? WHERE id = ?",
		db.Now(), ip, db.Now(), operatorID)
	if err != nil {
		return fmt.Errorf("auth: mark login: %w", err)
	}
	return nil
}

// Delete removes an operator.
func (s *OperatorStore) Delete(ctx context.Context, operatorID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM operators WHERE id = ?", operatorID)
	if err != nil {
		return fmt.Errorf("auth: delete operator: %w", err)
	}
	return nil
}

// RecordAttempt stores a login attempt for auditing and rate limiting.
func (s *OperatorStore) RecordAttempt(ctx context.Context, username, ip string, successful bool) {
	flag := 0
	if successful {
		flag = 1
	}
	_, _ = s.db.ExecContext(ctx,
		"INSERT INTO login_attempts (id, username, ip, successful, created_at) VALUES (?, ?, ?, ?, ?)",
		id.New(), strings.TrimSpace(username), ip, flag, db.Now())
}

// RecentFailures counts failed sign ins for a username since the given time.
func (s *OperatorStore) RecentFailures(ctx context.Context, username string, since int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM login_attempts WHERE username = ? AND successful = 0 AND created_at >= ?",
		strings.TrimSpace(username), since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("auth: count failures: %w", err)
	}
	return count, nil
}

const operatorColumns = `SELECT id, username, password_hash, role_id, is_owner, status,
	last_login_at, last_login_ip, totp_secret, totp_enabled_at, totp_last_used_at, created_at, updated_at FROM operators`

type scanner interface {
	Scan(dest ...any) error
}

func scanOperator(row scanner) (Operator, error) {
	var (
		op      Operator
		owner   int
		totpAt  int64
		totpUse int64
	)
	err := row.Scan(&op.ID, &op.Username, &op.PasswordHash, &op.RoleID, &owner, &op.Status,
		&op.LastLoginAt, &op.LastLoginIP, &op.TOTPSecret, &totpAt, &totpUse, &op.CreatedAt, &op.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Operator{}, err
		}
		return Operator{}, fmt.Errorf("auth: scan operator: %w", err)
	}
	op.IsOwner = owner != 0
	op.TOTPEnabled = totpAt > 0
	op.TOTPLastUsed = totpUse
	return op, nil
}

// isUniqueViolation reports whether the error is a unique constraint failure.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique violation")
}

// SessionTTL is the lifetime of a session without the remember flag.
const SessionTTL = 12 * time.Hour

// RememberTTL is the lifetime of a session created with the remember flag.
const RememberTTL = 30 * 24 * time.Hour
