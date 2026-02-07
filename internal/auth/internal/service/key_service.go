// Package service contains the business logic for API key operations.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
)

// KeyStore defines the port for API key persistence. This mirrors the
// top-level auth.KeyStore interface to avoid import cycles.
type KeyStore interface {
	FindByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	Create(ctx context.Context, key *domain.APIKey) error
	Revoke(ctx context.Context, id string) error
	ListByAppID(ctx context.Context, appID string) ([]domain.APIKey, error)
}

// Common errors returned by KeyService methods.
var (
	ErrKeyNotFound = errors.New("api key not found or revoked")
	ErrInvalidKey  = errors.New("invalid api key format")
	ErrEmptyAppID  = errors.New("app_id is required")
)

// KeyService provides business logic for API key management including
// creation, validation, revocation, and listing.
type KeyService struct {
	store  KeyStore
	logger *slog.Logger
}

// NewKeyService creates a new KeyService with the given store and logger.
func NewKeyService(store KeyStore, logger *slog.Logger) *KeyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &KeyService{
		store:  store,
		logger: logger.With("component", "key-service"),
	}
}

// ValidateKey validates an API key by its SHA256 hash. It returns the
// associated APIKey if found and not revoked, or nil if invalid.
func (s *KeyService) ValidateKey(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	key, err := s.store.FindByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find key: %w", err)
	}

	if key == nil {
		return nil, nil
	}

	if key.Revoked {
		s.logger.Warn("attempt to use revoked key",
			"key_id", key.ID,
			"app_id", key.AppID,
		)
		return nil, nil
	}

	return key, nil
}

// CreateKey generates a new API key for the given app. It returns the
// plaintext key (to be shown once to the user) and the persisted APIKey record.
func (s *KeyService) CreateKey(ctx context.Context, appID, name string) (plaintext string, key *domain.APIKey, err error) {
	if appID == "" {
		return "", nil, ErrEmptyAppID
	}

	plaintext, hash, err := domain.GenerateKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	key = &domain.APIKey{
		ID:      uuid.Must(uuid.NewV7()).String(),
		AppID:   appID,
		KeyHash: hash,
		Name:    name,
	}

	if err := s.store.Create(ctx, key); err != nil {
		return "", nil, fmt.Errorf("failed to store key: %w", err)
	}

	s.logger.Info("api key created",
		"key_id", key.ID,
		"app_id", appID,
		"name", name,
	)

	return plaintext, key, nil
}

// RevokeKey revokes an API key by its ID.
func (s *KeyService) RevokeKey(ctx context.Context, id string) error {
	if err := s.store.Revoke(ctx, id); err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	s.logger.Info("api key revoked", "key_id", id)
	return nil
}

// ListKeys returns all API keys for the given app ID.
func (s *KeyService) ListKeys(ctx context.Context, appID string) ([]domain.APIKey, error) {
	if appID == "" {
		return nil, ErrEmptyAppID
	}

	keys, err := s.store.ListByAppID(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return keys, nil
}
