package reaction

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/SebastienMelki/causality/internal/reaction/db"
)

// Dispatcher handles webhook delivery with retries.
type Dispatcher struct {
	deliveries *db.DeliveryRepository
	webhooks   *db.WebhookRepository
	config     DispatcherConfig
	logger     *slog.Logger
	httpClient *http.Client

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(
	deliveries *db.DeliveryRepository,
	webhooks *db.WebhookRepository,
	config DispatcherConfig,
	logger *slog.Logger,
) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &Dispatcher{
		deliveries: deliveries,
		webhooks:   webhooks,
		config:     config,
		logger:     logger.With("component", "reaction-dispatcher"),
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start starts the dispatcher workers.
func (d *Dispatcher) Start(ctx context.Context) {
	var wg sync.WaitGroup

	// Start workers
	for i := range d.config.Workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			d.worker(ctx, workerID)
		}(i)
	}

	d.logger.Info("dispatcher started", "workers", d.config.Workers)

	// Wait for stop signal then wait for workers
	go func() {
		<-d.stopCh
		wg.Wait()
		close(d.doneCh)
	}()
}

// Stop stops the dispatcher.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	<-d.doneCh
	d.logger.Info("dispatcher stopped")
}

// worker is a single delivery worker.
func (d *Dispatcher) worker(ctx context.Context, workerID int) {
	d.logger.Debug("worker started", "worker_id", workerID)

	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.processDeliveries(ctx)
		}
	}
}

// processDeliveries fetches and processes pending deliveries.
func (d *Dispatcher) processDeliveries(ctx context.Context) {
	deliveries, err := d.deliveries.GetPending(ctx, d.config.BatchSize)
	if err != nil {
		d.logger.Error("failed to get pending deliveries", "error", err)
		return
	}

	for _, delivery := range deliveries {
		if err := d.processDelivery(ctx, delivery); err != nil {
			d.logger.Error("failed to process delivery",
				"delivery_id", delivery.ID,
				"error", err,
			)
		}
	}
}

// processDelivery processes a single delivery.
func (d *Dispatcher) processDelivery(ctx context.Context, delivery *db.WebhookDelivery) error {
	// Mark as in progress
	if err := d.deliveries.MarkInProgress(ctx, delivery.ID); err != nil {
		return fmt.Errorf("failed to mark in progress: %w", err)
	}

	// Get webhook config
	webhook, err := d.webhooks.GetByID(ctx, delivery.WebhookID)
	if err != nil {
		errMsg := fmt.Sprintf("webhook not found: %v", err)
		nextAttempt := d.calculateNextAttempt(delivery.Attempts)
		return d.deliveries.MarkFailed(ctx, delivery.ID, nil, errMsg, nextAttempt)
	}

	if !webhook.Enabled {
		errMsg := "webhook is disabled"
		nextAttempt := d.calculateNextAttempt(delivery.Attempts)
		return d.deliveries.MarkFailed(ctx, delivery.ID, nil, errMsg, nextAttempt)
	}

	// Deliver webhook
	statusCode, err := d.deliver(ctx, webhook, delivery.Payload)
	if err != nil {
		errMsg := err.Error()
		nextAttempt := d.calculateNextAttempt(delivery.Attempts)
		d.logger.Warn("delivery failed",
			"delivery_id", delivery.ID,
			"webhook_id", webhook.ID,
			"attempt", delivery.Attempts+1,
			"error", errMsg,
			"next_attempt", nextAttempt,
		)
		return d.deliveries.MarkFailed(ctx, delivery.ID, statusCode, errMsg, nextAttempt)
	}

	// Success
	d.logger.Info("delivery successful",
		"delivery_id", delivery.ID,
		"webhook_id", webhook.ID,
		"status_code", *statusCode,
	)
	return d.deliveries.MarkDelivered(ctx, delivery.ID, *statusCode)
}

// deliver makes the HTTP request to the webhook endpoint.
func (d *Dispatcher) deliver(ctx context.Context, webhook *db.Webhook, payload []byte) (*int, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	// Add authentication
	if err := d.addAuth(req, webhook, payload); err != nil {
		return nil, fmt.Errorf("failed to add auth: %w", err)
	}

	// Make request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read body for error reporting
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	statusCode := resp.StatusCode
	if statusCode < 200 || statusCode >= 300 {
		return &statusCode, fmt.Errorf("%w: status %d, body: %s", ErrWebhookStatusError, statusCode, string(body))
	}

	return &statusCode, nil
}

// addAuth adds authentication to the request based on webhook config.
func (d *Dispatcher) addAuth(req *http.Request, webhook *db.Webhook, payload []byte) error {
	switch webhook.AuthType {
	case "none", "":
		return nil

	case "basic":
		var config BasicAuthConfig
		if err := json.Unmarshal(webhook.AuthConfig, &config); err != nil {
			return fmt.Errorf("invalid basic auth config: %w", err)
		}
		req.SetBasicAuth(config.Username, config.Password)

	case "bearer":
		var config BearerAuthConfig
		if err := json.Unmarshal(webhook.AuthConfig, &config); err != nil {
			return fmt.Errorf("invalid bearer auth config: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+config.Token)

	case "hmac":
		var config HMACAuthConfig
		if err := json.Unmarshal(webhook.AuthConfig, &config); err != nil {
			return fmt.Errorf("invalid hmac auth config: %w", err)
		}
		signature := d.computeHMAC(payload, config.Secret)
		header := config.Header
		if header == "" {
			header = "X-Signature"
		}
		req.Header.Set(header, signature)

	default:
		return fmt.Errorf("%w: %s", ErrInvalidAuthType, webhook.AuthType)
	}

	return nil
}

// computeHMAC computes HMAC-SHA256 signature.
func (d *Dispatcher) computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// calculateNextAttempt calculates the next retry time using exponential backoff.
func (d *Dispatcher) calculateNextAttempt(currentAttempts int) time.Time {
	backoff := d.config.InitialBackoff
	for range currentAttempts {
		backoff = time.Duration(float64(backoff) * d.config.BackoffMultiplier)
		if backoff > d.config.MaxBackoff {
			backoff = d.config.MaxBackoff
			break
		}
	}
	return time.Now().Add(backoff)
}
