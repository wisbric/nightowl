package mattermost

import (
	"fmt"

	"github.com/wisbric/nightowl/pkg/messaging"
)

// Attachment is a Mattermost message attachment.
type Attachment struct {
	Fallback string            `json:"fallback"`
	Color    string            `json:"color,omitempty"`
	Title    string            `json:"title,omitempty"`
	Text     string            `json:"text,omitempty"`
	Fields   []AttachmentField `json:"fields,omitempty"`
	Actions  []Action          `json:"actions,omitempty"`
}

// AttachmentField is a key-value pair in an attachment.
type AttachmentField struct {
	Short bool   `json:"short"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// Action is an interactive button in an attachment.
type Action struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Style       string      `json:"style,omitempty"`
	Integration Integration `json:"integration,omitempty"`
}

// Integration specifies the callback for an action.
type Integration struct {
	URL     string         `json:"url"`
	Context map[string]any `json:"context,omitempty"`
}

// AlertAttachments builds Mattermost attachments for an alert notification.
func AlertAttachments(msg messaging.AlertMessage, actionsURL string) []Attachment {
	emoji := messaging.SeverityEmoji(msg.Severity)
	label := messaging.SeverityLabel(msg.Severity)
	color := messaging.SeverityColor(msg.Severity)
	title := fmt.Sprintf("%s %s: %s", emoji, label, msg.Title)
	fallback := title

	var fields []AttachmentField
	if msg.Cluster != "" {
		fields = append(fields, AttachmentField{Short: true, Title: "Cluster", Value: msg.Cluster})
	}
	if msg.Namespace != "" {
		fields = append(fields, AttachmentField{Short: true, Title: "Namespace", Value: msg.Namespace})
	}
	if msg.Service != "" {
		fields = append(fields, AttachmentField{Short: true, Title: "Service", Value: msg.Service})
	}
	if msg.PrimaryOnCall != "" {
		onCallVal := msg.PrimaryOnCall
		if msg.PrimaryUserRef != "" {
			onCallVal = "@" + msg.PrimaryUserRef
		}
		fields = append(fields, AttachmentField{Short: true, Title: "On-Call", Value: onCallVal})
	}

	var text string
	if msg.Description != "" {
		text = messaging.Truncate(msg.Description, 500) + "\n\n"
	}
	if msg.Solution != "" {
		text += fmt.Sprintf("**Known Solution:** %s\n\n", messaging.Truncate(msg.Solution, 500))
	}

	var actions []Action
	actions = append(actions, Action{
		ID:   "ack_alert",
		Name: "Acknowledge",
		Integration: Integration{
			URL:     actionsURL,
			Context: map[string]any{"action": "ack", "alert_id": msg.AlertID},
		},
	})
	if msg.RunbookURL != "" {
		actions = append(actions, Action{
			ID:   "view_runbook",
			Name: "View Runbook",
			Type: "button",
			Integration: Integration{
				URL: msg.RunbookURL,
			},
		})
	}
	actions = append(actions, Action{
		ID:    "escalate_alert",
		Name:  "Escalate",
		Style: "danger",
		Integration: Integration{
			URL:     actionsURL,
			Context: map[string]any{"action": "escalate", "alert_id": msg.AlertID},
		},
	})

	return []Attachment{{
		Fallback: fallback,
		Color:    color,
		Title:    title,
		Text:     text,
		Fields:   fields,
		Actions:  actions,
	}}
}

// AlertAcknowledgedAttachments builds attachments for an acknowledged alert.
func AlertAcknowledgedAttachments(title, acknowledgedBy string) []Attachment {
	return []Attachment{{
		Color: "#22C55E",
		Text:  fmt.Sprintf("**Alert %s** acknowledged by %s.", title, acknowledgedBy),
	}}
}

// AlertResolvedAttachments builds attachments for a resolved alert.
func AlertResolvedAttachments(title, resolvedBy string, hasKBEntry bool) []Attachment {
	text := fmt.Sprintf("**Alert %s** resolved by %s.", title, resolvedBy)
	if !hasKBEntry {
		text += "\n\nThis looks like a **new issue** not in the knowledge base."
	}
	return []Attachment{{
		Color: "#22C55E",
		Text:  text,
	}}
}

// EscalationAttachment builds an attachment for an escalation notification.
func EscalationAttachment(msg messaging.EscalationMessage) Attachment {
	emoji := messaging.SeverityEmoji(msg.Severity)
	color := messaging.SeverityColor(msg.Severity)
	text := fmt.Sprintf("**Escalation — Tier %d**\n**Alert:** %s %s\n**Target:** %s\n**Method:** %s",
		msg.Tier, emoji, msg.Title, msg.TargetName, msg.NotifyMethod)
	if msg.TargetUserRef != "" {
		text += fmt.Sprintf("\n@%s", msg.TargetUserRef)
	}

	return Attachment{
		Fallback: fmt.Sprintf("Escalation Tier %d: %s", msg.Tier, msg.Title),
		Color:    color,
		Text:     text,
	}
}

// HandoffAttachment builds an attachment for a shift handoff.
func HandoffAttachment(msg messaging.HandoffMessage) Attachment {
	text := fmt.Sprintf("**Shift Handoff — %s**\n**Outgoing:** %s\n**Incoming:** %s\n**Week:** %s",
		msg.RosterName, msg.OutgoingName, msg.IncomingName, msg.WeekStart)
	if msg.OpenAlerts > 0 {
		text += fmt.Sprintf("\n**Open Alerts:** %d", msg.OpenAlerts)
	}
	if msg.HandoffSummary != "" {
		text += "\n\n" + msg.HandoffSummary
	}

	return Attachment{
		Fallback: fmt.Sprintf("Shift handoff: %s", msg.RosterName),
		Color:    "#3B82F6",
		Text:     text,
	}
}
