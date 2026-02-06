package mobile

import (
	"encoding/json"
	"fmt"
)

// UserIdentity represents user identification data passed via JSON bridge.
type UserIdentity struct {
	// UserID is the primary user identifier.
	UserID string `json:"user_id"`

	// Traits are key-value attributes about the user (e.g., name, email, plan).
	Traits map[string]string `json:"traits,omitempty"`

	// Aliases are alternative identifiers for the user (for identity resolution).
	Aliases []string `json:"aliases,omitempty"`
}

// parseConfig unmarshals a JSON string into a validated Config.
func parseConfig(jsonStr string) (*Config, error) {
	return configFromJSON(jsonStr)
}

// parseEvent unmarshals a JSON string into an Event.
func parseEvent(jsonStr string) (*Event, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("event JSON is empty")
	}

	var event Event
	if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
		return nil, fmt.Errorf("invalid event JSON: %w", err)
	}

	if event.Type == "" {
		return nil, fmt.Errorf("event type is required")
	}

	return &event, nil
}

// parseUser unmarshals a JSON string into a UserIdentity.
func parseUser(jsonStr string) (*UserIdentity, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("user JSON is empty")
	}

	var user UserIdentity
	if err := json.Unmarshal([]byte(jsonStr), &user); err != nil {
		return nil, fmt.Errorf("invalid user JSON: %w", err)
	}

	if user.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	return &user, nil
}

// serializeEvent marshals a typed event struct to a JSON string.
func serializeEvent(event interface{}) (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to serialize event: %w", err)
	}
	return string(data), nil
}

// marshalEvent creates a complete Event from a type and properties struct.
// This is used internally to wrap typed events for the queue.
func marshalEvent(eventType string, properties interface{}) (*Event, error) {
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event properties: %w", err)
	}

	return &Event{
		Type:       eventType,
		Properties: propsJSON,
	}, nil
}
