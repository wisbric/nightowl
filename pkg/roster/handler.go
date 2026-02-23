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
	r.Get("/coverage", h.handleGetCoverage)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.handleGetRoster)
		r.Put("/", h.handleUpdateRoster)
		r.Delete("/", h.handleDeleteRoster)

		// On-call
		r.Get("/oncall", h.handleGetOnCall)

		// Schedule
		r.Get("/schedule", h.handleListSchedule)
		r.Post("/schedule/generate", h.handleGenerateSchedule)
		r.Get("/schedule/{weekStart}", h.handleGetScheduleWeek)
		r.Put("/schedule/{weekStart}", h.handleUpdateScheduleWeek)
		r.Delete("/schedule/{weekStart}/lock", h.handleUnlockScheduleWeek)

		// Members
		r.Get("/members", h.handleListMembers)
		r.Post("/members", h.handleAddMember)
		r.Put("/members/{userID}", h.handleUpdateMember)
		r.Delete("/members/{userID}", h.handleDeactivateMember)

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

func parseRosterID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

// =====================
// Roster handlers
// =====================

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

	// Generate initial schedule.
	go func() {
		svc := h.service(r)
		if _, err := svc.GenerateSchedule(r.Context(), resp.ID, time.Now(), resp.ScheduleWeeksAhead); err != nil {
			h.logger.Error("generating initial schedule", "error", err, "roster_id", resp.ID)
		}
	}()

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
	id, err := parseRosterID(r)
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
	id, err := parseRosterID(r)
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
	id, err := parseRosterID(r)
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

// =====================
// On-call handler
// =====================

func (h *Handler) handleGetOnCall(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	at := time.Now()
	if v := r.URL.Query().Get("at"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid 'at' timestamp")
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
	httpserver.Respond(w, http.StatusOK, resp)
}

// =====================
// Schedule handlers
// =====================

func (h *Handler) handleListSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
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
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get roster")
		return
	}

	// Default range: 4 weeks ago to schedule_weeks_ahead from now.
	from := time.Now().AddDate(0, 0, -28)
	to := time.Now().AddDate(0, 0, roster.ScheduleWeeksAhead*7)

	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			from = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := time.Parse("2006-01-02", v); err == nil {
			to = parsed
		}
	}

	entries, err := svc.GetSchedule(r.Context(), id, from, to)
	if err != nil {
		h.logger.Error("listing schedule", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list schedule")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"schedule": entries,
		"count":    len(entries),
	})
}

func (h *Handler) handleGetScheduleWeek(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	weekStart, err := time.Parse("2006-01-02", chi.URLParam(r, "weekStart"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid weekStart (use YYYY-MM-DD)")
		return
	}

	svc := h.service(r)
	entry, err := svc.GetScheduleWeek(r.Context(), id, weekStart)
	if err != nil {
		h.logger.Error("getting schedule week", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get schedule week")
		return
	}
	if entry == nil {
		httpserver.RespondError(w, http.StatusNotFound, "not_found", "no schedule entry for this week")
		return
	}
	httpserver.Respond(w, http.StatusOK, entry)
}

func (h *Handler) handleUpdateScheduleWeek(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	weekStart, err := time.Parse("2006-01-02", chi.URLParam(r, "weekStart"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid weekStart (use YYYY-MM-DD)")
		return
	}

	var req UpdateScheduleWeekRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	// Validate primary != secondary.
	if req.PrimaryUserID != nil && req.SecondaryUserID != nil && *req.PrimaryUserID == *req.SecondaryUserID {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "primary and secondary must be different users")
		return
	}

	svc := h.service(r)
	entry, err := svc.UpdateScheduleWeek(r.Context(), id, weekStart, req)
	if err != nil {
		h.logger.Error("updating schedule week", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update schedule week")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"week_start": weekStart.Format("2006-01-02")})
		h.audit.LogFromRequest(r, "update_schedule", "roster", id, detail)
	}

	httpserver.Respond(w, http.StatusOK, entry)
}

func (h *Handler) handleUnlockScheduleWeek(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	weekStart, err := time.Parse("2006-01-02", chi.URLParam(r, "weekStart"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid weekStart (use YYYY-MM-DD)")
		return
	}

	svc := h.service(r)
	if err := svc.UnlockScheduleWeek(r.Context(), id, weekStart); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "no schedule entry for this week")
			return
		}
		h.logger.Error("unlocking schedule week", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to unlock schedule week")
		return
	}

	httpserver.Respond(w, http.StatusNoContent, nil)
}

func (h *Handler) handleGenerateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}

	var req GenerateScheduleRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	svc := h.service(r)

	roster, err := svc.GetRoster(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "roster not found")
			return
		}
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get roster")
		return
	}

	from := time.Now()
	if req.From != nil {
		parsed, err := time.Parse("2006-01-02", *req.From)
		if err != nil {
			httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid 'from' date (use YYYY-MM-DD)")
			return
		}
		from = parsed
	}

	weeks := roster.ScheduleWeeksAhead
	if req.Weeks != nil && *req.Weeks > 0 {
		weeks = *req.Weeks
	}

	entries, err := svc.GenerateSchedule(r.Context(), id, from, weeks)
	if err != nil {
		h.logger.Error("generating schedule", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to generate schedule")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]any{"weeks": weeks})
		h.audit.LogFromRequest(r, "generate_schedule", "roster", id, detail)
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"schedule": entries,
		"count":    len(entries),
	})
}

