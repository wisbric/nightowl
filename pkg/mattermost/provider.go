package mattermost

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wisbric/nightowl/pkg/messaging"
)

// Provider implements messaging.Provider for Mattermost.
type Provider struct {
	client    *Client
	channelID string // default channel for notifications
	actionURL string // URL for interactive action callbacks
	logger    *slog.Logger
	botUserID string // resolved on first use
}

// NewProvider creates a Mattermost messaging provider.
func NewProvider(client *Client, defaultChannelID, actionURL string, logger *slog.Logger) *Provider {
	return &Provider{
		client:    client,
		channelID: defaultChannelID,
		actionURL: actionURL,
		logger:    logger,
	}
}

func (p *Provider) Name() string { return "mattermost" }

func (p *Provider) PostAlert(ctx context.Context, msg messaging.AlertMessage) (*messaging.MessageRef, error) {
	if !p.client.IsEnabled() {
		return nil, nil
	}

	attachments := AlertAttachments(msg, p.actionURL)

	post, err := p.client.CreatePost(ctx, Post{
		ChannelID: p.channelID,
		Props:     map[string]any{"attachments": attachments},
	})
	if err != nil {
		return nil, fmt.Errorf("posting alert to mattermost: %w", err)
	}

	return &messaging.MessageRef{
		Provider:  "mattermost",
		ChannelID: p.channelID,
		MessageID: post.ID,
	}, nil
}

func (p *Provider) UpdateAlert(ctx context.Context, ref messaging.MessageRef, msg messaging.AlertMessage) error {
	if !p.client.IsEnabled() {
		return nil
	}

	var attachments []Attachment
	switch msg.Status {
	case "acknowledged":
		attachments = AlertAcknowledgedAttachments(msg.Title, msg.AcknowledgedBy)
	case "resolved":
		attachments = AlertResolvedAttachments(msg.Title, msg.ResolvedBy, msg.HasKBMatch)
	default:
		attachments = AlertAttachments(msg, p.actionURL)
	}

	_, err := p.client.UpdatePost(ctx, ref.MessageID, Post{
		ChannelID: ref.ChannelID,
		Props:     map[string]any{"attachments": attachments},
	})
	return err
}

func (p *Provider) PostEscalation(ctx context.Context, msg messaging.EscalationMessage) error {
	if !p.client.IsEnabled() {
		return nil
	}

	att := EscalationAttachment(msg)
	_, err := p.client.CreatePost(ctx, Post{
		ChannelID: p.channelID,
		Props:     map[string]any{"attachments": []Attachment{att}},
	})
	return err
}

func (p *Provider) PostHandoff(ctx context.Context, msg messaging.HandoffMessage) error {
	if !p.client.IsEnabled() {
		return nil
	}

	att := HandoffAttachment(msg)
	_, err := p.client.CreatePost(ctx, Post{
		ChannelID: p.channelID,
		Props:     map[string]any{"attachments": []Attachment{att}},
	})
	return err
}

func (p *Provider) PostResolutionPrompt(ctx context.Context, msg messaging.ResolutionPromptMessage) error {
	if !p.client.IsEnabled() || msg.ResolverRef == "" {
		return nil
	}

	text := fmt.Sprintf("Alert **%s** was resolved by %s.", msg.Title, msg.ResolvedBy)
	if msg.Resolution != "" {
		text += fmt.Sprintf("\n\n**Resolution notes:**\n%s", msg.Resolution)
	}
	text += "\n\nPlease add this to the knowledge base if it's a new issue."

	return p.SendDM(ctx, msg.ResolverRef, messaging.DirectMessage{Text: text, Urgency: "normal"})
}

func (p *Provider) SendDM(ctx context.Context, userRef string, msg messaging.DirectMessage) error {
	if !p.client.IsEnabled() {
		return nil
	}

	botID, err := p.getBotUserID(ctx)
	if err != nil {
		return fmt.Errorf("resolving bot user ID: %w", err)
	}

	dm, err := p.client.CreateDMChannel(ctx, [2]string{botID, userRef})
	if err != nil {
		return fmt.Errorf("creating DM channel: %w", err)
	}

	_, err = p.client.CreatePost(ctx, Post{
		ChannelID: dm.ID,
		Message:   msg.Text,
	})
	return err
}

func (p *Provider) LookupUser(ctx context.Context, email string) (string, error) {
	if !p.client.IsEnabled() {
		return "", nil
	}

	user, err := p.client.GetUserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("looking up mattermost user: %w", err)
	}
	return user.ID, nil
}

// getBotUserID lazily resolves the bot's own Mattermost user ID.
func (p *Provider) getBotUserID(ctx context.Context) (string, error) {
	if p.botUserID != "" {
		return p.botUserID, nil
	}

	me, err := p.client.GetMe(ctx)
	if err != nil {
		return "", err
	}
	p.botUserID = me.ID
	return p.botUserID, nil
}
