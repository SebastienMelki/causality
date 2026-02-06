package mobile

import (
	"encoding/json"
	"testing"
)

func TestConfigParsing_ValidConfig(t *testing.T) {
	configJSON := `{
		"api_key": "test-key-123",
		"endpoint": "https://analytics.example.com",
		"app_id": "my-app",
		"batch_size": 100,
		"flush_interval_ms": 15000,
		"max_queue_size": 5000,
		"session_timeout_ms": 900000,
		"debug_mode": true,
		"enable_session_tracking": false,
		"persistent_device_id": true,
		"offline_retention_ms": 172800000,
		"data_path": "/tmp/causality"
	}`

	cfg, err := configFromJSON(configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "test-key-123" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key-123")
	}
	if cfg.Endpoint != "https://analytics.example.com" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "https://analytics.example.com")
	}
	if cfg.AppID != "my-app" {
		t.Errorf("AppID = %q, want %q", cfg.AppID, "my-app")
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, 100)
	}
	if cfg.FlushIntervalMs != 15000 {
		t.Errorf("FlushIntervalMs = %d, want %d", cfg.FlushIntervalMs, 15000)
	}
	if cfg.MaxQueueSize != 5000 {
		t.Errorf("MaxQueueSize = %d, want %d", cfg.MaxQueueSize, 5000)
	}
	if cfg.SessionTimeoutMs != 900000 {
		t.Errorf("SessionTimeoutMs = %d, want %d", cfg.SessionTimeoutMs, 900000)
	}
	if !cfg.DebugMode {
		t.Error("DebugMode = false, want true")
	}
	if cfg.EnableSessionTracking == nil || *cfg.EnableSessionTracking {
		t.Error("EnableSessionTracking should be false")
	}
	if !cfg.PersistentDeviceID {
		t.Error("PersistentDeviceID = false, want true")
	}
	if cfg.OfflineRetentionMs != 172800000 {
		t.Errorf("OfflineRetentionMs = %d, want %d", cfg.OfflineRetentionMs, 172800000)
	}
	if cfg.DataPath != "/tmp/causality" {
		t.Errorf("DataPath = %q, want %q", cfg.DataPath, "/tmp/causality")
	}
}

func TestConfigParsing_MinimalConfig(t *testing.T) {
	configJSON := `{
		"api_key": "key",
		"endpoint": "https://api.example.com",
		"app_id": "app"
	}`

	cfg, err := configFromJSON(configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.APIKey != "key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "key")
	}
	if cfg.Endpoint != "https://api.example.com" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "https://api.example.com")
	}
	if cfg.AppID != "app" {
		t.Errorf("AppID = %q, want %q", cfg.AppID, "app")
	}
}

func TestConfigParsing_Defaults(t *testing.T) {
	configJSON := `{
		"api_key": "key",
		"endpoint": "https://api.example.com",
		"app_id": "app"
	}`

	cfg, err := configFromJSON(configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("BatchSize = %d, want default %d", cfg.BatchSize, DefaultBatchSize)
	}
	if cfg.FlushIntervalMs != DefaultFlushIntervalMs {
		t.Errorf("FlushIntervalMs = %d, want default %d", cfg.FlushIntervalMs, DefaultFlushIntervalMs)
	}
	if cfg.MaxQueueSize != DefaultMaxQueueSize {
		t.Errorf("MaxQueueSize = %d, want default %d", cfg.MaxQueueSize, DefaultMaxQueueSize)
	}
	if cfg.SessionTimeoutMs != DefaultSessionTimeoutMs {
		t.Errorf("SessionTimeoutMs = %d, want default %d", cfg.SessionTimeoutMs, DefaultSessionTimeoutMs)
	}
	if cfg.OfflineRetentionMs != DefaultOfflineRetentionMs {
		t.Errorf("OfflineRetentionMs = %d, want default %d", cfg.OfflineRetentionMs, DefaultOfflineRetentionMs)
	}
	if cfg.EnableSessionTracking == nil || !*cfg.EnableSessionTracking {
		t.Error("EnableSessionTracking should default to true")
	}
	if cfg.PersistentDeviceID {
		t.Error("PersistentDeviceID should default to false")
	}
	if cfg.DebugMode {
		t.Error("DebugMode should default to false")
	}
}

