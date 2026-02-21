package slack

import (
	"context"
	"fmt"
	"log/slog"

	goslack "github.com/slack-go/slack"
)

// Notifier sends messages to Slack channels.
type Notifier struct {
	client  *goslack.Client
	channel string
	logger  *slog.Logger
}

// NewNotifier creates a Slack Notifier. If botToken is empty, the notifier
// will be a noop (logging only).
func NewNotifier(botToken, channel string, logger *slog.Logger) *Notifier {
	var client *goslack.Client
	if botToken != "" {
		client = goslack.New(botToken)
	}
	return &Notifier{
		client:  client,
		channel: channel,
		logger:  logger,
	}
}

// IsEnabled returns true if the notifier has a valid Slack client.
func (n *Notifier) IsEnabled() bool {
	return n.client != nil && n.channel != ""
}

// PostAlert sends an alert notification to the configured channel.
// Returns the channel ID and message timestamp for tracking.
func (n *Notifier) PostAlert(ctx context.Context, alert AlertInfo) (channelID, ts string, err error) {
	if !n.IsEnabled() {
		n.logger.Debug("slack notifier disabled, skipping alert post",
			"alert_id", alert.AlertID,
			"title", alert.Title,
		)
		return "", "", nil
	}

	blocks := AlertNotificationBlocks(alert)
	opts := []goslack.MsgOption{
		goslack.MsgOptionBlocks(blocks...),
		goslack.MsgOptionText(fmt.Sprintf("%s %s: %s", SeverityEmoji(alert.Severity), severity(alert.Severity), alert.Title), false),
	}

	channelID, ts, err = n.client.PostMessageContext(ctx, n.channel, opts...)
	if err != nil {
		return "", "", fmt.Errorf("posting alert to slack: %w", err)
	}

	n.logger.Info("posted alert to slack",
		"alert_id", alert.AlertID,
		"channel", channelID,
		"ts", ts,
	)
	return channelID, ts, nil
}

// UpdateMessage updates an existing Slack message.
func (n *Notifier) UpdateMessage(ctx context.Context, channelID, ts string, blocks []goslack.Block, fallbackText string) error {
	if !n.IsEnabled() {
		return nil
	}

	opts := []goslack.MsgOption{
		goslack.MsgOptionBlocks(blocks...),
		goslack.MsgOptionText(fallbackText, false),
	}

	_, _, _, err := n.client.UpdateMessageContext(ctx, channelID, ts, opts...)
	if err != nil {
		return fmt.Errorf("updating slack message: %w", err)
	}
	return nil
}

// PostThreadReply posts a reply in a thread.
func (n *Notifier) PostThreadReply(ctx context.Context, channelID, threadTS, text string) error {
	if !n.IsEnabled() {
		return nil
	}

	opts := []goslack.MsgOption{
		goslack.MsgOptionText(text, false),
		goslack.MsgOptionTS(threadTS),
	}

	_, _, err := n.client.PostMessageContext(ctx, channelID, opts...)
	if err != nil {
		return fmt.Errorf("posting thread reply to slack: %w", err)
	}
	return nil
}

// PostEphemeral posts an ephemeral message visible only to the specified user.
func (n *Notifier) PostEphemeral(ctx context.Context, channelID, userID, text string) error {
	if !n.IsEnabled() {
		return nil
	}

	_, err := n.client.PostEphemeralContext(ctx, channelID, userID, goslack.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("posting ephemeral message: %w", err)
	}
	return nil
}

// PostBlocksEphemeral posts ephemeral blocks visible only to the specified user.
func (n *Notifier) PostBlocksEphemeral(ctx context.Context, channelID, userID string, blocks []goslack.Block) error {
	if !n.IsEnabled() {
		return nil
	}

	_, err := n.client.PostEphemeralContext(ctx, channelID, userID,
		goslack.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("posting ephemeral blocks: %w", err)
	}
	return nil
}

// OpenModal opens a Slack modal view.
func (n *Notifier) OpenModal(ctx context.Context, triggerID string, modal goslack.ModalViewRequest) error {
	if !n.IsEnabled() {
		return nil
	}

	_, err := n.client.OpenViewContext(ctx, triggerID, modal)
	if err != nil {
		return fmt.Errorf("opening slack modal: %w", err)
	}
	return nil
}

// SendDM sends a direct message to a user by their Slack user ID.
func (n *Notifier) SendDM(ctx context.Context, slackUserID, text string) error {
	if !n.IsEnabled() {
		return nil
	}

	// Open a conversation (DM channel) with the user.
	channel, _, _, err := n.client.OpenConversationContext(ctx, &goslack.OpenConversationParameters{
		Users: []string{slackUserID},
	})
	if err != nil {
		return fmt.Errorf("opening DM conversation: %w", err)
	}

	_, _, err = n.client.PostMessageContext(ctx, channel.ID, goslack.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("sending DM: %w", err)
	}
	return nil
}
