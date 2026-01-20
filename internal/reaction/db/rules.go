package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for rules.
var (
	ErrRuleNotFound = errors.New("rule not found")
)

// Condition represents a single condition in a rule.
type Condition struct {
	Path     string      `json:"path"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// Actions represents the actions to take when a rule matches.
type Actions struct {
	Webhooks        []string `json:"webhooks"`
	PublishSubjects []string `json:"publish_subjects"`
}

// Rule represents a rule definition for event matching.
type Rule struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   *string     `json:"description,omitempty"`
	AppID         *string     `json:"app_id,omitempty"`
	EventCategory *string     `json:"event_category,omitempty"`
	EventType     *string     `json:"event_type,omitempty"`
	Conditions    []Condition `json:"conditions"`
	Actions       Actions     `json:"actions"`
	Priority      int         `json:"priority"`
	Enabled       bool        `json:"enabled"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// RuleRepository provides CRUD operations for rules.
type RuleRepository struct {
	db *sql.DB
}

// NewRuleRepository creates a new rule repository.
func NewRuleRepository(client *Client) *RuleRepository {
	return &RuleRepository{db: client.DB()}
}

// Create creates a new rule.
func (r *RuleRepository) Create(ctx context.Context, rule *Rule) error {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return err
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO rules (name, description, app_id, event_category, event_type, conditions, actions, priority, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRowContext(
		ctx, query,
		rule.Name,
		rule.Description,
		rule.AppID,
		rule.EventCategory,
		rule.EventType,
		conditionsJSON,
		actionsJSON,
		rule.Priority,
		rule.Enabled,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
}

// GetByID retrieves a rule by ID.
func (r *RuleRepository) GetByID(ctx context.Context, id string) (*Rule, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules
		WHERE id = $1
	`

	rule := &Rule{}
	var conditionsJSON, actionsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID,
		&rule.Name,
		&rule.Description,
		&rule.AppID,
		&rule.EventCategory,
		&rule.EventType,
		&conditionsJSON,
		&actionsJSON,
		&rule.Priority,
		&rule.Enabled,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}

	if err := json.Unmarshal(conditionsJSON, &rule.Conditions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(actionsJSON, &rule.Actions); err != nil {
		return nil, err
	}

	return rule, nil
}

// GetEnabled retrieves all enabled rules ordered by priority.
func (r *RuleRepository) GetEnabled(ctx context.Context) ([]*Rule, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules
		WHERE enabled = true
		ORDER BY priority DESC, name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRules(rows)
}

// GetMatchingRules retrieves enabled rules that could match the given app_id, category, and type.
// Rules match if their filter is NULL (matches all) or equals the given value.
func (r *RuleRepository) GetMatchingRules(ctx context.Context, appID, category, eventType string) ([]*Rule, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules
		WHERE enabled = true
		  AND (app_id IS NULL OR app_id = $1)
		  AND (event_category IS NULL OR event_category = $2)
		  AND (event_type IS NULL OR event_type = $3)
		ORDER BY priority DESC, name
	`

	rows, err := r.db.QueryContext(ctx, query, appID, category, eventType)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRules(rows)
}

// scanRules scans multiple rules from rows.
func (r *RuleRepository) scanRules(rows *sql.Rows) ([]*Rule, error) {
	var rules []*Rule
	for rows.Next() {
		rule := &Rule{}
		var conditionsJSON, actionsJSON []byte

		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.Description,
			&rule.AppID,
			&rule.EventCategory,
			&rule.EventType,
			&conditionsJSON,
			&actionsJSON,
			&rule.Priority,
			&rule.Enabled,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(conditionsJSON, &rule.Conditions); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(actionsJSON, &rule.Actions); err != nil {
			return nil, err
		}

		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// Update updates a rule.
func (r *RuleRepository) Update(ctx context.Context, rule *Rule) error {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return err
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return err
	}

	query := `
		UPDATE rules
		SET name = $1, description = $2, app_id = $3, event_category = $4, event_type = $5,
		    conditions = $6, actions = $7, priority = $8, enabled = $9
		WHERE id = $10
	`

	result, err := r.db.ExecContext(
		ctx, query,
		rule.Name,
		rule.Description,
		rule.AppID,
		rule.EventCategory,
		rule.EventType,
		conditionsJSON,
		actionsJSON,
		rule.Priority,
		rule.Enabled,
		rule.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRuleNotFound
	}

	return nil
}

// Delete deletes a rule by ID.
func (r *RuleRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM rules WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRuleNotFound
	}

	return nil
}

// List retrieves all rules with pagination.
func (r *RuleRepository) List(ctx context.Context, limit, offset int) ([]*Rule, error) {
	query := `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules
		ORDER BY priority DESC, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return r.scanRules(rows)
}
