package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/engines/xray"
)

// TestNodeConfigRendersTheConfiguration checks that the panel shows the
// configuration a node would run.
func TestNodeConfigRendersTheConfiguration(t *testing.T) {
	env := newTestEnvWith(t, Config{Engine: xray.New()})
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d, body = %s", rec.Code, rec.Body.String())
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodPost, "/api/entrypoints", map[string]any{
		"name": "in", "tag": "inbound-443", "type": "vless", "nodeId": nodeID,
		"config": map[string]any{"port": 443, "clients": []any{}},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create entrypoint status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/config", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var document map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode: %v", err)
	}
	inbounds, _ := document["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(inbounds))
	}
	inbound, _ := inbounds[0].(map[string]any)
	if inbound["tag"] != "inbound-443" {
		t.Fatalf("inbound = %v", inbound)
	}
}

// TestNodeConfigNeedsAnEngine checks the answer when the panel has no engine.
func TestNodeConfigNeedsAnEngine(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d", rec.Code)
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/config", nil, cookie)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("config status = %d, want 503", rec.Code)
	}
}

// TestTheEnginesAreKnown checks that the panel reports its engine.
func TestTheEnginesAreKnown(t *testing.T) {
	env := newTestEnvWith(t, Config{Engine: xray.New()})
	cookie := env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/engines", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("engines status = %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["name"] != "xray" {
		t.Fatalf("name = %v", body["name"])
	}
}

var _ = engines.Names
var _ = context.Background
