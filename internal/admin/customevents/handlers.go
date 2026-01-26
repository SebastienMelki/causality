package customevents

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/SebastienMelki/causality/internal/admin/shared"
	"github.com/SebastienMelki/causality/internal/admin/templates"
)

// Handler handles custom event type requests.
type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

// NewHandler creates a new custom event types handler.
func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{
		repo:   NewRepository(db),
		logger: logger.With("handler", "custom-events"),
	}
}

// List displays all custom event types.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	types, err := h.repo.List(r.Context())
	if err != nil {
		h.logger.Error("failed to list custom event types", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Convert to templates.CustomEventType
	templTypes := make([]templates.CustomEventType, len(types))
	for i, t := range types {
		templTypes[i] = toTemplateType(t)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.CustomEventsListPage(templTypes).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render custom event types list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// New displays the new custom event type form.
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	data := templates.CustomEventFormData{
		EventType: templates.CustomEventType{
			Category: "custom",
			Schema:   []byte("{}"),
		},
		IsEdit: false,
		Errors: make(map[string]string),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.CustomEventFormPage(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render new custom event type form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Create creates a new custom event type.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	eventType := h.parseForm(r)
	validationErrors := h.validate(eventType)

	if len(validationErrors) > 0 {
		data := templates.CustomEventFormData{
			EventType: toTemplateType(eventType),
			IsEdit:    false,
			Errors:    validationErrors,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		templates.CustomEventFormPage(data).Render(r.Context(), w)
		return
	}

	if err := h.repo.Create(r.Context(), eventType); err != nil {
		h.logger.Error("failed to create custom event type", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Custom event type created successfully")
	w.Header().Set("HX-Redirect", "/admin/custom-events")
	w.WriteHeader(http.StatusOK)
}

// Edit displays the edit custom event type form.
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	eventType, err := h.repo.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("failed to get custom event type", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := templates.CustomEventFormData{
		EventType: toTemplateType(eventType),
		IsEdit:    true,
		Errors:    make(map[string]string),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.CustomEventFormPage(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render edit custom event type form", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Update updates a custom event type.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	eventType := h.parseForm(r)
	eventType.ID = id
	validationErrors := h.validate(eventType)

	if len(validationErrors) > 0 {
		data := templates.CustomEventFormData{
			EventType: toTemplateType(eventType),
			IsEdit:    true,
			Errors:    validationErrors,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		templates.CustomEventFormPage(data).Render(r.Context(), w)
		return
	}

	if err := h.repo.Update(r.Context(), eventType); err != nil {
		h.logger.Error("failed to update custom event type", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Custom event type updated successfully")
	w.Header().Set("HX-Redirect", "/admin/custom-events")
	w.WriteHeader(http.StatusOK)
}

// Delete deletes a custom event type.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.logger.Error("failed to delete custom event type", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shared.SetSuccessFlash(w, "Custom event type deleted successfully")
	w.Header().Set("HX-Redirect", "/admin/custom-events")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) parseForm(r *http.Request) CustomEventType {
	schema := r.FormValue("schema")
	if schema == "" {
		schema = "{}"
	}

	return CustomEventType{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Category:    r.FormValue("category"),
		Schema:      json.RawMessage(schema),
	}
}

func (h *Handler) validate(eventType CustomEventType) map[string]string {
	validationErrors := make(map[string]string)

	if eventType.Name == "" {
		validationErrors["name"] = "Name is required"
	}
	if eventType.Category == "" {
		validationErrors["category"] = "Category is required"
	}

	// Validate JSON schema
	var temp interface{}
	if err := json.Unmarshal(eventType.Schema, &temp); err != nil {
		validationErrors["schema"] = "Schema must be valid JSON"
	}

	return validationErrors
}

// toTemplateType converts a local CustomEventType to templates.CustomEventType.
func toTemplateType(t CustomEventType) templates.CustomEventType {
	return templates.CustomEventType{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Category:    t.Category,
		Schema:      t.Schema,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
