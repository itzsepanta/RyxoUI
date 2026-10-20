package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/ryxoui/ryxoui/internal/nodes"
)

// TestProbeNode checks the probe of a node: the answer of the node becomes
// the status of the node and a health sample is recorded.
func TestProbeNode(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	env.server.probe = func(ctx context.Context, node nodes.Node, privateKey string) (nodes.ProbeResult, error) {
		return nodes.ProbeResult{
			OS:          "linux",
			Arch:        "x86_64",
			CoreVersion: "Xray 1.8.4",
			MemTotal:    1024,
			MemUsed:     256,
			DiskTotal:   4096,
			DiskUsed:    1024,
			Load1:       0.5,
			HostKey:     "pinned-key",
		}, nil
	}

	rec := env.do(t, http.MethodPost, "/api/ssh-keys", map[string]any{"name": "node-key"}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create key status = %d, body = %s", rec.Code, rec.Body.String())
	}
	keyID, _ := decodeBody(t, rec)["id"].(string)
	if keyID == "" {
		t.Fatal("expected a key identifier")
	}
	if _, ok := decodeBody(t, rec)["privateKey"]; ok {
		t.Fatal("the private key must never leave the panel")
	}

	rec = env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
		"sshHost": "10.0.0.5", "sshPort": 22, "sshUser": "root",
		"sshKeyId": keyID, "keyInstalled": true,
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d, body = %s", rec.Code, rec.Body.String())
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/probe", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe status = %d, body = %s", rec.Code, rec.Body.String())
	}
	result := decodeBody(t, rec)
	if result["os"] != "linux" {
		t.Fatalf("result = %v", result)
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("read node status = %d", rec.Code)
	}
	node := decodeBody(t, rec)
	if node["status"] != "connected" {
		t.Fatalf("status = %v", node["status"])
	}
	if node["os"] != "linux" || node["arch"] != "x86_64" {
		t.Fatalf("node = %v", node)
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/health", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}
	samples, _ := decodeBody(t, rec)["data"].([]any)
	if len(samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(samples))
	}
}

// TestProbeNodeWithoutAKey checks that a node without a key is refused.
func TestProbeNodeWithoutAKey(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d", rec.Code)
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/probe", nil, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("probe status = %d, want 409", rec.Code)
	}
}

// TestProbeNeedsTheScope checks that the viewer role cannot probe a node.
func TestProbeNeedsTheScope(t *testing.T) {
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

	rec = env.do(t, http.MethodPost, "/api/nodes/some-node/probe", nil, viewerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("probe status = %d, want 403", rec.Code)
	}
}
