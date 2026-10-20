package api

import (
	"net/http"
	"testing"
)

func TestNodeLifecycle(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name":    "node-1",
		"address": "10.0.0.5",
		"tags":    []string{"eu"},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	created := decodeBody(t, rec)
	nodeID, _ := created["id"].(string)
	if nodeID == "" {
		t.Fatal("expected a node identifier")
	}
	if created["status"] != "connecting" {
		t.Fatalf("status = %v", created["status"])
	}

	// The same name twice is refused.
	rec = env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name":    "node-1",
		"address": "10.0.0.6",
	}, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", rec.Code)
	}

	// A node without an address is refused.
	rec = env.do(t, http.MethodPost, "/api/nodes", map[string]any{"name": "node-2"}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 400", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/api/nodes", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("nodes = %d, want 1", len(data))
	}

	rec = env.do(t, http.MethodPut, "/api/nodes/"+nodeID, map[string]any{
		"remark":     "the first one",
		"multiplier": 2,
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := decodeBody(t, rec)
	if updated["remark"] != "the first one" {
		t.Fatalf("remark = %v", updated["remark"])
	}
	if multiplier, _ := updated["multiplier"].(float64); multiplier != 2 {
		t.Fatalf("multiplier = %v", updated["multiplier"])
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/health", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}

	rec = env.do(t, http.MethodDelete, "/api/nodes/"+nodeID, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}
	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID, nil, cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
}

func TestEntryPointsAndTemplates(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/entrypoints", map[string]any{
		"name":   "vless in",
		"tag":    "inbound-443",
		"type":   "vless",
		"config": map[string]any{"port": 443},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// The same tag twice is refused.
	rec = env.do(t, http.MethodPost, "/api/entrypoints", map[string]any{
		"name": "again", "tag": "inbound-443", "type": "vless",
	}, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/api/entrypoints", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("entry points = %d", len(data))
	}
	entry, _ := data[0].(map[string]any)
	config, _ := entry["config"].(map[string]any)
	if port, _ := config["port"].(float64); port != 443 {
		t.Fatalf("config = %v", config)
	}

	rec = env.do(t, http.MethodPost, "/api/templates", map[string]any{
		"name": "default",
		"route": map[string]any{
			"rules": []any{map[string]any{"type": "field"}},
		},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("template status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/templates", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("templates status = %d", rec.Code)
	}
}

func TestNodeEndpointsNeedTheScope(t *testing.T) {
	env := newTestEnv(t)
	ownerCookie := env.setup(t)

	// The viewer role may read but not write nodes.
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

	rec = env.do(t, http.MethodGet, "/api/nodes", nil, viewerCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer read status = %d, want 200", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "nope", "address": "10.0.0.9",
	}, viewerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer write status = %d, want 403", rec.Code)
	}
}
