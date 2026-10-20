package auth

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

func mfaDB(t *testing.T) *db.DB {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(4)
	t.Cleanup(func() { handle.Close() })

	q := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(q).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return q
}

func createOperator(t *testing.T, q *db.DB, username string) Operator {
	t.Helper()
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	op, err := NewOperatorStore(q).Create(context.Background(), Operator{
		Username: username, PasswordHash: hash, RoleID: "role-1", Status: StatusActive,
	})
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	return op
}

func TestVerifyTOTP(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	code := hotp(secret, time.Now().UTC().Unix()/TOTPStep)
	if !VerifyTOTP(secret, code, time.Now().UTC()) {
		t.Fatal("a fresh code must verify")
	}
	if VerifyTOTP(secret, "000000", time.Now().UTC()) {
		t.Fatal("a wrong code must not verify")
	}
	if VerifyTOTP("", code, time.Now().UTC()) {
		t.Fatal("an empty secret must not verify")
	}
}

func TestTOTPURI(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	uri := TOTPURI("RyxoUI", "owner", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("uri = %q", uri)
	}
	if !strings.Contains(uri, "secret="+secret) {
		t.Fatalf("uri does not carry the secret: %q", uri)
	}
}

func TestEnrollmentAndComplete(t *testing.T) {
	q := mfaDB(t)
	ctx := context.Background()
	op := createOperator(t, q, "owner")
	service := NewMFAService(q)

	secret, uri, err := service.StartEnrollment(ctx, op.ID)
	if err != nil {
		t.Fatalf("StartEnrollment: %v", err)
	}
	if secret == "" || uri == "" {
		t.Fatal("expected a secret and an uri")
	}

	// The factor is not active before it is confirmed.
	updated, _ := NewOperatorStore(q).Get(ctx, op.ID)
	if updated.TOTPEnabled {
		t.Fatal("the factor must stay disabled until it is confirmed")
	}

	// A wrong code must not switch it on.
	if _, err := service.ConfirmEnrollment(ctx, op.ID, "000000"); err == nil {
		t.Fatal("expected the wrong code to be rejected")
	}

	codes, err := service.ConfirmEnrollment(ctx, op.ID, hotp(secret, time.Now().UTC().Unix()/TOTPStep))
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("recovery codes = %d, want 8", len(codes))
	}

	updated, _ = NewOperatorStore(q).Get(ctx, op.ID)
	if !updated.TOTPEnabled {
		t.Fatal("the factor must be active after the confirmation")
	}

	// A challenge is answered with a fresh code.
	pending, err := service.Begin(ctx, op.ID, true)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := service.Complete(ctx, pending, "000000"); err != ErrInvalidCode {
		t.Fatalf("Complete(bad) = %v, want ErrInvalidCode", err)
	}

	pending, _ = service.Begin(ctx, op.ID, false)
	if _, err := service.Complete(ctx, pending, hotp(secret, time.Now().UTC().Unix()/TOTPStep)); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// The challenge is consumed and cannot be replayed.
	if _, err := service.Complete(ctx, pending, hotp(secret, time.Now().UTC().Unix()/TOTPStep)); err == nil {
		t.Fatal("a challenge must not be reusable")
	}
}

func TestRecoveryCodeIsConsumed(t *testing.T) {
	q := mfaDB(t)
	ctx := context.Background()
	op := createOperator(t, q, "owner")
	service := NewMFAService(q)

	secret, _, err := service.StartEnrollment(ctx, op.ID)
	if err != nil {
		t.Fatalf("StartEnrollment: %v", err)
	}
	codes, err := service.ConfirmEnrollment(ctx, op.ID, hotp(secret, time.Now().UTC().Unix()/TOTPStep))
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}

	count, err := service.RecoveryCodeCount(ctx, op.ID)
	if err != nil {
		t.Fatalf("RecoveryCodeCount: %v", err)
	}
	if count != 8 {
		t.Fatalf("recovery codes = %d, want 8", count)
	}

	used, err := service.UseRecoveryCode(ctx, op.ID, codes[0])
	if err != nil {
		t.Fatalf("UseRecoveryCode: %v", err)
	}
	if !used {
		t.Fatal("the recovery code must be accepted")
	}

	used, err = service.UseRecoveryCode(ctx, op.ID, codes[0])
	if err != nil {
		t.Fatalf("UseRecoveryCode: %v", err)
	}
	if used {
		t.Fatal("a recovery code must not work twice")
	}

	count, _ = service.RecoveryCodeCount(ctx, op.ID)
	if count != 7 {
		t.Fatalf("recovery codes = %d, want 7", count)
	}

	// A recovery code also completes a challenge.
	pending, err := service.Begin(ctx, op.ID, false)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := service.Complete(ctx, pending, codes[1]); err != nil {
		t.Fatalf("Complete with a recovery code: %v", err)
	}
}

func TestDisableRemovesTheFactor(t *testing.T) {
	q := mfaDB(t)
	ctx := context.Background()
	op := createOperator(t, q, "owner")
	service := NewMFAService(q)

	secret, _, _ := service.StartEnrollment(ctx, op.ID)
	if _, err := service.ConfirmEnrollment(ctx, op.ID, hotp(secret, time.Now().UTC().Unix()/TOTPStep)); err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}
	if err := service.Disable(ctx, op.ID); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	updated, _ := NewOperatorStore(q).Get(ctx, op.ID)
	if updated.TOTPEnabled || updated.TOTPSecret != "" || updated.RecoveryCodes != "" {
		t.Fatalf("the factor was not removed: %+v", updated)
	}
}

func TestPendingExpires(t *testing.T) {
	q := mfaDB(t)
	ctx := context.Background()
	op := createOperator(t, q, "owner")
	service := NewMFAService(q)

	pending, err := service.Begin(ctx, op.ID, false)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := service.Pending(ctx, pending); err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if _, err := service.Pending(ctx, "unknown"); err != ErrInvalidPending {
		t.Fatalf("Pending(unknown) = %v, want ErrInvalidPending", err)
	}
}
