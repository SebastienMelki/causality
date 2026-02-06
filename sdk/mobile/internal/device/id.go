package device

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
	"github.com/google/uuid"
)

// deviceIDKey is the key used to store the device ID in the device_info table.
const deviceIDKey = "device_id"

// IDManager handles device ID generation and persistence.
// The device ID is generated on first launch, persisted to SQLite,
// and cached in memory for fast access.
//
// IDManager is safe for concurrent use by multiple goroutines.
type IDManager struct {
	db         *storage.DB
	deviceID   string
	mu         sync.RWMutex
	persistent bool // Whether native wrappers should use Keychain/EncryptedPrefs
}

// NewIDManager creates a new IDManager backed by the given database.
// The persistent flag indicates whether native wrappers should use
// Keychain (iOS) or EncryptedSharedPreferences (Android) for additional
// persistence. The Go layer always uses SQLite.
func NewIDManager(db *storage.DB, persistent bool) *IDManager {
	return &IDManager{
		db:         db,
		persistent: persistent,
	}
}

// GetOrCreateDeviceID returns the device ID, creating and persisting one if
// it does not exist. The ID is cached in memory after the first call.
//
// The generated ID is a standard UUID v4 string.
func (m *IDManager) GetOrCreateDeviceID() string {
	// Fast path: return cached ID.
	m.mu.RLock()
	if m.deviceID != "" {
		id := m.deviceID
		m.mu.RUnlock()
		return id
	}
	m.mu.RUnlock()

	// Slow path: check DB, possibly create.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock.
	if m.deviceID != "" {
		return m.deviceID
	}

	// Try to load from DB.
	id, err := m.loadFromDB()
	if err == nil && id != "" {
		m.deviceID = id
		return m.deviceID
	}

	// Generate new device ID.
	m.deviceID = uuid.New().String()
	if err := m.saveToDB(m.deviceID); err != nil {
		// Log but don't fail -- ID still works in memory for this session.
		// Future sessions will regenerate if DB write fails.
		_ = err
	}

	return m.deviceID
}

// RegenerateDeviceID creates a new device ID, updates the DB, and returns it.
// This is used for ResetAll (complete privacy reset).
func (m *IDManager) RegenerateDeviceID() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deviceID = uuid.New().String()
	if err := m.saveToDB(m.deviceID); err != nil {
		// Same as above: memory-only fallback.
		_ = err
	}

	return m.deviceID
}

// IsPersistent returns whether the device ID should use native secure storage
// (Keychain/EncryptedPrefs) in addition to SQLite.
func (m *IDManager) IsPersistent() bool {
	return m.persistent
}

// loadFromDB retrieves the device ID from the device_info table.
// Returns empty string if not found.
func (m *IDManager) loadFromDB() (string, error) {
	var value string
	err := m.db.QueryRow(
		"SELECT value FROM device_info WHERE key = ?",
		deviceIDKey,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load device ID: %w", err)
	}

	return value, nil
}

// saveToDB persists the device ID to the device_info table.
// Uses INSERT OR REPLACE to handle both create and update.
func (m *IDManager) saveToDB(id string) error {
	_, err := m.db.Exec(
		"INSERT OR REPLACE INTO device_info (key, value) VALUES (?, ?)",
		deviceIDKey, id,
	)
	if err != nil {
		return fmt.Errorf("save device ID: %w", err)
	}
	return nil
}
