package apikey

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"

	"github.com/wisbric/nightowl/internal/audit"
)

// Handler provides HTTP handlers for the API keys API.
type Handler struct {
	logger  *slog.Logger
	audit   *audit.Writer
	service *Service
}

// NewHandler creates an API key Handler backed by the given global pool.
func NewHandler(logger *slog.Logger, audit *audit.Writer, pool *pgxpool.Pool) *Handler {
	return &Handler{
		logger:  logger,
		audit:   audit,
		service: NewService(pool, logger),
	}
}

// Routes returns a chi.Router with all API key routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Delete("/{id}", h.handleDelete)
	return r
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	id := auth.FromContext(r.Context())
	if id == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	resp, err := h.service.Create(r.Context(), id.TenantID, req)
	if err != nil {
		h.logger.Error("creating api key", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create api key")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"description": resp.Description})
		h.audit.LogFromRequest(r, "create", "api_key", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	items, err := h.service.List(r.Context(), id.TenantID)
	if err != nil {
		h.logger.Error("listing api keys", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list api keys")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"keys":  items,
		"count": len(items),
	})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid api key ID")
		return
	}

	if err := h.service.Delete(r.Context(), keyID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "api key not found")
			return
		}
		h.logger.Error("deleting api key", "error", err, "id", keyID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete api key")
		return
	}

	if h.audit != nil {
		h.audit.LogFromRequest(r, "delete", "api_key", keyID, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}
