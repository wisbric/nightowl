package slack

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for Slack integration.
type Handler struct {
	notifier       *Notifier
	pool           *pgxpool.Pool
	logger         *slog.Logger
	signingSecret  string
	defaultTenant  string // slug of the default tenant for Slack interactions
}

// NewHandler creates a Slack Handler.
func NewHandler(notifier *Notifier, pool *pgxpool.Pool, logger *slog.Logger, signingSecret, defaultTenant string) *Handler {
	return &Handler{
		notifier:      notifier,
		pool:          pool,
		logger:        logger,
		signingSecret: signingSecret,
		defaultTenant: defaultTenant,
	}
}

// Routes returns a chi.Router with Slack webhook routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(VerifyMiddleware(h.signingSecret))
	r.Post("/events", h.handleEvents)
	r.Post("/interactions", h.handleInteractions)
	r.Post("/commands", h.handleCommands)
	return r
}

// acquireTenantConn acquires a connection with the default tenant's search_path.
func (h *Handler) acquireTenantConn(r *http.Request) (*pgxpool.Conn, *db.Queries, error) {
	schema := tenant.SchemaName(h.defaultTenant)
	conn, err := h.pool.Acquire(r.Context())
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.Exec(r.Context(), "SET search_path TO "+schema+", public"); err != nil {
		conn.Release()
		return nil, nil, err
	}
	return conn, db.New(conn), nil
}

