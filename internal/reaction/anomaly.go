package reaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/SebastienMelki/causality/internal/events"
	"github.com/SebastienMelki/causality/internal/reaction/db"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// ThresholdConfig holds configuration for threshold-based anomaly detection.
type ThresholdConfig struct {
	Path string   `json:"path"`
	Min  *float64 `json:"min"`
	Max  *float64 `json:"max"`
}

// RateConfig holds configuration for rate-based anomaly detection.
type RateConfig struct {
	MaxPerMinute int `json:"max_per_minute"`
}

// CountConfig holds configuration for count-based anomaly detection.
type CountConfig struct {
	WindowSeconds int `json:"window_seconds"`
	MaxCount      int `json:"max_count"`
}

// AnomalyDetector detects anomalies in events.
type AnomalyDetector struct {
	anomalyConfigs *db.AnomalyConfigRepository
	js             jetstream.JetStream
	config         AnomalyConfig
	logger         *slog.Logger

	mu            sync.RWMutex
	cachedConfigs []*db.AnomalyConfig
	stopCh        chan struct{}
	doneCh        chan struct{}
}

// NewAnomalyDetector creates a new anomaly detector.
func NewAnomalyDetector(
	anomalyConfigs *db.AnomalyConfigRepository,
	js jetstream.JetStream,
	config AnomalyConfig,
	logger *slog.Logger,
) *AnomalyDetector {
	if logger == nil {
		logger = slog.Default()
	}

	return &AnomalyDetector{
		anomalyConfigs: anomalyConfigs,
		js:             js,
		config:         config,
		logger:         logger.With("component", "anomaly-detector"),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

// Start starts the anomaly detector's background tasks.
func (a *AnomalyDetector) Start(ctx context.Context) error {
	// Load initial configs
	if err := a.refreshConfigs(ctx); err != nil {
		return fmt.Errorf("failed to load initial anomaly configs: %w", err)
	}

	// Start background tasks
	go a.refreshLoop(ctx)
	go a.cleanupLoop(ctx)

	a.logger.Info("anomaly detector started",
		"config_count", len(a.cachedConfigs),
		"refresh_interval", a.config.ConfigRefreshInterval,
	)

	return nil
}

// Stop stops the anomaly detector.
func (a *AnomalyDetector) Stop() {
	close(a.stopCh)
	<-a.doneCh
}

// refreshLoop periodically refreshes anomaly configs.
func (a *AnomalyDetector) refreshLoop(ctx context.Context) {
	defer close(a.doneCh)

	ticker := time.NewTicker(a.config.ConfigRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.refreshConfigs(ctx); err != nil {
				a.logger.Error("failed to refresh anomaly configs", "error", err)
			}
		}
	}
}

// cleanupLoop periodically cleans up old state.
func (a *AnomalyDetector) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(a.config.StateCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.cleanup(ctx)
		}
	}
}

// cleanup removes old state and event records.
func (a *AnomalyDetector) cleanup(ctx context.Context) {
	cutoff := time.Now().Add(-a.config.StateRetentionDuration)

	stateCount, err := a.anomalyConfigs.CleanupOldState(ctx, cutoff)
	if err != nil {
		a.logger.Error("failed to cleanup old state", "error", err)
	} else if stateCount > 0 {
		a.logger.Debug("cleaned up old state", "count", stateCount)
	}

	eventCount, err := a.anomalyConfigs.CleanupOldEvents(ctx, cutoff)
	if err != nil {
		a.logger.Error("failed to cleanup old events", "error", err)
	} else if eventCount > 0 {
		a.logger.Debug("cleaned up old events", "count", eventCount)
	}
}

// refreshConfigs loads anomaly configs from the database.
func (a *AnomalyDetector) refreshConfigs(ctx context.Context) error {
	configs, err := a.anomalyConfigs.GetEnabled(ctx)
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.cachedConfigs = configs
	a.mu.Unlock()

	a.logger.Debug("anomaly configs refreshed", "count", len(configs))
	return nil
}

// ProcessEvent checks an event against all matching anomaly configs.
func (a *AnomalyDetector) ProcessEvent(ctx context.Context, event *pb.EventEnvelope) error {
	category, eventType := events.GetCategoryAndType(event)
	appID := event.AppId

	a.mu.RLock()
	configs := a.cachedConfigs
	a.mu.RUnlock()

	// Convert event to JSON for threshold evaluation
	eventJSON, err := a.eventToJSON(event)
	if err != nil {
		return fmt.Errorf("failed to convert event to JSON: %w", err)
	}

	for _, config := range configs {
		if !a.matchesFilter(config, appID, category, eventType) {
			continue
		}

		if err := a.evaluateConfig(ctx, config, event, eventJSON); err != nil {
			a.logger.Error("failed to evaluate anomaly config",
				"config_id", config.ID,
				"config_name", config.Name,
				"error", err,
			)
		}
	}

	return nil
}

