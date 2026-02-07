// Package identity provides user identity management for the Causality mobile SDK.
//
// User identity enables cross-device correlation when users log in. The SDK
// supports optional user identification with custom traits and aliases for
// identity resolution.
//
// Identity is persisted to SQLite so it survives app restarts. Reset clears
// the user identity (soft reset for logout) while keeping the device ID intact.
package identity

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/SebastienMelki/causality/sdk/mobile/internal/storage"
)

// userIdentityKey is the key used to store user identity in the device_info table.
const userIdentityKey = "user_identity"

// UserIdentity represents the current user's identification data.
type UserIdentity struct {
	// UserID is the primary user identifier set by the app (e.g., "user-123").
	UserID string `json:"user_id"`

	// Traits are custom user properties (e.g., name, email, plan).
	Traits map[string]interface{} `json:"traits,omitempty"`

	// Aliases are alternative identifiers for identity resolution.
	Aliases []string `json:"aliases,omitempty"`
}

// IdentityManager manages user identity with thread-safe access and SQLite persistence.
// It is safe for concurrent use by multiple goroutines.
type IdentityManager struct {
	mu      sync.RWMutex
	current *UserIdentity
	db      *storage.DB
}

// NewIdentityManager creates a new IdentityManager backed by the given database.
// Call LoadFromDB after creation to restore any persisted identity.
func NewIdentityManager(db *storage.DB) *IdentityManager {
	return &IdentityManager{
		db: db,
	}
}

// SetUser sets the current user identity, persisting it to the database.
// Passing an empty userID returns an error.
func (m *IdentityManager) SetUser(userID string, traits map[string]interface{}, aliases []string) error {
	if userID == "" {
		return fmt.Errorf("user ID must not be empty")
	}

	identity := &UserIdentity{
		UserID:  userID,
		Traits:  traits,
		Aliases: aliases,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.saveToDB(identity); err != nil {
		return fmt.Errorf("persist user identity: %w", err)
	}

	m.current = identity
	return nil
}

// GetUser returns a copy of the current user identity, or nil if no user is set.
func (m *IdentityManager) GetUser() *UserIdentity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return nil
	}

	// Return a copy to prevent external mutation.
	copy := &UserIdentity{
		UserID: m.current.UserID,
	}

	if m.current.Traits != nil {
		copy.Traits = make(map[string]interface{}, len(m.current.Traits))
		for k, v := range m.current.Traits {
			copy.Traits[k] = v
		}
	}

	if m.current.Aliases != nil {
		copy.Aliases = make([]string, len(m.current.Aliases))
		copySlice(copy.Aliases, m.current.Aliases)
	}

	return copy
}

// Reset clears the current user identity and removes it from the database.
// This is a soft reset: the device ID remains intact.
func (m *IdentityManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = nil
	// Best-effort delete from DB; ignore errors.
	_ = m.deleteFromDB()
}

// LoadFromDB restores any persisted user identity from the database.
// Call this on SDK initialization. Returns nil if no identity is persisted.
func (m *IdentityManager) LoadFromDB() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var value string
	err := m.db.QueryRow(
		"SELECT value FROM device_info WHERE key = ?",
		userIdentityKey,
	).Scan(&value)

	if err == sql.ErrNoRows {
		// No persisted identity; this is normal on first launch.
		return nil
	}
	if err != nil {
		return fmt.Errorf("load user identity: %w", err)
	}

	var identity UserIdentity
	if err := json.Unmarshal([]byte(value), &identity); err != nil {
		return fmt.Errorf("unmarshal user identity: %w", err)
	}

	m.current = &identity
	return nil
}

// saveToDB persists the user identity as JSON in the device_info table.
// Must be called with mu held.
func (m *IdentityManager) saveToDB(identity *UserIdentity) error {
	data, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("marshal user identity: %w", err)
	}

	_, err = m.db.Exec(
		"INSERT OR REPLACE INTO device_info (key, value) VALUES (?, ?)",
		userIdentityKey, string(data),
	)
	if err != nil {
		return fmt.Errorf("save user identity: %w", err)
	}
	return nil
}

// deleteFromDB removes the persisted user identity from the device_info table.
// Must be called with mu held.
func (m *IdentityManager) deleteFromDB() error {
	_, err := m.db.Exec(
		"DELETE FROM device_info WHERE key = ?",
		userIdentityKey,
	)
	if err != nil {
		return fmt.Errorf("delete user identity: %w", err)
	}
	return nil
}

// copySlice copies src into dst. dst must be pre-allocated to len(src).
func copySlice(dst, src []string) {
	for i, s := range src {
		dst[i] = s
	}
}
