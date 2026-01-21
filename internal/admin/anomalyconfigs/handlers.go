// Package anomalyconfigs provides the admin anomaly configs handler.
package anomalyconfigs

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/SebastienMelki/causality/internal/admin/shared"
	"github.com/SebastienMelki/causality/internal/admin/templates"
)

// AnomalyConfig represents an anomaly config from the database.
type AnomalyConfig struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	AppID           sql.NullString  `json:"app_id"`
	EventCategory   sql.NullString  `json:"event_category"`
	EventType       sql.NullString  `json:"event_type"`
	DetectionType   string          `json:"detection_type"`
	Config          json.RawMessage `json:"config"`
	CooldownSeconds int             `json:"cooldown_seconds"`
	Enabled         bool            `json:"enabled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ListData holds data for the anomaly configs list page.
type ListData struct {
	Configs []AnomalyConfig
}

// FormData holds data for the anomaly config form.
type FormData struct {
	Config AnomalyConfig
	IsEdit bool
	Errors map[string]string
}

// Handler handles anomaly config requests.
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewHandler creates a new anomaly configs handler.
func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With("handler", "anomaly-configs"),
	}
}

func toTemplateAnomalyConfig(cfg AnomalyConfig) templates.AnomalyConfig {
	return templates.AnomalyConfig{
		ID:            cfg.ID,
		Name:          cfg.Name,
		Description:   cfg.Description,
		DetectionType: cfg.DetectionType,
		AppID:         cfg.AppID,
		EventCategory: cfg.EventCategory,
		EventType:     cfg.EventType,
		Config:        cfg.Config,
		Actions:       []byte("{}"),
		WindowSizeMs:  60000,
		CooldownMs:    cfg.CooldownSeconds * 1000,
		Enabled:       cfg.Enabled,
	}
}

func toTemplateAnomalyConfigs(configs []AnomalyConfig) []templates.AnomalyConfig {
	result := make([]templates.AnomalyConfig, len(configs))
	for i, cfg := range configs {
		result[i] = toTemplateAnomalyConfig(cfg)
	}
	return result
}

// List displays all anomaly configs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	configs, err := h.listConfigs(r)
	if err != nil {
		h.logger.Error("failed to list anomaly configs", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	component := templates.AnomalyConfigsListPage(toTemplateAnomalyConfigs(configs))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render anomaly configs list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// New displays the new anomaly config form.
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	component := templates.AnomalyConfigFormPage(templates.AnomalyConfigFormData{
		Config: templates.AnomalyConfig{
			DetectionType: "threshold",
			Config:        []byte("{}"),
			Actions:       []byte("{}"),
			WindowSizeMs:  60000,
			CooldownMs:    300000,
			Enabled:       true,
		},
		IsEdit: false,
		Errors: make(map[string]string),
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render new anomaly config form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Create creates a new anomaly config.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	config := h.parseConfigForm(r)
	errs := h.validateConfig(config)

	if len(errs) > 0 {
		component := templates.AnomalyConfigFormPage(templates.AnomalyConfigFormData{
			Config: toTemplateAnomalyConfig(config),
			IsEdit: false,
			Errors: errs,
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		component.Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO anomaly_configs (name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9)
	`, config.Name, config.Description, config.AppID.String, config.EventCategory.String, config.EventType.String,
		config.DetectionType, config.Config, config.CooldownSeconds, config.Enabled)

	if err != nil {
		h.logger.Error("failed to create anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Anomaly config created successfully")
	w.Header().Set("HX-Redirect", "/admin/anomaly-configs")
	w.WriteHeader(http.StatusOK)
}

