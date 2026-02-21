package alert

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/wisbric/opswatch/internal/audit"
	"github.com/wisbric/opswatch/internal/httpserver"
	"github.com/wisbric/opswatch/pkg/tenant"
)

// --- Alertmanager payload types ---

type alertmanagerPayload struct {
	Version  string                `json:"version"`
	GroupKey string                `json:"groupKey"`
	Status   string                `json:"status"`
	Alerts   []alertmanagerAlert   `json:"alerts"`
}

type alertmanagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	Fingerprint string            `json:"fingerprint"`
}

// --- Keep payload types ---

type keepPayload struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Severity    string            `json:"severity"`
	Source      []string          `json:"source"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Description string            `json:"description"`
}

// --- Generic payload types ---

type genericPayload struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Fingerprint string            `json:"fingerprint"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Source      string            `json:"source"`
}

// WebhookHandler provides HTTP handlers for alert webhook endpoints.
type WebhookHandler struct {
	logger *slog.Logger
	audit  *audit.Writer
	dedup  *Deduplicator
}

// NewWebhookHandler creates a WebhookHandler.
func NewWebhookHandler(logger *slog.Logger, audit *audit.Writer, dedup *Deduplicator) *WebhookHandler {
	return &WebhookHandler{logger: logger, audit: audit, dedup: dedup}
}

// Routes returns a chi.Router with webhook routes mounted.
func (h *WebhookHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/alertmanager", h.handleAlertmanager)
	r.Post("/keep", h.handleKeep)
	r.Post("/generic", h.handleGeneric)
	return r
}

// store creates a per-request Store from the tenant-scoped connection.
func (h *WebhookHandler) store(r *http.Request) *Store {
	conn := tenant.ConnFromContext(r.Context())
	return NewStore(conn)
}

// tenantSchema returns the tenant schema name from the request context.
func tenantSchema(r *http.Request) string {
	if info := tenant.FromContext(r.Context()); info != nil {
		return info.Schema
	}
	return "unknown"
}

// createOrDedup checks for a duplicate alert and either increments the existing
// alert's occurrence count or creates a new one.
func (h *WebhookHandler) createOrDedup(r *http.Request, store *Store, normalized NormalizedAlert) (Response, bool, error) {
	ctx := r.Context()
	conn := tenant.ConnFromContext(ctx)
	schema := tenantSchema(r)

	if h.dedup != nil && normalized.Status == "firing" {
		result, err := h.dedup.Check(ctx, schema, normalized.Fingerprint, conn)
		if err != nil {
			h.logger.Warn("dedup check failed, creating new alert", "error", err)
		} else if result.IsDuplicate {
			resp, err := h.dedup.IncrementAndReturn(ctx, conn, result.AlertID)
			if err != nil {
				return Response{}, false, fmt.Errorf("incrementing duplicate alert: %w", err)
			}
			return resp, true, nil
		}
	}

	resp, err := store.Create(ctx, normalized)
	if err != nil {
		return Response{}, false, fmt.Errorf("creating alert: %w", err)
	}

	if h.dedup != nil {
		h.dedup.RecordNew(ctx, schema, normalized.Fingerprint, resp.ID)
	}

	return resp, false, nil
}

