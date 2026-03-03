package alertgroup

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/core/pkg/httpserver"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/alert"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the alert grouping API.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates an alertgroup Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with alert grouping routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	// Rule management
	r.Route("/rules", func(r chi.Router) {
		r.Post("/", h.handleCreateRule)
		r.Get("/", h.handleListRules)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGetRule)
			r.Put("/", h.handleUpdateRule)
			r.Delete("/", h.handleDeleteRule)
		})
	})

	// Group browsing
	r.Get("/", h.handleListGroups)
	r.Get("/{id}", h.handleGetGroup)
	r.Get("/{id}/alerts", h.handleListGroupAlerts)

	return r
}

func (h *Handler) store(r *http.Request) *Store {
	conn := tenant.ConnFromContext(r.Context())
	return NewStore(conn)
}

// --- Rule handlers ---

func (h *Handler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateRuleRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	s := h.store(r)
	resp, err := s.CreateRule(r.Context(), db.CreateAlertGroupingRuleParams{
		Name:        req.Name,
		Description: req.Description,
		Position:    req.Position,
		IsEnabled:   isEnabled,
		Matchers:    marshalMatchers(req.Matchers),
		GroupBy:     req.GroupBy,
	})
	if err != nil {
		h.logger.Error("creating alert grouping rule", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create rule")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "create", "alert_grouping_rule", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleListRules(w http.ResponseWriter, r *http.Request) {
	s := h.store(r)
	items, err := s.ListRules(r.Context())
	if err != nil {
		h.logger.Error("listing alert grouping rules", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list rules")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"rules": items,
		"count": len(items),
	})
}

func (h *Handler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid rule ID")
		return
	}

	s := h.store(r)
	resp, err := s.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "rule not found")
			return
		}
		h.logger.Error("getting alert grouping rule", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get rule")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid rule ID")
		return
	}

	var req UpdateRuleRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	s := h.store(r)
	resp, err := s.UpdateRule(r.Context(), db.UpdateAlertGroupingRuleParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Position:    req.Position,
		IsEnabled:   isEnabled,
		Matchers:    marshalMatchers(req.Matchers),
		GroupBy:     req.GroupBy,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "rule not found")
			return
		}
		h.logger.Error("updating alert grouping rule", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update rule")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "update", "alert_grouping_rule", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid rule ID")
		return
	}

	s := h.store(r)
	if err := s.DeleteRule(r.Context(), id); err != nil {
		h.logger.Error("deleting alert grouping rule", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete rule")
		return
	}

	if h.audit != nil {
		h.audit.LogFromRequest(r, "delete", "alert_grouping_rule", id, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

// --- Group handlers ---

func (h *Handler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	s := h.store(r)
	items, err := s.ListGroups(r.Context(), status)
	if err != nil {
		h.logger.Error("listing alert groups", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list groups")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"groups": items,
		"count":  len(items),
	})
}

func (h *Handler) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid group ID")
		return
	}

	s := h.store(r)
	resp, err := s.GetGroup(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "group not found")
			return
		}
		h.logger.Error("getting alert group", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get group")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleListGroupAlerts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid group ID")
		return
	}

	s := h.store(r)
	rows, err := s.ListGroupAlerts(r.Context(), id)
	if err != nil {
		h.logger.Error("listing group alerts", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list group alerts")
		return
	}

	// Convert db.Alert rows to alert.Response using the alert package's converter.
	alerts := make([]alert.Response, 0, len(rows))
	for _, row := range rows {
		alerts = append(alerts, alert.AlertRowToResponse(row))
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"alerts": alerts,
		"count":  len(alerts),
	})
}