// matchesFilter checks if an event matches the config's filters.
func (a *AnomalyDetector) matchesFilter(config *db.AnomalyConfig, appID, category, eventType string) bool {
	if config.AppID != nil && *config.AppID != appID {
		return false
	}
	if config.EventCategory != nil && *config.EventCategory != category {
		return false
	}
	if config.EventType != nil && *config.EventType != eventType {
		return false
	}
	return true
}

// evaluateConfig evaluates a single anomaly config against an event.
func (a *AnomalyDetector) evaluateConfig(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope, eventJSON map[string]interface{}) error {
	switch config.DetectionType {
	case db.DetectionTypeThreshold:
		return a.evaluateThreshold(ctx, config, event, eventJSON)
	case db.DetectionTypeRate:
		return a.evaluateRate(ctx, config, event)
	case db.DetectionTypeCount:
		return a.evaluateCount(ctx, config, event)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidDetectionType, config.DetectionType)
	}
}

// evaluateThreshold checks if a value exceeds min/max bounds.
func (a *AnomalyDetector) evaluateThreshold(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope, eventJSON map[string]interface{}) error {
	var tc ThresholdConfig
	if err := json.Unmarshal(config.Config, &tc); err != nil {
		return fmt.Errorf("invalid threshold config: %w", err)
	}

	// Extract value at path
	value, exists := a.extractJSONPath(eventJSON, tc.Path)
	if !exists {
		return nil // Path doesn't exist, skip
	}

	numValue, ok := toFloat64Value(value)
	if !ok {
		return nil // Not a number, skip
	}

	// Check bounds
	var anomalyDetected bool
	var details map[string]interface{}

	if tc.Min != nil && numValue < *tc.Min {
		anomalyDetected = true
		details = map[string]interface{}{
			"value":         numValue,
			"threshold_min": *tc.Min,
			"violation":     "below_min",
		}
	} else if tc.Max != nil && numValue > *tc.Max {
		anomalyDetected = true
		details = map[string]interface{}{
			"value":         numValue,
			"threshold_max": *tc.Max,
			"violation":     "above_max",
		}
	}

	if anomalyDetected {
		if err := a.checkCooldownAndAlert(ctx, config, event, details, eventJSON); err != nil {
			return err
		}
	}

	return nil
}

// evaluateRate checks if event rate exceeds max per minute.
func (a *AnomalyDetector) evaluateRate(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope) error {
	var rc RateConfig
	if err := json.Unmarshal(config.Config, &rc); err != nil {
		return fmt.Errorf("invalid rate config: %w", err)
	}

	appID := event.AppId
	windowKey := time.Now().UTC().Format("2006-01-02T15:04") // Minute-based window

	// Increment counter
	count, err := a.anomalyConfigs.IncrementStateCount(ctx, config.ID, appID, windowKey)
	if err != nil {
		return fmt.Errorf("failed to increment state count: %w", err)
	}

	if count > rc.MaxPerMinute {
		details := map[string]interface{}{
			"rate":           count,
			"max_per_minute": rc.MaxPerMinute,
			"window":         windowKey,
		}
		if err := a.checkCooldownAndAlert(ctx, config, event, details, nil); err != nil {
			return err
		}
	}

	return nil
}

// evaluateCount checks if event count in window exceeds threshold.
func (a *AnomalyDetector) evaluateCount(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope) error {
	var cc CountConfig
	if err := json.Unmarshal(config.Config, &cc); err != nil {
		return fmt.Errorf("invalid count config: %w", err)
	}

	appID := event.AppId
	// Create a window key based on window size
	windowStart := time.Now().UTC().Truncate(time.Duration(cc.WindowSeconds) * time.Second)
	windowKey := windowStart.Format(time.RFC3339)

	// Increment counter
	count, err := a.anomalyConfigs.IncrementStateCount(ctx, config.ID, appID, windowKey)
	if err != nil {
		return fmt.Errorf("failed to increment state count: %w", err)
	}

	if count > cc.MaxCount {
		details := map[string]interface{}{
			"count":          count,
			"max_count":      cc.MaxCount,
			"window_seconds": cc.WindowSeconds,
			"window_start":   windowKey,
		}
		if err := a.checkCooldownAndAlert(ctx, config, event, details, nil); err != nil {
			return err
		}
	}

	return nil
}

