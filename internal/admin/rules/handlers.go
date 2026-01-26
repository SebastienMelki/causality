// Package rules provides the admin rules handler.
package rules

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

// Rule represents a rule from the database.
type Rule struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	AppID         sql.NullString  `json:"app_id"`
	EventCategory sql.NullString  `json:"event_category"`
	EventType     sql.NullString  `json:"event_type"`
	Conditions    json.RawMessage `json:"conditions"`
	Actions       json.RawMessage `json:"actions"`
	Priority      int             `json:"priority"`
	Enabled       bool            `json:"enabled"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Webhook represents a webhook for the rule form.
type Webhook struct {
	ID   string
	Name string
}

// Handler handles rule requests.
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewHandler creates a new rules handler.
func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With("handler", "rules"),
	}
}

func toTemplateRule(r Rule) templates.Rule {
	return templates.Rule{
		ID:            r.ID,
		Name:          r.Name,
		Description:   r.Description,
		AppID:         r.AppID,
		EventCategory: r.EventCategory,
		EventType:     r.EventType,
		Conditions:    r.Conditions,
		Actions:       r.Actions,
		Priority:      r.Priority,
		Enabled:       r.Enabled,
	}
}

func toTemplateRules(rules []Rule) []templates.Rule {
	result := make([]templates.Rule, len(rules))
	for i, r := range rules {
		result[i] = toTemplateRule(r)
	}
	return result
}

func toTemplateWebhooks(webhooks []Webhook) []templates.Webhook {
	result := make([]templates.Webhook, len(webhooks))
	for i, w := range webhooks {
		result[i] = templates.Webhook{ID: w.ID, Name: w.Name}
	}
	return result
}

// List displays all rules.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.listRules(r)
	if err != nil {
		h.logger.Error("failed to list rules", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	component := templates.RulesListPage(toTemplateRules(rules))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render rules list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// New displays the new rule form.
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	webhooks, _ := h.listWebhooks(r)

	component := templates.RuleFormPage(templates.RuleFormData{
		Rule:     templates.Rule{Conditions: []byte("[]"), Actions: []byte("{}")},
		Webhooks: toTemplateWebhooks(webhooks),
		IsEdit:   false,
		Errors:   make(map[string]string),
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render new rule form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Create creates a new rule.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rule := h.parseRuleForm(r)
	errs := h.validateRule(rule)

	if len(errs) > 0 {
		webhooks, _ := h.listWebhooks(r)
		component := templates.RuleFormPage(templates.RuleFormData{
			Rule:     toTemplateRule(rule),
			Webhooks: toTemplateWebhooks(webhooks),
			IsEdit:   false,
			Errors:   errs,
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		component.Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		INSERT INTO rules (name, description, app_id, event_category, event_type, conditions, actions, priority, enabled)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9)
	`, rule.Name, rule.Description, rule.AppID.String, rule.EventCategory.String, rule.EventType.String,
		rule.Conditions, rule.Actions, rule.Priority, rule.Enabled)

	if err != nil {
		h.logger.Error("failed to create rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Rule created successfully")
	w.Header().Set("HX-Redirect", "/admin/rules")
	w.WriteHeader(http.StatusOK)
}

