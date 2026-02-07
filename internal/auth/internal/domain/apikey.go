// Package domain contains the core domain types and business logic for
// API key authentication.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"
)

// APIKey represents an API key in the system. The plaintext key is never
// stored; only its SHA256 hash is persisted.
type APIKey struct {
	// ID is the unique identifier (UUID) for this key record.
	ID string

	// AppID is the application this key belongs to.
	AppID string

	// KeyHash is the SHA256 hex-encoded hash of the plaintext API key.
	KeyHash string

	// Name is a human-readable label for the key (e.g., "Production iOS").
	Name string

	// Revoked indicates whether this key has been revoked.
	Revoked bool

	// CreatedAt is when the key was created.
	CreatedAt time.Time

	// RevokedAt is when the key was revoked, nil if still active.
	RevokedAt *time.Time
}

// hexKeyRegex matches exactly 64 lowercase hexadecimal characters.
var hexKeyRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// GenerateKey creates a new random API key. It returns the plaintext key
// (64-char hex string from 32 random bytes) and its SHA256 hash. The plaintext
// must be shown to the user once and never stored.
func GenerateKey() (plaintext string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	plaintext = hex.EncodeToString(b)
	hash = HashKey(plaintext)

	return plaintext, hash, nil
}

// HashKey computes the SHA256 hash of a plaintext API key and returns it
// as a lowercase hex-encoded string. SHA256 is used instead of bcrypt because
// API keys are high-entropy random strings (256 bits), making brute-force
// attacks infeasible, and SHA256 enables fast constant-time lookup.
func HashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ValidateKeyFormat checks whether a key string is a valid 64-character
// lowercase hex string (the expected format for generated API keys).
func ValidateKeyFormat(key string) bool {
	return hexKeyRegex.MatchString(key)
}
