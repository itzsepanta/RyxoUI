package api

import (
	"net/http"
	"testing"
)

// TestTheWholeFlowOfThePanel walks the definition of done of the first
// milestone in one go: the owner registers, signs in, builds a role, the
// role protects the endpoints, every change lands in the audit log and the
// operator signs out.
func TestTheWholeFlowOfThePanel(t *testing.T) {
	env := newTestEnv(t)

	// The owner registers.
	rec := env.do(t, http.MethodPost, "/api/setup", map[string]string{
		"username": "owner", "password": "correct horse battery",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ownerCookie := cookieValue(t, rec)

	// The owner signs in.
	rec = env.do(t, http.MethodPost, "/api/login", map[string]string{
		"username": "owner", "password": "correct horse battery",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ownerCookie = cookieValue(t, rec)

	// The owner builds a role with one scope.
	rec = env.do(t, http.MethodPost, "/api/roles", map[string]any{
		"name": "watcher", "description": "may only look", "scopes": []string{"nodes:read"},
	}, ownerCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create role status = %d, body = %s", rec.Code, rec.Body.String())
	}
	roleID, _ := decodeBody(t, rec)["id"].(string)
	if roleID == "" {
		t.Fatal("expected a role identifier")
	}

	// The owner makes an operator with that role.
	rec = env.do(t, http.MethodPost, "/api/operators", map[string]any{
		"username": "watcher", "password": "watcher password", "roleId": roleID,
	}, ownerCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create operator status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// The operator signs in and may not write what the role does not carry.
	rec = env.do(t, http.MethodPost, "/api/login", map[string]string{
		"username": "watcher", "password": "watcher password",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	watcherCookie := cookieValue(t, rec)

	rec = env.do(t, http.MethodGet, "/api/nodes", nil, watcherCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("read status = %d, want 200", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/api/nodes", map[string]any{
		"name": "nope", "address": "10.0.0.9",
	}, watcherCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("write status = %d, want 403", rec.Code)
	}

	// Every change is in the audit log.
	rec = env.do(t, http.MethodGet, "/api/audit?entity=operator", nil, ownerCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit status = %d", rec.Code)
	}
	entries, _ := decodeBody(t, rec)["data"].([]any)
	if len(entries) == 0 {
		t.Fatal("the change of the operator is not in the audit log")
	}

	found := false
	for _, item := range entries {
		entry, _ := item.(map[string]any)
		if entry["action"] != "operator.create" {
			continue
		}
		found = true
		if entry["actorName"] != "owner" {
			t.Fatalf("the actor of the entry is %v", entry["actorName"])
		}
	}
	if !found {
		t.Fatalf("the change of the operator is not in the audit log: %v", entries)
	}

	// The operator signs out.
	rec = env.do(t, http.MethodPost, "/api/logout", nil, watcherCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/api/nodes", nil, watcherCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("the session still works after the logout: %d", rec.Code)
	}
}
