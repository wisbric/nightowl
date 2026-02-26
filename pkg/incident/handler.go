package incident

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the incidents API.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates an incident Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with all incident routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Get("/search", h.handleSearch)
	r.Get("/fingerprint/{fp}", h.handleGetByFingerprint)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGet)
		r.Put("/", h.handleUpdate)
		r.Delete("/", h.handleDelete)
		r.Post("/merge", h.handleMerge)
		r.Get("/history", h.handleListHistory)
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

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"title": resp.Title})
		h.audit.LogFromRequest(r, "create", "incident", resp.ID, detail)
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

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"title": resp.Title})
		h.audit.LogFromRequest(r, "update", "incident", resp.ID, detail)
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

	if h.audit != nil {
		h.audit.LogFromRequest(r, "delete", "incident", id, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "query parameter 'q' is required")
		return
	}

	limit := 25
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "limit must be a positive integer")
			return
		}
		if n > 100 {
			n = 100
		}
		limit = n
	}

	svc := h.service(r)
	results, err := svc.Search(r.Context(), q, limit)
	if err != nil {
		h.logger.Error("searching incidents", "error", err, "query", q)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to search incidents")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"query":   q,
		"results": results,
		"count":   len(results),
	})
}

func (h *Handler) handleListHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid incident ID")
		return
	}

	svc := h.service(r)
	entries, err := svc.ListHistory(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		h.logger.Error("listing incident history", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list incident history")
		return
	}

	httpserver.Respond(w, http.StatusOK, entries)
}

func (h *Handler) handleMerge(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid target incident ID")
		return
	}

	var req MergeRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid source_id")
		return
	}

	svc := h.service(r)
	resp, err := svc.Merge(r.Context(), targetID, sourceID, callerUUID(r))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		// Surface specific validation errors as 422.
		msg := err.Error()
		if strings.Contains(msg, "cannot merge") || strings.Contains(msg, "already merged") {
			httpserver.RespondError(w, http.StatusUnprocessableEntity, "merge_error", msg)
			return
		}
		h.logger.Error("merging incidents", "error", err, "target", targetID, "source", sourceID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to merge incidents")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"source_id": sourceID.String(), "target_id": targetID.String()})
		h.audit.LogFromRequest(r, "merge", "incident", targetID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleGetByFingerprint(w http.ResponseWriter, r *http.Request) {
	fp := chi.URLParam(r, "fp")
	if fp == "" {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "fingerprint is required")
		return
	}

	svc := h.service(r)
	resp, err := svc.GetByFingerprint(r.Context(), fp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "no incident matches this fingerprint")
			return
		}
		h.logger.Error("getting incident by fingerprint", "error", err, "fingerprint", fp)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get incident by fingerprint")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}
