package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestEmptyListsAreArrays keeps the collection endpoints honest. A client
// that reads the list must always find an array, never a null.
func TestEmptyListsAreArrays(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.setup(t)

	for _, path := range []string{"/api/nodes", "/api/entrypoints", "/api/tokens"} {
		rec := env.do(t, http.MethodGet, path, nil, cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"data":[]`) {
			t.Fatalf("%s body = %s, want an empty array", path, rec.Body.String())
		}
	}
}
