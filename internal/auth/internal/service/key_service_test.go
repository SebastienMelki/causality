// Package service tests the API key service business logic.
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
)

// mockKeyStore is a test double for KeyStore.
type mockKeyStore struct {
	keys       map[string]*domain.APIKey // keyed by hash
	createErr  error
	revokeErr  error
	findErr    error
	listErr    error
	createCalls int
	revokeCalls int
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{
		keys: make(map[string]*domain.APIKey),
	}
}

func (m *mockKeyStore) FindByHash(_ context.Context, keyHash string) (*domain.APIKey, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.keys[keyHash], nil
}

func (m *mockKeyStore) Create(_ context.Context, key *domain.APIKey) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	m.keys[key.KeyHash] = key
	return nil
}

func (m *mockKeyStore) Revoke(_ context.Context, id string) error {
	m.revokeCalls++
	if m.revokeErr != nil {
		return m.revokeErr
	}
	// Mark key as revoked
	for _, key := range m.keys {
		if key.ID == id {
			key.Revoked = true
			now := time.Now()
			key.RevokedAt = &now
			break
		}
	}
	return nil
}

func (m *mockKeyStore) ListByAppID(_ context.Context, appID string) ([]domain.APIKey, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []domain.APIKey
	for _, key := range m.keys {
		if key.AppID == appID {
			result = append(result, *key)
		}
	}
	return result, nil
}

func TestValidateKey_ValidKey(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	// Add a valid key to the store
	hash := "abc123def456abc123def456abc123def456abc123def456abc123def456abc1"
	store.keys[hash] = &domain.APIKey{
		ID:      "key-1",
		AppID:   "app-1",
		KeyHash: hash,
		Name:    "Test Key",
		Revoked: false,
	}

	key, err := svc.ValidateKey(context.Background(), hash)
	if err != nil {
		t.Fatalf("ValidateKey() returned unexpected error: %v", err)
	}
	if key == nil {
		t.Fatal("ValidateKey() returned nil for valid key")
	}
	if key.AppID != "app-1" {
		t.Errorf("ValidateKey() returned wrong app_id: got %q, want %q", key.AppID, "app-1")
	}
}

func TestValidateKey_RevokedKey(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	// Add a revoked key to the store
	hash := "revoked123revoked123revoked123revoked123revoked123revoked1234"
	now := time.Now()
	store.keys[hash] = &domain.APIKey{
		ID:        "key-revoked",
		AppID:     "app-1",
		KeyHash:   hash,
		Name:      "Revoked Key",
		Revoked:   true,
		RevokedAt: &now,
	}

	key, err := svc.ValidateKey(context.Background(), hash)
	if err != nil {
		t.Fatalf("ValidateKey() returned unexpected error: %v", err)
	}
	if key != nil {
		t.Error("ValidateKey() should return nil for revoked key")
	}
}

func TestValidateKey_UnknownHash(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	key, err := svc.ValidateKey(context.Background(), "unknown12345678901234567890123456789012345678901234567890123456")
	if err != nil {
		t.Fatalf("ValidateKey() returned unexpected error: %v", err)
	}
	if key != nil {
		t.Error("ValidateKey() should return nil for unknown hash")
	}
}

func TestValidateKey_StoreError(t *testing.T) {
	store := newMockKeyStore()
	store.findErr = errors.New("database connection failed")
	svc := NewKeyService(store, nil)

	_, err := svc.ValidateKey(context.Background(), "somehash12345678901234567890123456789012345678901234567890")
	if err == nil {
		t.Error("ValidateKey() should return error when store fails")
	}
}

