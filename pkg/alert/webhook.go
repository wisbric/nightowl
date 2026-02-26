package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// --- Alertmanager payload types ---

type alertmanagerPayload struct {
	Version  string              `json:"version"`
	GroupKey string              `json:"groupKey"`
	Status   string              `json:"status"`
	Alerts   []alertmanagerAlert `json:"alerts"`
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
	Title         string            `json:"title"`
	Severity      string            `json:"severity"`
	Fingerprint   string            `json:"fingerprint"`
	Description   string            `json:"description"`
	Labels        map[string]string `json:"labels"`
	Source        string            `json:"source"`
	AgentMetadata *agentMetadata    `json:"agent_metadata,omitempty"`
}

type agentMetadata struct {
	AgentID      string  `json:"agent_id"`
	ActionTaken  string  `json:"action_taken"`
	ActionResult string  `json:"action_result"`
	AutoResolved bool    `json:"auto_resolved"`
	Confidence   float64 `json:"confidence"`
}

// WebhookMetrics holds the Prometheus metrics for webhook alert processing.
type WebhookMetrics struct {
	ReceivedTotal      *prometheus.CounterVec
	ProcessingDuration *prometheus.HistogramVec
	KBHitsTotal        prometheus.Counter
	AgentResolvedTotal prometheus.Counter
}

// WebhookHandler provides HTTP handlers for alert webhook endpoints.
type WebhookHandler struct {
	logger  *slog.Logger
	audit   *audit.Writer
	dedup   *Deduplicator
	enrich  *Enricher
	metrics *WebhookMetrics
}

// NewWebhookHandler creates a WebhookHandler.
func NewWebhookHandler(logger *slog.Logger, audit *audit.Writer, dedup *Deduplicator, enrich *Enricher, metrics *WebhookMetrics) *WebhookHandler {
	return &WebhookHandler{logger: logger, audit: audit, dedup: dedup, enrich: enrich, metrics: metrics}
}

// Routes returns a chi.Router with webhook routes mounted.
func (h *WebhookHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/alertmanager", h.handleAlertmanager)
	r.Post("/keep", h.handleKeep)
	r.Post("/generic", h.handleGeneric)
	return r
}

// recordReceived increments the received counter for the given source and severity.
func (h *WebhookHandler) recordReceived(source, severity string) {
	if h.metrics != nil && h.metrics.ReceivedTotal != nil {
		h.metrics.ReceivedTotal.WithLabelValues(source, severity).Inc()
	}
}

// recordDuration observes the processing duration for the given source.
func (h *WebhookHandler) recordDuration(source string, start time.Time) {
	if h.metrics != nil && h.metrics.ProcessingDuration != nil {
		h.metrics.ProcessingDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	}
}

