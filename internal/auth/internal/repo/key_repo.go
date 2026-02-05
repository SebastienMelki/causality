// Package repo provides the PostgreSQL implementation of the KeyStore port.
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
)

// KeyRepository implements the KeyStore interface using PostgreSQL.
type KeyRepository struct {
	db *sql.DB
}

// NewKeyRepository creates a new KeyRepository backed by the given database.
func NewKeyRepository(db *sql.DB) *KeyRepository {
	return &KeyRepository{db: db}
}

// FindByHash retrieves an active (non-revoked) API key by its SHA256 hash.
// Returns nil, nil if no matching key is found.
func (r *KeyRepository) FindByHash(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	query := `
		SELECT id, app_id, key_hash, name, revoked, created_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1 AND NOT revoked
	`

	var key domain.APIKey
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID,
		&key.AppID,
		&key.KeyHash,
		&key.Name,
		&key.Revoked,
		&key.CreatedAt,
		&key.RevokedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query api key by hash: %w", err)
	}

	return &key, nil
}

// Create inserts a new API key record into the database.
func (r *KeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	query := `
		INSERT INTO api_keys (id, app_id, key_hash, name)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, key.ID, key.AppID, key.KeyHash, key.Name)
	if err != nil {
		return fmt.Errorf("failed to insert api key: %w", err)
	}

	return nil
}

// Revoke marks an API key as revoked by setting revoked=true and revoked_at=now().
func (r *KeyRepository) Revoke(ctx context.Context, id string) error {
	query := `
		UPDATE api_keys SET revoked = true, revoked_at = now()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke api key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}

	return nil
}

// ListByAppID returns all API keys for the given app, ordered by creation date descending.
func (r *KeyRepository) ListByAppID(ctx context.Context, appID string) ([]domain.APIKey, error) {
	query := `
		SELECT id, app_id, key_hash, name, revoked, created_at, revoked_at
		FROM api_keys
		WHERE app_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query api keys by app_id: %w", err)
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var key domain.APIKey
		if err := rows.Scan(
			&key.ID,
			&key.AppID,
			&key.KeyHash,
			&key.Name,
			&key.Revoked,
			&key.CreatedAt,
			&key.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan api key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating api keys: %w", err)
	}

	return keys, nil
}
