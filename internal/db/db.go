// Package db is the storage layer of the panel.
//
// It speaks plain database/sql and hides the differences between the
// supported engines behind a small dialect. Two rules keep the schema
// portable across engines:
//
//   - Identifiers are TEXT and are generated in Go (see internal/id), so no
//     engine specific auto-increment syntax leaks into the schema.
//   - Timestamps are BIGINT holding Unix seconds and booleans are INTEGER
//     holding 0 or 1, which every engine stores identically.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Dialect names a supported storage engine.
type Dialect string

const (
	// SQLite is the default engine, backed by modernc.org/sqlite.
	SQLite Dialect = "sqlite"
	// Postgres is the optional engine, backed by pgx/v5.
	Postgres Dialect = "postgres"
)

// Options configures a connection.
type Options struct {
	// Driver is the database/sql driver name.
	Driver string
	// DSN is the driver specific data source name.
	DSN string
	// Dialect selects the SQL dialect.
	Dialect Dialect
	// MaxOpenConns bounds the connection pool. Zero means the driver default.
	MaxOpenConns int
	// MaxIdleConns bounds idle connections. Zero means the driver default.
	MaxIdleConns int
	// ConnMaxLifetime caps the age of a pooled connection.
	ConnMaxLifetime time.Duration
}

// DB is a pooled connection to a storage engine.
type DB struct {
	sql     *sql.DB
	dialect Dialect
}

// Querier is the subset of database/sql shared by *DB and *Tx.
//
// Every query passed to a Querier must use ? placeholders; the dialect
// rewrites them when the engine needs $n.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open connects to the database and verifies the connection.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if opts.Driver == "" {
		return nil, fmt.Errorf("db: driver is required")
	}
	if opts.Dialect == "" {
		return nil, fmt.Errorf("db: dialect is required")
	}

	handle, err := sql.Open(opts.Driver, opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if opts.MaxOpenConns > 0 {
		handle.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		handle.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime > 0 {
		handle.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := handle.PingContext(pingCtx); err != nil {
		handle.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &DB{sql: handle, dialect: opts.Dialect}, nil
}

// New wraps an already opened handle. It is used by tests and by callers
// that need to control the handle lifecycle themselves.
func New(handle *sql.DB, dialect Dialect) *DB {
	return &DB{sql: handle, dialect: dialect}
}

// SQL exposes the underlying handle for engine specific escape hatches.
func (db *DB) SQL() *sql.DB { return db.sql }

// Dialect returns the dialect of the connection.
func (db *DB) Dialect() Dialect { return db.dialect }

// Close releases the pool.
func (db *DB) Close() error { return db.sql.Close() }

// Rebind rewrites ? placeholders for the engine.
func (db *DB) Rebind(query string) string { return Rebind(db.dialect, query) }

// ExecContext runs a statement.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.sql.ExecContext(ctx, db.Rebind(query), args...)
}

// QueryContext runs a query.
func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.sql.QueryContext(ctx, db.Rebind(query), args...)
}

// QueryRowContext runs a single row query.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.sql.QueryRowContext(ctx, db.Rebind(query), args...)
}

// Tx is an open transaction bound to a dialect.
type Tx struct {
	tx      *sql.Tx
	dialect Dialect
}

// Rebind rewrites ? placeholders for the engine.
func (tx *Tx) Rebind(query string) string { return Rebind(tx.dialect, query) }

// ExecContext runs a statement inside the transaction.
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, tx.Rebind(query), args...)
}

// QueryContext runs a query inside the transaction.
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.tx.QueryContext(ctx, tx.Rebind(query), args...)
}

// QueryRowContext runs a single row query inside the transaction.
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.tx.QueryRowContext(ctx, tx.Rebind(query), args...)
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.tx.Commit() }

// Rollback aborts the transaction.
func (tx *Tx) Rollback() error { return tx.tx.Rollback() }

// InTx runs fn inside a transaction, committing on success and rolling back
// on error or panic.
func (db *DB) InTx(ctx context.Context, fn func(*Tx) error) error {
	handle, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	tx := &Tx{tx: handle, dialect: db.dialect}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// Rebind rewrites ? placeholders into the form the engine expects.
func Rebind(dialect Dialect, query string) string {
	if dialect != Postgres {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] != '?' {
			b.WriteByte(query[i])
			continue
		}
		n++
		b.WriteByte('$')
		b.WriteString(itoa(n))
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
