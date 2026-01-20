package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for deliveries.
var (
	ErrDeliveryNotFound = errors.New("delivery not found")
)

// DeliveryStatus represents the status of a webhook delivery.
type DeliveryStatus string

const (
	DeliveryStatusPending    DeliveryStatus = "pending"
	DeliveryStatusInProgress DeliveryStatus = "in_progress"
	DeliveryStatusDelivered  DeliveryStatus = "delivered"
	DeliveryStatusFailed     DeliveryStatus = "failed"
	DeliveryStatusDeadLetter DeliveryStatus = "dead_letter"
)

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID              string          `json:"id"`
	WebhookID       string          `json:"webhook_id"`
	RuleID          *string         `json:"rule_id,omitempty"`
	AnomalyConfigID *string         `json:"anomaly_config_id,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	Status          DeliveryStatus  `json:"status"`
	Attempts        int             `json:"attempts"`
	MaxAttempts     int             `json:"max_attempts"`
	NextAttemptAt   time.Time       `json:"next_attempt_at"`
	LastAttemptAt   *time.Time      `json:"last_attempt_at,omitempty"`
	LastError       *string         `json:"last_error,omitempty"`
	LastStatusCode  *int            `json:"last_status_code,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	DeliveredAt     *time.Time      `json:"delivered_at,omitempty"`
}

// DeliveryRepository provides CRUD operations for webhook deliveries.
type DeliveryRepository struct {
	db *sql.DB
}

// NewDeliveryRepository creates a new delivery repository.
func NewDeliveryRepository(client *Client) *DeliveryRepository {
	return &DeliveryRepository{db: client.DB()}
}

// Create creates a new delivery.
func (r *DeliveryRepository) Create(ctx context.Context, delivery *WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (webhook_id, rule_id, anomaly_config_id, payload, status, max_attempts, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	return r.db.QueryRowContext(
		ctx, query,
		delivery.WebhookID,
		delivery.RuleID,
		delivery.AnomalyConfigID,
		delivery.Payload,
		delivery.Status,
		delivery.MaxAttempts,
		delivery.NextAttemptAt,
	).Scan(&delivery.ID, &delivery.CreatedAt)
}

// CreateBatch creates multiple deliveries in a single transaction.
func (r *DeliveryRepository) CreateBatch(ctx context.Context, deliveries []*WebhookDelivery) error {
	if len(deliveries) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO webhook_deliveries (webhook_id, rule_id, anomaly_config_id, payload, status, max_attempts, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, delivery := range deliveries {
		err := stmt.QueryRowContext(
			ctx,
			delivery.WebhookID,
			delivery.RuleID,
			delivery.AnomalyConfigID,
			delivery.Payload,
			delivery.Status,
			delivery.MaxAttempts,
			delivery.NextAttemptAt,
		).Scan(&delivery.ID, &delivery.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPending retrieves pending deliveries ready for processing.
func (r *DeliveryRepository) GetPending(ctx context.Context, limit int) ([]*WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, rule_id, anomaly_config_id, payload, status, attempts, max_attempts,
		       next_attempt_at, last_attempt_at, last_error, last_status_code, created_at, delivered_at
		FROM webhook_deliveries
		WHERE status IN ('pending', 'in_progress')
		  AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanDeliveries(rows)
}

// GetByID retrieves a delivery by ID.
func (r *DeliveryRepository) GetByID(ctx context.Context, id string) (*WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, rule_id, anomaly_config_id, payload, status, attempts, max_attempts,
		       next_attempt_at, last_attempt_at, last_error, last_status_code, created_at, delivered_at
		FROM webhook_deliveries
		WHERE id = $1
	`

	delivery := &WebhookDelivery{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&delivery.ID,
		&delivery.WebhookID,
		&delivery.RuleID,
		&delivery.AnomalyConfigID,
		&delivery.Payload,
		&delivery.Status,
		&delivery.Attempts,
		&delivery.MaxAttempts,
		&delivery.NextAttemptAt,
		&delivery.LastAttemptAt,
		&delivery.LastError,
		&delivery.LastStatusCode,
		&delivery.CreatedAt,
		&delivery.DeliveredAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}

	return delivery, nil
}

// scanDeliveries scans multiple deliveries from rows.
func (r *DeliveryRepository) scanDeliveries(rows *sql.Rows) ([]*WebhookDelivery, error) {
	var deliveries []*WebhookDelivery
	for rows.Next() {
		delivery := &WebhookDelivery{}
		if err := rows.Scan(
			&delivery.ID,
			&delivery.WebhookID,
			&delivery.RuleID,
			&delivery.AnomalyConfigID,
			&delivery.Payload,
			&delivery.Status,
			&delivery.Attempts,
			&delivery.MaxAttempts,
			&delivery.NextAttemptAt,
			&delivery.LastAttemptAt,
			&delivery.LastError,
			&delivery.LastStatusCode,
			&delivery.CreatedAt,
			&delivery.DeliveredAt,
		); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}

	return deliveries, rows.Err()
}

// MarkInProgress marks a delivery as in progress.
func (r *DeliveryRepository) MarkInProgress(ctx context.Context, id string) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'in_progress', last_attempt_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// MarkDelivered marks a delivery as successfully delivered.
func (r *DeliveryRepository) MarkDelivered(ctx context.Context, id string, statusCode int) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'delivered', delivered_at = NOW(), last_status_code = $2, attempts = attempts + 1
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, statusCode)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// MarkFailed marks a delivery attempt as failed with retry scheduling.
func (r *DeliveryRepository) MarkFailed(ctx context.Context, id string, statusCode *int, errMsg string, nextAttemptAt time.Time) error {
	query := `
		UPDATE webhook_deliveries
		SET status = CASE
		               WHEN attempts + 1 >= max_attempts THEN 'dead_letter'
		               ELSE 'pending'
		             END,
		    attempts = attempts + 1,
		    last_error = $2,
		    last_status_code = $3,
		    next_attempt_at = $4
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, errMsg, statusCode, nextAttemptAt)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// GetDeadLettered retrieves dead-lettered deliveries for review.
func (r *DeliveryRepository) GetDeadLettered(ctx context.Context, limit, offset int) ([]*WebhookDelivery, error) {
	query := `
		SELECT id, webhook_id, rule_id, anomaly_config_id, payload, status, attempts, max_attempts,
		       next_attempt_at, last_attempt_at, last_error, last_status_code, created_at, delivered_at
		FROM webhook_deliveries
		WHERE status = 'dead_letter'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanDeliveries(rows)
}

// Retry resets a dead-lettered delivery for retry.
func (r *DeliveryRepository) Retry(ctx context.Context, id string) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'pending', next_attempt_at = NOW()
		WHERE id = $1 AND status = 'dead_letter'
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDeliveryNotFound
	}

	return nil
}

// DeleteOld deletes old delivered/dead-lettered deliveries.
func (r *DeliveryRepository) DeleteOld(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `
		DELETE FROM webhook_deliveries
		WHERE status IN ('delivered', 'dead_letter')
		  AND created_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetStats retrieves delivery statistics.
func (r *DeliveryRepository) GetStats(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM webhook_deliveries
		GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	stats := make(map[string]int64)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}

	return stats, rows.Err()
}