func TestCreateKey_Success(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	plaintext, key, err := svc.CreateKey(context.Background(), "app-1", "Production Key")
	if err != nil {
		t.Fatalf("CreateKey() returned unexpected error: %v", err)
	}

	// Verify plaintext is returned
	if plaintext == "" {
		t.Error("CreateKey() should return non-empty plaintext")
	}
	if len(plaintext) != 64 {
		t.Errorf("plaintext should be 64 chars, got %d", len(plaintext))
	}

	// Verify key is returned
	if key == nil {
		t.Fatal("CreateKey() should return non-nil key")
	}
	if key.AppID != "app-1" {
		t.Errorf("key.AppID = %q, want %q", key.AppID, "app-1")
	}
	if key.Name != "Production Key" {
		t.Errorf("key.Name = %q, want %q", key.Name, "Production Key")
	}
	if key.ID == "" {
		t.Error("key.ID should be generated")
	}

	// Verify store.Create was called
	if store.createCalls != 1 {
		t.Errorf("store.Create() called %d times, want 1", store.createCalls)
	}
}

func TestCreateKey_EmptyAppID(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	_, _, err := svc.CreateKey(context.Background(), "", "Some Key")
	if err == nil {
		t.Error("CreateKey() should return error for empty app_id")
	}
	if !errors.Is(err, ErrEmptyAppID) {
		t.Errorf("CreateKey() error = %v, want ErrEmptyAppID", err)
	}
}

func TestCreateKey_StoreError(t *testing.T) {
	store := newMockKeyStore()
	store.createErr = errors.New("failed to insert")
	svc := NewKeyService(store, nil)

	_, _, err := svc.CreateKey(context.Background(), "app-1", "Test Key")
	if err == nil {
		t.Error("CreateKey() should return error when store fails")
	}
}

func TestRevokeKey_Success(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	// Add a key first
	hash := "revoketest123revoketest123revoketest123revoketest123revoketest1"
	store.keys[hash] = &domain.APIKey{
		ID:      "key-to-revoke",
		AppID:   "app-1",
		KeyHash: hash,
		Name:    "Key to Revoke",
		Revoked: false,
	}

	err := svc.RevokeKey(context.Background(), "key-to-revoke")
	if err != nil {
		t.Fatalf("RevokeKey() returned unexpected error: %v", err)
	}

	if store.revokeCalls != 1 {
		t.Errorf("store.Revoke() called %d times, want 1", store.revokeCalls)
	}
}

func TestRevokeKey_StoreError(t *testing.T) {
	store := newMockKeyStore()
	store.revokeErr = errors.New("revoke failed")
	svc := NewKeyService(store, nil)

	err := svc.RevokeKey(context.Background(), "key-123")
	if err == nil {
		t.Error("RevokeKey() should return error when store fails")
	}
}

func TestListKeys_ByAppID(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	// Add keys for different apps
	store.keys["hash1"] = &domain.APIKey{
		ID:      "key-1",
		AppID:   "app-1",
		KeyHash: "hash1",
		Name:    "Key 1",
	}
	store.keys["hash2"] = &domain.APIKey{
		ID:      "key-2",
		AppID:   "app-1",
		KeyHash: "hash2",
		Name:    "Key 2",
	}
	store.keys["hash3"] = &domain.APIKey{
		ID:      "key-3",
		AppID:   "app-2", // different app
		KeyHash: "hash3",
		Name:    "Key 3",
	}

	keys, err := svc.ListKeys(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("ListKeys() returned unexpected error: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("ListKeys() returned %d keys, want 2", len(keys))
	}

	// Verify all returned keys belong to app-1
	for _, key := range keys {
		if key.AppID != "app-1" {
			t.Errorf("ListKeys() returned key with wrong app_id: %q", key.AppID)
		}
	}
}

func TestListKeys_EmptyAppID(t *testing.T) {
	store := newMockKeyStore()
	svc := NewKeyService(store, nil)

	_, err := svc.ListKeys(context.Background(), "")
	if err == nil {
		t.Error("ListKeys() should return error for empty app_id")
	}
	if !errors.Is(err, ErrEmptyAppID) {
		t.Errorf("ListKeys() error = %v, want ErrEmptyAppID", err)
	}
}

func TestListKeys_StoreError(t *testing.T) {
	store := newMockKeyStore()
	store.listErr = errors.New("list failed")
	svc := NewKeyService(store, nil)

	_, err := svc.ListKeys(context.Background(), "app-1")
	if err == nil {
		t.Error("ListKeys() should return error when store fails")
	}
}
