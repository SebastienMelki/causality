// Package handler provides HTTP handlers for admin API key management.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/SebastienMelki/causality/internal/auth/internal/service"
)

// KeyHandler handles HTTP requests for API key management (create, revoke, list).
type KeyHandler struct {
	service *service.KeyService
	logger  *slog.Logger
}

// NewKeyHandler creates a new KeyHandler with the given service and logger.
func NewKeyHandler(svc *service.KeyService, logger *slog.Logger) *KeyHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &KeyHandler{
		service: svc,
		logger:  logger.With("component", "key-handler"),
	}
}

// RegisterRoutes mounts admin key management endpoints on the given ServeMux.
//
// Endpoints:
//   - POST   /api/admin/keys       - Create a new API key
//   - DELETE  /api/admin/keys/{id}  - Revoke an API key
//   - GET     /api/admin/keys       - List API keys for an app
//
// TODO(phase-3): Protect these endpoints with session auth + RBAC.
func (h *KeyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/keys", h.handleCreate)
	mux.HandleFunc("DELETE /api/admin/keys/{id}", h.handleRevoke)
	mux.HandleFunc("GET /api/admin/keys", h.handleList)
}

// createKeyRequest is the JSON request body for creating a new API key.
type createKeyRequest struct {
	AppID string `json:"app_id"`
	Name  string `json:"name"`
}

// createKeyResponse is the JSON response for a newly created API key.
// The plaintext key is only returned once at creation time.
type createKeyResponse struct {
	ID       string `json:"id"`
	AppID    string `json:"app_id"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Messsage string `json:"message"`
}

// handleCreate handles POST /api/admin/keys - creates a new API key.
func (h *KeyHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.AppID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "app_id is required",
		})
		return
	}

	plaintext, key, err := h.service.CreateKey(r.Context(), req.AppID, req.Name)
	if err != nil {
		h.logger.Error("failed to create API key",
			"app_id", req.AppID,
			"error", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create API key",
		})
		return
	}

	writeJSON(w, http.StatusCreated, createKeyResponse{
		ID:       key.ID,
		AppID:    key.AppID,
		Name:     key.Name,
		Key:      plaintext,
		Messsage: "Store this key securely. It will not be shown again.",
	})
}

// handleRevoke handles DELETE /api/admin/keys/{id} - revokes an API key.
func (h *KeyHandler) handleRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Fallback: extract from URL path for Go < 1.22 compatibility
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 5 {
			id = parts[4]
		}
	}

	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "key id is required",
		})
		return
	}

	if err := h.service.RevokeKey(r.Context(), id); err != nil {
		h.logger.Error("failed to revoke API key",
			"key_id", id,
			"error", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to revoke API key",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "revoked",
		"id":     id,
	})
}

// handleList handles GET /api/admin/keys?app_id={app_id} - lists API keys.
func (h *KeyHandler) handleList(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "app_id query parameter is required",
		})
		return
	}

	keys, err := h.service.ListKeys(r.Context(), appID)
	if err != nil {
		h.logger.Error("failed to list API keys",
			"app_id", appID,
			"error", err,
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to list API keys",
		})
		return
	}

	// Build response that never exposes key hashes
	type keyItem struct {
		ID        string  `json:"id"`
		AppID     string  `json:"app_id"`
		Name      string  `json:"name"`
		Revoked   bool    `json:"revoked"`
		CreatedAt string  `json:"created_at"`
		RevokedAt *string `json:"revoked_at,omitempty"`
	}

	items := make([]keyItem, len(keys))
	for i, k := range keys {
		item := keyItem{
			ID:        k.ID,
			AppID:     k.AppID,
			Name:      k.Name,
			Revoked:   k.Revoked,
			CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if k.RevokedAt != nil {
			formatted := k.RevokedAt.Format("2006-01-02T15:04:05Z07:00")
			item.RevokedAt = &formatted
		}
		items[i] = item
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys":  items,
		"count": len(items),
	})
}

// writeJSON writes a JSON response with the given status code and body.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
