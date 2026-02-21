package roster

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the rosters API.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates a roster Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with all roster routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreateRoster)
	r.Get("/", h.handleListRosters)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGetRoster)
		r.Put("/", h.handleUpdateRoster)
		r.Delete("/", h.handleDeleteRoster)

		// On-call
		r.Get("/oncall", h.handleGetOnCall)

		// Members
		r.Get("/members", h.handleListMembers)
		r.Post("/members", h.handleAddMember)
		r.Delete("/members/{memberID}", h.handleRemoveMember)

		// Overrides
		r.Get("/overrides", h.handleListOverrides)
		r.Post("/overrides", h.handleCreateOverride)
		r.Delete("/overrides/{overrideID}", h.handleDeleteOverride)

		// Calendar export
		r.Get("/export.ics", h.handleExportICS)
	})
	return r
}

func (h *Handler) service(r *http.Request) *Service {
	conn := tenant.ConnFromContext(r.Context())
	return NewService(conn, h.logger)
}

func callerUUID(r *http.Request) pgtype.UUID {
	id := auth.FromContext(r.Context())
	if id != nil && id.UserID != nil {
		return pgtype.UUID{Bytes: *id.UserID, Valid: true}
	}
	return pgtype.UUID{}
}

// --- Roster handlers ---

func (h *Handler) handleCreateRoster(w http.ResponseWriter, r *http.Request) {
	var req CreateRosterRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.CreateRoster(r.Context(), req)
	if err != nil {
		h.logger.Error("creating roster", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create roster")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "create", "roster", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleListRosters(w http.ResponseWriter, r *http.Request) {
	svc := h.service(r)
	items, err := svc.ListRosters(r.Context())
	if err != nil {
		h.logger.Error("listing rosters", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list rosters")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"rosters": items,
		"count":   len(items),
	})
}

func (h *Handler) handleGetRoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	svc := h.service(r)
	resp, err := svc.GetRoster(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		h.logger.Error("getting roster", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get roster")
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleUpdateRoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	var req UpdateRosterRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.UpdateRoster(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		h.logger.Error("updating roster", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update roster")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"name": resp.Name})
		h.audit.LogFromRequest(r, "update", "roster", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleDeleteRoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	svc := h.service(r)
	if err := svc.DeleteRoster(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		h.logger.Error("deleting roster", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete roster")
		return
	}

	if h.audit != nil {
		h.audit.LogFromRequest(r, "delete", "roster", id, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

// --- On-call handler ---

func (h *Handler) handleGetOnCall(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	at := time.Now()
	if v := r.URL.Query().Get("at"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid 'at' timestamp, use RFC3339 format")
			return
		}
		at = parsed
	}

	svc := h.service(r)
	resp, err := svc.GetOnCall(r.Context(), id, at)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		h.logger.Error("getting on-call", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get on-call")
		return
	}

	if resp == nil {
		httpserver.Respond(w, http.StatusOK, map[string]any{
			"on_call": nil,
			"message": "no members in roster",
		})
		return
	}

	httpserver.Respond(w, http.StatusOK, resp)
}

// --- Member handlers ---

func (h *Handler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	svc := h.service(r)
	items, err := svc.ListMembers(r.Context(), id)
	if err != nil {
		h.logger.Error("listing roster members", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list members")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"members": items,
		"count":   len(items),
	})
}

func (h *Handler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	var req AddMemberRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.AddMember(r.Context(), id, req)
	if err != nil {
		h.logger.Error("adding roster member", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to add member")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"user_id": req.UserID.String(), "roster_id": id.String()})
		h.audit.LogFromRequest(r, "add_member", "roster", id, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	memberID, err := uuid.Parse(chi.URLParam(r, "memberID"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid member ID")
		return
	}

	svc := h.service(r)
	if err := svc.RemoveMember(r.Context(), memberID); err != nil {
		h.logger.Error("removing roster member", "error", err, "member_id", memberID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to remove member")
		return
	}

	if h.audit != nil {
		rosterID, _ := uuid.Parse(chi.URLParam(r, "id"))
		h.audit.LogFromRequest(r, "remove_member", "roster", rosterID, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

// --- Override handlers ---

func (h *Handler) handleListOverrides(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	svc := h.service(r)
	items, err := svc.ListOverrides(r.Context(), id)
	if err != nil {
		h.logger.Error("listing roster overrides", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list overrides")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"overrides": items,
		"count":     len(items),
	})
}

func (h *Handler) handleCreateOverride(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	var req CreateOverrideRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)
	resp, err := svc.CreateOverride(r.Context(), id, req, callerUUID(r))
	if err != nil {
		h.logger.Error("creating roster override", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to create override")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"user_id": req.UserID.String(), "roster_id": id.String()})
		h.audit.LogFromRequest(r, "create_override", "roster", id, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleDeleteOverride(w http.ResponseWriter, r *http.Request) {
	overrideID, err := uuid.Parse(chi.URLParam(r, "overrideID"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid override ID")
		return
	}

	svc := h.service(r)
	if err := svc.DeleteOverride(r.Context(), overrideID); err != nil {
		h.logger.Error("deleting roster override", "error", err, "override_id", overrideID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to delete override")
		return
	}

	if h.audit != nil {
		rosterID, _ := uuid.Parse(chi.URLParam(r, "id"))
		h.audit.LogFromRequest(r, "delete_override", "roster", rosterID, nil)
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

// --- Calendar export ---

func (h *Handler) handleExportICS(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	svc := h.service(r)
	roster, err := svc.GetRoster(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		h.logger.Error("getting roster for ical export", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get roster")
		return
	}

	members, err := svc.ListMembers(r.Context(), id)
	if err != nil {
		h.logger.Error("listing members for ical export", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list members")
		return
	}

	overrides, err := svc.ListOverrides(r.Context(), id)
	if err != nil {
		h.logger.Error("listing overrides for ical export", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list overrides")
		return
	}

	ical := generateICS(roster, members, overrides)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.ics"`, roster.Name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(ical))
}