// Edit displays the edit rule form.
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rule, err := h.getRule(r, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	webhooks, _ := h.listWebhooks(r)

	component := templates.RuleFormPage(templates.RuleFormData{
		Rule:     toTemplateRule(rule),
		Webhooks: toTemplateWebhooks(webhooks),
		IsEdit:   true,
		Errors:   make(map[string]string),
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render edit rule form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Update updates a rule.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rule := h.parseRuleForm(r)
	rule.ID = id
	errs := h.validateRule(rule)

	if len(errs) > 0 {
		webhooks, _ := h.listWebhooks(r)
		component := templates.RuleFormPage(templates.RuleFormData{
			Rule:     toTemplateRule(rule),
			Webhooks: toTemplateWebhooks(webhooks),
			IsEdit:   true,
			Errors:   errs,
		})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		component.Render(r.Context(), w)
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE rules SET
			name = $1, description = $2, app_id = NULLIF($3, ''), event_category = NULLIF($4, ''),
			event_type = NULLIF($5, ''), conditions = $6, actions = $7, priority = $8, enabled = $9
		WHERE id = $10
	`, rule.Name, rule.Description, rule.AppID.String, rule.EventCategory.String, rule.EventType.String,
		rule.Conditions, rule.Actions, rule.Priority, rule.Enabled, id)

	if err != nil {
		h.logger.Error("failed to update rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Rule updated successfully")
	w.Header().Set("HX-Redirect", "/admin/rules")
	w.WriteHeader(http.StatusOK)
}

// Delete deletes a rule.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), "DELETE FROM rules WHERE id = $1", id)
	if err != nil {
		h.logger.Error("failed to delete rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Rule deleted successfully")
	w.Header().Set("HX-Redirect", "/admin/rules")
	w.WriteHeader(http.StatusOK)
}

// Toggle toggles a rule's enabled status.
func (h *Handler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.db.ExecContext(r.Context(), "UPDATE rules SET enabled = NOT enabled WHERE id = $1", id)
	if err != nil {
		h.logger.Error("failed to toggle rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Return updated row
	rule, err := h.getRule(r, id)
	if err != nil {
		h.logger.Error("failed to get updated rule", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	component := templates.RuleRow(toTemplateRule(rule))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render rule row", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) listRules(r *http.Request) ([]Rule, error) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules ORDER BY priority DESC, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.AppID, &rule.EventCategory,
			&rule.EventType, &rule.Conditions, &rule.Actions, &rule.Priority, &rule.Enabled,
			&rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (h *Handler) getRule(r *http.Request, id string) (Rule, error) {
	var rule Rule
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, description, app_id, event_category, event_type, conditions, actions, priority, enabled, created_at, updated_at
		FROM rules WHERE id = $1
	`, id).Scan(&rule.ID, &rule.Name, &rule.Description, &rule.AppID, &rule.EventCategory,
		&rule.EventType, &rule.Conditions, &rule.Actions, &rule.Priority, &rule.Enabled,
		&rule.CreatedAt, &rule.UpdatedAt)
	return rule, err
}

func (h *Handler) listWebhooks(r *http.Request) ([]Webhook, error) {
	rows, err := h.db.QueryContext(r.Context(), "SELECT id, name FROM webhooks WHERE enabled = true ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var wh Webhook
		if err := rows.Scan(&wh.ID, &wh.Name); err != nil {
			return nil, err
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

func (h *Handler) parseRuleForm(r *http.Request) Rule {
	priority := 0
	if p := r.FormValue("priority"); p != "" {
		json.Unmarshal([]byte(p), &priority)
	}

	conditions := r.FormValue("conditions")
	if conditions == "" {
		conditions = "[]"
	}

	actions := r.FormValue("actions")
	if actions == "" {
		actions = "{}"
	}

	return Rule{
		Name:          r.FormValue("name"),
		Description:   r.FormValue("description"),
		AppID:         sql.NullString{String: r.FormValue("app_id"), Valid: r.FormValue("app_id") != ""},
		EventCategory: sql.NullString{String: r.FormValue("event_category"), Valid: r.FormValue("event_category") != ""},
		EventType:     sql.NullString{String: r.FormValue("event_type"), Valid: r.FormValue("event_type") != ""},
		Conditions:    json.RawMessage(conditions),
		Actions:       json.RawMessage(actions),
		Priority:      priority,
		Enabled:       r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true",
	}
}

func (h *Handler) validateRule(rule Rule) map[string]string {
	errs := make(map[string]string)

	if rule.Name == "" {
		errs["name"] = "Name is required"
	}

	// Validate JSON
	var temp interface{}
	if err := json.Unmarshal(rule.Conditions, &temp); err != nil {
		errs["conditions"] = "Conditions must be valid JSON"
	}
	if err := json.Unmarshal(rule.Actions, &temp); err != nil {
		errs["actions"] = "Actions must be valid JSON"
	}

	return errs
}
