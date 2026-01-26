// Package webhooks provides the admin webhooks handler.
package webhooks

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/SebastienMelki/causality/internal/admin/shared"
	"github.com/SebastienMelki/causality/internal/admin/templates"
)

// Webhook represents a webhook from the database.
type Webhook struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	URL        string          `json:"url"`
	AuthType   string          `json:"auth_type"`
	AuthConfig json.RawMessage `json:"auth_config"`
	Headers    json.RawMessage `json:"headers"`
	Enabled    bool            `json:"enabled"`
	TimeoutMs  int             `json:"timeout_ms"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ListData holds data for the webhooks list page.
type ListData struct {
	Webhooks []Webhook
}

// FormData holds data for the webhook form.
type FormData struct {
	Webhook Webhook
	IsEdit  bool
	Errors  map[string]string
}

// Handler handles webhook requests.
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewHandler creates a new webhooks handler.
func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With("handler", "webhooks"),
	}
}

// List displays all webhooks.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.listWebhooks(r)
	if err != nil {
		h.logger.Error("failed to list webhooks", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templWebhooks := convertToTemplWebhooks(webhooks)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.WebhooksListPage(templWebhooks).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render webhooks list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// New displays the new webhook form.
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	data := templates.WebhookFormData{
		Webhook: templates.Webhook{
			AuthType:   "none",
			AuthConfig: []byte("{}"),
			Headers:    []byte("{}"),
			TimeoutMs:  30000,
			Enabled:    true,
		},
		IsEdit: false,
		Errors: make(map[string]string),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.WebhookFormPage(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render new webhook form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Create creates a new webhook.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	webhook := h.parseWebhookForm(r)
	validationErrors := h.validateWebhook(webhook)

	if len(validationErrors) > 0 {
		data := templates.WebhookFormData{
			Webhook: convertToTemplWebhook(webhook),
			IsEdit:  false,
			Errors:  validationErrors,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		templates.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO webhooks (name, url, auth_type, auth_config, headers, timeout_ms, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, webhook.Name, webhook.URL, webhook.AuthType, webhook.AuthConfig, webhook.Headers, webhook.TimeoutMs, webhook.Enabled)

	if err != nil {
		h.logger.Error("failed to create webhook", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Webhook created successfully")
	w.Header().Set("HX-Redirect", "/admin/webhooks")
	w.WriteHeader(http.StatusOK)
}

// Edit displays the edit webhook form.
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	webhook, err := h.getWebhook(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get webhook", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := templates.WebhookFormData{
		Webhook: convertToTemplWebhook(webhook),
		IsEdit:  true,
		Errors:  make(map[string]string),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.WebhookFormPage(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render edit webhook form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Update updates a webhook.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	webhook := h.parseWebhookForm(r)
	webhook.ID = id
	validationErrors := h.validateWebhook(webhook)

	if len(validationErrors) > 0 {
		data := templates.WebhookFormData{
			Webhook: convertToTemplWebhook(webhook),
			IsEdit:  true,
			Errors:  validationErrors,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		templates.WebhookFormPage(data).Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE webhooks SET
			name = $1, url = $2, auth_type = $3, auth_config = $4, headers = $5, timeout_ms = $6, enabled = $7
		WHERE id = $8
	`, webhook.Name, webhook.URL, webhook.AuthType, webhook.AuthConfig, webhook.Headers, webhook.TimeoutMs, webhook.Enabled, id)

	if err != nil {
		h.logger.Error("failed to update webhook", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Webhook updated successfully")
	w.Header().Set("HX-Redirect", "/admin/webhooks")
	w.WriteHeader(http.StatusOK)
}

// Delete deletes a webhook.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), "DELETE FROM webhooks WHERE id = $1", id)
	if err != nil {
		h.logger.Error("failed to delete webhook", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Webhook deleted successfully")
	w.Header().Set("HX-Redirect", "/admin/webhooks")
	w.WriteHeader(http.StatusOK)
}

// Test sends a test request to the webhook.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	webhook, err := h.getWebhook(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get webhook", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Send test request
	testPayload := map[string]interface{}{
		"test":      true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"message":   "Test webhook from Causality Admin",
	}
	payloadBytes, _ := json.Marshal(testPayload)

	client := &http.Client{Timeout: time.Duration(webhook.TimeoutMs) * time.Millisecond}
	req, err := http.NewRequestWithContext(r.Context(), "POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		shared.SetErrorFlash(w, "Failed to create request: "+err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Add auth headers
	h.addAuthHeaders(req, webhook)

	// Add custom headers
	var headers map[string]string
	json.Unmarshal(webhook.Headers, &headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		shared.SetErrorFlash(w, "Request failed: "+err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		shared.SetSuccessFlash(w, "Test successful! Status: "+resp.Status)
	} else {
		shared.SetErrorFlash(w, "Test failed. Status: "+resp.Status)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) addAuthHeaders(req *http.Request, webhook Webhook) {
	var authConfig map[string]string
	json.Unmarshal(webhook.AuthConfig, &authConfig)

	switch webhook.AuthType {
	case "basic":
		req.SetBasicAuth(authConfig["username"], authConfig["password"])
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+authConfig["token"])
	case "hmac":
		// HMAC signing would be done here with the payload
	}
}

func (h *Handler) listWebhooks(r *http.Request) ([]Webhook, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, url, auth_type, auth_config, headers, timeout_ms, enabled, created_at, updated_at
		FROM webhooks ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var wh Webhook
		if err := rows.Scan(&wh.ID, &wh.Name, &wh.URL, &wh.AuthType, &wh.AuthConfig, &wh.Headers,
			&wh.TimeoutMs, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

func (h *Handler) getWebhook(r *http.Request, id string) (Webhook, error) {
	var wh Webhook
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, url, auth_type, auth_config, headers, timeout_ms, enabled, created_at, updated_at
		FROM webhooks WHERE id = $1
	`, id).Scan(&wh.ID, &wh.Name, &wh.URL, &wh.AuthType, &wh.AuthConfig, &wh.Headers,
		&wh.TimeoutMs, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt)
	return wh, err
}

func (h *Handler) parseWebhookForm(r *http.Request) Webhook {
	timeoutMs := 30000
	if t := r.FormValue("timeout_ms"); t != "" {
		json.Unmarshal([]byte(t), &timeoutMs)
	}

	authConfig := r.FormValue("auth_config")
	if authConfig == "" {
		authConfig = "{}"
	}

	headers := r.FormValue("headers")
	if headers == "" {
		headers = "{}"
	}

	return Webhook{
		Name:       r.FormValue("name"),
		URL:        r.FormValue("url"),
		AuthType:   r.FormValue("auth_type"),
		AuthConfig: json.RawMessage(authConfig),
		Headers:    json.RawMessage(headers),
		TimeoutMs:  timeoutMs,
		Enabled:    r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true",
	}
}

func (h *Handler) validateWebhook(webhook Webhook) map[string]string {
	errs := make(map[string]string)

	if webhook.Name == "" {
		errs["name"] = "Name is required"
	}
	if webhook.URL == "" {
		errs["url"] = "URL is required"
	}

	// Validate JSON
	var temp interface{}
	if err := json.Unmarshal(webhook.AuthConfig, &temp); err != nil {
		errs["auth_config"] = "Auth config must be valid JSON"
	}
	if err := json.Unmarshal(webhook.Headers, &temp); err != nil {
		errs["headers"] = "Headers must be valid JSON"
	}

	return errs
}

// convertToTemplWebhook converts a local Webhook to templates.Webhook.
func convertToTemplWebhook(wh Webhook) templates.Webhook {
	return templates.Webhook{
		ID:         wh.ID,
		Name:       wh.Name,
		URL:        wh.URL,
		AuthType:   wh.AuthType,
		AuthConfig: wh.AuthConfig,
		Headers:    wh.Headers,
		TimeoutMs:  wh.TimeoutMs,
		Enabled:    wh.Enabled,
	}
}

// convertToTemplWebhooks converts a slice of local Webhooks to templates.Webhook.
func convertToTemplWebhooks(webhooks []Webhook) []templates.Webhook {
	result := make([]templates.Webhook, len(webhooks))
	for i, wh := range webhooks {
		result[i] = convertToTemplWebhook(wh)
	}
	return result
}
