package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestTheInstallScriptIsServed checks that a machine can fetch the installer.
func TestTheInstallScriptIsServed(t *testing.T) {
	env := newTestEnv(t)

	rec := env.doInstall(t, http.MethodGet, "/install/ryxoui.sh", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ryxoui.sh") {
		t.Fatalf("the script does not look right: %s", rec.Body.String())
	}
}

// TestTheAuthorityOfThePanelIsServed checks that the panel hands out its
// authority.
func TestTheAuthorityOfThePanelIsServed(t *testing.T) {
	env := newTestEnv(t)

	rec := env.doInstall(t, http.MethodGet, "/install/ca.pem", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BEGIN CERTIFICATE") {
		t.Fatalf("the authority is not a certificate: %s", rec.Body.String())
	}

	// The authority is the same every time.
	second := env.doInstall(t, http.MethodGet, "/install/ca.pem", nil, "")
	if second.Body.String() != rec.Body.String() {
		t.Fatal("the authority of the panel changed")
	}
}

// TestAMachineRegistersItself checks the way a fresh machine hands itself to
// the panel.
func TestAMachineRegistersItself(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	// A token with the scope to register nodes.
	rec := env.do(t, http.MethodPost, "/api/tokens", map[string]any{
		"name": "installer", "scopes": []string{"nodes:write"},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token status = %d, body = %s", rec.Code, rec.Body.String())
	}
	token, _ := decodeBody(t, rec)["plaintext"].(string)
	if token == "" {
		t.Fatalf("expected a token: %v", decodeBody(t, rec))
	}

	// Without a token nothing happens.
	rec = env.doInstall(t, http.MethodPost, "/install/register", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("register status = %d, want 401", rec.Code)
	}

	// With the token the machine is registered and gets the key of the panel.
	rec = env.do(t, http.MethodPost, "/install/register", map[string]any{
		"name": "node-1", "address": "10.0.0.5", "sshPort": 22, "sshUser": "root",
	}, "")
	rec = env.doInstall(t, http.MethodPost, "/install/register", map[string]any{
		"name": "node-1", "address": "10.0.0.5", "sshPort": 22, "sshUser": "root",
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := decodeBody(t, rec)
	nodeID, _ := body["id"].(string)
	if nodeID == "" {
		t.Fatalf("expected a node identifier: %v", body)
	}
	publicKey, _ := body["sshPublicKey"].(string)
	if !strings.HasPrefix(publicKey, "ssh-ed25519") {
		t.Fatalf("the key of the panel is missing: %v", body)
	}
	if strings.ContainsAny(publicKey, "\n\r") {
		t.Fatalf("the key of the panel holds a line break: %q", publicKey)
	}

	// The machine confirms that the key works.
	rec = env.doInstall(t, http.MethodPost, "/install/confirm", map[string]any{
		"nodeId": nodeID,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("read node status = %d", rec.Code)
	}
	node := decodeBody(t, rec)
	if node["keyInstalled"] != true {
		t.Fatalf("the key was not marked as installed: %v", node)
	}
	if node["sshHost"] != "10.0.0.5" {
		t.Fatalf("ssh host = %v", node["sshHost"])
	}
}

// TestTheTokenNeedsTheScope checks that a token without the right scope may
// not register a node.
func TestTheTokenNeedsTheScope(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/tokens", map[string]any{
		"name": "reader", "scopes": []string{"nodes:read"},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token status = %d", rec.Code)
	}
	token, _ := decodeBody(t, rec)["plaintext"].(string)

	rec = env.doInstall(t, http.MethodPost, "/install/register", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("register status = %d, want 403", rec.Code)
	}
}

var _ = json.Marshal
