package reaction

import "errors"

// Sentinel errors for the reaction package.
var (
	// ErrRuleNotFound indicates a rule was not found.
	ErrRuleNotFound = errors.New("rule not found")

	// ErrWebhookNotFound indicates a webhook was not found.
	ErrWebhookNotFound = errors.New("webhook not found")

	// ErrDeliveryNotFound indicates a delivery was not found.
	ErrDeliveryNotFound = errors.New("delivery not found")

	// ErrAnomalyConfigNotFound indicates an anomaly config was not found.
	ErrAnomalyConfigNotFound = errors.New("anomaly config not found")

	// ErrInvalidCondition indicates a rule condition is invalid.
	ErrInvalidCondition = errors.New("invalid condition")

	// ErrInvalidOperator indicates an unknown comparison operator.
	ErrInvalidOperator = errors.New("invalid operator")

	// ErrInvalidJSONPath indicates an invalid JSONPath expression.
	ErrInvalidJSONPath = errors.New("invalid JSON path")

	// ErrInvalidAuthType indicates an unknown auth type.
	ErrInvalidAuthType = errors.New("invalid auth type")

	// ErrInvalidDetectionType indicates an unknown detection type.
	ErrInvalidDetectionType = errors.New("invalid detection type")

	// ErrDeliveryMaxAttemptsReached indicates max delivery attempts reached.
	ErrDeliveryMaxAttemptsReached = errors.New("max delivery attempts reached")

	// ErrWebhookDisabled indicates the webhook is disabled.
	ErrWebhookDisabled = errors.New("webhook is disabled")

	// ErrRuleDisabled indicates the rule is disabled.
	ErrRuleDisabled = errors.New("rule is disabled")

	// ErrAnomalyCooldown indicates the anomaly is in cooldown period.
	ErrAnomalyCooldown = errors.New("anomaly in cooldown period")

	// ErrDatabaseConnection indicates a database connection error.
	ErrDatabaseConnection = errors.New("database connection error")

	// ErrNoRowsAffected indicates no rows were affected by an operation.
	ErrNoRowsAffected = errors.New("no rows affected")

	// ErrWebhookStatusError indicates the webhook returned a non-2xx status.
	ErrWebhookStatusError = errors.New("webhook returned non-2xx status")

	// ErrAnomalyStateNotFound indicates no anomaly state was found.
	ErrAnomalyStateNotFound = errors.New("anomaly state not found")
)
