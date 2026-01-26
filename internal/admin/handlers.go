package admin

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/SebastienMelki/causality/internal/admin/anomalyconfigs"
	"github.com/SebastienMelki/causality/internal/admin/customevents"
	"github.com/SebastienMelki/causality/internal/admin/dashboard"
	"github.com/SebastienMelki/causality/internal/admin/events"
	"github.com/SebastienMelki/causality/internal/admin/rules"
	"github.com/SebastienMelki/causality/internal/admin/shared"
	"github.com/SebastienMelki/causality/internal/admin/webhooks"
)

// TrinoConfig is an alias for events.TrinoConfig for external use.
type TrinoConfig = events.TrinoConfig

// Handler handles admin UI requests.
type Handler struct {
	db     *sql.DB
	trino  *events.TrinoClient
	logger *slog.Logger
}

// NewHandler creates a new admin handler.
func NewHandler(db *sql.DB, trinoCfg events.TrinoConfig, logger *slog.Logger) (*Handler, error) {
	trinoClient, err := events.NewTrinoClient(trinoCfg)
	if err != nil {
		logger.Warn("failed to create Trino client, event browser will be disabled", "error", err)
	}

	return &Handler{
		db:     db,
		trino:  trinoClient,
		logger: logger.With("component", "admin"),
	}, nil
}

// RegisterRoutes registers admin routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Static files
	mux.Handle("GET /static/", shared.StaticHandler())

	// Dashboard
	dashboardHandler := dashboard.NewHandler(h.db, h.logger)
	mux.HandleFunc("GET /admin/", dashboardHandler.Index)
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// Rules
	rulesHandler := rules.NewHandler(h.db, h.logger)
	mux.HandleFunc("GET /admin/rules", rulesHandler.List)
	mux.HandleFunc("GET /admin/rules/new", rulesHandler.New)
	mux.HandleFunc("POST /admin/rules", rulesHandler.Create)
	mux.HandleFunc("GET /admin/rules/{id}/edit", rulesHandler.Edit)
	mux.HandleFunc("PUT /admin/rules/{id}", rulesHandler.Update)
	mux.HandleFunc("DELETE /admin/rules/{id}", rulesHandler.Delete)
	mux.HandleFunc("POST /admin/rules/{id}/toggle", rulesHandler.Toggle)

	// Webhooks
	webhooksHandler := webhooks.NewHandler(h.db, h.logger)
	mux.HandleFunc("GET /admin/webhooks", webhooksHandler.List)
	mux.HandleFunc("GET /admin/webhooks/new", webhooksHandler.New)
	mux.HandleFunc("POST /admin/webhooks", webhooksHandler.Create)
	mux.HandleFunc("GET /admin/webhooks/{id}/edit", webhooksHandler.Edit)
	mux.HandleFunc("PUT /admin/webhooks/{id}", webhooksHandler.Update)
	mux.HandleFunc("DELETE /admin/webhooks/{id}", webhooksHandler.Delete)
	mux.HandleFunc("POST /admin/webhooks/{id}/test", webhooksHandler.Test)

	// Anomaly Configs
	anomalyHandler := anomalyconfigs.NewHandler(h.db, h.logger)
	mux.HandleFunc("GET /admin/anomaly-configs", anomalyHandler.List)
	mux.HandleFunc("GET /admin/anomaly-configs/new", anomalyHandler.New)
	mux.HandleFunc("POST /admin/anomaly-configs", anomalyHandler.Create)
	mux.HandleFunc("GET /admin/anomaly-configs/{id}/edit", anomalyHandler.Edit)
	mux.HandleFunc("PUT /admin/anomaly-configs/{id}", anomalyHandler.Update)
	mux.HandleFunc("DELETE /admin/anomaly-configs/{id}", anomalyHandler.Delete)
	mux.HandleFunc("POST /admin/anomaly-configs/{id}/toggle", anomalyHandler.Toggle)

	// Events Browser
	if h.trino != nil {
		eventsHandler := events.NewHandler(h.trino, h.logger)
		mux.HandleFunc("GET /admin/events", eventsHandler.List)
		mux.HandleFunc("GET /admin/api/events", eventsHandler.Query)
	}

	// Custom Event Types
	customEventsHandler := customevents.NewHandler(h.db, h.logger)
	mux.HandleFunc("GET /admin/custom-events", customEventsHandler.List)
	mux.HandleFunc("GET /admin/custom-events/new", customEventsHandler.New)
	mux.HandleFunc("POST /admin/custom-events", customEventsHandler.Create)
	mux.HandleFunc("GET /admin/custom-events/{id}/edit", customEventsHandler.Edit)
	mux.HandleFunc("PUT /admin/custom-events/{id}", customEventsHandler.Update)
	mux.HandleFunc("DELETE /admin/custom-events/{id}", customEventsHandler.Delete)
}
