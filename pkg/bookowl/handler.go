package bookowl

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"

	"github.com/wisbric/nightowl/pkg/tenantconfig"
)

// Handler provides proxy HTTP handlers for BookOwl integration.
type Handler struct {
	logger *slog.Logger
	client *Client
	cfgSvc *tenantconfig.Service
}

// NewHandler creates a BookOwl proxy handler.
func NewHandler(logger *slog.Logger, pool *pgxpool.Pool) *Handler {
	return &Handler{
		logger: logger,
		client: NewClient(),
		cfgSvc: tenantconfig.NewService(pool, logger),
	}
}

// Routes returns a chi.Router with BookOwl proxy routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/status", h.handleStatus)
	r.Get("/runbooks", h.handleListRunbooks)
	r.Get("/runbooks/{id}", h.handleGetRunbook)
	r.Post("/post-mortems", h.handleCreatePostMortem)
	return r
}

// bookOwlConfig reads the BookOwl API URL and key from tenant config.
// Returns empty strings if not configured.
func (h *Handler) bookOwlConfig(r *http.Request) (apiURL, apiKey string, err error) {
	id := auth.FromContext(r.Context())
	if id == nil {
		return "", "", nil
	}

	cfg, err := h.cfgSvc.Get(r.Context(), id.TenantID)
	if err != nil {
		return "", "", err
	}

	return cfg.BookOwlAPIURL, cfg.BookOwlAPIKey, nil
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	apiURL, apiKey, err := h.bookOwlConfig(r)
	if err != nil {
		h.logger.Error("getting bookowl config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get configuration")
		return
	}

	httpserver.Respond(w, http.StatusOK, StatusResponse{
		Integrated: apiURL != "" && apiKey != "",
		URL:        apiURL,
	})
}

func (h *Handler) handleListRunbooks(w http.ResponseWriter, r *http.Request) {
	apiURL, apiKey, err := h.bookOwlConfig(r)
	if err != nil {
		h.logger.Error("getting bookowl config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get configuration")
		return
	}
	if apiURL == "" || apiKey == "" {
		httpserver.RespondError(w, http.StatusServiceUnavailable, "bookowl_not_configured", "BookOwl integration is not configured")
		return
	}

	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	result, err := h.client.ListRunbooks(r.Context(), apiURL, apiKey, query, limit, offset)
	if err != nil {
		h.logger.Error("listing bookowl runbooks", "error", err)
		httpserver.RespondError(w, http.StatusBadGateway, "bookowl_error", err.Error())
		return
	}

	httpserver.Respond(w, http.StatusOK, result)
}

func (h *Handler) handleGetRunbook(w http.ResponseWriter, r *http.Request) {
	apiURL, apiKey, err := h.bookOwlConfig(r)
	if err != nil {
		h.logger.Error("getting bookowl config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get configuration")
		return
	}
	if apiURL == "" || apiKey == "" {
		httpserver.RespondError(w, http.StatusServiceUnavailable, "bookowl_not_configured", "BookOwl integration is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.client.GetRunbook(r.Context(), apiURL, apiKey, id)
	if err != nil {
		h.logger.Error("getting bookowl runbook", "id", id, "error", err)
		httpserver.RespondError(w, http.StatusBadGateway, "bookowl_error", err.Error())
		return
	}

	httpserver.Respond(w, http.StatusOK, result)
}

func (h *Handler) handleCreatePostMortem(w http.ResponseWriter, r *http.Request) {
	apiURL, apiKey, err := h.bookOwlConfig(r)
	if err != nil {
		h.logger.Error("getting bookowl config", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get configuration")
		return
	}
	if apiURL == "" || apiKey == "" {
		httpserver.RespondError(w, http.StatusServiceUnavailable, "bookowl_not_configured", "BookOwl integration is not configured")
		return
	}

	var pmReq PostMortemRequest
	if err := json.NewDecoder(r.Body).Decode(&pmReq); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	result, err := h.client.CreatePostMortem(r.Context(), apiURL, apiKey, pmReq)
	if err != nil {
		h.logger.Error("creating bookowl post-mortem", "error", err)
		httpserver.RespondError(w, http.StatusBadGateway, "bookowl_error", err.Error())
		return
	}

	httpserver.Respond(w, http.StatusCreated, result)
}