// decodeWebhookBody reads and decodes a webhook JSON body.
// Unlike httpserver.Decode, this is lenient about unknown fields since external
// systems may include additional data.
func decodeWebhookBody(r *http.Request, dst any) error {
	const maxBody = 1 << 20 // 1 MiB
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return fmt.Errorf("reading request body: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("request body is empty")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// handleAlertmanager processes Alertmanager webhook payloads containing one or
// more alerts, normalizes each to the internal format, and persists them.
func (h *WebhookHandler) handleAlertmanager(w http.ResponseWriter, r *http.Request) {
	var payload alertmanagerPayload
	if err := decodeWebhookBody(r, &payload); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if len(payload.Alerts) == 0 {
		httpserver.RespondError(w, http.StatusUnprocessableEntity, "validation_error", "no alerts in payload")
		return
	}

	store := h.store(r)
	var results []Response
	for _, a := range payload.Alerts {
		normalized := normalizeAlertmanager(a)
		resp, isDup, err := h.createOrDedup(r, store, normalized)
		if err != nil {
			h.logger.Error("processing alert from alertmanager", "error", err, "fingerprint", normalized.Fingerprint)
			continue
		}
		results = append(results, resp)

		if h.audit != nil {
			action := "create"
			if isDup {
				action = "deduplicate"
			}
			detail, _ := json.Marshal(map[string]string{"title": resp.Title, "source": "alertmanager"})
			h.audit.LogFromRequest(r, action, "alert", resp.ID, detail)
		}
	}

	httpserver.Respond(w, http.StatusCreated, BatchResponse{
		AlertsProcessed: len(results),
		Alerts:          results,
	})
}

// handleKeep processes Keep webhook payloads.
func (h *WebhookHandler) handleKeep(w http.ResponseWriter, r *http.Request) {
	var payload keepPayload
	if err := decodeWebhookBody(r, &payload); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if payload.Name == "" {
		httpserver.RespondError(w, http.StatusUnprocessableEntity, "validation_error", "name is required")
		return
	}

	store := h.store(r)
	normalized := normalizeKeep(payload)
	resp, isDup, err := h.createOrDedup(r, store, normalized)
	if err != nil {
		h.logger.Error("processing alert from keep", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to process alert")
		return
	}

	if h.audit != nil {
		action := "create"
		if isDup {
			action = "deduplicate"
		}
		detail, _ := json.Marshal(map[string]string{"title": resp.Title, "source": "keep"})
		h.audit.LogFromRequest(r, action, "alert", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

// handleGeneric processes generic JSON webhook payloads.
func (h *WebhookHandler) handleGeneric(w http.ResponseWriter, r *http.Request) {
	var payload genericPayload
	if err := decodeWebhookBody(r, &payload); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	if payload.Title == "" {
		httpserver.RespondError(w, http.StatusUnprocessableEntity, "validation_error", "title is required")
		return
	}

	store := h.store(r)
	normalized := normalizeGeneric(payload)
	resp, isDup, err := h.createOrDedup(r, store, normalized)
	if err != nil {
		h.logger.Error("processing alert from generic webhook", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to process alert")
		return
	}

	if h.audit != nil {
		action := "create"
		if isDup {
			action = "deduplicate"
		}
		detail, _ := json.Marshal(map[string]string{"title": resp.Title, "source": normalized.Source})
		h.audit.LogFromRequest(r, action, "alert", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

// --- Normalization functions ---

// normalizeAlertmanager converts an Alertmanager alert to the internal format.
func normalizeAlertmanager(a alertmanagerAlert) NormalizedAlert {
	title := a.Labels["alertname"]
	if title == "" {
		title = "Unnamed Alertmanager Alert"
	}

	var desc *string
	if summary := a.Annotations["summary"]; summary != "" {
		desc = &summary
	} else if description := a.Annotations["description"]; description != "" {
		desc = &description
	}

	labels, _ := json.Marshal(a.Labels)
	annotations, _ := json.Marshal(a.Annotations)

	fp := a.Fingerprint
	if fp == "" {
		fp = generateFingerprint(title, labels)
	}

	return NormalizedAlert{
		Fingerprint: fp,
		Status:      normalizeStatus(a.Status),
		Severity:    normalizeSeverity(a.Labels["severity"]),
		Source:      "alertmanager",
		Title:       title,
		Description: desc,
		Labels:      labels,
		Annotations: annotations,
	}
}

// normalizeKeep converts a Keep alert to the internal format.
func normalizeKeep(p keepPayload) NormalizedAlert {
	labels, _ := json.Marshal(p.Labels)

	fp := p.Fingerprint
	if fp == "" {
		fp = generateFingerprint(p.Name, labels)
	}

	var desc *string
	if p.Description != "" {
		desc = &p.Description
	}

	annotations, _ := json.Marshal(map[string]any{
		"sources": p.Source,
		"keep_id": p.ID,
	})

	return NormalizedAlert{
		Fingerprint: fp,
		Status:      normalizeStatus(p.Status),
		Severity:    normalizeSeverity(p.Severity),
		Source:      "keep",
		Title:       p.Name,
		Description: desc,
		Labels:      labels,
		Annotations: annotations,
	}
}

// normalizeGeneric converts a generic webhook payload to the internal format.
func normalizeGeneric(p genericPayload) NormalizedAlert {
	labels, _ := json.Marshal(p.Labels)

	fp := p.Fingerprint
	if fp == "" {
		fp = generateFingerprint(p.Title, labels)
	}

	var desc *string
	if p.Description != "" {
		desc = &p.Description
	}

	source := p.Source
	if source == "" {
		source = "generic"
	}

	return NormalizedAlert{
		Fingerprint: fp,
		Status:      "firing",
		Severity:    normalizeSeverity(p.Severity),
		Source:      source,
		Title:       p.Title,
		Description: desc,
		Labels:      labels,
		Annotations: json.RawMessage(`{}`),
	}
}
