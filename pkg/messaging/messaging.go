// Package messaging defines the provider-agnostic interface for sending
// notifications through Slack, Mattermost, or other chat platforms.
package messaging

import "context"

// Provider is the interface that all messaging platforms implement.
type Provider interface {
	// Name returns the provider identifier ("slack", "mattermost").
	Name() string

	// PostAlert sends an alert notification to the configured channel.
	// Returns a MessageRef for future updates.
	PostAlert(ctx context.Context, msg AlertMessage) (*MessageRef, error)

	// UpdateAlert updates an existing alert message (status change, ack, resolve).
	UpdateAlert(ctx context.Context, ref MessageRef, msg AlertMessage) error

	// PostEscalation sends an escalation notification.
	PostEscalation(ctx context.Context, msg EscalationMessage) error

	// PostHandoff sends shift handoff notifications (outgoing + incoming).
	PostHandoff(ctx context.Context, msg HandoffMessage) error

	// PostResolutionPrompt asks the resolver to add the solution to the KB.
	PostResolutionPrompt(ctx context.Context, msg ResolutionPromptMessage) error

	// SendDM sends a direct message to a user (for on-call notifications).
	SendDM(ctx context.Context, userRef string, msg DirectMessage) error

	// LookupUser resolves a NightOwl user to a platform-specific user reference.
	// Returns empty string if user not found on this platform.
	LookupUser(ctx context.Context, email string) (string, error)
}

// CommandHandler handles incoming slash commands from the platform.
type CommandHandler interface {
	// HandleCommand processes a slash command and returns a response.
	HandleCommand(ctx context.Context, cmd IncomingCommand) (*CommandResponse, error)
}

// InteractionHandler handles button clicks, dialog submissions, etc.
type InteractionHandler interface {
	// HandleInteraction processes an interactive action.
	HandleInteraction(ctx context.Context, action IncomingAction) (*InteractionResponse, error)
}