// =====================
// Member handlers
// =====================

func (h *Handler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
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
	id, err := parseRosterID(r)
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
		detail, _ := json.Marshal(map[string]string{"user_id": req.UserID.String()})
		h.audit.LogFromRequest(r, "add_member", "roster", id, detail)
	}
	httpserver.Respond(w, http.StatusCreated, resp)
}

func (h *Handler) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid user ID")
		return
	}
	var req UpdateMemberRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}
	svc := h.service(r)
	if err := svc.SetMemberActive(r.Context(), id, userID, req.IsActive); err != nil {
		h.logger.Error("updating roster member", "error", err, "roster_id", id, "user_id", userID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to update member")
		return
	}
	if h.audit != nil {
		action := "activate_member"
		if !req.IsActive {
			action = "deactivate_member"
		}
		detail, _ := json.Marshal(map[string]string{"user_id": userID.String()})
		h.audit.LogFromRequest(r, action, "roster", id, detail)
	}
	httpserver.Respond(w, http.StatusNoContent, nil)
}

func (h *Handler) handleDeactivateMember(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid roster ID")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid user ID")
		return
	}
	svc := h.service(r)
	if err := svc.DeactivateMember(r.Context(), id, userID); err != nil {
		h.logger.Error("deactivating roster member", "error", err, "roster_id", id, "user_id", userID)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to deactivate member")
		return
	}
	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"user_id": userID.String()})
		h.audit.LogFromRequest(r, "deactivate_member", "roster", id, detail)
	}
	httpserver.Respond(w, http.StatusNoContent, nil)
}

// =====================
// Override handlers
// =====================

func (h *Handler) handleListOverrides(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
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
	id, err := parseRosterID(r)
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
		detail, _ := json.Marshal(map[string]string{"user_id": req.UserID.String()})
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
		rosterID, _ := parseRosterID(r)
		h.audit.LogFromRequest(r, "delete_override", "roster", rosterID, nil)
	}
	httpserver.Respond(w, http.StatusNoContent, nil)
}

// =====================
// Calendar export
// =====================

func (h *Handler) handleGetCoverage(w http.ResponseWriter, r *http.Request) {
	from := time.Now().UTC().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 14)
	resolution := 60

	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			from = parsed
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			to = parsed
		}
	}
	if v := r.URL.Query().Get("resolution"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &resolution); n == 1 && err == nil && resolution > 0 {
			// valid
		}
	}

	svc := h.service(r)
	resp, err := svc.GetCoverage(r.Context(), CoverageRequest{
		From:       from,
		To:         to,
		Resolution: resolution,
	})
	if err != nil {
		h.logger.Error("getting coverage", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get coverage")
		return
	}
	httpserver.Respond(w, http.StatusOK, resp)
}

func (h *Handler) handleExportICS(w http.ResponseWriter, r *http.Request) {
	id, err := parseRosterID(r)
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

	// Get schedule for the next schedule_weeks_ahead weeks plus 4 weeks of history.
	from := time.Now().AddDate(0, 0, -28)
	to := time.Now().AddDate(0, 0, roster.ScheduleWeeksAhead*7)
	schedule, err := svc.GetSchedule(r.Context(), id, from, to)
	if err != nil {
		h.logger.Error("listing schedule for ical export", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list schedule")
		return
	}

	overrides, err := svc.ListOverrides(r.Context(), id)
	if err != nil {
		h.logger.Error("listing overrides for ical export", "error", err, "roster_id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list overrides")
		return
	}

	ical := generateICSFromSchedule(roster, schedule, overrides)
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.ics"`, roster.Name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(ical))
}
