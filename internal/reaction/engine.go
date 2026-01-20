package reaction

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/SebastienMelki/causality/internal/events"
	"github.com/SebastienMelki/causality/internal/reaction/db"
	pb "github.com/SebastienMelki/causality/pkg/proto/causality/v1"
)

// Engine evaluates events against rules and triggers actions.
type Engine struct {
	rules         *db.RuleRepository
	webhooks      *db.WebhookRepository
	deliveries    *db.DeliveryRepository
	js            jetstream.JetStream
	config        EngineConfig
	dispatcherCfg DispatcherConfig
	logger        *slog.Logger

	mu          sync.RWMutex
	cachedRules []*db.Rule
	stopCh      chan struct{}
	doneCh      chan struct{}
}

// NewEngine creates a new rule engine.
func NewEngine(
	rules *db.RuleRepository,
	webhooks *db.WebhookRepository,
	deliveries *db.DeliveryRepository,
	js jetstream.JetStream,
	config EngineConfig,
	dispatcherCfg DispatcherConfig,
	logger *slog.Logger,
) *Engine {
	if logger == nil {
		logger = slog.Default()
	}

	return &Engine{
		rules:         rules,
		webhooks:      webhooks,
		deliveries:    deliveries,
		js:            js,
		config:        config,
		dispatcherCfg: dispatcherCfg,
		logger:        logger.With("component", "reaction-engine"),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
}

// Start starts the engine's background tasks (rule refresh).
func (e *Engine) Start(ctx context.Context) error {
	// Load initial rules
	if err := e.refreshRules(ctx); err != nil {
		return fmt.Errorf("failed to load initial rules: %w", err)
	}

	// Start background rule refresh
	go e.refreshLoop(ctx)

	e.logger.Info("rule engine started",
		"rule_count", len(e.cachedRules),
		"refresh_interval", e.config.RuleRefreshInterval,
	)

	return nil
}

// Stop stops the engine.
func (e *Engine) Stop() {
	close(e.stopCh)
	<-e.doneCh
}

// refreshLoop periodically refreshes rules from the database.
func (e *Engine) refreshLoop(ctx context.Context) {
	defer close(e.doneCh)

	ticker := time.NewTicker(e.config.RuleRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := e.refreshRules(ctx); err != nil {
				e.logger.Error("failed to refresh rules", "error", err)
			}
		}
	}
}

// refreshRules loads rules from the database.
func (e *Engine) refreshRules(ctx context.Context) error {
	rules, err := e.rules.GetEnabled(ctx)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.cachedRules = rules
	e.mu.Unlock()

	e.logger.Debug("rules refreshed", "count", len(rules))
	return nil
}

// ProcessEvent evaluates an event against all matching rules.
func (e *Engine) ProcessEvent(ctx context.Context, event *pb.EventEnvelope) error {
	category, eventType := events.GetCategoryAndType(event)
	appID := event.AppId

	e.mu.RLock()
	rules := e.cachedRules
	e.mu.RUnlock()

	// Convert event to JSON for condition evaluation
	eventJSON, err := e.eventToJSON(event)
	if err != nil {
		return fmt.Errorf("failed to convert event to JSON: %w", err)
	}

	matchedRules := e.findMatchingRules(rules, appID, category, eventType, eventJSON)

	if len(matchedRules) == 0 {
		e.logger.Debug("no rules matched",
			"event_id", event.Id,
			"category", category,
			"type", eventType,
		)
		return nil
	}

	e.logger.Info("rules matched",
		"event_id", event.Id,
		"app_id", appID,
		"category", category,
		"type", eventType,
		"matched_rules", len(matchedRules),
	)

	// Execute actions for each matched rule
	for _, rule := range matchedRules {
		if err := e.executeActions(ctx, rule, event, eventJSON); err != nil {
			e.logger.Error("failed to execute rule actions",
				"rule_id", rule.ID,
				"rule_name", rule.Name,
				"error", err,
			)
		}
	}

	return nil
}

// findMatchingRules finds rules that match the event.
func (e *Engine) findMatchingRules(rules []*db.Rule, appID, category, eventType string, eventJSON map[string]interface{}) []*db.Rule {
	var matched []*db.Rule

	for _, rule := range rules {
		if !e.matchesFilter(rule, appID, category, eventType) {
			continue
		}

		if !e.evaluateConditions(rule.Conditions, eventJSON) {
			continue
		}

		matched = append(matched, rule)
	}

	return matched
}

// matchesFilter checks if an event matches the rule's basic filters.
func (e *Engine) matchesFilter(rule *db.Rule, appID, category, eventType string) bool {
	if rule.AppID != nil && *rule.AppID != appID {
		return false
	}
	if rule.EventCategory != nil && *rule.EventCategory != category {
		return false
	}
	if rule.EventType != nil && *rule.EventType != eventType {
		return false
	}
	return true
}

