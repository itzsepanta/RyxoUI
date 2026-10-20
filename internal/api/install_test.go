package api

import (
	"net/http"
	"testing"
)

// TestInstallNode checks that the install of a node starts a job and that the
// log of the job can be read.
func TestInstallNode(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "node-1", "address": "127.0.0.1", "sshHost": "127.0.0.1", "sshPort": 1, "sshUser": "root",
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create node status = %d, body = %s", rec.Code, rec.Body.String())
	}
	nodeID, _ := decodeBody(t, rec)["id"].(string)

	// A password is needed for the first contact.
	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/install", map[string]any{}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("install status = %d, want 400", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/nodes/"+nodeID+"/install", map[string]any{
		"password": "the password of the node",
	}, cookie)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("install status = %d, body = %s", rec.Code, rec.Body.String())
	}
	job := decodeBody(t, rec)
	jobID, _ := job["id"].(string)
	if jobID == "" {
		t.Fatalf("expected a job identifier, got %v", job)
	}
	if job["status"] != "queued" && job["status"] != "running" {
		t.Fatalf("status = %v", job["status"])
	}

	rec = env.do(t, http.MethodGet, "/api/nodes/"+nodeID+"/install", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("jobs status = %d, body = %s", rec.Code, rec.Body.String())
	}
	jobs, _ := decodeBody(t, rec)["data"].([]any)
	if len(jobs) == 0 {
		t.Fatal("expected at least one job")
	}
}

// TestInstallNeedsTheScope checks that the viewer role cannot install.
func TestInstallNeedsTheScope(t *testing.T) {
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

	rec = env.do(t, http.MethodPost, "/api/nodes/some-node/install", map[string]any{
		"password": "the password of the node",
	}, viewerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("install status = %d, want 403", rec.Code)
	}
}
