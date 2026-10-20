package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migration is one versioned schema change.
type migration struct {
	version    string
	name       string
	checksum   string
	statements []string
}

// Migrator applies versioned schema changes to a database.
type Migrator struct {
	db     *DB
	source fs.FS
	dir    string
}

// NewMigrator returns a migrator that reads embedded migrations.
func NewMigrator(db *DB) *Migrator {
	return &Migrator{db: db, source: migrationFS, dir: "migrations"}
}

// NewMigratorFS returns a migrator that reads migrations from source.
func NewMigratorFS(db *DB, source fs.FS, dir string) *Migrator {
	if dir == "" {
		dir = "migrations"
	}
	return &Migrator{db: db, source: source, dir: dir}
}

// Migrate applies every pending migration and returns the versions applied.
func (m *Migrator) Migrate() ([]string, error) {
	if err := m.ensureTable(); err != nil {
		return nil, err
	}

	applied, err := m.appliedVersions()
	if err != nil {
		return nil, err
	}

	migrations, err := m.load()
	if err != nil {
		return nil, err
	}

	var ran []string
	for _, mig := range migrations {
		if prev, ok := applied[mig.version]; ok {
			if prev != mig.checksum {
				return ran, fmt.Errorf("db: migration %s was modified after it was applied", mig.version)
			}
			continue
		}
		if err := m.apply(mig); err != nil {
			return ran, err
		}
		ran = append(ran, mig.version)
	}
	return ran, nil
}

func (m *Migrator) ensureTable() error {
	const stmt = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at BIGINT NOT NULL
	)`
	if _, err := m.db.ExecContext(context.Background(), stmt); err != nil {
		return fmt.Errorf("db: create schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) appliedVersions() (map[string]string, error) {
	rows, err := m.db.QueryContext(context.Background(), "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("db: read schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("db: scan schema_migrations: %w", err)
		}
		out[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: iterate schema_migrations: %w", err)
	}
	return out, nil
}

func (m *Migrator) load() ([]migration, error) {
	entries, err := fs.ReadDir(m.source, m.dir)
	if err != nil {
		return nil, fmt.Errorf("db: read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	out := make([]migration, 0, len(names))
	for _, name := range names {
		body, err := fs.ReadFile(m.source, path.Join(m.dir, name))
		if err != nil {
			return nil, fmt.Errorf("db: read migration %s: %w", name, err)
		}
		version, label, ok := splitVersion(name)
		if !ok {
			return nil, fmt.Errorf("db: migration %s must be named <version>_<name>.sql", name)
		}
		sum := sha256.Sum256(body)
		out = append(out, migration{
			version:    version,
			name:       label,
			checksum:   hex.EncodeToString(sum[:]),
			statements: splitStatements(string(body)),
		})
	}
	return out, nil
}

func (m *Migrator) apply(mig migration) error {
	return m.db.InTx(context.Background(), func(tx *Tx) error {
		for _, stmt := range mig.statements {
			if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
				return fmt.Errorf("db: apply migration %s: %w", mig.version, err)
			}
		}
		_, err := tx.ExecContext(context.Background(),
			"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
			mig.version, mig.name, mig.checksum, nowUnix())
		if err != nil {
			return fmt.Errorf("db: record migration %s: %w", mig.version, err)
		}
		return nil
	})
}

// splitVersion extracts the numeric version and the label from a file name.
func splitVersion(name string) (string, string, bool) {
	base := strings.TrimSuffix(name, ".sql")
	idx := strings.Index(base, "_")
	if idx <= 0 {
		return "", "", false
	}
	version := base[:idx]
	if _, err := strconv.Atoi(version); err != nil {
		return "", "", false
	}
	return version, base[idx+1:], true
}

// splitStatements breaks a script into individual statements. Statements are
// separated by a semicolon at the end of a line; comments and blank lines
// are dropped.
func splitStatements(script string) []string {
	var out []string
	var current []string

	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		current = append(current, line)
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.Join(current, "\n")), ";"))
			if stmt != "" {
				out = append(out, stmt)
			}
			current = nil
		}
	}
	if rest := strings.TrimSpace(strings.Join(current, "\n")); rest != "" {
		out = append(out, rest)
	}
	return out
}
