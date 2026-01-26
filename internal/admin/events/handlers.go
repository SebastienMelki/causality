package events

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/SebastienMelki/causality/internal/admin/templates"
)

// Handler handles event browser requests.
type Handler struct {
	trino  *TrinoClient
	logger *slog.Logger
}

// NewHandler creates a new events handler.
func NewHandler(trino *TrinoClient, logger *slog.Logger) *Handler {
	return &Handler{
		trino:  trino,
		logger: logger.With("handler", "events"),
	}
}

// List displays the events browser.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filter := h.parseFilter(r)

	// Get filter options
	appIDs, _ := h.trino.GetDistinctValues(r.Context(), "app_id")
	categories, _ := h.trino.GetDistinctValues(r.Context(), "event_category")
	eventTypes, _ := h.trino.GetDistinctValues(r.Context(), "event_type")

	// Query events
	result, err := h.trino.QueryEvents(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query events", "error", err)
		result = &QueryResult{}
	}

	data := templates.EventsPageData{
		Events:     convertEvents(result.Events),
		HasMore:    result.HasMore,
		Filter:     convertFilter(filter),
		AppIDs:     appIDs,
		Categories: categories,
		EventTypes: eventTypes,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.EventsListPage(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render events list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Query returns filtered events as an HTML partial.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	filter := h.parseFilter(r)

	result, err := h.trino.QueryEvents(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query events", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := templates.EventsPageData{
		Events:  convertEvents(result.Events),
		HasMore: result.HasMore,
		Filter:  convertFilter(filter),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.EventsTable(data).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render events table", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) parseFilter(r *http.Request) EventFilter {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	return EventFilter{
		AppID:         r.URL.Query().Get("app_id"),
		EventCategory: r.URL.Query().Get("event_category"),
		EventType:     r.URL.Query().Get("event_type"),
		StartDate:     r.URL.Query().Get("start_date"),
		EndDate:       r.URL.Query().Get("end_date"),
		Limit:         limit,
		Offset:        offset,
	}
}

// convertEvents converts trino Events to templates Events.
func convertEvents(events []Event) []templates.Event {
	result := make([]templates.Event, len(events))
	for i, e := range events {
		result[i] = templates.Event{
			ID:            e.ID,
			AppID:         e.AppID,
			EventType:     e.EventType,
			EventCategory: e.EventCategory,
			DeviceID:      e.DeviceID,
			Platform:      e.Platform,
			Timestamp:     e.Timestamp,
			Parameters:    e.Parameters,
		}
	}
	return result
}

// convertFilter converts EventFilter to templates.EventFilter.
func convertFilter(f EventFilter) templates.EventFilter {
	return templates.EventFilter{
		AppID:         f.AppID,
		EventCategory: f.EventCategory,
		EventType:     f.EventType,
		StartDate:     f.StartDate,
		EndDate:       f.EndDate,
		Limit:         f.Limit,
		Offset:        f.Offset,
	}
}