// --- Event handler ---

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Parse the outer envelope to determine the event type.
	var envelope struct {
		Type      string `json:"type"`
		Token     string `json:"token"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge (required during Slack app setup).
	if envelope.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": envelope.Challenge})
		return
	}

	// Parse the full event.
	evt, err := slackevents.ParseEvent(body, slackevents.OptionNoVerifyToken())
	if err != nil {
		h.logger.Error("parsing slack event", "error", err)
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}

	switch evt.Type {
	case slackevents.CallbackEvent:
		h.handleCallbackEvent(evt)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCallbackEvent(evt slackevents.EventsAPIEvent) {
	switch ev := evt.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		h.logger.Info("app mention received",
			"user", ev.User,
			"channel", ev.Channel,
			"text", ev.Text,
		)
	case *slackevents.MessageEvent:
		h.logger.Info("dm received",
			"user", ev.User,
			"channel", ev.Channel,
			"text", ev.Text,
		)
	default:
		h.logger.Debug("unhandled callback event", "type", evt.InnerEvent.Type)
	}
}

// --- Interaction handler ---

func (h *Handler) handleInteractions(w http.ResponseWriter, r *http.Request) {
	payload := r.FormValue("payload")
	if payload == "" {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	var ic goslack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &ic); err != nil {
		h.logger.Error("parsing interaction callback", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	switch ic.Type {
	case goslack.InteractionTypeBlockActions:
		h.handleBlockActions(w, r, ic)
	case goslack.InteractionTypeViewSubmission:
		h.handleViewSubmission(w, r, ic)
	default:
		h.logger.Debug("unhandled interaction type", "type", ic.Type)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) handleBlockActions(w http.ResponseWriter, r *http.Request, ic goslack.InteractionCallback) {
	for _, action := range ic.ActionCallback.BlockActions {
		switch action.ActionID {
		case "ack_alert":
			h.handleAckAction(r, ic, action.Value)
		case "escalate_alert":
			h.handleEscalateAction(r, ic, action.Value)
		case "create_incident_modal":
			h.handleCreateIncidentModalAction(r, ic)
		case "skip_kb_entry":
			h.logger.Info("user skipped KB entry", "user", ic.User.ID)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleAckAction(r *http.Request, ic goslack.InteractionCallback, alertIDStr string) {
	alertID, err := uuid.Parse(alertIDStr)
	if err != nil {
		h.logger.Error("invalid alert_id in ack action", "value", alertIDStr)
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		h.logger.Error("acquiring tenant connection for ack", "error", err)
		return
	}
	defer conn.Release()

	alert, err := q.GetAlert(r.Context(), alertID)
	if err != nil {
		h.logger.Error("getting alert for ack", "error", err, "alert_id", alertID)
		return
	}

	if alert.Status == "acknowledged" || alert.Status == "resolved" {
		_ = h.notifier.PostEphemeral(r.Context(), ic.Channel.ID, ic.User.ID,
			"Alert is already "+alert.Status+".")
		return
	}

	// Acknowledge the alert.
	_, err = q.AcknowledgeAlert(r.Context(), db.AcknowledgeAlertParams{
		ID: alertID,
	})
	if err != nil {
		h.logger.Error("acknowledging alert from slack", "error", err, "alert_id", alertID)
		return
	}

	// Post thread reply using the message_mappings table.
	var channelID2, messageID string
	_ = conn.QueryRow(r.Context(),
		"SELECT channel_id, message_id FROM message_mappings WHERE alert_id = $1 AND provider = 'slack' LIMIT 1",
		alertID,
	).Scan(&channelID2, &messageID)
	if messageID != "" {
		_ = h.notifier.PostThreadReply(r.Context(), channelID2, messageID,
			"✅ Acknowledged by <@"+ic.User.ID+">")
	}

	_ = h.notifier.PostEphemeral(r.Context(), ic.Channel.ID, ic.User.ID,
		"Alert acknowledged.")

	h.logger.Info("alert acknowledged via slack",
		"alert_id", alertID,
		"user", ic.User.ID,
	)
}

func (h *Handler) handleEscalateAction(r *http.Request, ic goslack.InteractionCallback, alertIDStr string) {
	h.logger.Info("escalation requested via slack",
		"alert_id", alertIDStr,
		"user", ic.User.ID,
	)
	_ = h.notifier.PostEphemeral(r.Context(), ic.Channel.ID, ic.User.ID,
		"Escalation triggered for alert "+alertIDStr+".")
}

func (h *Handler) handleCreateIncidentModalAction(r *http.Request, ic goslack.InteractionCallback) {
	// Find the alert info to pre-fill the modal.
	alertIDStr := ""
	for _, action := range ic.ActionCallback.BlockActions {
		if action.ActionID == "create_incident_modal" {
			alertIDStr = action.Value
			break
		}
	}

	var alertTitle, alertDesc, alertSeverity string
	if alertIDStr != "" {
		if alertID, err := uuid.Parse(alertIDStr); err == nil {
			conn, q, err := h.acquireTenantConn(r)
			if err == nil {
				alert, err := q.GetAlert(r.Context(), alertID)
				if err == nil {
					alertTitle = alert.Title
					alertSeverity = alert.Severity
					if alert.Description != nil {
						alertDesc = *alert.Description
					}
				}
				conn.Release()
			}
		}
	}

	modal := CreateIncidentModal(alertIDStr, alertTitle, alertDesc, alertSeverity)
	_ = h.notifier.OpenModal(r.Context(), ic.TriggerID, modal)
}

func (h *Handler) handleViewSubmission(w http.ResponseWriter, r *http.Request, ic goslack.InteractionCallback) {
	switch ic.View.CallbackID {
	case "create_incident_submit":
		h.handleCreateIncidentSubmit(r, ic)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleCreateIncidentSubmit(r *http.Request, ic goslack.InteractionCallback) {
	values := ic.View.State.Values

	title := values["title"]["title_input"].Value
	severityOpt := values["severity"]["severity_input"].SelectedOption.Value
	symptoms := values["symptoms"]["symptoms_input"].Value
	solution := values["solution"]["solution_input"].Value

	h.logger.Info("incident created from slack modal",
		"title", title,
		"severity", severityOpt,
		"symptoms_len", len(symptoms),
		"solution_len", len(solution),
		"user", ic.User.ID,
	)

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		h.logger.Error("acquiring tenant connection for incident creation", "error", err)
		return
	}
	defer conn.Release()

	_, err = q.CreateIncident(r.Context(), db.CreateIncidentParams{
		Title:    title,
		Severity: severityOpt,
		Symptoms: &symptoms,
		Solution: &solution,
	})
	if err != nil {
		h.logger.Error("creating incident from slack modal", "error", err)
		return
	}

	h.logger.Info("incident created successfully from slack", "title", title)
}

// --- Command handler ---

func (h *Handler) handleCommands(w http.ResponseWriter, r *http.Request) {
	cmd, err := goslack.SlashCommandParse(r)
	if err != nil {
		http.Error(w, "invalid command", http.StatusBadRequest)
		return
	}

	h.logger.Info("slash command received",
		"command", cmd.Command,
		"text", cmd.Text,
		"user", cmd.UserID,
		"channel", cmd.ChannelID,
	)

	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		respondJSON(w, map[string]string{
			"response_type": "ephemeral",
			"text":          "Usage: /nightowl <search|oncall|ack|resolve|roster> [args]",
		})
		return
	}

	subcommand := strings.ToLower(parts[0])
	args := parts[1:]

	switch subcommand {
	case "search":
		h.handleSearchCommand(w, r, cmd, args)
	case "oncall":
		h.handleOnCallCommand(w, r, cmd, args)
	case "ack":
		h.handleAckCommand(w, r, cmd, args)
	case "resolve":
		h.handleResolveCommand(w, r, cmd, args)
	case "roster":
		h.handleRosterCommand(w, r, cmd, args)
	default:
		respondJSON(w, map[string]string{
			"response_type": "ephemeral",
			"text":          "Unknown command: " + subcommand + ". Available: search, oncall, ack, resolve, roster",
		})
	}
}

func (h *Handler) handleSearchCommand(w http.ResponseWriter, r *http.Request, cmd goslack.SlashCommand, args []string) {
	if len(args) == 0 {
		respondJSON(w, map[string]string{
			"response_type": "ephemeral",
			"text":          "Usage: /nightowl search <query>",
		})
		return
	}

	query := strings.Join(args, " ")

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Internal error."})
		return
	}
	defer conn.Release()

	rows, err := q.SearchIncidents(r.Context(), db.SearchIncidentsParams{
		PlaintoTsquery: query,
		Limit:          3,
	})
	if err != nil {
		h.logger.Error("searching incidents from slack", "error", err)
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Search failed."})
		return
	}

	var results []SearchResult
	for _, row := range rows {
		sol := ""
		if row.Solution != nil {
			sol = *row.Solution
		}
		results = append(results, SearchResult{
			ID:       row.ID.String(),
			Title:    row.Title,
			Severity: row.Severity,
			Solution: sol,
		})
	}

	blocks := SearchResultBlocks(query, results)
	respondBlocks(w, "ephemeral", blocks)
}

func (h *Handler) handleOnCallCommand(w http.ResponseWriter, r *http.Request, cmd goslack.SlashCommand, args []string) {
	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Internal error."})
		return
	}
	defer conn.Release()

	rosters, err := q.ListRosters(r.Context())
	if err != nil {
		h.logger.Error("listing rosters from slack", "error", err)
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Failed to list rosters."})
		return
	}

	// If a roster name is specified, filter.
	filterName := ""
	if len(args) > 0 {
		filterName = strings.ToLower(strings.Join(args, " "))
	}

	var entries []OnCallEntry
	for _, roster := range rosters {
		if filterName != "" && !strings.Contains(strings.ToLower(roster.Name), filterName) {
			continue
		}
		entries = append(entries, OnCallEntry{
			RosterName:  roster.Name,
			UserDisplay: "calculating...", // placeholder — requires full on-call logic
			Timezone:    roster.Timezone,
		})
	}

	blocks := OnCallBlocks(entries)
	respondBlocks(w, "ephemeral", blocks)
}

func (h *Handler) handleAckCommand(w http.ResponseWriter, r *http.Request, cmd goslack.SlashCommand, args []string) {
	if len(args) == 0 {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Usage: /nightowl ack <alert-id>"})
		return
	}

	alertID, err := uuid.Parse(args[0])
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Invalid alert ID."})
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Internal error."})
		return
	}
	defer conn.Release()

	_, err = q.AcknowledgeAlert(r.Context(), db.AcknowledgeAlertParams{
		ID: alertID,
	})
	if err != nil {
		h.logger.Error("acknowledging alert from slash command", "error", err, "alert_id", alertID)
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Failed to acknowledge alert."})
		return
	}

	respondJSON(w, map[string]string{
		"response_type": "in_channel",
		"text":          "✅ Alert `" + alertID.String() + "` acknowledged by <@" + cmd.UserID + ">.",
	})
}

func (h *Handler) handleResolveCommand(w http.ResponseWriter, r *http.Request, cmd goslack.SlashCommand, args []string) {
	if len(args) == 0 {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Usage: /nightowl resolve <alert-id> [notes]"})
		return
	}

	alertID, err := uuid.Parse(args[0])
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Invalid alert ID."})
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Internal error."})
		return
	}
	defer conn.Release()

	_, err = q.ResolveAlert(r.Context(), db.ResolveAlertParams{
		ID: alertID,
	})
	if err != nil {
		h.logger.Error("resolving alert from slash command", "error", err, "alert_id", alertID)
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Failed to resolve alert."})
		return
	}

	text := "✅ Alert `" + alertID.String() + "` resolved by <@" + cmd.UserID + ">."
	if len(args) > 1 {
		notes := strings.Join(args[1:], " ")
		text += " Notes: " + notes
	}

	respondJSON(w, map[string]string{
		"response_type": "in_channel",
		"text":          text,
	})
}

func (h *Handler) handleRosterCommand(w http.ResponseWriter, r *http.Request, cmd goslack.SlashCommand, args []string) {
	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Internal error."})
		return
	}
	defer conn.Release()

	rosters, err := q.ListRosters(r.Context())
	if err != nil {
		h.logger.Error("listing rosters from slash command", "error", err)
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "Failed to list rosters."})
		return
	}

	filterName := ""
	if len(args) > 0 {
		filterName = strings.ToLower(strings.Join(args, " "))
	}

	var lines []string
	for _, roster := range rosters {
		if filterName != "" && !strings.Contains(strings.ToLower(roster.Name), filterName) {
			continue
		}
		lines = append(lines, "• *"+roster.Name+"* — weekly rotation ("+roster.Timezone+")")
	}

	if len(lines) == 0 {
		respondJSON(w, map[string]string{"response_type": "ephemeral", "text": "No rosters found."})
		return
	}

	respondJSON(w, map[string]string{
		"response_type": "ephemeral",
		"text":          "*Rosters:*\n" + strings.Join(lines, "\n"),
	})
}

// --- Helpers ---

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func respondBlocks(w http.ResponseWriter, responseType string, blocks []goslack.Block) {
	resp := map[string]any{
		"response_type": responseType,
		"blocks":        blocks,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