// evaluateConditions evaluates all conditions against the event.
func (e *Engine) evaluateConditions(conditions []db.Condition, eventJSON map[string]interface{}) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, cond := range conditions {
		if !e.evaluateCondition(cond, eventJSON) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a single condition.
func (e *Engine) evaluateCondition(cond db.Condition, eventJSON map[string]interface{}) bool {
	// Extract value at path
	value, exists := e.extractJSONPath(eventJSON, cond.Path)

	switch cond.Operator {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	}

	if !exists {
		return false
	}

	return e.compareValues(value, cond.Operator, cond.Value)
}

// extractJSONPath extracts a value from JSON using a simple path notation.
// Supports paths like "$.field.subfield" or "field.subfield".
func (e *Engine) extractJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	// Remove leading "$." if present
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

// compareValues compares two values using the specified operator.
func (e *Engine) compareValues(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq":
		return e.equals(actual, expected)
	case "ne":
		return !e.equals(actual, expected)
	case "gt", "gte", "lt", "lte":
		return e.compareNumeric(actual, operator, expected)
	case "contains":
		return e.contains(actual, expected)
	case "regex":
		return e.matchesRegex(actual, expected)
	case "in":
		return e.inList(actual, expected)
	default:
		return false
	}
}

// equals checks if two values are equal.
func (e *Engine) equals(actual, expected interface{}) bool {
	// Handle nil
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Try numeric comparison
	actualNum, actualOK := toFloat64(actual)
	expectedNum, expectedOK := toFloat64(expected)
	if actualOK && expectedOK {
		return actualNum == expectedNum
	}

	// String comparison
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

// compareNumeric performs numeric comparison.
func (e *Engine) compareNumeric(actual interface{}, operator string, expected interface{}) bool {
	actualNum, actualOK := toFloat64(actual)
	expectedNum, expectedOK := toFloat64(expected)
	if !actualOK || !expectedOK {
		return false
	}

	switch operator {
	case "gt":
		return actualNum > expectedNum
	case "gte":
		return actualNum >= expectedNum
	case "lt":
		return actualNum < expectedNum
	case "lte":
		return actualNum <= expectedNum
	default:
		return false
	}
}

// contains checks if actual contains expected as a substring.
func (e *Engine) contains(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return strings.Contains(actualStr, expectedStr)
}

// matchesRegex checks if actual matches the expected regex pattern.
func (e *Engine) matchesRegex(actual, expected interface{}) bool {
	actualStr := fmt.Sprintf("%v", actual)
	pattern := fmt.Sprintf("%v", expected)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return re.MatchString(actualStr)
}

// inList checks if actual is in the expected list.
func (e *Engine) inList(actual, expected interface{}) bool {
	list, ok := expected.([]interface{})
	if !ok {
		return false
	}

	for _, item := range list {
		if e.equals(actual, item) {
			return true
		}
	}

	return false
}

// toFloat64 converts a value to float64.
func toFloat64(v interface{}) (float64, bool) {
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

// executeActions executes the actions for a matched rule.
func (e *Engine) executeActions(ctx context.Context, rule *db.Rule, event *pb.EventEnvelope, eventJSON map[string]interface{}) error {
	// Create payload for webhooks
	payload := map[string]interface{}{
		"rule_id":        rule.ID,
		"rule_name":      rule.Name,
		"event_id":       event.Id,
		"app_id":         event.AppId,
		"device_id":      event.DeviceId,
		"timestamp_ms":   event.TimestampMs,
		"correlation_id": event.CorrelationId,
		"event":          eventJSON,
		"triggered_at":   time.Now().UTC().Format(time.RFC3339),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Queue webhook deliveries
	if len(rule.Actions.Webhooks) > 0 {
		if err := e.queueWebhooks(ctx, rule, payloadJSON); err != nil {
			e.logger.Error("failed to queue webhooks",
				"rule_id", rule.ID,
				"error", err,
			)
		}
	}

	// Publish to NATS subjects
	if len(rule.Actions.PublishSubjects) > 0 {
		e.publishToSubjects(ctx, rule.Actions.PublishSubjects, event.AppId, payloadJSON)
	}

	return nil
}

// queueWebhooks creates delivery records for the specified webhooks.
func (e *Engine) queueWebhooks(ctx context.Context, rule *db.Rule, payload []byte) error {
	var deliveries []*db.WebhookDelivery

	for _, webhookID := range rule.Actions.Webhooks {
		delivery := &db.WebhookDelivery{
			WebhookID:     webhookID,
			RuleID:        &rule.ID,
			Payload:       payload,
			Status:        db.DeliveryStatusPending,
			MaxAttempts:   e.dispatcherCfg.MaxAttempts,
			NextAttemptAt: time.Now(),
		}
		deliveries = append(deliveries, delivery)
	}

	return e.deliveries.CreateBatch(ctx, deliveries)
}

// publishToSubjects publishes to NATS subjects with template substitution.
func (e *Engine) publishToSubjects(ctx context.Context, subjects []string, appID string, payload []byte) {
	for _, subjectTemplate := range subjects {
		subject := strings.ReplaceAll(subjectTemplate, "{app_id}", events.SanitizeSubjectName(appID))

		if _, err := e.js.Publish(ctx, subject, payload); err != nil {
			e.logger.Error("failed to publish to subject",
				"subject", subject,
				"error", err,
			)
		} else {
			e.logger.Debug("published to subject", "subject", subject)
		}
	}
}

// eventToJSON converts a protobuf event to a JSON map.
func (e *Engine) eventToJSON(event *pb.EventEnvelope) (map[string]interface{}, error) {
	// We need to convert the event to JSON
	// First, let's build a map from the event fields
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
			"build_number":  dc.BuildNumber,
			"device_model":  dc.DeviceModel,
			"manufacturer":  dc.Manufacturer,
			"screen_width":  dc.ScreenWidth,
			"screen_height": dc.ScreenHeight,
			"locale":        dc.Locale,
			"timezone":      dc.Timezone,
			"network_type":  dc.NetworkType.String(),
			"carrier":       dc.Carrier,
			"is_jailbroken": dc.IsJailbroken,
			"is_emulator":   dc.IsEmulator,
			"sdk_version":   dc.SdkVersion,
		}
	}

	// Add payload based on type - using switch to handle each type
	switch p := event.Payload.(type) {
	case *pb.EventEnvelope_ScreenView:
		result["screen_view"] = structToMap(p.ScreenView)
	case *pb.EventEnvelope_ScreenExit:
		result["screen_exit"] = structToMap(p.ScreenExit)
	case *pb.EventEnvelope_ButtonTap:
		result["button_tap"] = structToMap(p.ButtonTap)
	case *pb.EventEnvelope_SwipeGesture:
		result["swipe_gesture"] = structToMap(p.SwipeGesture)
	case *pb.EventEnvelope_ScrollEvent:
		result["scroll_event"] = structToMap(p.ScrollEvent)
	case *pb.EventEnvelope_TextInput:
		result["text_input"] = structToMap(p.TextInput)
	case *pb.EventEnvelope_LongPress:
		result["long_press"] = structToMap(p.LongPress)
	case *pb.EventEnvelope_DoubleTap:
		result["double_tap"] = structToMap(p.DoubleTap)
	case *pb.EventEnvelope_UserLogin:
		result["user_login"] = structToMap(p.UserLogin)
	case *pb.EventEnvelope_UserLogout:
		result["user_logout"] = structToMap(p.UserLogout)
	case *pb.EventEnvelope_UserSignup:
		result["user_signup"] = structToMap(p.UserSignup)
	case *pb.EventEnvelope_UserProfileUpdate:
		result["user_profile_update"] = structToMap(p.UserProfileUpdate)
	case *pb.EventEnvelope_ProductView:
		result["product_view"] = structToMap(p.ProductView)
	case *pb.EventEnvelope_AddToCart:
		result["add_to_cart"] = structToMap(p.AddToCart)
	case *pb.EventEnvelope_RemoveFromCart:
		result["remove_from_cart"] = structToMap(p.RemoveFromCart)
	case *pb.EventEnvelope_CheckoutStart:
		result["checkout_start"] = structToMap(p.CheckoutStart)
	case *pb.EventEnvelope_CheckoutStep:
		result["checkout_step"] = structToMap(p.CheckoutStep)
	case *pb.EventEnvelope_PurchaseComplete:
		result["purchase_complete"] = structToMap(p.PurchaseComplete)
	case *pb.EventEnvelope_PurchaseFailed:
		result["purchase_failed"] = structToMap(p.PurchaseFailed)
	case *pb.EventEnvelope_AppStart:
		result["app_start"] = structToMap(p.AppStart)
	case *pb.EventEnvelope_AppBackground:
		result["app_background"] = structToMap(p.AppBackground)
	case *pb.EventEnvelope_AppForeground:
		result["app_foreground"] = structToMap(p.AppForeground)
	case *pb.EventEnvelope_AppCrash:
		result["app_crash"] = structToMap(p.AppCrash)
	case *pb.EventEnvelope_NetworkChange:
		result["network_change"] = structToMap(p.NetworkChange)
	case *pb.EventEnvelope_PermissionRequest:
		result["permission_request"] = structToMap(p.PermissionRequest)
	case *pb.EventEnvelope_PermissionResult:
		result["permission_result"] = structToMap(p.PermissionResult)
	case *pb.EventEnvelope_MemoryWarning:
		result["memory_warning"] = structToMap(p.MemoryWarning)
	case *pb.EventEnvelope_BatteryChange:
		result["battery_change"] = structToMap(p.BatteryChange)
	case *pb.EventEnvelope_CustomEvent:
		result["custom_event"] = structToMap(p.CustomEvent)
	}

	return result, nil
}

// structToMap converts a protobuf struct to a map via JSON marshaling.
func structToMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return result
}
