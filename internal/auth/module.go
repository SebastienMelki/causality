package auth

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/SebastienMelki/causality/internal/auth/internal/domain"
	"github.com/SebastienMelki/causality/internal/auth/internal/handler"
	"github.com/SebastienMelki/causality/internal/auth/internal/repo"
	"github.com/SebastienMelki/causality/internal/auth/internal/service"
)

// Module is the auth module facade. It wires together the domain, service,
// repository, and handler layers, and exposes the public API for key management
// and HTTP middleware.
type Module struct {
	service *service.KeyService
	repo    *repo.KeyRepository
	handler *handler.KeyHandler
	logger  *slog.Logger
}

// New creates a new auth Module. It initializes the PostgreSQL repository,
// key service, and admin handler.
func New(db *sql.DB, logger *slog.Logger) *Module {
	if logger == nil {
		logger = slog.Default()
	}

	keyRepo := repo.NewKeyRepository(db)
	keySvc := service.NewKeyService(keyRepo, logger)
	keyHandler := handler.NewKeyHandler(keySvc, logger)

	return &Module{
		service: keySvc,
		repo:    keyRepo,
		handler: keyHandler,
		logger:  logger.With("component", "auth-module"),
	}
}

// CreateKey generates a new API key for the given app. The returned plaintext
// key must be shown to the user once and cannot be retrieved again.
func (m *Module) CreateKey(ctx context.Context, appID, name string) (string, error) {
	plaintext, _, err := m.service.CreateKey(ctx, appID, name)
	if err != nil {
		return "", err
	}
	return plaintext, nil
}

// RevokeKey revokes an API key by its ID.
func (m *Module) RevokeKey(ctx context.Context, id string) error {
	return m.service.RevokeKey(ctx, id)
}

// ListKeys returns all API keys for the given app ID.
func (m *Module) ListKeys(ctx context.Context, appID string) ([]domain.APIKey, error) {
	return m.service.ListKeys(ctx, appID)
}

// AuthMiddleware returns HTTP middleware that validates API keys from the
// X-API-Key header and injects the authenticated app_id into the request
// context. Health, readiness, and metrics endpoints are excluded from auth.
func (m *Module) AuthMiddleware() func(http.Handler) http.Handler {
	return m.authMiddleware()
}

// RegisterAdminRoutes mounts the admin API key management endpoints onto the
// given ServeMux. These endpoints are:
//   - POST /api/admin/keys       - Create a new API key
//   - DELETE /api/admin/keys/{id} - Revoke an API key
//   - GET /api/admin/keys         - List API keys for an app
//
// TODO(phase-3): These admin endpoints must be protected by session auth + RBAC
// once the web application is built. Currently they are unprotected.
func (m *Module) RegisterAdminRoutes(mux *http.ServeMux) {
	m.handler.RegisterRoutes(mux)
}
