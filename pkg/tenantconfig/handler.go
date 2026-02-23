package tenantconfig

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goslack "github.com/slack-go/slack"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/httpserver"
	nightowlmm "github.com/wisbric/nightowl/pkg/mattermost"
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
	r.Post("/messaging/test", h.handleTestMessaging)
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

// TestMessagingRequest is the JSON body for POST /admin/config/messaging/test.
type TestMessagingRequest struct {
	Provider string `json:"provider" validate:"required"`
	// Slack fields
	BotToken string `json:"bot_token"`
	// Mattermost fields
	URL string `json:"url"`
}

// TestMessagingResponse is the JSON response for the test connection endpoint.
type TestMessagingResponse struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	BotName   string `json:"bot_name,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

func (h *Handler) handleTestMessaging(w http.ResponseWriter, r *http.Request) {
	var req TestMessagingRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	ctx := r.Context()

	switch req.Provider {
	case "slack":
		h.testSlack(ctx, w, req)
	case "mattermost":
		h.testMattermost(ctx, w, req)
	default:
		httpserver.Respond(w, http.StatusOK, TestMessagingResponse{
			OK:    false,
			Error: "unknown provider: " + req.Provider,
		})
	}
}

func (h *Handler) testSlack(ctx context.Context, w http.ResponseWriter, req TestMessagingRequest) {
	if req.BotToken == "" {
		httpserver.Respond(w, http.StatusOK, TestMessagingResponse{OK: false, Error: "bot_token is required"})
		return
	}

	client := goslack.New(req.BotToken)
	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		httpserver.Respond(w, http.StatusOK, TestMessagingResponse{OK: false, Error: err.Error()})
		return
	}

	httpserver.Respond(w, http.StatusOK, TestMessagingResponse{
		OK:        true,
		BotName:   resp.User,
		Workspace: resp.Team,
	})
}

func (h *Handler) testMattermost(ctx context.Context, w http.ResponseWriter, req TestMessagingRequest) {
	if req.URL == "" || req.BotToken == "" {
		httpserver.Respond(w, http.StatusOK, TestMessagingResponse{OK: false, Error: "url and bot_token are required"})
		return
	}

	client := nightowlmm.NewClient(req.URL, req.BotToken, h.logger)
	me, err := client.GetMe(ctx)
	if err != nil {
		httpserver.Respond(w, http.StatusOK, TestMessagingResponse{OK: false, Error: err.Error()})
		return
	}

	httpserver.Respond(w, http.StatusOK, TestMessagingResponse{
		OK:      true,
		BotName: me.Username,
	})
}
