package slack

import (
	"encoding/json"
	"fmt"

	goslack "github.com/slack-go/slack"
)

// SeverityEmoji returns the emoji prefix for a given severity level.
func SeverityEmoji(severity string) string {
	switch severity {
	case "critical":
		return "🔴"
	case "major":
		return "🟠"
	case "warning":
		return "🟡"
	case "info":
		return "🔵"
	default:
		return "⚪"
	}
}

// AlertNotificationBlocks builds Slack Block Kit blocks for an alert notification.
func AlertNotificationBlocks(alert AlertInfo) []goslack.Block {
	header := goslack.NewHeaderBlock(
		goslack.NewTextBlockObject(goslack.PlainTextType,
			fmt.Sprintf("%s %s: %s", SeverityEmoji(alert.Severity), severity(alert.Severity), alert.Title), true, false),
	)

	var fields []*goslack.TextBlockObject
	if alert.Cluster != "" {
		fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, fmt.Sprintf("*Cluster:* %s", alert.Cluster), false, false))
	}
	if alert.Namespace != "" {
		fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, fmt.Sprintf("*Namespace:* %s", alert.Namespace), false, false))
	}
	if alert.Service != "" {
		fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, fmt.Sprintf("*Service:* %s", alert.Service), false, false))
	}
	if alert.OnCallUser != "" {
		fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, fmt.Sprintf("*On-Call:* %s", alert.OnCallUser), false, false))
	}

	var blocks []goslack.Block
	blocks = append(blocks, header)

	if len(fields) > 0 {
		section := goslack.NewSectionBlock(nil, fields, nil)
		blocks = append(blocks, section)
	}

	if alert.Description != "" {
		descSection := goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType, truncate(alert.Description, 500), false, false),
			nil, nil,
		)
		blocks = append(blocks, descSection)
	}

	if alert.SuggestedSolution != "" {
		solutionSection := goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType,
				fmt.Sprintf("💡 *Known Solution:* %s", truncate(alert.SuggestedSolution, 500)), false, false),
			nil, nil,
		)
		blocks = append(blocks, solutionSection)
	}

	// Action buttons
	ackBtn := goslack.NewButtonBlockElement("ack_alert", alert.AlertID,
		goslack.NewTextBlockObject(goslack.PlainTextType, "✅ Acknowledge", true, false))

	escalateBtn := goslack.NewButtonBlockElement("escalate_alert", alert.AlertID,
		goslack.NewTextBlockObject(goslack.PlainTextType, "🔼 Escalate", true, false))
	escalateBtn.Style = goslack.StyleDanger

	actionElements := []goslack.BlockElement{ackBtn}

	if alert.RunbookURL != "" {
		runbookBtn := goslack.NewButtonBlockElement("view_runbook", alert.AlertID,
			goslack.NewTextBlockObject(goslack.PlainTextType, "📋 View Runbook", true, false))
		runbookBtn.URL = alert.RunbookURL
		actionElements = append(actionElements, runbookBtn)
	}

	actionElements = append(actionElements, escalateBtn)
	actions := goslack.NewActionBlock("alert_actions", actionElements...)
	blocks = append(blocks, actions)

	return blocks
}

// AlertAcknowledgedBlocks builds blocks for an acknowledgment update message.
func AlertAcknowledgedBlocks(alertTitle, acknowledgedBy string) []goslack.Block {
	return []goslack.Block{
		goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType,
				fmt.Sprintf("✅ Alert *%s* acknowledged by %s.", alertTitle, acknowledgedBy), false, false),
			nil, nil,
		),
	}
}

// AlertResolvedBlocks builds blocks for a resolution notification.
func AlertResolvedBlocks(alertTitle, resolvedBy string, hasKBEntry bool) []goslack.Block {
	text := fmt.Sprintf("✅ Alert *%s* resolved by %s.", alertTitle, resolvedBy)
	if !hasKBEntry {
		text += "\n\nThis looks like a *new issue* not in the knowledge base."
	}

	blocks := []goslack.Block{
		goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType, text, false, false),
			nil, nil,
		),
	}

	if !hasKBEntry {
		addKBBtn := goslack.NewButtonBlockElement("create_incident_modal", "",
			goslack.NewTextBlockObject(goslack.PlainTextType, "📝 Add to Knowledge Base", true, false))

		skipBtn := goslack.NewButtonBlockElement("skip_kb_entry", "",
			goslack.NewTextBlockObject(goslack.PlainTextType, "Skip", true, false))

		actions := goslack.NewActionBlock("resolve_actions", addKBBtn, skipBtn)
		blocks = append(blocks, actions)
	}

	return blocks
}

// SearchResultBlocks builds blocks for KB search results in Slack.
func SearchResultBlocks(query string, results []SearchResult) []goslack.Block {
	if len(results) == 0 {
		return []goslack.Block{
			goslack.NewSectionBlock(
				goslack.NewTextBlockObject(goslack.MarkdownType,
					fmt.Sprintf("No results found for *%s*.", query), false, false),
				nil, nil,
			),
		}
	}

	blocks := []goslack.Block{
		goslack.NewHeaderBlock(
			goslack.NewTextBlockObject(goslack.PlainTextType,
				fmt.Sprintf("Search results for \"%s\"", truncate(query, 50)), true, false),
		),
	}

	for i, r := range results {
		if i >= 3 {
			break
		}
		text := fmt.Sprintf("*%s %s*\n%s", SeverityEmoji(r.Severity), r.Title, truncate(r.Solution, 200))
		section := goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType, text, false, false),
			nil, nil,
		)
		blocks = append(blocks, section)
		if i < len(results)-1 && i < 2 {
			blocks = append(blocks, goslack.NewDividerBlock())
		}
	}

	return blocks
}

