package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for anomaly configs.
var (
	ErrAnomalyConfigNotFound = errors.New("anomaly config not found")
	ErrAnomalyStateNotFound  = errors.New("anomaly state not found")
)

// DetectionType represents the type of anomaly detection.
type DetectionType string

const (
	DetectionTypeThreshold DetectionType = "threshold"
	DetectionTypeRate      DetectionType = "rate"
	DetectionTypeCount     DetectionType = "count"
)

// AnomalyConfig represents an anomaly detection configuration.
type AnomalyConfig struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	AppID           *string         `json:"app_id,omitempty"`
	EventCategory   *string         `json:"event_category,omitempty"`
	EventType       *string         `json:"event_type,omitempty"`
	DetectionType   DetectionType   `json:"detection_type"`
	Config          json.RawMessage `json:"config"`
	CooldownSeconds int             `json:"cooldown_seconds"`
	Enabled         bool            `json:"enabled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// AnomalyEvent represents a detected anomaly.
type AnomalyEvent struct {
	ID              string          `json:"id"`
	AnomalyConfigID string          `json:"anomaly_config_id"`
	AppID           *string         `json:"app_id,omitempty"`
	EventCategory   *string         `json:"event_category,omitempty"`
	EventType       *string         `json:"event_type,omitempty"`
	DetectionType   string          `json:"detection_type"`
	Details         json.RawMessage `json:"details"`
	EventData       json.RawMessage `json:"event_data,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// AnomalyState represents the sliding window state for rate/count detection.
