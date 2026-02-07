package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDB_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Verify file does not exist yet.
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("database file should not exist before NewDB")
	}

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	// Verify file was created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file should exist after NewDB")
	}
}

func TestNewDB_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	// Verify events table exists by inserting a row.
	_, err = db.Exec(
		"INSERT INTO events (event_json, idempotency_key, created_at) VALUES (?, ?, ?)",
		`{"type":"test"}`, "key-1", 1000,
	)
	if err != nil {
		t.Fatalf("insert into events should succeed: %v", err)
	}

	// Verify schema_version table has the migration recorded.
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("expected schema version 2, got %d", version)
	}
}

func TestNewDB_RunsMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open and close twice; migrations should be idempotent.
	db1, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("first NewDB: %v", err)
	}
	db1.Close()

	db2, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("second NewDB: %v", err)
	}
	defer db2.Close()

	var version int
	err = db2.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("expected schema version 2, got %d", version)
	}
}

func TestNewDB_EmptyPath(t *testing.T) {
	_, err := NewDB("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestClose_ReleasesLock(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db1, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("first NewDB: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Should be able to reopen after close.
	db2, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("second NewDB after close: %v", err)
	}
	defer db2.Close()

	// Verify the database is usable.
	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		t.Fatalf("query after reopen: %v", err)
	}
}

func TestClose_NilInner(t *testing.T) {
	// Closing a DB with nil inner should not panic.
	db := &DB{}
	if err := db.Close(); err != nil {
		t.Fatalf("close nil inner: %v", err)
	}
}

func TestDB_WALMode(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	var mode string
	err = db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected WAL mode, got %q", mode)
	}
}

func TestDB_Inner(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	inner := db.Inner()
	if inner == nil {
		t.Fatal("Inner() should return non-nil *sql.DB")
	}

	// Verify Inner() returns a usable connection.
	var count int
	if err := inner.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatalf("query via Inner(): %v", err)
	}
}

func TestDB_Begin(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	_, err = tx.Exec("INSERT INTO events (event_json, idempotency_key, created_at) VALUES (?, ?, ?)",
		`{"type":"test"}`, "tx-key", 1000)
	if err != nil {
		t.Fatalf("tx exec: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("tx commit: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}
