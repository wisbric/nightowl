package escalation

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the escalation policies API.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates an escalation Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with escalation policy routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGet)
		r.Put("/", h.handleUpdate)
		r.Delete("/", h.handleDelete)
		r.Post("/dry-run", h.handleDryRun)
		r.Get("/events/{alertID}", h.handleListEvents)
	})
	return r
}

func (h *Handler) store(r *http.Request) *Store {
	conn := tenant.ConnFromContext(r.Context())
	return NewStore(conn)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreatePolicyRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	var repeatCount *int32
	if req.RepeatCount != nil {
		repeatCount = req.RepeatCount
	}

	s := h.store(r)
	resp, err := s.CreatePolicy(r.Context(), db.CreateEscalationPolicyParams{
		Name:        req.Name,
		Description: req.Description,
		Tiers:       marshalTiers(req.Tiers),
		RepeatCount: repeatCount,
	})
	if err != nil {
		h.logger.Error("creating escalation policy", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create policy")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "create", "escalation_policy", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	s := h.store(r)
	items, err := s.ListPolicies(r.Context())
	if err != nil {
		h.logger.Error("listing escalation policies", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list policies")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"policies": items,
		"count":    len(items),
	})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid policy ID")
		return
	}

	s := h.store(r)
	resp, err := s.GetPolicy(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "policy not found")
			return
		}
		h.logger.Error("getting escalation policy", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get policy")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid policy ID")
		return
	}

	var req UpdatePolicyRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	s := h.store(r)
	resp, err := s.UpdatePolicy(r.Context(), db.UpdateEscalationPolicyParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Tiers:       marshalTiers(req.Tiers),
		RepeatCount: req.RepeatCount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "policy not found")
			return
		}
		h.logger.Error("updating escalation policy", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update policy")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "update", "escalation_policy", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid policy ID")
		return
	}

	s := h.store(r)
	if err := s.DeletePolicy(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "policy not found")
			return
		}
		h.logger.Error("deleting escalation policy", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete policy")
		return
	}

	if h.audit != nil {
		h.audit.LogFromRequest(r, "delete", "escalation_policy", id, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

func (h *Handler) handleDryRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid policy ID")
		return
	}

	s := h.store(r)
	policy, err := s.GetPolicy(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "policy not found")
			return
		}
		h.logger.Error("getting escalation policy for dry-run", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get policy")
		return
	}

	// Simulate escalation.
	var steps []DryRunStep
	cumulative := 0
	for _, tier := range policy.Tiers {
		cumulative += tier.TimeoutMinutes
		steps = append(steps, DryRunStep{
			Tier:              tier.Tier,
			TimeoutMinutes:    tier.TimeoutMinutes,
			CumulativeMinutes: cumulative,
			NotifyVia:         tier.NotifyVia,
			Targets:           tier.Targets,
			Action:            "notify",
		})
	}

	httpserver.Respond(w, http.StatusOK, DryRunResponse{
		PolicyID:   policy.ID,
		PolicyName: policy.Name,
		Steps:      steps,
		TotalTime:  cumulative,
	})
}

func (h *Handler) handleListEvents(w http.ResponseWriter, r *http.Request) {
	alertID, err := uuid.Parse(chi.URLParam(r, "alertID"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	s := h.store(r)
	items, err := s.ListEvents(r.Context(), alertID)
	if err != nil {
		h.logger.Error("listing escalation events", "error", err, "alert_id", alertID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list events")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"events": items,
		"count":  len(items),
	})
}
