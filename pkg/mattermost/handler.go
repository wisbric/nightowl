package mattermost

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler provides HTTP handlers for Mattermost integration.
type Handler struct {
	provider      *Provider
	pool          *pgxpool.Pool
	logger        *slog.Logger
	webhookSecret string
	defaultTenant string
}

// NewHandler creates a Mattermost handler.
func NewHandler(provider *Provider, pool *pgxpool.Pool, logger *slog.Logger, webhookSecret, defaultTenant string) *Handler {
	return &Handler{
		provider:      provider,
		pool:          pool,
		logger:        logger,
		webhookSecret: webhookSecret,
		defaultTenant: defaultTenant,
	}
}

// Routes returns a chi.Router with Mattermost webhook routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(VerifyMiddleware(h.webhookSecret))
	r.Post("/commands", h.handleCommands)
	r.Post("/actions", h.handleActions)
	r.Post("/dialogs", h.handleDialogs)
	return r
}
