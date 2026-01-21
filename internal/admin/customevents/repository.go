// Package customevents provides the admin custom event types handler.
package customevents

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// CustomEventType represents a custom event type from the database.
type CustomEventType struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Schema      json.RawMessage `json:"schema"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Repository provides access to custom event type storage.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns all custom event types.
func (r *Repository) List(ctx context.Context) ([]CustomEventType, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, category, schema, created_at, updated_at
		FROM custom_event_types ORDER BY category, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []CustomEventType
	for rows.Next() {
		var t CustomEventType
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Schema, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

// Get returns a custom event type by ID.
func (r *Repository) Get(ctx context.Context, id string) (CustomEventType, error) {
	var t CustomEventType
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, category, schema, created_at, updated_at
		FROM custom_event_types WHERE id = $1
	`, id).Scan(&t.ID, &t.Name, &t.Description, &t.Category, &t.Schema, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// Create creates a new custom event type.
func (r *Repository) Create(ctx context.Context, t CustomEventType) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO custom_event_types (name, description, category, schema)
		VALUES ($1, $2, $3, $4)
	`, t.Name, t.Description, t.Category, t.Schema)
	return err
}

// Update updates a custom event type.
func (r *Repository) Update(ctx context.Context, t CustomEventType) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE custom_event_types SET
			name = $1, description = $2, category = $3, schema = $4
		WHERE id = $5
	`, t.Name, t.Description, t.Category, t.Schema, t.ID)
	return err
}

// Delete deletes a custom event type.
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM custom_event_types WHERE id = $1", id)
	return err
}