func TestConfigParsing_TrailingSlashTrimmed(t *testing.T) {
	configJSON := `{
		"api_key": "key",
		"endpoint": "https://api.example.com/",
		"app_id": "app"
	}`

	cfg, err := configFromJSON(configJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Endpoint != "https://api.example.com" {
		t.Errorf("Endpoint = %q, want trailing slash trimmed", cfg.Endpoint)
	}
}

func TestConfigValidation_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "missing api_key",
			config:  `{"endpoint": "https://api.example.com", "app_id": "app"}`,
			wantErr: "api_key is required",
		},
		{
			name:    "empty api_key",
			config:  `{"api_key": "", "endpoint": "https://api.example.com", "app_id": "app"}`,
			wantErr: "api_key is required",
		},
		{
			name:    "whitespace api_key",
			config:  `{"api_key": "  ", "endpoint": "https://api.example.com", "app_id": "app"}`,
			wantErr: "api_key is required",
		},
		{
			name:    "missing endpoint",
			config:  `{"api_key": "key", "app_id": "app"}`,
			wantErr: "endpoint is required",
		},
		{
			name:    "empty endpoint",
			config:  `{"api_key": "key", "endpoint": "", "app_id": "app"}`,
			wantErr: "endpoint is required",
		},
		{
			name:    "missing app_id",
			config:  `{"api_key": "key", "endpoint": "https://api.example.com"}`,
			wantErr: "app_id is required",
		},
		{
			name:    "empty app_id",
			config:  `{"api_key": "key", "endpoint": "https://api.example.com", "app_id": ""}`,
			wantErr: "app_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configFromJSON(tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestConfigValidation_InvalidEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  string
	}{
		{
			name:     "no scheme",
			endpoint: "api.example.com",
			wantErr:  "must include scheme and host",
		},
		{
			name:     "just path",
			endpoint: "/v1/events",
			wantErr:  "must include scheme and host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := `{"api_key": "key", "endpoint": "` + tt.endpoint + `", "app_id": "app"}`
			_, err := configFromJSON(config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestConfigValidation_InvalidJSON(t *testing.T) {
	_, err := configFromJSON("not valid json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if got := err.Error(); !contains(got, "invalid config JSON") {
		t.Errorf("error = %q, want to contain 'invalid config JSON'", got)
	}
}

func TestConfigValidation_EmptyJSON(t *testing.T) {
	_, err := configFromJSON("{}")
	if err == nil {
		t.Fatal("expected error for empty JSON object, got nil")
	}
}

func TestConfigValidation_NegativeValues(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "negative batch_size",
			config:  `{"api_key":"k","endpoint":"https://a.com","app_id":"a","batch_size":-1}`,
			wantErr: "batch_size must be non-negative",
		},
		{
			name:    "negative flush_interval_ms",
			config:  `{"api_key":"k","endpoint":"https://a.com","app_id":"a","flush_interval_ms":-1}`,
			wantErr: "flush_interval_ms must be non-negative",
		},
		{
			name:    "negative max_queue_size",
			config:  `{"api_key":"k","endpoint":"https://a.com","app_id":"a","max_queue_size":-1}`,
			wantErr: "max_queue_size must be non-negative",
		},
		{
			name:    "negative session_timeout_ms",
			config:  `{"api_key":"k","endpoint":"https://a.com","app_id":"a","session_timeout_ms":-1}`,
			wantErr: "session_timeout_ms must be non-negative",
		},
		{
			name:    "negative offline_retention_ms",
			config:  `{"api_key":"k","endpoint":"https://a.com","app_id":"a","offline_retention_ms":-1}`,
			wantErr: "offline_retention_ms must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configFromJSON(tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestConfigJSON_RoundTrip(t *testing.T) {
	original := Config{
		APIKey:             "test-key",
		Endpoint:           "https://api.example.com",
		AppID:              "my-app",
		BatchSize:          75,
		FlushIntervalMs:    10000,
		MaxQueueSize:       2000,
		SessionTimeoutMs:   600000,
		DebugMode:          true,
		PersistentDeviceID: true,
		OfflineRetentionMs: 3600000,
		DataPath:           "/data/causality",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	cfg, err := configFromJSON(string(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if cfg.APIKey != original.APIKey {
		t.Errorf("APIKey mismatch: %q vs %q", cfg.APIKey, original.APIKey)
	}
	if cfg.BatchSize != original.BatchSize {
		t.Errorf("BatchSize mismatch: %d vs %d", cfg.BatchSize, original.BatchSize)
	}
	if cfg.FlushIntervalMs != original.FlushIntervalMs {
		t.Errorf("FlushIntervalMs mismatch: %d vs %d", cfg.FlushIntervalMs, original.FlushIntervalMs)
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
