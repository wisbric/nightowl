package user

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the users API.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates a user Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with all user routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGet)
		r.Put("/", h.handleUpdate)
		r.Delete("/", h.handleDeactivate)
	})
	return r
}

// service creates a per-request Service from the tenant-scoped connection.
func (h *Handler) service(r *http.Request) *Service {
	conn := tenant.ConnFromContext(r.Context())
	return NewService(conn, h.logger)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.Create(r.Context(), req)
	if err != nil {
		h.logger.Error("creating user", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"email": resp.Email})
		h.audit.LogFromRequest(r, "create", "user", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	svc := h.service(r)
	items, err := svc.List(r.Context())
	if err != nil {
		h.logger.Error("listing users", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"users": items,
		"count": len(items),
	})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid user ID")
		return
	}

	svc := h.service(r)
	resp, err := svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		h.logger.Error("getting user", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get user")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid user ID")
		return
	}

	var req UpdateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		h.logger.Error("updating user", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update user")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"email": resp.Email})
		h.audit.LogFromRequest(r, "update", "user", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid user ID")
		return
	}

	svc := h.service(r)
	if err := svc.Deactivate(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		h.logger.Error("deactivating user", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate user")
		return
	}

	if h.audit != nil {
		h.audit.LogFromRequest(r, "deactivate", "user", id, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

// PreferencesRoutes returns a chi.Router for /user/preferences endpoints.
func (h *Handler) PreferencesRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.handleGetPreferences)
	r.Put("/", h.handleUpdatePreferences)
	return r
}

func (h *Handler) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil || id.UserID == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "user authentication required")
		return
	}

	conn := tenant.ConnFromContext(r.Context())
	store := NewPreferencesStore(conn)

	prefs, err := store.GetPreferences(r.Context(), *id.UserID)
	if err != nil {
		h.logger.Error("getting preferences", "error", err, "user_id", id.UserID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get preferences")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(prefs) //nolint:errcheck
}

func (h *Handler) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil || id.UserID == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "user authentication required")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "failed to read body")
		return
	}

	// Validate it's valid JSON.
	if !json.Valid(body) {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}

	conn := tenant.ConnFromContext(r.Context())
	store := NewPreferencesStore(conn)

	if err := store.UpdatePreferences(r.Context(), *id.UserID, body); err != nil {
		h.logger.Error("updating preferences", "error", err, "user_id", id.UserID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update preferences")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body) //nolint:errcheck
}
