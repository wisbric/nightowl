package tenantconfig

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/httpserver"
)

// Handler provides HTTP handlers for the tenant configuration API.
type Handler struct {
	logger  *slog.Logger
	audit   *audit.Writer
	service *Service
}

// NewHandler creates a tenant config Handler backed by the given global pool.
func NewHandler(logger *slog.Logger, audit *audit.Writer, pool *pgxpool.Pool) *Handler {
	return &Handler{
		logger:  logger,
		audit:   audit,
		service: NewService(pool, logger),
	}
}

// Routes returns a chi.Router with tenant config routes mounted.
// All routes require the admin role.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireRole(auth.RoleAdmin))
	r.Get("/", h.handleGet)
	r.Put("/", h.handleUpdate)
	return r
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	resp, err := h.service.Get(r.Context(), id.TenantID)
	if err != nil {
		h.logger.Error("getting tenant config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get configuration")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "missing authentication")
		return
	}

	var req UpdateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	resp, err := h.service.Update(r.Context(), id.TenantID, req)
	if err != nil {
		h.logger.Error("updating tenant config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update configuration")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"default_timezone": req.DefaultTimezone})
		h.audit.LogFromRequest(r, "update", "tenant_config", id.TenantID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}
