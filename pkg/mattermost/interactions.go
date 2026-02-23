package mattermost

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// actionPayload is the JSON body Mattermost sends for button clicks.
type actionPayload struct {
	UserID    string         `json:"user_id"`
	UserName  string         `json:"user_name"`
	ChannelID string         `json:"channel_id"`
	PostID    string         `json:"post_id"`
	TriggerID string         `json:"trigger_id"`
	Context   map[string]any `json:"context"`
}

// actionResponse is what we return to Mattermost after an action.
type actionResponse struct {
	Update        *actionUpdate `json:"update,omitempty"`
	EphemeralText string        `json:"ephemeral_text,omitempty"`
}

type actionUpdate struct {
	Message string         `json:"message"`
	Props   map[string]any `json:"props,omitempty"`
}

// handleActions processes interactive message button clicks.
func (h *Handler) handleActions(w http.ResponseWriter, r *http.Request) {
	var payload actionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("decoding mattermost action payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	action, _ := payload.Context["action"].(string)
	alertIDStr, _ := payload.Context["alert_id"].(string)

	h.logger.Info("mattermost action received",
		"action", action,
		"alert_id", alertIDStr,
		"user", payload.UserID,
	)

	switch action {
	case "ack":
		h.handleAckAction(w, r, payload, alertIDStr)
	case "escalate":
		h.handleEscalateAction(w, r, payload, alertIDStr)
	default:
		respondActionJSON(w, actionResponse{EphemeralText: "Unknown action: " + action})
	}
}

func (h *Handler) handleAckAction(w http.ResponseWriter, r *http.Request, payload actionPayload, alertIDStr string) {
	alertID, err := uuid.Parse(alertIDStr)
	if err != nil {
		respondActionJSON(w, actionResponse{EphemeralText: "Invalid alert ID."})
		return
	}

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		h.logger.Error("acquiring tenant connection for mattermost ack", "error", err)
		respondActionJSON(w, actionResponse{EphemeralText: "Internal error."})
		return
	}
	defer conn.Release()

	alert, err := q.GetAlert(r.Context(), alertID)
	if err != nil {
		h.logger.Error("getting alert for mattermost ack", "error", err, "alert_id", alertID)
		respondActionJSON(w, actionResponse{EphemeralText: "Alert not found."})
		return
	}

	if alert.Status == "acknowledged" || alert.Status == "resolved" {
		respondActionJSON(w, actionResponse{EphemeralText: "Alert is already " + alert.Status + "."})
		return
	}

	_, err = q.AcknowledgeAlert(r.Context(), db.AcknowledgeAlertParams{ID: alertID})
	if err != nil {
		h.logger.Error("acknowledging alert from mattermost", "error", err, "alert_id", alertID)
		respondActionJSON(w, actionResponse{EphemeralText: "Failed to acknowledge alert."})
		return
	}

	acked := AlertAcknowledgedAttachments(alert.Title, "@"+payload.UserName)
	respondActionJSON(w, actionResponse{
		Update: &actionUpdate{
			Props: map[string]any{"attachments": acked},
		},
		EphemeralText: "Alert acknowledged.",
	})

	h.logger.Info("alert acknowledged via mattermost",
		"alert_id", alertID,
		"user", payload.UserID,
	)
}

func (h *Handler) handleEscalateAction(w http.ResponseWriter, r *http.Request, payload actionPayload, alertIDStr string) {
	h.logger.Info("escalation requested via mattermost",
		"alert_id", alertIDStr,
		"user", payload.UserID,
	)

	respondActionJSON(w, actionResponse{
		EphemeralText: "Escalation triggered for alert " + alertIDStr + ".",
	})
}

// dialogPayload is the JSON body Mattermost sends for dialog submissions.
type dialogPayload struct {
	UserID     string            `json:"user_id"`
	ChannelID  string            `json:"channel_id"`
	CallbackID string            `json:"callback_id"`
	Submission map[string]string `json:"submission"`
	State      string            `json:"state"` // we can store alert_id here
}

// handleDialogs processes interactive dialog submissions.
func (h *Handler) handleDialogs(w http.ResponseWriter, r *http.Request) {
	var payload dialogPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error("decoding mattermost dialog payload", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	h.logger.Info("mattermost dialog submission",
		"callback_id", payload.CallbackID,
		"user", payload.UserID,
		"submission", payload.Submission,
	)

	switch payload.CallbackID {
	case "create_incident":
		h.handleCreateIncidentDialog(w, r, payload)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) handleCreateIncidentDialog(w http.ResponseWriter, r *http.Request, payload dialogPayload) {
	title := payload.Submission["title"]
	sev := payload.Submission["severity"]
	symptoms := payload.Submission["symptoms"]
	solution := payload.Submission["solution"]

	conn, q, err := h.acquireTenantConn(r)
	if err != nil {
		h.logger.Error("acquiring tenant connection for mattermost incident creation", "error", err)
		respondDialogError(w, "Internal error")
		return
	}
	defer conn.Release()

	_, err = q.CreateIncident(r.Context(), db.CreateIncidentParams{
		Title:    title,
		Severity: sev,
		Symptoms: &symptoms,
		Solution: &solution,
	})
	if err != nil {
		h.logger.Error("creating incident from mattermost dialog", "error", err)
		respondDialogError(w, "Failed to create incident")
		return
	}

	h.logger.Info("incident created from mattermost dialog", "title", title)
	w.WriteHeader(http.StatusOK) // empty 200 = success, closes dialog
}

func respondActionJSON(w http.ResponseWriter, resp actionResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func respondDialogError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": map[string]string{"": msg},
	})
}

