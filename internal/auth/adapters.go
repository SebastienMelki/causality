package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
)

// skipAuthPaths lists URL path prefixes that bypass API key authentication.
// These are infrastructure endpoints that must remain accessible without auth.
var skipAuthPaths = []string{
	"/health",
	"/ready",
	"/metrics",
}

// authMiddleware returns HTTP middleware that validates the X-API-Key header.
// On success it injects the authenticated app_id into the request context.
// On failure it returns 401 Unauthorized with a JSON error body.
func (m *Module) authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health/ready/metrics endpoints
			for _, prefix := range skipAuthPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				writeAuthError(w, "missing API key")
				return
			}

			// Validate key format before hashing
			if !domain.ValidateKeyFormat(apiKey) {
				writeAuthError(w, "invalid API key")
				return
			}

			keyHash := domain.HashKey(apiKey)

			key, err := m.service.ValidateKey(r.Context(), keyHash)
			if err != nil {
				m.logger.Error("failed to validate API key",
					"error", err,
					"path", r.URL.Path,
				)
				writeAuthError(w, "invalid API key")
				return
			}

			if key == nil {
				writeAuthError(w, "invalid API key")
				return
			}

			// Inject app_id into context for downstream handlers
			ctx := context.WithValue(r.Context(), AppIDContextKey, key.AppID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAppID retrieves the authenticated app_id from the request context.
// Returns an empty string if no app_id is present (e.g., unauthenticated request).
func GetAppID(ctx context.Context) string {
	if appID, ok := ctx.Value(AppIDContextKey).(string); ok {
		return appID
	}
	return ""
}

// writeAuthError writes a 401 Unauthorized JSON response.
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
