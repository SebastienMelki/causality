package storage

import (
	"database/sql"
	"fmt"
)

// migration represents a versioned schema change.
type migration struct {
	version int
	up      string
}

// migrations is the ordered list of schema migrations.
// Each migration has a version number and SQL to execute.
// New migrations MUST be appended (never modify existing ones).
var migrations = []migration{
	{
		version: 1,
		up: `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_json TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    retry_count INTEGER DEFAULT 0,
    last_retry_at INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_retry ON events(retry_count, last_retry_at);
`,
	},
	{
		version: 2,
		up: `
CREATE TABLE IF NOT EXISTS device_info (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`,
	},
}

// runMigrations applies all pending migrations inside a transaction.
func runMigrations(db *sql.DB) error {
	// Ensure the schema_version table exists (bootstrap).
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	current, err := getCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration v%d: %w", m.version, err)
		}

		if _, err := tx.Exec(m.up); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration v%d: %w", m.version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration v%d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d: %w", m.version, err)
		}
	}

	return nil
}

// getCurrentVersion returns the highest applied migration version, or 0 if none.
func getCurrentVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}
