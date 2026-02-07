// Package storage provides SQLite-based persistent event storage for the mobile SDK.
//
// It uses modernc.org/sqlite (pure Go, no CGO) for gomobile cross-compilation
// compatibility. The database operates in WAL mode for concurrent read/write access
// and automatically runs schema migrations on open.
package storage

import (
	"database/sql"
	"fmt"

	// Register the pure-Go SQLite driver. This does NOT require CGO.
	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB connection to a SQLite database.
// It manages the connection lifecycle and ensures migrations run on open.
type DB struct {
	inner *sql.DB
	path  string
}

// NewDB opens (or creates) a SQLite database at dbPath with WAL mode and busy timeout.
// Migrations are applied automatically on open.
func NewDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}

	// WAL mode for concurrent access, 5s busy timeout for lock contention.
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify the connection is usable.
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Run schema migrations.
	if err := runMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &DB{
		inner: sqlDB,
		path:  dbPath,
	}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db.inner == nil {
		return nil
	}
	return db.inner.Close()
}

// Exec executes a query without returning rows.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.inner.Exec(query, args...)
}

// Query executes a query that returns rows.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.inner.Query(query, args...)
}

// QueryRow executes a query that returns at most one row.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.inner.QueryRow(query, args...)
}

// Begin starts a new transaction.
func (db *DB) Begin() (*sql.Tx, error) {
	return db.inner.Begin()
}

// Inner returns the underlying *sql.DB for advanced use cases.
// Prefer the convenience wrappers when possible.
func (db *DB) Inner() *sql.DB {
	return db.inner
}
