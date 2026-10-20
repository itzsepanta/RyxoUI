package authz

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// sqlOpen opens a private in-memory SQLite database for the test.
func sqlOpen(t *testing.T) (*sql.DB, error) {
	t.Helper()
	handle, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		return nil, err
	}
	handle.SetMaxOpenConns(1)
	t.Cleanup(func() { handle.Close() })
	return handle, nil
}
