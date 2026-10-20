package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryxoui/ryxoui/internal/db"
	_ "modernc.org/sqlite"
)

type testEnv struct {
	server *Server
	db     *db.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	handle.SetMaxOpenConns(4)
	t.Cleanup(func() { handle.Close() })

	store := db.New(handle, db.SQLite)
	if _, err := db.NewMigrator(store).Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	server, err := New(Config{DB: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return &testEnv{server: server, db: store}
}

func (e *testEnv) do(t *testing.T, method, path string, body any, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	}
	rec := httptest.NewRecorder()
	e.server.Handler().ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) setup(t *testing.T) string {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/setup", map[string]string{
		"username": "owner", "password": "correct horse",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup status = %d, body = %s", rec.Code, rec.Body.String())
	}
	return cookieValue(t, rec)
}

func cookieValue(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			return c.Value
		}
	}
	t.Fatalf("no session cookie in response")
	return ""
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestSetupCreatesOwnerAndSession(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/api/setup", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("setup state status = %d", rec.Code)
	}
	if initialized := decodeBody(t, rec)["initialized"]; initialized != false {
		t.Fatalf("initialized = %v, want false", initialized)
	}

	cookie := env.setup(t)
	if cookie == "" {
		t.Fatal("expected a session cookie")
	}

	rec = env.do(t, http.MethodGet, "/api/setup", nil, "")
	if initialized := decodeBody(t, rec)["initialized"]; initialized != true {
		t.Fatalf("initialized = %v, want true", initialized)
	}

	// The panel must refuse a second setup.
	rec = env.do(t, http.MethodPost, "/api/setup", map[string]string{
		"username": "other", "password": "another password",
	}, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("second setup status = %d, want 409", rec.Code)
	}
}

func TestSetupRejectsShortPassword(t *testing.T) {
	env := newTestEnv(t)
	rec := env.do(t, http.MethodPost, "/api/setup", map[string]string{
		"username": "owner", "password": "short",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUnauthenticatedRequestIsRejected(t *testing.T) {
	env := newTestEnv(t)
	env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/operators", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	body := decodeBody(t, rec)
	if errBody, ok := body["error"].(map[string]any); !ok || errBody["code"] != CodeUnauthorized {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/login", map[string]any{
		"username": "owner", "password": "wrong",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLoginAndMe(t *testing.T) {
	env := newTestEnv(t)
	env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/login", map[string]any{
		"username": "owner", "password": "correct horse",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookie := cookieValue(t, rec)

	rec = env.do(t, http.MethodGet, "/api/me", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	body := decodeBody(t, rec)
	operator, _ := body["operator"].(map[string]any)
	if operator["username"] != "owner" {
		t.Fatalf("operator = %v", operator)
	}
}

func TestViewerCannotWriteOperators(t *testing.T) {
	env := newTestEnv(t)
	ownerCookie := env.setup(t)
	viewerID := roleIDByName(t, env, "viewer")

	// Create a viewer operator.
	rec := env.do(t, http.MethodPost, "/api/operators", map[string]any{
		"username": "watcher", "password": "watcher password", "roleId": viewerID,
	}, ownerCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create operator status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodPost, "/api/login", map[string]any{
		"username": "watcher", "password": "watcher password",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer login status = %d", rec.Code)
	}
	viewerCookie := cookieValue(t, rec)

	// Reading is allowed.
	rec = env.do(t, http.MethodGet, "/api/operators", nil, viewerCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer list status = %d, want 200", rec.Code)
	}

	// Writing is not.
	rec = env.do(t, http.MethodPost, "/api/operators", map[string]any{
		"username": "intruder", "password": "intruder pass", "roleId": viewerID,
	}, viewerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create status = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestRoleCreateRejectsUnknownScope(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/roles", map[string]any{
		"name":   "weird",
		"base":   "viewer",
		"scopes": []string{"users:write"},
	}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestRoleCreateAndList(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/roles", map[string]any{
		"name":   "auditor",
		"base":   "viewer",
		"scopes": []string{"audit:read"},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/roles", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) < 4 {
		t.Fatalf("expected at least 4 roles, got %d", len(data))
	}
}

func TestAuditLogRecordsChanges(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/audit", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit status = %d", rec.Code)
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected audit entries after setup")
	}

	entry, _ := data[0].(map[string]any)
	if entry["action"] != "operator.create" {
		t.Fatalf("first action = %v, want operator.create", entry["action"])
	}
}

func TestSettingsUpdate(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPut, "/api/settings/theme", map[string]any{"value": "dark"}, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/api/settings", nil, cookie)
	data, _ := decodeBody(t, rec)["data"].([]any)
	found := false
	for _, item := range data {
		setting, _ := item.(map[string]any)
		if setting["key"] == "theme" && setting["value"] == "dark" {
			found = true
		}
	}
	if !found {
		t.Fatalf("setting not stored: %v", data)
	}
}

func TestAPITokenAuthentication(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodPost, "/api/tokens", map[string]any{
		"name":   "ci",
		"scopes": []string{"operators:read"},
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("token status = %d, body = %s", rec.Code, rec.Body.String())
	}
	token, _ := decodeBody(t, rec)["plaintext"].(string)
	if token == "" {
		t.Fatal("expected a plaintext token")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/operators", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("token auth status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// The token scope does not allow writing operators.
	req = httptest.NewRequest(http.MethodPost, "/api/operators", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("token write status = %d, want 403", rec.Code)
	}
}

func TestEventCatalogueIsExposed(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	rec := env.do(t, http.MethodGet, "/api/event-types", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	data, _ := decodeBody(t, rec)["data"].([]any)
	if len(data) == 0 {
		t.Fatal("expected event types")
	}
}

func roleIDByName(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	roles, err := env.server.Roles().List(t.Context())
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	for _, role := range roles {
		if role.Name == name {
			return role.ID
		}
	}
	t.Fatalf("role %q not found", name)
	return ""
}

func operatorIDByName(t *testing.T, env *testEnv, username string) string {
	t.Helper()
	operators, err := env.server.Operators().List(t.Context())
	if err != nil {
		t.Fatalf("list operators: %v", err)
	}
	for _, op := range operators {
		if op.Username == username {
			return op.ID
		}
	}
	t.Fatalf("operator %q not found", username)
	return ""
}