type AnomalyState struct {
	ID              string     `json:"id"`
	AnomalyConfigID string     `json:"anomaly_config_id"`
	AppID           string     `json:"app_id"`
	WindowKey       string     `json:"window_key"`
	EventCount      int        `json:"event_count"`
	LastAlertAt     *time.Time `json:"last_alert_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AnomalyConfigRepository provides CRUD operations for anomaly configs.
type AnomalyConfigRepository struct {
	db *sql.DB
}

// NewAnomalyConfigRepository creates a new anomaly config repository.
func NewAnomalyConfigRepository(client *Client) *AnomalyConfigRepository {
	return &AnomalyConfigRepository{db: client.DB()}
}

// Create creates a new anomaly config.
func (r *AnomalyConfigRepository) Create(ctx context.Context, config *AnomalyConfig) error {
	query := `
		INSERT INTO anomaly_configs (name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRowContext(
		ctx, query,
		config.Name,
		config.Description,
		config.AppID,
		config.EventCategory,
		config.EventType,
		config.DetectionType,
		config.Config,
		config.CooldownSeconds,
		config.Enabled,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
}

// GetByID retrieves an anomaly config by ID.
func (r *AnomalyConfigRepository) GetByID(ctx context.Context, id string) (*AnomalyConfig, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs
		WHERE id = $1
	`

	config := &AnomalyConfig{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&config.ID,
		&config.Name,
		&config.Description,
		&config.AppID,
		&config.EventCategory,
		&config.EventType,
		&config.DetectionType,
		&config.Config,
		&config.CooldownSeconds,
		&config.Enabled,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAnomalyConfigNotFound
		}
		return nil, err
	}

	return config, nil
}

// GetEnabled retrieves all enabled anomaly configs.
func (r *AnomalyConfigRepository) GetEnabled(ctx context.Context) ([]*AnomalyConfig, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs
		WHERE enabled = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanConfigs(rows)
}

// GetMatchingConfigs retrieves enabled anomaly configs that could match the given app_id, category, and type.
func (r *AnomalyConfigRepository) GetMatchingConfigs(ctx context.Context, appID, category, eventType string) ([]*AnomalyConfig, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs
		WHERE enabled = true
		  AND (app_id IS NULL OR app_id = $1)
		  AND (event_category IS NULL OR event_category = $2)
		  AND (event_type IS NULL OR event_type = $3)
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, appID, category, eventType)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanConfigs(rows)
}

// scanConfigs scans multiple configs from rows.
func (r *AnomalyConfigRepository) scanConfigs(rows *sql.Rows) ([]*AnomalyConfig, error) {
	var configs []*AnomalyConfig
	for rows.Next() {
		config := &AnomalyConfig{}
		if err := rows.Scan(
			&config.ID,
			&config.Name,
			&config.Description,
			&config.AppID,
			&config.EventCategory,
			&config.EventType,
			&config.DetectionType,
			&config.Config,
			&config.CooldownSeconds,
			&config.Enabled,
			&config.CreatedAt,
			&config.UpdatedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// Update updates an anomaly config.
func (r *AnomalyConfigRepository) Update(ctx context.Context, config *AnomalyConfig) error {
	query := `
		UPDATE anomaly_configs
		SET name = $1, description = $2, app_id = $3, event_category = $4, event_type = $5,
		    detection_type = $6, config = $7, cooldown_seconds = $8, enabled = $9
		WHERE id = $10
	`

	result, err := r.db.ExecContext(
		ctx, query,
		config.Name,
		config.Description,
		config.AppID,
		config.EventCategory,
		config.EventType,
		config.DetectionType,
		config.Config,
		config.CooldownSeconds,
		config.Enabled,
		config.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAnomalyConfigNotFound
	}

	return nil
}

// Delete deletes an anomaly config by ID.
func (r *AnomalyConfigRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM anomaly_configs WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrAnomalyConfigNotFound
	}

	return nil
}

// List retrieves all anomaly configs with pagination.
func (r *AnomalyConfigRepository) List(ctx context.Context, limit, offset int) ([]*AnomalyConfig, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanConfigs(rows)
}

// RecordAnomalyEvent records a detected anomaly event.
func (r *AnomalyConfigRepository) RecordAnomalyEvent(ctx context.Context, event *AnomalyEvent) error {
	query := `
		INSERT INTO anomaly_events (anomaly_config_id, app_id, event_category, event_type, detection_type, details, event_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	return r.db.QueryRowContext(
		ctx, query,
		event.AnomalyConfigID,
		event.AppID,
		event.EventCategory,
		event.EventType,
		event.DetectionType,
		event.Details,
		event.EventData,
	).Scan(&event.ID, &event.CreatedAt)
}

// GetAnomalyEvents retrieves anomaly events for a config with pagination.
func (r *AnomalyConfigRepository) GetAnomalyEvents(ctx context.Context, configID string, limit, offset int) ([]*AnomalyEvent, error) {
	query := `
		SELECT id, anomaly_config_id, app_id, event_category, event_type, detection_type, details, event_data, created_at
		FROM anomaly_events
		WHERE anomaly_config_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, configID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var events []*AnomalyEvent
	for rows.Next() {
		event := &AnomalyEvent{}
		if err := rows.Scan(
			&event.ID,
			&event.AnomalyConfigID,
			&event.AppID,
			&event.EventCategory,
			&event.EventType,
			&event.DetectionType,
			&event.Details,
			&event.EventData,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetOrCreateState gets or creates an anomaly state record.
func (r *AnomalyConfigRepository) GetOrCreateState(ctx context.Context, configID, appID, windowKey string) (*AnomalyState, error) {
	// Try to insert, on conflict return existing
	query := `
		INSERT INTO anomaly_state (anomaly_config_id, app_id, window_key, event_count)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (anomaly_config_id, app_id, window_key)
		DO UPDATE SET updated_at = NOW()
		RETURNING id, anomaly_config_id, app_id, window_key, event_count, last_alert_at, created_at, updated_at
	`

	state := &AnomalyState{}
	err := r.db.QueryRowContext(ctx, query, configID, appID, windowKey).Scan(
		&state.ID,
		&state.AnomalyConfigID,
		&state.AppID,
		&state.WindowKey,
		&state.EventCount,
		&state.LastAlertAt,
		&state.CreatedAt,
		&state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return state, nil
}

// IncrementStateCount increments the event count and returns the new count.
func (r *AnomalyConfigRepository) IncrementStateCount(ctx context.Context, configID, appID, windowKey string) (int, error) {
	query := `
		INSERT INTO anomaly_state (anomaly_config_id, app_id, window_key, event_count)
		VALUES ($1, $2, $3, 1)
		ON CONFLICT (anomaly_config_id, app_id, window_key)
		DO UPDATE SET event_count = anomaly_state.event_count + 1, updated_at = NOW()
		RETURNING event_count
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, configID, appID, windowKey).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateLastAlertAt updates the last alert time for a state.
func (r *AnomalyConfigRepository) UpdateLastAlertAt(ctx context.Context, configID, appID, windowKey string) error {
	query := `
		UPDATE anomaly_state
		SET last_alert_at = NOW()
		WHERE anomaly_config_id = $1 AND app_id = $2 AND window_key = $3
	`

	_, err := r.db.ExecContext(ctx, query, configID, appID, windowKey)
	return err
}

// GetLastAlertAt retrieves the last alert time for a config and app.
func (r *AnomalyConfigRepository) GetLastAlertAt(ctx context.Context, configID, appID string) (*time.Time, error) {
	query := `
		SELECT MAX(last_alert_at)
		FROM anomaly_state
		WHERE anomaly_config_id = $1 AND app_id = $2
	`

	var lastAlertAt *time.Time
	err := r.db.QueryRowContext(ctx, query, configID, appID).Scan(&lastAlertAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAnomalyStateNotFound
		}
		return nil, err
	}

	// Return nil with sentinel error if no alert time exists
	if lastAlertAt == nil {
		return nil, ErrAnomalyStateNotFound
	}

	return lastAlertAt, nil
}

// CleanupOldState deletes old state records.
func (r *AnomalyConfigRepository) CleanupOldState(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `
		DELETE FROM anomaly_state
		WHERE updated_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// CleanupOldEvents deletes old anomaly events.
func (r *AnomalyConfigRepository) CleanupOldEvents(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `
		DELETE FROM anomaly_events
		WHERE created_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
