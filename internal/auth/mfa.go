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

// PendingTTL is how long a second factor challenge stays valid.
const PendingTTL = 5 * time.Minute

// PendingMaxAttempts is how often a challenge may be answered wrongly.
const PendingMaxAttempts = 5

// ErrInvalidPending is returned for an unknown or expired challenge.
var ErrInvalidPending = errors.New("auth: invalid second factor challenge")

// ErrInvalidCode is returned when the second factor does not match.
var ErrInvalidCode = errors.New("auth: invalid second factor")

// ErrMFADisabled is returned when an operator has no second factor.
var ErrMFADisabled = errors.New("auth: no second factor is enabled")

// Pending is a second factor challenge.
type Pending struct {
	ID         string `json:"id"`
	OperatorID string `json:"operatorId"`
	Remember   bool   `json:"remember"`
	Attempts   int    `json:"attempts"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
}

// MFAService drives the second factor: enrollment, challenges and recovery
// codes.
type MFAService struct {
	db        db.Querier
	operators *OperatorStore
}

// NewMFAService returns a second factor service.
func NewMFAService(q db.Querier) *MFAService {
	return &MFAService{db: q, operators: NewOperatorStore(q)}
}

// Begin creates a challenge for an operator and returns its plaintext token.
func (s *MFAService) Begin(ctx context.Context, operatorID string, remember bool) (string, error) {
	token, err := RandomToken(24)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	remembered := 0
	if remember {
		remembered = 1
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO mfa_pending (id, operator_id, token_hash, remember, attempts, created_at, expires_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)`,
		id.New(), operatorID, HashToken(token), remembered, now.Unix(), now.Add(PendingTTL).Unix())
	if err != nil {
		return "", fmt.Errorf("auth: insert pending challenge: %w", err)
	}
	return token, nil
}

