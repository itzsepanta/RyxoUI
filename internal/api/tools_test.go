package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestTheKeyToolsMakeKeys checks the tools of the panel.
func TestTheKeyToolsMakeKeys(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/keys/x25519", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("x25519 status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["publicKey"] == nil || body["privateKey"] == nil {
		t.Fatalf("the key is not complete: %v", body)
	}

	rec = env.do(t, http.MethodPost, "/api/keys/ech", map[string]any{"publicName": "example.com"}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("ech status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body = decodeBody(t, rec)
	config, _ := body["config"].(string)
	if config == "" {
		t.Fatalf("the configuration list is missing: %v", body)
	}

	rec = env.do(t, http.MethodPost, "/api/keys/vless-encryption", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("vless status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body = decodeBody(t, rec)
	if body["clientKey"] == nil || body["serverKey"] == nil {
		t.Fatalf("the key is not complete: %v", body)
	}
}

// TestTheToolsNeedTheScope checks that the viewer role may not use them.
func TestTheToolsNeedTheScope(t *testing.T) {
	env := newTestEnv(t)
	ownerCookie := env.setup(t)

	viewerID := roleIDByName(t, env, "viewer")
	rec := env.do(t, http.MethodPost, "/api/operators", map[string]any{
		"username": "watcher", "password": "watcher password", "roleId": viewerID,
	}, ownerCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create operator status = %d", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/login", map[string]any{
		"username": "watcher", "password": "watcher password",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	viewerCookie := cookieValue(t, rec)

	rec = env.do(t, http.MethodPost, "/api/keys/x25519", nil, viewerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("x25519 status = %d, want 403", rec.Code)
	}
}

// TestThePresetsAreThere checks that the panel ships with its presets.
func TestThePresetsAreThere(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/presets", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("presets status = %d", rec.Code)
	}
	presets, _ := decodeBody(t, rec)["data"].([]any)
	if len(presets) == 0 {
		t.Fatal("the panel must ship with presets")
	}

	first, _ := presets[0].(map[string]any)
	name, _ := first["name"].(string)
	if !strings.Contains(name, "-") {
		t.Fatalf("the name of a preset looks wrong: %v", first)
	}
}

// TestAPresetIsChangedAndRemoved checks that a preset can be made and removed.
func TestAPresetIsChangedAndRemoved(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/presets", map[string]any{
		"name": "my-preset", "kind": "entrypoint", "type": "vless",
		"config": map[string]any{"port": 443},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create preset status = %d, body = %s", rec.Code, rec.Body.String())
	}
	presetID, _ := decodeBody(t, rec)["id"].(string)
	if presetID == "" {
		t.Fatal("expected a preset identifier")
	}

	// The same name twice is refused.
	rec = env.do(t, http.MethodPost, "/api/presets", map[string]any{
		"name": "my-preset", "kind": "entrypoint", "type": "vless",
	}, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate preset status = %d, want 409", rec.Code)
	}

	rec = env.do(t, http.MethodDelete, "/api/presets/"+presetID, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}
}
