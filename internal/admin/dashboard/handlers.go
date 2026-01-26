// Package dashboard provides the admin dashboard handler.
package dashboard

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/SebastienMelki/causality/internal/admin/templates"
)

// Handler handles dashboard requests.
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewHandler creates a new dashboard handler.
func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With("handler", "dashboard"),
	}
}

// Index displays the dashboard.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	stats := h.getStats(r)

	component := templates.DashboardPage(templates.DashboardStats{
		TotalRules:            stats.TotalRules,
		EnabledRules:          stats.EnabledRules,
		TotalWebhooks:         stats.TotalWebhooks,
		EnabledWebhooks:       stats.EnabledWebhooks,
		TotalAnomalyConfigs:   stats.TotalAnomalyConfigs,
		EnabledAnomalyConfigs: stats.EnabledAnomalyConfigs,
		TotalCustomEvents:     stats.TotalCustomEventTypes,
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Stats holds dashboard statistics.
type Stats struct {
	TotalRules            int
	EnabledRules          int
	TotalWebhooks         int
	EnabledWebhooks       int
	TotalAnomalyConfigs   int
	EnabledAnomalyConfigs int
	TotalCustomEventTypes int
}

func (h *Handler) getStats(r *http.Request) Stats {
	var stats Stats

	if h.db == nil {
		return stats
	}

	ctx := r.Context()

	// Rules stats
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules").Scan(&stats.TotalRules)
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rules WHERE enabled = true").Scan(&stats.EnabledRules)

	// Webhooks stats
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webhooks").Scan(&stats.TotalWebhooks)
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webhooks WHERE enabled = true").Scan(&stats.EnabledWebhooks)

	// Anomaly configs stats
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM anomaly_configs").Scan(&stats.TotalAnomalyConfigs)
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM anomaly_configs WHERE enabled = true").Scan(&stats.EnabledAnomalyConfigs)

	// Custom event types stats (ignore errors if table doesn't exist)
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM custom_event_types").Scan(&stats.TotalCustomEventTypes)

	return stats
}
