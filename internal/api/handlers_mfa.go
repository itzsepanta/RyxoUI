package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ryxoui/ryxoui/internal/audit"
	"github.com/ryxoui/ryxoui/internal/auth"
	"github.com/ryxoui/ryxoui/internal/authz"
	"github.com/ryxoui/ryxoui/internal/db"
)

// handleLoginMFA completes a sign in that requires a second factor.
func (s *Server) handleLoginMFA(w http.ResponseWriter, r *http.Request) {
	var body mfaRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if body.Pending == "" || body.Code == "" {
		handleError(w, r, s.log, badRequest("pending token and code are required"))
		return
	}

	pending, err := s.mfa.Pending(r.Context(), body.Pending)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidPending) {
			handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the sign in challenge expired"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	operator, err := s.mfa.Complete(r.Context(), body.Pending, body.Code)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidPending):
			handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the sign in challenge expired"))
		case errors.Is(err, auth.ErrInvalidCode):
			handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the code is not correct"))
		default:
			handleError(w, r, s.log, err)
		}
		return
	}

	s.completeLogin(w, r, operator, pending.Remember)
}

// handleTOTPStart starts the enrollment of a second factor.
func (s *Server) handleTOTPStart(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	secret, uri, err := s.mfa.StartEnrollment(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"secret": secret,
		"uri":    uri,
	})
}

// totpConfirmRequest confirms the enrollment of a second factor.
type totpConfirmRequest struct {
	Code string `json:"code"`
}

// handleTOTPConfirm switches the second factor on and returns the recovery
// codes. The codes are shown only once.
func (s *Server) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	var body totpConfirmRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if strings.TrimSpace(body.Code) == "" {
		handleError(w, r, s.log, badRequest("code is required"))
		return
	}

	codes, err := s.mfa.ConfirmEnrollment(r.Context(), p.Operator.ID, body.Code)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCode) {
			handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the code is not correct"))
			return
		}
		handleError(w, r, s.log, err)
		return
	}

	err = s.db.InTx(r.Context(), func(tx *db.Tx) error {
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   p.Operator.ID,
			ActorName: p.Operator.Username,
			Action:    "operator.2fa.enable",
			Entity:    "operator",
			EntityID:  p.Operator.ID,
			After:     map[string]any{"totpEnabled": true},
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.2fa_enabled", map[string]any{
			"operatorId": p.Operator.ID,
			"username":   p.Operator.Username,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recoveryCodes": codes,
	})
}

// disable2FARequest turns the second factor off.
type disable2FARequest struct {
	Password string `json:"password"`
}

// handle2FADisable turns the second factor off after the password was
// confirmed.
func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	var body disable2FARequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if !auth.VerifyPassword(p.Operator.PasswordHash, body.Password) {
		handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the password is not correct"))
		return
	}

	if err := s.mfa.Disable(r.Context(), p.Operator.ID); err != nil {
		handleError(w, r, s.log, err)
		return
	}

	err := s.db.InTx(r.Context(), func(tx *db.Tx) error {
		if err := audit.NewStore(tx).Record(r.Context(), tx, audit.Entry{
			ActorID:   p.Operator.ID,
			ActorName: p.Operator.Username,
			Action:    "operator.2fa.disable",
			Entity:    "operator",
			EntityID:  p.Operator.ID,
			After:     map[string]any{"totpEnabled": false},
			IP:        clientIPAddress(r),
			UserAgent: userAgent(r),
			RequestID: requestID(r.Context()),
		}); err != nil {
			return err
		}
		return s.outbox.Publish(r.Context(), tx, "operator.2fa_disabled", map[string]any{
			"operatorId": p.Operator.ID,
			"username":   p.Operator.Username,
		})
	})
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handle2FAStatus reports whether a second factor is enabled and how many
// recovery codes are left.
func (s *Server) handle2FAStatus(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	count, err := s.mfa.RecoveryCodeCount(r.Context(), p.Operator.ID)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":       p.Operator.TOTPEnabled,
		"recoveryCodes": count,
	})
}

// handle2FARecovery consumes a recovery code. It is used when the
// authenticator application is lost.
func (s *Server) handle2FARecovery(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	var body mfaRequest
	if err := decodeJSON(r, &body); err != nil {
		handleError(w, r, s.log, err)
		return
	}
	used, err := s.mfa.UseRecoveryCode(r.Context(), p.Operator.ID, body.Code)
	if err != nil {
		handleError(w, r, s.log, err)
		return
	}
	if !used {
		handleError(w, r, s.log, errorf(http.StatusUnauthorized, CodeUnauthorized, "the recovery code is not correct"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// scopesForMFA documents the scopes that guard the second factor endpoints.
func scopesForMFA() []string {
	return []string{string(authz.ScopeSettingsRead)}
}
