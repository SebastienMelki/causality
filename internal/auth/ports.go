// Package auth provides API key authentication for the Causality platform.
// It follows the hexagonal architecture pattern with ports (interfaces) and
// adapters (HTTP middleware, PostgreSQL repository).
package auth

import (
	"context"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
)

// KeyStore defines the port for API key persistence operations.
type KeyStore interface {
	// FindByHash retrieves an active (non-revoked) API key by its SHA256 hash.
	FindByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)

	// Create persists a new API key.
	Create(ctx context.Context, key *domain.APIKey) error

	// Revoke marks an API key as revoked.
	Revoke(ctx context.Context, id string) error

	// ListByAppID returns all API keys for a given app, ordered by creation date descending.
	ListByAppID(ctx context.Context, appID string) ([]domain.APIKey, error)
}

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

// AppIDContextKey is the context key used to inject the authenticated app_id
// into the request context after successful API key validation.
const AppIDContextKey contextKey = "app_id"