// Pending returns the challenge behind a plaintext token.
func (s *MFAService) Pending(ctx context.Context, token string) (Pending, error) {
	if token == "" {
		return Pending{}, ErrInvalidPending
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, operator_id, remember, attempts, created_at, expires_at FROM mfa_pending WHERE token_hash = ?`,
		HashToken(token))

	var (
		pending  Pending
		remember int
	)
	err := row.Scan(&pending.ID, &pending.OperatorID, &remember, &pending.Attempts, &pending.CreatedAt, &pending.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Pending{}, ErrInvalidPending
	}
	if err != nil {
		return Pending{}, fmt.Errorf("auth: read pending challenge: %w", err)
	}
	pending.Remember = remember != 0

	if pending.ExpiresAt < time.Now().UTC().Unix() {
		_ = s.Cancel(ctx, token)
		return Pending{}, ErrInvalidPending
	}
	if pending.Attempts >= PendingMaxAttempts {
		_ = s.Cancel(ctx, token)
		return Pending{}, ErrInvalidPending
	}
	return pending, nil
}

// Fail records a wrong answer and drops the challenge when too many
// attempts were made.
func (s *MFAService) Fail(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE mfa_pending SET attempts = attempts + 1 WHERE token_hash = ?", HashToken(token))
	if err != nil {
		return fmt.Errorf("auth: record failed challenge: %w", err)
	}
	return nil
}

// Cancel removes a challenge.
func (s *MFAService) Cancel(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mfa_pending WHERE token_hash = ?", HashToken(token))
	if err != nil {
		return fmt.Errorf("auth: cancel challenge: %w", err)
	}
	return nil
}

// Complete verifies a second factor and removes the challenge.
//
// The code is either a time based one time password or one of the recovery
// codes. Recovery codes are consumed on use.
func (s *MFAService) Complete(ctx context.Context, token, code string) (Operator, error) {
	pending, err := s.Pending(ctx, token)
	if err != nil {
		return Operator{}, err
	}
	operator, err := s.operators.Get(ctx, pending.OperatorID)
	if err != nil {
		return Operator{}, err
	}
	if !operator.TOTPEnabled || operator.TOTPSecret == "" {
		return Operator{}, ErrMFADisabled
	}

	code = strings.TrimSpace(code)
	switch {
	case len(code) == TOTPDigits:
		if !VerifyTOTP(operator.TOTPSecret, code, time.Now().UTC()) {
			_ = s.Fail(ctx, token)
			return Operator{}, ErrInvalidCode
		}
	default:
		used, err := s.UseRecoveryCode(ctx, operator.ID, code)
		if err != nil {
			return Operator{}, err
		}
		if !used {
			_ = s.Fail(ctx, token)
			return Operator{}, ErrInvalidCode
		}
	}

	if err := s.Cancel(ctx, token); err != nil {
		return Operator{}, err
	}
	return operator, nil
}

// StartEnrollment generates a secret for a new second factor. The factor
// stays disabled until it is confirmed with a code.
func (s *MFAService) StartEnrollment(ctx context.Context, operatorID string) (secret, uri string, err error) {
	operator, err := s.operators.Get(ctx, operatorID)
	if err != nil {
		return "", "", err
	}
	if operator.TOTPEnabled {
		return "", "", errors.New("auth: a second factor is already enabled")
	}

	secret, err = GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	if _, err := s.db.ExecContext(ctx,
		"UPDATE operators SET totp_secret = ?, totp_enabled_at = 0, updated_at = ? WHERE id = ?",
		secret, db.Now(), operatorID); err != nil {
		return "", "", fmt.Errorf("auth: store totp secret: %w", err)
	}
	return secret, TOTPURI("RyxoUI", operator.Username, secret), nil
}

// ConfirmEnrollment checks the first code and switches the factor on. It
// returns the recovery codes, which are shown only once.
func (s *MFAService) ConfirmEnrollment(ctx context.Context, operatorID, code string) ([]string, error) {
	operator, err := s.operators.Get(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	if operator.TOTPSecret == "" {
		return nil, errors.New("auth: no second factor has been started")
	}
	if !VerifyTOTP(operator.TOTPSecret, strings.TrimSpace(code), time.Now().UTC()) {
		return nil, ErrInvalidCode
	}

	codes, err := GenerateRecoveryCodes(8)
	if err != nil {
		return nil, err
	}
	hashed := make([]string, 0, len(codes))
	for _, code := range codes {
		hashed = append(hashed, HashToken(strings.ToUpper(code)))
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE operators SET totp_enabled_at = ?, recovery_codes = ?, updated_at = ? WHERE id = ?",
		db.Now(), strings.Join(hashed, ","), db.Now(), operatorID); err != nil {
		return nil, fmt.Errorf("auth: enable totp: %w", err)
	}
	return codes, nil
}

// Disable turns the second factor off after the password was confirmed.
func (s *MFAService) Disable(ctx context.Context, operatorID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE operators SET totp_secret = '', totp_enabled_at = 0, recovery_codes = '', updated_at = ? WHERE id = ?",
		db.Now(), operatorID)
	if err != nil {
		return fmt.Errorf("auth: disable totp: %w", err)
	}
	return nil
}

// UseRecoveryCode consumes a recovery code and reports whether it was
// valid.
func (s *MFAService) UseRecoveryCode(ctx context.Context, operatorID, code string) (bool, error) {
	operator, err := s.operators.Get(ctx, operatorID)
	if err != nil {
		return false, err
	}
	if operator.RecoveryCodes == "" {
		return false, nil
	}

	wanted := HashToken(strings.ToUpper(strings.TrimSpace(code)))
	kept := make([]string, 0, 8)
	found := false
	for _, stored := range strings.Split(operator.RecoveryCodes, ",") {
		stored = strings.TrimSpace(stored)
		if stored == "" {
			continue
		}
		if !found && stored == wanted {
			found = true
			continue
		}
		kept = append(kept, stored)
	}
	if !found {
		return false, nil
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE operators SET recovery_codes = ?, updated_at = ? WHERE id = ?",
		strings.Join(kept, ","), db.Now(), operatorID); err != nil {
		return false, fmt.Errorf("auth: store recovery codes: %w", err)
	}
	return true, nil
}

// RecoveryCodeCount returns how many recovery codes are left.
func (s *MFAService) RecoveryCodeCount(ctx context.Context, operatorID string) (int, error) {
	operator, err := s.operators.Get(ctx, operatorID)
	if err != nil {
		return 0, err
	}
	if operator.RecoveryCodes == "" {
		return 0, nil
	}
	count := 0
	for _, code := range strings.Split(operator.RecoveryCodes, ",") {
		if strings.TrimSpace(code) != "" {
			count++
		}
	}
	return count, nil
}
