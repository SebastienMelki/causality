package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for webhooks.
var (
	ErrWebhookNotFound = errors.New("webhook not found")
)

// Webhook represents a webhook endpoint configuration.
type Webhook struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	AuthType   string            `json:"auth_type"` // none, basic, bearer, hmac
	AuthConfig json.RawMessage   `json:"auth_config"`
	Headers    map[string]string `json:"headers"`
	Enabled    bool              `json:"enabled"`
	TimeoutMs  int               `json:"timeout_ms"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// WebhookRepository provides CRUD operations for webhooks.
type WebhookRepository struct {
	db *sql.DB
}

// NewWebhookRepository creates a new webhook repository.
func NewWebhookRepository(client *Client) *WebhookRepository {
	return &WebhookRepository{db: client.DB()}
}

// Create creates a new webhook.
func (r *WebhookRepository) Create(ctx context.Context, webhook *Webhook) error {
	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO webhooks (name, url, auth_type, auth_config, headers, enabled, timeout_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRowContext(
		ctx, query,
		webhook.Name,
		webhook.URL,
		webhook.AuthType,
		webhook.AuthConfig,
		headersJSON,
		webhook.Enabled,
		webhook.TimeoutMs,
	).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)
}

// GetByID retrieves a webhook by ID.
func (r *WebhookRepository) GetByID(ctx context.Context, id string) (*Webhook, error) {
	query := `
		SELECT id, name, url, auth_type, auth_config, headers, enabled, timeout_ms, created_at, updated_at
		FROM webhooks
		WHERE id = $1
	`

	webhook := &Webhook{}
	var headersJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&webhook.ID,
		&webhook.Name,
		&webhook.URL,
		&webhook.AuthType,
		&webhook.AuthConfig,
		&headersJSON,
		&webhook.Enabled,
		&webhook.TimeoutMs,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
		return nil, err
	}

	return webhook, nil
}

// GetEnabled retrieves all enabled webhooks.
func (r *WebhookRepository) GetEnabled(ctx context.Context) ([]*Webhook, error) {
	query := `
		SELECT id, name, url, auth_type, auth_config, headers, enabled, timeout_ms, created_at, updated_at
		FROM webhooks
		WHERE enabled = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var headersJSON []byte

		if err := rows.Scan(
			&webhook.ID,
			&webhook.Name,
			&webhook.URL,
			&webhook.AuthType,
			&webhook.AuthConfig,
			&headersJSON,
			&webhook.Enabled,
			&webhook.TimeoutMs,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, err
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, rows.Err()
}

// GetByIDs retrieves webhooks by their IDs.
func (r *WebhookRepository) GetByIDs(ctx context.Context, ids []string) ([]*Webhook, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, name, url, auth_type, auth_config, headers, enabled, timeout_ms, created_at, updated_at
		FROM webhooks
		WHERE id = ANY($1)
	`

	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var headersJSON []byte

		if err := rows.Scan(
			&webhook.ID,
			&webhook.Name,
			&webhook.URL,
			&webhook.AuthType,
			&webhook.AuthConfig,
			&headersJSON,
			&webhook.Enabled,
			&webhook.TimeoutMs,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, err
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, rows.Err()
}

// Update updates a webhook.
func (r *WebhookRepository) Update(ctx context.Context, webhook *Webhook) error {
	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		return err
	}

	query := `
		UPDATE webhooks
		SET name = $1, url = $2, auth_type = $3, auth_config = $4, headers = $5, enabled = $6, timeout_ms = $7
		WHERE id = $8
		RETURNING updated_at
	`

	result, err := r.db.ExecContext(
		ctx, query,
		webhook.Name,
		webhook.URL,
		webhook.AuthType,
		webhook.AuthConfig,
		headersJSON,
		webhook.Enabled,
		webhook.TimeoutMs,
		webhook.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrWebhookNotFound
	}

	return nil
}

// Delete deletes a webhook by ID.
func (r *WebhookRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM webhooks WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrWebhookNotFound
	}

	return nil
}

// List retrieves all webhooks with pagination.
func (r *WebhookRepository) List(ctx context.Context, limit, offset int) ([]*Webhook, error) {
	query := `
		SELECT id, name, url, auth_type, auth_config, headers, enabled, timeout_ms, created_at, updated_at
		FROM webhooks
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var headersJSON []byte

		if err := rows.Scan(
			&webhook.ID,
			&webhook.Name,
			&webhook.URL,
			&webhook.AuthType,
			&webhook.AuthConfig,
			&headersJSON,
			&webhook.Enabled,
			&webhook.TimeoutMs,
			&webhook.CreatedAt,
			&webhook.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(headersJSON, &webhook.Headers); err != nil {
			return nil, err
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, rows.Err()
}