// checkCooldownAndAlert checks cooldown period and alerts if not in cooldown.
func (a *AnomalyDetector) checkCooldownAndAlert(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope, details map[string]interface{}, eventJSON map[string]interface{}) error {
	appID := event.AppId
	windowKey := time.Now().UTC().Format("2006-01-02T15:04")

	// Check cooldown
	lastAlert, err := a.anomalyConfigs.GetLastAlertAt(ctx, config.ID, appID)
	if err != nil && !errors.Is(err, db.ErrAnomalyStateNotFound) {
		return fmt.Errorf("failed to get last alert time: %w", err)
	}

	cooldownDuration := time.Duration(config.CooldownSeconds) * time.Second
	if lastAlert != nil && time.Since(*lastAlert) < cooldownDuration {
		a.logger.Debug("skipping alert due to cooldown",
			"config_id", config.ID,
			"last_alert", lastAlert,
			"cooldown", cooldownDuration,
		)
		return nil
	}

	// Update last alert time
	if err := a.anomalyConfigs.UpdateLastAlertAt(ctx, config.ID, appID, windowKey); err != nil {
		a.logger.Error("failed to update last alert time", "error", err)
	}

	// Record anomaly event
	category, eventType := events.GetCategoryAndType(event)
	detailsJSON, _ := json.Marshal(details)
	var eventDataJSON []byte
	if eventJSON != nil {
		eventDataJSON, _ = json.Marshal(eventJSON)
	}

	anomalyEvent := &db.AnomalyEvent{
		AnomalyConfigID: config.ID,
		AppID:           &appID,
		EventCategory:   &category,
		EventType:       &eventType,
		DetectionType:   string(config.DetectionType),
		Details:         detailsJSON,
		EventData:       eventDataJSON,
	}

	if err := a.anomalyConfigs.RecordAnomalyEvent(ctx, anomalyEvent); err != nil {
		a.logger.Error("failed to record anomaly event", "error", err)
	}

	// Publish to NATS
	a.publishAnomaly(ctx, config, event, details)

	a.logger.Warn("anomaly detected",
		"config_id", config.ID,
		"config_name", config.Name,
		"app_id", appID,
		"detection_type", config.DetectionType,
		"details", details,
	)

	return nil
}

// publishAnomaly publishes an anomaly alert to NATS.
func (a *AnomalyDetector) publishAnomaly(ctx context.Context, config *db.AnomalyConfig, event *pb.EventEnvelope, details map[string]interface{}) {
	category, eventType := events.GetCategoryAndType(event)
	appID := event.AppId

	payload := map[string]interface{}{
		"anomaly_config_id":   config.ID,
		"anomaly_config_name": config.Name,
		"detection_type":      config.DetectionType,
		"app_id":              appID,
		"event_category":      category,
		"event_type":          eventType,
		"event_id":            event.Id,
		"device_id":           event.DeviceId,
		"timestamp_ms":        event.TimestampMs,
		"details":             details,
		"detected_at":         time.Now().UTC().Format(time.RFC3339),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("failed to marshal anomaly payload", "error", err)
		return
	}

	// Publish to anomalies.{app_id}.{config_name}
	configName := events.SanitizeSubjectName(config.Name)
	subject := fmt.Sprintf("anomalies.%s.%s", events.SanitizeSubjectName(appID), configName)

	if _, err := a.js.Publish(ctx, subject, payloadJSON); err != nil {
		a.logger.Error("failed to publish anomaly",
			"subject", subject,
			"error", err,
		)
	} else {
		a.logger.Debug("anomaly published", "subject", subject)
	}
}

// extractJSONPath extracts a value from JSON using a simple path notation.
func (a *AnomalyDetector) extractJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	path = strings.TrimPrefix(path, "$.")
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// eventToJSON converts a protobuf event to a JSON map.
func (a *AnomalyDetector) eventToJSON(event *pb.EventEnvelope) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"id":             event.Id,
		"app_id":         event.AppId,
		"device_id":      event.DeviceId,
		"timestamp_ms":   event.TimestampMs,
		"correlation_id": event.CorrelationId,
	}

	// Add device context
	if dc := event.DeviceContext; dc != nil {
		result["device_context"] = map[string]interface{}{
			"platform":      dc.Platform.String(),
			"os_version":    dc.OsVersion,
			"app_version":   dc.AppVersion,
			"device_model":  dc.DeviceModel,
			"manufacturer":  dc.Manufacturer,
			"screen_width":  dc.ScreenWidth,
			"screen_height": dc.ScreenHeight,
			"locale":        dc.Locale,
			"timezone":      dc.Timezone,
			"network_type":  dc.NetworkType.String(),
			"is_jailbroken": dc.IsJailbroken,
			"is_emulator":   dc.IsEmulator,
		}
	}

	// Add payload based on type
	switch p := event.Payload.(type) {
	case *pb.EventEnvelope_PurchaseComplete:
		result["purchase_complete"] = structToMap(p.PurchaseComplete)
	case *pb.EventEnvelope_AddToCart:
		result["add_to_cart"] = structToMap(p.AddToCart)
	case *pb.EventEnvelope_ProductView:
		result["product_view"] = structToMap(p.ProductView)
	case *pb.EventEnvelope_CustomEvent:
		result["custom_event"] = structToMap(p.CustomEvent)
	// Add other types as needed for anomaly detection
	}

	return result, nil
}

// toFloat64Value converts a value to float64.
func toFloat64Value(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
