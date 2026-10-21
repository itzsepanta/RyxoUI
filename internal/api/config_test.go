package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ryxoui/ryxoui/internal/engines"
	"github.com/ryxoui/ryxoui/internal/engines/xray"
)

// fakeEngine remembers what the panel asked it to do.
type fakeEngine struct {
	applied  []engines.Config
	rendered int
}

func (e *fakeEngine) Name() string { return "fake" }

func (e *fakeEngine) Versions(ctx context.Context) ([]engines.Release, error) {
	return []engines.Release{{Version: "1.0.0"}}, nil
}

func (e *fakeEngine) Install(ctx context.Context, target engines.Target, version string) error {
	return nil
}

func (e *fakeEngine) Render(ctx context.Context, target engines.Target, config engines.Config) ([]byte, error) {
	e.rendered++
	return []byte(`{"inbounds":[]}`), nil
}

func (e *fakeEngine) Apply(ctx context.Context, target engines.Target, config engines.Config) error {
	e.applied = append(e.applied, config)
	return nil
}

func (e *fakeEngine) Uninstall(ctx context.Context, target engines.Target) error {
	return nil
}

func (e *fakeEngine) Current(ctx context.Context, target engines.Target) ([]byte, error) {
	return []byte(`{"inbounds":[]}`), nil
}

func (e *fakeEngine) Health(ctx context.Context, target engines.Target) (engines.Health, error) {
	return engines.Health{Running: true}, nil
}

func (e *fakeEngine) Users(ctx context.Context, target engines.Target) ([]engines.User, error) {
	return nil, nil
}

func (e *fakeEngine) UpsertUser(ctx context.Context, target engines.Target, user engines.User) error {
	return nil
}

func (e *fakeEngine) SuspendUser(ctx context.Context, target engines.Target, userID string) error {
	return nil
}

func (e *fakeEngine) RemoveUser(ctx context.Context, target engines.Target, userID string) error {
	return nil
}

// TestApplyNodeWritesTheConfiguration checks that the configuration of a node
// is written on the node itself.
func TestApplyNodeWritesTheConfiguration(t *testing.T) {
	engine := &fakeEngine{}
	env := newTestEnvWith(t, Config{Engine: engine})
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d", rec.Code)
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	// A node without a key cannot be written on.
	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/apply", nil, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("apply status = %d, want 409", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/ssh-keys", map[string]any{"name": "k"}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create key status = %d", rec.Code)
	}
	keyID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodPut, "/api/nodes/"+nodeID, map[string]any{
		"sshKeyId": keyID, "keyInstalled": true,
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update node status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodPost, "/api/entrypoints", map[string]any{
		"name": "in", "tag": "inbound-443", "type": "vless", "nodeId": nodeID,
		"config": map[string]any{"port": 443, "clients": []any{}},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create entrypoint status = %d", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/apply", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["ok"] != true {
		t.Fatalf("body = %v", body)
	}

	if len(engine.applied) != 1 {
		t.Fatalf("the engine was called %d times, want 1", len(engine.applied))
	}
	if len(engine.applied[0].Entrypoints) != 1 {
		t.Fatalf("the configuration of the node was empty: %+v", engine.applied[0])
	}
	if engine.applied[0].Entrypoints[0].Tag != "inbound-443" {
		t.Fatalf("entrypoint = %+v", engine.applied[0].Entrypoints[0])
	}
}

// TestTheConfigurationIsChanged checks that a listener can be changed and
// removed.
func TestTheConfigurationIsChanged(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/entrypoints", map[string]any{
		"name": "in", "tag": "inbound-443", "type": "vless",
		"config": map[string]any{"port": 443},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create entrypoint status = %d", rec.Code)
	}
	entryID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodPut, "/api/entrypoints/"+entryID, map[string]any{
		"name": "the listener", "enabled": false, "config": map[string]any{"port": 8443},
	}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	updated := decodeBody(t, rec)
	if updated["name"] != "the listener" {
		t.Fatalf("name = %v", updated["name"])
	}
	if updated["enabled"] != false {
		t.Fatalf("enabled = %v", updated["enabled"])
	}

	rec = env.do(t, http.MethodDelete, "/api/entrypoints/"+entryID, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/api/entrypoints", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	items, _ := decodeBody(t, rec)["data"].([]any)
	if len(items) != 0 {
		t.Fatalf("the listener was not removed: %v", items)
	}
}

// TestTheEngineOfThePanelIsXray checks that the panel knows its engine.
func TestTheEngineOfThePanelIsXray(t *testing.T) {
	env := newTestEnvWith(t, Config{Engine: xray.New()})
	cookie := env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/engines", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("engines status = %d", rec.Code)
	}
	if decodeBody(t, rec)["name"] != "xray" {
		t.Fatalf("name = %v", decodeBody(t, rec)["name"])
	}
}

var _ = json.Marshal