// Edit displays the edit anomaly config form.
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	config, err := h.getConfig(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	component := templates.AnomalyConfigFormPage(templates.AnomalyConfigFormData{
		Config: toTemplateAnomalyConfig(config),
		IsEdit: true,
		Errors: make(map[string]string),
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render edit anomaly config form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Update updates an anomaly config.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	config := h.parseConfigForm(r)
	config.ID = id
	errs := h.validateConfig(config)

	if len(errs) > 0 {
		component := templates.AnomalyConfigFormPage(templates.AnomalyConfigFormData{
			Config: toTemplateAnomalyConfig(config),
			IsEdit: true,
			Errors: errs,
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		component.Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE anomaly_configs SET
			name = $1, description = $2, app_id = NULLIF($3, ''), event_category = NULLIF($4, ''),
			event_type = NULLIF($5, ''), detection_type = $6, config = $7, cooldown_seconds = $8, enabled = $9
		WHERE id = $10
	`, config.Name, config.Description, config.AppID.String, config.EventCategory.String, config.EventType.String,
		config.DetectionType, config.Config, config.CooldownSeconds, config.Enabled, id)

	if err != nil {
		h.logger.Error("failed to update anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Anomaly config updated successfully")
	w.Header().Set("HX-Redirect", "/admin/anomaly-configs")
	w.WriteHeader(http.StatusOK)
}

// Delete deletes an anomaly config.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), "DELETE FROM anomaly_configs WHERE id = $1", id)
	if err != nil {
		h.logger.Error("failed to delete anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Anomaly config deleted successfully")
	w.Header().Set("HX-Redirect", "/admin/anomaly-configs")
	w.WriteHeader(http.StatusOK)
}

// Toggle toggles an anomaly config's enabled status.
func (h *Handler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), "UPDATE anomaly_configs SET enabled = NOT enabled WHERE id = $1", id)
	if err != nil {
		h.logger.Error("failed to toggle anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated row
	config, err := h.getConfig(r, id)
	if err != nil {
		h.logger.Error("failed to get updated anomaly config", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	component := templates.AnomalyConfigRow(toTemplateAnomalyConfig(config))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render anomaly config row", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) listConfigs(r *http.Request) ([]AnomalyConfig, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []AnomalyConfig
	for rows.Next() {
		var cfg AnomalyConfig
		if err := rows.Scan(&cfg.ID, &cfg.Name, &cfg.Description, &cfg.AppID, &cfg.EventCategory,
			&cfg.EventType, &cfg.DetectionType, &cfg.Config, &cfg.CooldownSeconds, &cfg.Enabled,
			&cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (h *Handler) getConfig(r *http.Request, id string) (AnomalyConfig, error) {
	var cfg AnomalyConfig
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, description, app_id, event_category, event_type, detection_type, config, cooldown_seconds, enabled, created_at, updated_at
		FROM anomaly_configs WHERE id = $1
	`, id).Scan(&cfg.ID, &cfg.Name, &cfg.Description, &cfg.AppID, &cfg.EventCategory,
		&cfg.EventType, &cfg.DetectionType, &cfg.Config, &cfg.CooldownSeconds, &cfg.Enabled,
		&cfg.CreatedAt, &cfg.UpdatedAt)
	return cfg, err
}

func (h *Handler) parseConfigForm(r *http.Request) AnomalyConfig {
	cooldownSeconds := 300
	if c := r.FormValue("cooldown_seconds"); c != "" {
		json.Unmarshal([]byte(c), &cooldownSeconds)
	}

	config := r.FormValue("config")
	if config == "" {
		config = "{}"
	}

	return AnomalyConfig{
		Name:            r.FormValue("name"),
		Description:     r.FormValue("description"),
		AppID:           sql.NullString{String: r.FormValue("app_id"), Valid: r.FormValue("app_id") != ""},
		EventCategory:   sql.NullString{String: r.FormValue("event_category"), Valid: r.FormValue("event_category") != ""},
		EventType:       sql.NullString{String: r.FormValue("event_type"), Valid: r.FormValue("event_type") != ""},
		DetectionType:   r.FormValue("detection_type"),
		Config:          json.RawMessage(config),
		CooldownSeconds: cooldownSeconds,
		Enabled:         r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true",
	}
}

func (h *Handler) validateConfig(config AnomalyConfig) map[string]string {
	errors := make(map[string]string)

	if config.Name == "" {
		errors["name"] = "Name is required"
	}
	if config.DetectionType == "" {
		errors["detection_type"] = "Detection type is required"
	}

	// Validate JSON
	var temp interface{}
	if err := json.Unmarshal(config.Config, &temp); err != nil {
		errors["config"] = "Config must be valid JSON"
	}

	return errors
}
