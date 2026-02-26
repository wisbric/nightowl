package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for alert lifecycle endpoints.
type Handler struct {
	logger *slog.Logger
	audit  *audit.Writer
}

// NewHandler creates a Handler.
func NewHandler(logger *slog.Logger, audit *audit.Writer) *Handler {
	return &Handler{logger: logger, audit: audit}
}

// Routes returns a chi.Router with alert lifecycle routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.handleList)
	r.Get("/{id}", h.handleGet)
	r.Patch("/{id}/acknowledge", h.handleAcknowledge)
	r.Patch("/{id}/resolve", h.handleResolve)
	return r
}

// handleList returns alerts with optional filters.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn := tenant.ConnFromContext(ctx)

	f := parseAlertFilters(r)
	alerts, err := listAlertsFiltered(ctx, conn, f)
	if err != nil {
		h.logger.Error("listing alerts", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list alerts")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]any{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// handleGet returns a single alert by ID.
func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn := tenant.ConnFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	q := db.New(conn)
	row, err := q.GetAlert(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "alert not found")
			return
		}
		h.logger.Error("getting alert", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to get alert")
		return
	}

	httpserver.Respond(w, http.StatusOK, alertRowToResponse(row))
}

// handleAcknowledge sets an alert to acknowledged status.
func (h *Handler) handleAcknowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn := tenant.ConnFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	identity := auth.FromContext(ctx)
	var acknowledgedBy pgtype.UUID
	if identity != nil && identity.UserID != nil {
		acknowledgedBy = pgtype.UUID{Bytes: *identity.UserID, Valid: true}
	}

	q := db.New(conn)
	row, err := q.AcknowledgeAlert(ctx, db.AcknowledgeAlertParams{
		ID:             id,
		AcknowledgedBy: acknowledgedBy,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "alert not found")
			return
		}
		h.logger.Error("acknowledging alert", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to acknowledge alert")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"title": row.Title})
		h.audit.LogFromRequest(r, "acknowledge", "alert", row.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, alertRowToResponse(row))
}

// handleResolve sets an alert to resolved status.
func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn := tenant.ConnFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert ID")
		return
	}

	identity := auth.FromContext(ctx)
	var resolvedBy pgtype.UUID
	if identity != nil && identity.UserID != nil {
		resolvedBy = pgtype.UUID{Bytes: *identity.UserID, Valid: true}
	}

	q := db.New(conn)
	row, err := q.ResolveAlert(ctx, db.ResolveAlertParams{
		ID:         id,
		ResolvedBy: resolvedBy,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpserver.RespondError(w, http.StatusNotFound, "not_found", "alert not found")
			return
		}
		h.logger.Error("resolving alert", "error", err, "id", id)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to resolve alert")
		return
	}

	if h.audit != nil {
		detail, _ := json.Marshal(map[string]string{"title": row.Title})
		h.audit.LogFromRequest(r, "resolve", "alert", row.ID, detail)
	}

	httpserver.Respond(w, http.StatusOK, alertRowToResponse(row))
}

// --- Filtered list ---

// alertFilters holds query parameters for filtering alerts.
type alertFilters struct {
	Status   string
	Severity string
	Source   string
	After    *time.Time
	Before   *time.Time
	Limit    int
	Offset   int
}

func parseAlertFilters(r *http.Request) alertFilters {
	f := alertFilters{
		Status:   r.URL.Query().Get("status"),
		Severity: r.URL.Query().Get("severity"),
		Source:   r.URL.Query().Get("source"),
		Limit:    50,
		Offset:   0,
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			f.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			f.Offset = n
		}
	}
	if v := r.URL.Query().Get("after"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.After = &t
		}
	}
	if v := r.URL.Query().Get("before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Before = &t
		}
	}
	return f
}

func listAlertsFiltered(ctx context.Context, dbtx db.DBTX, f alertFilters) ([]Response, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if f.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.Severity != "" {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, f.Severity)
		argIdx++
	}
	if f.Source != "" {
		conditions = append(conditions, fmt.Sprintf("source = $%d", argIdx))
		args = append(args, f.Source)
		argIdx++
	}
	if f.After != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *f.After)
		argIdx++
	}
	if f.Before != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *f.Before)
		argIdx++
	}

	query := `SELECT id, fingerprint, status, severity, source, title, description,
		labels, annotations, service_id, matched_incident_id, suggested_solution,
		acknowledged_by, acknowledged_at, resolved_by, resolved_at,
		resolved_by_agent, agent_resolution_notes,
		occurrence_count, first_fired_at, last_fired_at,
		escalation_policy_id, current_escalation_tier,
		created_at, updated_at
	FROM alerts`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := dbtx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing alerts: %w", err)
	}
	defer rows.Close()

	var results []Response
	for rows.Next() {
		var a db.Alert
		if err := rows.Scan(
			&a.ID, &a.Fingerprint, &a.Status, &a.Severity, &a.Source, &a.Title,
			&a.Description, &a.Labels, &a.Annotations, &a.ServiceID,
			&a.MatchedIncidentID, &a.SuggestedSolution,
			&a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedBy, &a.ResolvedAt,
			&a.ResolvedByAgent, &a.AgentResolutionNotes,
			&a.OccurrenceCount, &a.FirstFiredAt, &a.LastFiredAt,
			&a.EscalationPolicyID, &a.CurrentEscalationTier,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning alert row: %w", err)
		}
		results = append(results, alertRowToResponse(a))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating alert rows: %w", err)
	}
	if results == nil {
		results = []Response{}
	}
	return results, nil
}