// recordKBHit increments the KB hits counter.
func (h *WebhookHandler) recordKBHit() {
	if h.metrics != nil && h.metrics.KBHitsTotal != nil {
		h.metrics.KBHitsTotal.Inc()
	}
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

	// Enrich new alerts with knowledge base matches.
	if h.enrich != nil && normalized.Status == "firing" {
		result := h.enrich.Enrich(ctx, conn, resp.ID, normalized.Fingerprint, normalized.Title, normalized.Description)
		if result.IsEnriched {
			resp.MatchedIncidentID = &result.MatchedIncidentID
			if result.SuggestedSolution != "" {
				resp.SuggestedSolution = &result.SuggestedSolution
			}
			if result.RunbookURL != "" {
				resp.RunbookURL = &result.RunbookURL
			}
			h.recordKBHit()
		}
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
	start := time.Now()
	defer h.recordDuration("alertmanager", start)

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
	conn := tenant.ConnFromContext(r.Context())
	var results []Response
	for _, a := range payload.Alerts {
		normalized := normalizeAlertmanager(a)
		h.recordReceived("alertmanager", normalized.Severity)

		// Auto-resolve: if Alertmanager sends status=resolved, resolve the existing alert.
		if normalized.Status == "resolved" {
			q := db.New(conn)
			row, err := q.ResolveAlertByFingerprint(r.Context(), normalized.Fingerprint)
			if err != nil {
				h.logger.Warn("auto-resolve by fingerprint failed", "error", err, "fingerprint", normalized.Fingerprint)
				continue
			}
			resp := alertRowToResponse(row)
			results = append(results, resp)

			if h.audit != nil {
				detail, _ := json.Marshal(map[string]string{"title": resp.Title, "source": "alertmanager"})
				h.audit.LogFromRequest(r, "auto_resolve", "alert", resp.ID, detail)
			}
			continue
		}

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
	start := time.Now()
	defer h.recordDuration("keep", start)

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
	h.recordReceived("keep", normalized.Severity)
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
	start := time.Now()
	defer h.recordDuration("generic", start)

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
	h.recordReceived(normalized.Source, normalized.Severity)
	resp, isDup, err := h.createOrDedup(r, store, normalized)
	if err != nil {
		h.logger.Error("processing alert from generic webhook", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to process alert")
		return
	}

	// Agent auto-resolve: mark as resolved by agent and auto-create KB entry.
	if normalized.ResolvedByAgent && !isDup {
		ctx := r.Context()
		conn := tenant.ConnFromContext(ctx)
		q := db.New(conn)

		row, err := q.ResolveAlertByAgent(ctx, db.ResolveAlertByAgentParams{
			ID:                   resp.ID,
			AgentResolutionNotes: &normalized.AgentResolutionNotes,
		})
		if err != nil {
			h.logger.Error("resolving alert by agent", "error", err, "id", resp.ID)
		} else {
			resp = alertRowToResponse(row)
		}

		// Auto-create KB entry with the agent's action as the solution.
		h.createAgentKBEntry(ctx, conn, normalized)

		if h.metrics != nil && h.metrics.AgentResolvedTotal != nil {
			h.metrics.AgentResolvedTotal.Inc()
		}
	}

	if h.audit != nil {
		action := "create"
		if isDup {
			action = "deduplicate"
		} else if normalized.ResolvedByAgent {
			action = "agent_resolve"
		}
		detail, _ := json.Marshal(map[string]string{"title": resp.Title, "source": normalized.Source})
		h.audit.LogFromRequest(r, action, "alert", resp.ID, detail)
	}

	httpserver.Respond(w, http.StatusCreated, resp)
}

// createAgentKBEntry creates a knowledge base (incident) entry from an agent-resolved alert.
func (h *WebhookHandler) createAgentKBEntry(ctx context.Context, dbtx db.DBTX, normalized NormalizedAlert) {
	q := db.New(dbtx)
	category := "agent-resolved"
	_, err := q.CreateIncident(ctx, db.CreateIncidentParams{
		Title:         normalized.Title,
		Fingerprints:  []string{normalized.Fingerprint},
		Severity:      normalized.Severity,
		Category:      &category,
		Tags:          []string{"auto-resolved", "agent"},
		Services:      []string{},
		Clusters:      []string{},
		Namespaces:    []string{},
		Symptoms:      normalized.Description,
		ErrorPatterns: []string{},
		Solution:      &normalized.AgentResolutionNotes,
	})
	if err != nil {
		h.logger.Error("creating agent KB entry", "error", err, "title", normalized.Title)
	}
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

	status := "firing"
	var resolvedByAgent bool
	var agentNotes string
	if p.AgentMetadata != nil && p.AgentMetadata.AutoResolved {
		status = "resolved"
		resolvedByAgent = true
		agentNotes = fmt.Sprintf("Agent %s: %s (result: %s, confidence: %.2f)",
			p.AgentMetadata.AgentID, p.AgentMetadata.ActionTaken,
			p.AgentMetadata.ActionResult, p.AgentMetadata.Confidence)
	}

	return NormalizedAlert{
		Fingerprint:          fp,
		Status:               status,
		Severity:             normalizeSeverity(p.Severity),
		Source:               source,
		Title:                p.Title,
		Description:          desc,
		Labels:               labels,
		Annotations:          json.RawMessage(`{}`),
		ResolvedByAgent:      resolvedByAgent,
		AgentResolutionNotes: agentNotes,
	}
}
