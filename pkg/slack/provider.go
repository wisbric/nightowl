package slack

import (
	"context"
	"fmt"
	"log/slog"

	goslack "github.com/slack-go/slack"

	"github.com/wisbric/nightowl/pkg/messaging"
)

// Provider implements messaging.Provider for Slack.
type Provider struct {
	notifier *Notifier
	logger   *slog.Logger
}

// NewProvider creates a Slack messaging provider wrapping the existing notifier.
func NewProvider(notifier *Notifier, logger *slog.Logger) *Provider {
	return &Provider{notifier: notifier, logger: logger}
}

func (p *Provider) Name() string { return "slack" }

func (p *Provider) PostAlert(ctx context.Context, msg messaging.AlertMessage) (*messaging.MessageRef, error) {
	alert := AlertInfo{
		AlertID:           msg.AlertID,
		Title:             msg.Title,
		Severity:          msg.Severity,
		Description:       msg.Description,
		Cluster:           msg.Cluster,
		Namespace:         msg.Namespace,
		Service:           msg.Service,
		OnCallUser:        msg.PrimaryOnCall,
		SuggestedSolution: msg.Solution,
		RunbookURL:        msg.RunbookURL,
	}

	channelID, ts, err := p.notifier.PostAlert(ctx, alert)
	if err != nil {
		return nil, err
	}
	if channelID == "" {
		return nil, nil // notifier disabled
	}

	return &messaging.MessageRef{
		Provider:  "slack",
		ChannelID: channelID,
		MessageID: ts,
	}, nil
}

func (p *Provider) UpdateAlert(ctx context.Context, ref messaging.MessageRef, msg messaging.AlertMessage) error {
	var blocks []goslack.Block
	switch msg.Status {
	case "acknowledged":
		blocks = AlertAcknowledgedBlocks(msg.Title, msg.AcknowledgedBy)
	case "resolved":
		blocks = AlertResolvedBlocks(msg.Title, msg.ResolvedBy, msg.HasKBMatch)
	default:
		alert := AlertInfo{
			AlertID:           msg.AlertID,
			Title:             msg.Title,
			Severity:          msg.Severity,
			Description:       msg.Description,
			Cluster:           msg.Cluster,
			Namespace:         msg.Namespace,
			Service:           msg.Service,
			OnCallUser:        msg.PrimaryOnCall,
			SuggestedSolution: msg.Solution,
			RunbookURL:        msg.RunbookURL,
		}
		blocks = AlertNotificationBlocks(alert)
	}

	summary := messaging.AlertSummary(msg)
	return p.notifier.UpdateMessage(ctx, ref.ChannelID, ref.MessageID, blocks, summary)
}

func (p *Provider) PostEscalation(ctx context.Context, msg messaging.EscalationMessage) error {
	text := fmt.Sprintf("%s *Escalation — Tier %d*\n*Alert:* %s %s\n*Target:* %s\n*Method:* %s",
		messaging.SeverityEmoji(msg.Severity),
		msg.Tier, messaging.SeverityEmoji(msg.Severity), msg.Title,
		msg.TargetName, msg.NotifyMethod,
	)
	if msg.TargetUserRef != "" {
		text += fmt.Sprintf("\n<@%s>", msg.TargetUserRef)
	}
	if msg.AlertURL != "" {
		text += fmt.Sprintf("\n<%s|View Alert>", msg.AlertURL)
	}

	if !p.notifier.IsEnabled() {
		return nil
	}

	_, _, err := p.notifier.client.PostMessageContext(ctx, p.notifier.channel,
		goslack.MsgOptionText(text, false))
	return err
}

func (p *Provider) PostHandoff(ctx context.Context, msg messaging.HandoffMessage) error {
	text := fmt.Sprintf("*Shift Handoff — %s*\n*Outgoing:* %s\n*Incoming:* %s\n*Week:* %s",
		msg.RosterName, msg.OutgoingName, msg.IncomingName, msg.WeekStart)
	if msg.OpenAlerts > 0 {
		text += fmt.Sprintf("\n*Open Alerts:* %d", msg.OpenAlerts)
	}
	if msg.HandoffSummary != "" {
		text += fmt.Sprintf("\n\n%s", msg.HandoffSummary)
	}

	if !p.notifier.IsEnabled() {
		return nil
	}

	_, _, err := p.notifier.client.PostMessageContext(ctx, p.notifier.channel,
		goslack.MsgOptionText(text, false))
	return err
}

func (p *Provider) PostResolutionPrompt(ctx context.Context, msg messaging.ResolutionPromptMessage) error {
	text := fmt.Sprintf("Alert *%s* was resolved by %s.", msg.Title, msg.ResolvedBy)
	if msg.Resolution != "" {
		text += fmt.Sprintf("\n\n*Resolution notes:*\n%s", msg.Resolution)
	}
	text += "\n\nPlease add this to the knowledge base if it's a new issue."

	if msg.ResolverRef != "" {
		return p.notifier.SendDM(ctx, msg.ResolverRef, text)
	}
	return nil
}

func (p *Provider) SendDM(ctx context.Context, userRef string, msg messaging.DirectMessage) error {
	return p.notifier.SendDM(ctx, userRef, msg.Text)
}

func (p *Provider) LookupUser(ctx context.Context, email string) (string, error) {
	if !p.notifier.IsEnabled() {
		return "", nil
	}

	user, err := p.notifier.client.GetUserByEmailContext(ctx, email)
	if err != nil {
		return "", fmt.Errorf("looking up slack user by email: %w", err)
	}
	return user.ID, nil
}
