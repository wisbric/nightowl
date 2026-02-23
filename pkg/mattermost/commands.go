package mattermost

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// commandPayload is the JSON body Mattermost sends for slash commands.
type commandPayload struct {
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Command     string `json:"command"`
	Text        string `json:"text"`
	Token       string `json:"token"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
}

// handleCommands processes Mattermost slash commands.
func (h *Handler) handleCommands(w http.ResponseWriter, r *http.Request) {
	// Mattermost sends slash commands as form-encoded.
	var cmd commandPayload
	cmd.ChannelID = r.FormValue("channel_id")
	cmd.ChannelName = r.FormValue("channel_name")
	cmd.Command = r.FormValue("command")
	cmd.Text = r.FormValue("text")
	cmd.Token = r.FormValue("token")
	cmd.UserID = r.FormValue("user_id")
	cmd.UserName = r.FormValue("user_name")

	h.logger.Info("mattermost slash command received",
		"command", cmd.Command,
		"text", cmd.Text,
		"user", cmd.UserID,
	)

	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		respondMM(w, "ephemeral", "Usage: /nightowl <search|oncall|ack|resolve|roster> [args]")
		return
	}

	subcommand := strings.ToLower(parts[0])
	args := parts[1:]

	switch subcommand {
	case "search":
		h.handleSearchCmd(w, r, cmd, args)
	case "oncall":
		h.handleOnCallCmd(w, r, cmd, args)
	case "ack":
		h.handleAckCmd(w, r, cmd, args)
	case "resolve":
		h.handleResolveCmd(w, r, cmd, args)
	case "roster":
		h.handleRosterCmd(w, r, cmd, args)
	default:
		respondMM(w, "ephemeral", "Unknown command: "+subcommand+". Available: search, oncall, ack, resolve, roster")
	}
}

func (h *Handler) handleSearchCmd(w http.ResponseWriter, r *http.Request, cmd commandPayload, args []string) {
	if len(args) == 0 {
		respondMM(w, "ephemeral", "Usage: /nightowl search <query>")
		return
	}

	query := strings.Join(args, " ")

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondMM(w, "ephemeral", "Internal error.")
		return
	}
	defer conn.Release()

	rows, err := q.SearchIncidents(r.Context(), db.SearchIncidentsParams{
		PlaintoTsquery: query,
		Limit:          3,
	})
	if err != nil {
		h.logger.Error("searching incidents from mattermost", "error", err)
		respondMM(w, "ephemeral", "Search failed.")
		return
	}

	if len(rows) == 0 {
		respondMM(w, "ephemeral", fmt.Sprintf("No results found for **%s**.", query))
		return
	}

	text := fmt.Sprintf("### KB Search Results for \"%s\"\n| # | Title | Severity |\n|---|---|---|\n", query)
	for i, row := range rows {
		text += fmt.Sprintf("| %d | %s | %s |\n", i+1, row.Title, row.Severity)
	}

	respondMM(w, "ephemeral", text)
}

func (h *Handler) handleOnCallCmd(w http.ResponseWriter, r *http.Request, cmd commandPayload, args []string) {
	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondMM(w, "ephemeral", "Internal error.")
		return
	}
	defer conn.Release()

	rosters, err := q.ListRosters(r.Context())
	if err != nil {
		h.logger.Error("listing rosters from mattermost", "error", err)
		respondMM(w, "ephemeral", "Failed to list rosters.")
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
		lines = append(lines, fmt.Sprintf("- **%s** (%s)", roster.Name, roster.Timezone))
	}

	if len(lines) == 0 {
		respondMM(w, "ephemeral", "No on-call rosters found.")
		return
	}

	respondMM(w, "ephemeral", "### Current On-Call\n"+strings.Join(lines, "\n"))
}

func (h *Handler) handleAckCmd(w http.ResponseWriter, r *http.Request, cmd commandPayload, args []string) {
	if len(args) == 0 {
		respondMM(w, "ephemeral", "Usage: /nightowl ack <alert-id>")
		return
	}

	alertID, err := uuid.Parse(args[0])
	if err != nil {
		respondMM(w, "ephemeral", "Invalid alert ID.")
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondMM(w, "ephemeral", "Internal error.")
		return
	}
	defer conn.Release()

	_, err = q.AcknowledgeAlert(r.Context(), db.AcknowledgeAlertParams{
		ID: alertID,
	})
	if err != nil {
		h.logger.Error("acknowledging alert from mattermost", "error", err, "alert_id", alertID)
		respondMM(w, "ephemeral", "Failed to acknowledge alert.")
		return
	}

	respondMM(w, "in_channel", fmt.Sprintf("Alert `%s` acknowledged by @%s.", alertID.String(), cmd.UserName))
}

func (h *Handler) handleResolveCmd(w http.ResponseWriter, r *http.Request, cmd commandPayload, args []string) {
	if len(args) == 0 {
		respondMM(w, "ephemeral", "Usage: /nightowl resolve <alert-id> [notes]")
		return
	}

	alertID, err := uuid.Parse(args[0])
	if err != nil {
		respondMM(w, "ephemeral", "Invalid alert ID.")
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondMM(w, "ephemeral", "Internal error.")
		return
	}
	defer conn.Release()

	_, err = q.ResolveAlert(r.Context(), db.ResolveAlertParams{
		ID: alertID,
	})
	if err != nil {
		h.logger.Error("resolving alert from mattermost", "error", err, "alert_id", alertID)
		respondMM(w, "ephemeral", "Failed to resolve alert.")
		return
	}

	text := fmt.Sprintf("Alert `%s` resolved by @%s.", alertID.String(), cmd.UserName)
	if len(args) > 1 {
		text += " Notes: " + strings.Join(args[1:], " ")
	}

	respondMM(w, "in_channel", text)
}

func (h *Handler) handleRosterCmd(w http.ResponseWriter, r *http.Request, cmd commandPayload, args []string) {
	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		respondMM(w, "ephemeral", "Internal error.")
		return
	}
	defer conn.Release()

	rosters, err := q.ListRosters(r.Context())
	if err != nil {
		h.logger.Error("listing rosters from mattermost", "error", err)
		respondMM(w, "ephemeral", "Failed to list rosters.")
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
		lines = append(lines, fmt.Sprintf("- **%s** — %s rotation (%s)", roster.Name, roster.RotationType, roster.Timezone))
	}

	if len(lines) == 0 {
		respondMM(w, "ephemeral", "No rosters found.")
		return
	}

	respondMM(w, "ephemeral", "**Rosters:**\n"+strings.Join(lines, "\n"))
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

// respondMM writes a Mattermost slash command response.
func respondMM(w http.ResponseWriter, responseType, text string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"response_type": responseType,
		"text":          text,
	})
}
