package incident

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/opswatch/internal/auth"
	"github.com/wisbric/opswatch/internal/httpserver"
	"github.com/wisbric/opswatch/pkg/tenant"
)

// Handler provides HTTP handlers for the incidents API.
type Handler struct {
	logger *slog.Logger
}

// NewHandler creates an incident Handler.
func NewHandler(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// Routes returns a chi.Router with all incident routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGet)
		r.Put("/", h.handleUpdate)
		r.Delete("/", h.handleDelete)
	})
	return r
}

// service creates a per-request Service from the tenant-scoped connection.
func (h *Handler) service(r *http.Request) *Service {
	conn := tenant.ConnFromContext(r.Context())
	return NewService(conn, h.logger)
}

// callerUUID extracts the authenticated user's UUID as pgtype.UUID.
func callerUUID(r *http.Request) pgtype.UUID {
	id := auth.FromContext(r.Context())
	if id != nil && id.UserID != nil {
		return pgtype.UUID{Bytes: *id.UserID, Valid: true}
	}
	return pgtype.UUID{}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.Create(r.Context(), req, callerUUID(r))
	if err != nil {
		h.logger.Error("creating incident", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create incident")
		return
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	params, err := httpserver.ParseOffsetParams(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	filters := ListFilters{
		Severity: r.URL.Query().Get("severity"),
		Category: r.URL.Query().Get("category"),
		Service:  r.URL.Query().Get("service"),
		Tags:     r.URL.Query()["tag"],
	}

	svc := h.service(r)
	items, total, err := svc.List(r.Context(), filters, params.PageSize, params.Offset)
	if err != nil {
		h.logger.Error("listing incidents", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list incidents")
		return
	}

	page := httpserver.NewOffsetPage(items, params, total)
	httpserver.Respond(w, http.StatusOK, page)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid incident ID")
		return
	}

	svc := h.service(r)
	resp, err := svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		h.logger.Error("getting incident", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get incident")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid incident ID")
		return
	}

	var req UpdateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.Update(r.Context(), id, req, callerUUID(r))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		h.logger.Error("updating incident", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update incident")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid incident ID")
		return
	}

	svc := h.service(r)
	if err := svc.Delete(r.Context(), id, callerUUID(r)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		h.logger.Error("deleting incident", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete incident")
		return
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}
