package session

// SessionStartEvent returns event properties for a session_start event.
// The returned map is intended for use by the main SDK layer when
// constructing the full event with metadata.
func SessionStartEvent(sessionID string) map[string]interface{} {
	return map[string]interface{}{
		"session_id": sessionID,
		"event_type": "session_start",
	}
}

// SessionEndEvent returns event properties for a session_end event.
// durationMs is the session duration from start to last activity.
func SessionEndEvent(sessionID string, durationMs int64) map[string]interface{} {
	return map[string]interface{}{
		"session_id":  sessionID,
		"event_type":  "session_end",
		"duration_ms": durationMs,
	}
}