// OnCallBlocks builds blocks showing who is currently on-call.
func OnCallBlocks(entries []OnCallEntry) []goslack.Block {
	if len(entries) == 0 {
		return []goslack.Block{
			goslack.NewSectionBlock(
				goslack.NewTextBlockObject(goslack.MarkdownType, "No on-call rosters found.", false, false),
				nil, nil,
			),
		}
	}

	blocks := []goslack.Block{
		goslack.NewHeaderBlock(
			goslack.NewTextBlockObject(goslack.PlainTextType, "Current On-Call", true, false),
		),
	}

	for _, e := range entries {
		text := fmt.Sprintf("*%s:* %s", e.RosterName, e.UserDisplay)
		if e.Timezone != "" {
			text += fmt.Sprintf(" (%s)", e.Timezone)
		}
		if e.IsOverride {
			text += " _(override)_"
		}
		section := goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType, text, false, false),
			nil, nil,
		)
		blocks = append(blocks, section)
	}

	return blocks
}

// CreateIncidentModal builds a Slack modal for creating a KB entry from an alert.
func CreateIncidentModal(alertID, alertTitle, alertDescription, alertSeverity string) goslack.ModalViewRequest {
	titleInput := goslack.NewInputBlock("title", goslack.NewTextBlockObject(goslack.PlainTextType, "Title", false, false),
		nil, goslack.NewPlainTextInputBlockElement(goslack.NewTextBlockObject(goslack.PlainTextType, "Incident title", false, false), "title_input"))
	titleInput.DispatchAction = false

	// Pre-fill with alert title
	titleElement := titleInput.Element.(*goslack.PlainTextInputBlockElement)
	titleElement.InitialValue = alertTitle

	severityInput := goslack.NewInputBlock("severity",
		goslack.NewTextBlockObject(goslack.PlainTextType, "Severity", false, false), nil,
		goslack.NewOptionsSelectBlockElement(goslack.OptTypeStatic,
			goslack.NewTextBlockObject(goslack.PlainTextType, "Select severity", false, false), "severity_input",
			goslack.NewOptionBlockObject("critical", goslack.NewTextBlockObject(goslack.PlainTextType, "Critical", false, false), nil),
			goslack.NewOptionBlockObject("major", goslack.NewTextBlockObject(goslack.PlainTextType, "Major", false, false), nil),
			goslack.NewOptionBlockObject("warning", goslack.NewTextBlockObject(goslack.PlainTextType, "Warning", false, false), nil),
			goslack.NewOptionBlockObject("info", goslack.NewTextBlockObject(goslack.PlainTextType, "Info", false, false), nil),
		),
	)

	// Pre-select the alert severity
	selectElement := severityInput.Element.(*goslack.SelectBlockElement)
	selectElement.InitialOption = goslack.NewOptionBlockObject(alertSeverity,
		goslack.NewTextBlockObject(goslack.PlainTextType, severity(alertSeverity), false, false), nil)

	symptomsInput := goslack.NewInputBlock("symptoms",
		goslack.NewTextBlockObject(goslack.PlainTextType, "Symptoms", false, false), nil,
		goslack.NewPlainTextInputBlockElement(goslack.NewTextBlockObject(goslack.PlainTextType, "What did you observe?", false, false), "symptoms_input"))
	symptomsElement := symptomsInput.Element.(*goslack.PlainTextInputBlockElement)
	symptomsElement.Multiline = true
	if alertDescription != "" {
		symptomsElement.InitialValue = alertDescription
	}

	solutionInput := goslack.NewInputBlock("solution",
		goslack.NewTextBlockObject(goslack.PlainTextType, "Solution", false, false), nil,
		goslack.NewPlainTextInputBlockElement(goslack.NewTextBlockObject(goslack.PlainTextType, "How did you resolve it?", false, false), "solution_input"))
	solutionElement := solutionInput.Element.(*goslack.PlainTextInputBlockElement)
	solutionElement.Multiline = true

	privateMetadata, _ := json.Marshal(map[string]string{"alert_id": alertID})

	return goslack.ModalViewRequest{
		Type:            goslack.VTModal,
		Title:           goslack.NewTextBlockObject(goslack.PlainTextType, "Add to Knowledge Base", false, false),
		Submit:          goslack.NewTextBlockObject(goslack.PlainTextType, "Create", false, false),
		Close:           goslack.NewTextBlockObject(goslack.PlainTextType, "Cancel", false, false),
		CallbackID:      "create_incident_submit",
		PrivateMetadata: string(privateMetadata),
		Blocks: goslack.Blocks{
			BlockSet: []goslack.Block{titleInput, severityInput, symptomsInput, solutionInput},
		},
	}
}

// severity returns a human-readable severity label.
func severity(s string) string {
	switch s {
	case "critical":
		return "CRITICAL"
	case "major":
		return "MAJOR"
	case "warning":
		return "WARNING"
	case "info":
		return "INFO"
	default:
		return s
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
