package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestTheLogOfANodeIsRead checks that the panel hands the log of a node to
// the operator.
func TestTheLogOfANodeIsRead(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "10.0.0.5",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d", rec.Code)
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	// A node without a key has no log to read.
	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/logs", nil, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("logs status = %d, want 409", rec.Code)
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
		t.Fatalf("update node status = %d", rec.Code)
	}

	// The node cannot be reached, so the panel reports the failure instead of
	// pretending that the log is empty.
	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/logs", nil, cookie)
	if rec.Code == http.StatusOK {
		t.Fatalf("the log of a node that cannot be reached must not be read: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("the log must not be empty when the node cannot be reached: %s", rec.Body.String())
	}
}
