// Package events provides the admin events browser handler.
package events

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/trinodb/trino-go-client/trino"
)

// TrinoConfig holds Trino configuration.
type TrinoConfig struct {
	Host    string
	Port    int
	Catalog string
	Schema  string
	User    string
}

// TrinoClient handles Trino queries.
type TrinoClient struct {
	db      *sql.DB
	catalog string
	schema  string
}

// NewTrinoClient creates a new Trino client.
func NewTrinoClient(cfg TrinoConfig) (*TrinoClient, error) {
	dsn := fmt.Sprintf("http://%s@%s:%d?catalog=%s&schema=%s",
		cfg.User, cfg.Host, cfg.Port, cfg.Catalog, cfg.Schema)

	db, err := sql.Open("trino", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Trino: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping Trino: %w", err)
	}

	return &TrinoClient{
		db:      db,
		catalog: cfg.Catalog,
		schema:  cfg.Schema,
	}, nil
}

// Close closes the Trino connection.
func (c *TrinoClient) Close() error {
	return c.db.Close()
}

// Event represents an event from Trino.
type Event struct {
	ID            string
	AppID         string
	EventType     string
	EventCategory string
	DeviceID      string
	Platform      string
	Timestamp     time.Time
	Parameters    string
}

// EventFilter holds query parameters for filtering events.
type EventFilter struct {
	AppID         string
	EventCategory string
	EventType     string
	StartDate     string
	EndDate       string
	Limit         int
	Offset        int
}

// QueryResult holds the result of an event query.
type QueryResult struct {
	Events     []Event
	TotalCount int
	HasMore    bool
}

// QueryEvents queries events from Trino with the given filter.
func (c *TrinoClient) QueryEvents(ctx context.Context, filter EventFilter) (*QueryResult, error) {
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Build query with conditions (Trino uses ? placeholders)
	// Note: timestamp_ms is stored as bigint (milliseconds since epoch)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(CAST(id AS VARCHAR), '') as id,
			COALESCE(app_id, '') as app_id,
			COALESCE(event_type, '') as event_type,
			COALESCE(event_category, '') as event_category,
			COALESCE(device_id, '') as device_id,
			COALESCE(platform, '') as platform,
			from_unixtime(timestamp_ms / 1000) as timestamp,
			COALESCE(payload_json, '{}') as parameters
		FROM %s.%s.events
		WHERE 1=1
	`, c.catalog, c.schema)

	var args []interface{}

	if filter.AppID != "" {
		query += " AND app_id = ?"
		args = append(args, filter.AppID)
	}
	if filter.EventCategory != "" {
		query += " AND event_category = ?"
		args = append(args, filter.EventCategory)
	}
	if filter.EventType != "" {
		query += " AND event_type = ?"
		args = append(args, filter.EventType)
	}
	if filter.StartDate != "" {
		query += " AND from_unixtime(timestamp_ms / 1000) >= TIMESTAMP ?"
		args = append(args, filter.StartDate+" 00:00:00")
	}
	if filter.EndDate != "" {
		query += " AND from_unixtime(timestamp_ms / 1000) <= TIMESTAMP ?"
		args = append(args, filter.EndDate+" 23:59:59")
	}

	query += " ORDER BY timestamp_ms DESC"
	// Trino uses OFFSET after LIMIT: LIMIT n OFFSET m
	query += fmt.Sprintf(" LIMIT %d", filter.Limit+1)
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AppID, &e.EventType, &e.EventCategory,
			&e.DeviceID, &e.Platform, &e.Timestamp, &e.Parameters); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	hasMore := len(events) > filter.Limit
	if hasMore {
		events = events[:filter.Limit]
	}

	return &QueryResult{
		Events:  events,
		HasMore: hasMore,
	}, rows.Err()
}

// GetDistinctValues returns distinct values for a column.
func (c *TrinoClient) GetDistinctValues(ctx context.Context, column string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT COALESCE(%s, '') as value
		FROM %s.%s.events
		WHERE %s IS NOT NULL
		ORDER BY value
		LIMIT 100
	`, column, c.catalog, c.schema, column)

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v != "" {
			values = append(values, v)
		}
	}
	return values, rows.Err()
}
