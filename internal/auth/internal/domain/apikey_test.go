// Package domain tests the API key generation, hashing, and validation logic.
package domain

import (
	"regexp"
	"sync"
	"testing"
)

// hexPattern matches exactly 64 lowercase hex characters.
var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestGenerateKey_ReturnsValidKeyAndHash(t *testing.T) {
	plaintext, hash, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() returned unexpected error: %v", err)
	}

	// Key should be 64 hex characters (32 bytes encoded)
	if !hexPattern.MatchString(plaintext) {
		t.Errorf("plaintext key is not 64 hex chars: got %q (len=%d)", plaintext, len(plaintext))
	}

	// Hash should be 64 hex characters (SHA256)
	if !hexPattern.MatchString(hash) {
		t.Errorf("hash is not 64 hex chars: got %q (len=%d)", hash, len(hash))
	}

	// Key and hash must be different
	if plaintext == hash {
		t.Error("plaintext key and hash should be different")
	}
}

func TestGenerateKey_UniquenessAcrossCalls(t *testing.T) {
	key1, hash1, err1 := GenerateKey()
	if err1 != nil {
		t.Fatalf("First GenerateKey() call failed: %v", err1)
	}

	key2, hash2, err2 := GenerateKey()
	if err2 != nil {
		t.Fatalf("Second GenerateKey() call failed: %v", err2)
	}

	if key1 == key2 {
		t.Error("two generated keys should be different")
	}
	if hash1 == hash2 {
		t.Error("two generated hashes should be different")
	}
}

func TestHashKey_Deterministic(t *testing.T) {
	plaintext := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	hash1 := HashKey(plaintext)
	hash2 := HashKey(plaintext)

	if hash1 != hash2 {
		t.Errorf("HashKey is not deterministic: got %q and %q", hash1, hash2)
	}

	// Verify hash is valid 64-char hex
	if !hexPattern.MatchString(hash1) {
		t.Errorf("hash is not 64 hex chars: got %q", hash1)
	}
}

func TestHashKey_DifferentInputsDifferentHashes(t *testing.T) {
	key1 := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	key2 := "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"

	hash1 := HashKey(key1)
	hash2 := HashKey(key2)

	if hash1 == hash2 {
		t.Error("different keys should produce different hashes")
	}
}

func TestValidateKeyFormat_ValidKey(t *testing.T) {
	validKeys := []string{
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, key := range validKeys {
		if !ValidateKeyFormat(key) {
			t.Errorf("ValidateKeyFormat(%q) = false, want true", key)
		}
	}
}

func TestValidateKeyFormat_InvalidKey(t *testing.T) {
	invalidKeys := []struct {
		name string
		key  string
	}{
		{"too short", "a1b2c3"},
		{"too long", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2extra"},
		{"uppercase hex", "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2"},
		{"non-hex chars", "g1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"},
		{"special chars", "a1b2c3d4e5f6!@#$a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6"},
		{"empty string", ""},
		{"whitespace", " "},
		{"63 chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b"},
		{"65 chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c"},
	}

	for _, tc := range invalidKeys {
		t.Run(tc.name, func(t *testing.T) {
			if ValidateKeyFormat(tc.key) {
				t.Errorf("ValidateKeyFormat(%q) = true, want false", tc.key)
			}
		})
	}
}

func TestGenerateKey_ConcurrentSafety(t *testing.T) {
	const goroutines = 100

	var wg sync.WaitGroup
	keys := make(chan string, goroutines)
	errs := make(chan error, goroutines)

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			key, _, err := GenerateKey()
			if err != nil {
				errs <- err
				return
			}
			keys <- key
		}()
	}

	wg.Wait()
	close(keys)
	close(errs)

	// Check for errors
	for err := range errs {
		t.Errorf("concurrent GenerateKey() failed: %v", err)
	}

	// Check uniqueness
	seen := make(map[string]bool)
	for key := range keys {
		if seen[key] {
			t.Error("concurrent GenerateKey() produced duplicate key")
		}
		seen[key] = true
	}
}
