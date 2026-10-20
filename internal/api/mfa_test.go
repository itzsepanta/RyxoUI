package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/ryxoui/ryxoui/internal/auth"
)

// TestSecondFactorFlow walks through the whole life cycle of a second
// factor: enrollment, confirmation, sign in, recovery code and removal.
func TestSecondFactorFlow(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	// The factor is off to begin with.
	rec := env.do(t, http.MethodGet, "/api/me/2fa", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if enabled := decodeBody(t, rec)["enabled"]; enabled != false {
		t.Fatalf("enabled = %v, want false", enabled)
	}

	// Start the enrollment.
	rec = env.do(t, http.MethodPost, "/api/me/2fa/totp", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, body = %s", rec.Code, rec.Body.String())
	}
	secret, _ := decodeBody(t, rec)["secret"].(string)
	if secret == "" {
		t.Fatal("expected a secret")
	}

	// A wrong code is refused.
	rec = env.do(t, http.MethodPost, "/api/me/2fa/totp/confirm", map[string]any{"code": "000000"}, cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code status = %d, want 401", rec.Code)
	}

	// The right code switches the factor on and returns recovery codes.
	code := auth.CodeForTime(secret, time.Now().UTC())
	rec = env.do(t, http.MethodPost, "/api/me/2fa/totp/confirm", map[string]any{"code": code}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", rec.Code, rec.Body.String())
	}
	recovery, _ := decodeBody(t, rec)["recoveryCodes"].([]any)
	if len(recovery) != 8 {
		t.Fatalf("recovery codes = %d, want 8", len(recovery))
	}

	// Signing in now needs a second factor.
	rec = env.do(t, http.MethodPost, "/api/login", map[string]any{
		"username": "owner", "password": "correct horse",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	login := decodeBody(t, rec)
	if needs, _ := login["needsMfa"].(bool); !needs {
		t.Fatalf("expected the sign in to ask for a second factor: %v", login)
	}
	pending, _ := login["pending"].(string)
	if pending == "" {
		t.Fatal("expected a pending token")
	}

	// A wrong code does not open the door.
	rec = env.do(t, http.MethodPost, "/api/login/mfa", map[string]any{
		"pending": pending, "code": "000000",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("mfa status = %d, want 401", rec.Code)
	}

	// The right code does.
	rec = env.do(t, http.MethodPost, "/api/login/mfa", map[string]any{
		"pending": pending, "code": auth.CodeForTime(secret, time.Now().UTC()),
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("mfa status = %d, body = %s", rec.Code, rec.Body.String())
	}
	mfaCookie := cookieValue(t, rec)

	rec = env.do(t, http.MethodGet, "/api/me", nil, mfaCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	if enabled := decodeBody(t, rec)["operator"].(map[string]any)["totpEnabled"]; enabled != true {
		t.Fatalf("totpEnabled = %v, want true", enabled)
	}

	// A recovery code can replace the authenticator application.
	rec = env.do(t, http.MethodPost, "/api/me/2fa/recovery", map[string]any{
		"code": recovery[0],
	}, mfaCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("recovery status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Removing the factor needs the password.
	rec = env.do(t, http.MethodDelete, "/api/me/2fa", map[string]any{"password": "wrong"}, mfaCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disable status = %d, want 401", rec.Code)
	}

	rec = env.do(t, http.MethodDelete, "/api/me/2fa", map[string]any{"password": "correct horse"}, mfaCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/me/2fa", nil, mfaCookie)
	if enabled := decodeBody(t, rec)["enabled"]; enabled != false {
		t.Fatalf("enabled = %v, want false", enabled)
	}
}

func TestSecondFactorRequiresAuthentication(t *testing.T) {
	env := newTestEnv(t)
	env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/me/2fa/totp", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
