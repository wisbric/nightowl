package messaging

import "time"

// MessageRef identifies a sent message for future updates.
type MessageRef struct {
	Provider  string `json:"provider"`   // "slack" or "mattermost"
	ChannelID string `json:"channel_id"` // platform channel identifier
	MessageID string `json:"message_id"` // platform message identifier (Slack: ts, Mattermost: post_id)
}

// AlertMessage is the platform-agnostic alert notification.
type AlertMessage struct {
	AlertID        string
	Title          string
	Severity       string // critical, warning, info
	Status         string // firing, acknowledged, resolved
	Cluster        string
	Namespace      string
	Service        string
	Description    string
	FiredAt        time.Time
	AcknowledgedBy string // display name, empty if not acked
	ResolvedBy     string // display name, empty if not resolved

	// Enrichment from KB
	HasKBMatch   bool
	Solution     string // plain text solution summary
	RunbookTitle string // linked runbook title (empty if none)
	RunbookURL   string // deep link to runbook in NightOwl UI

	// On-call context
	PrimaryOnCall  string // display name
	PrimaryUserRef string // platform user ref for @mention
	SecondaryOnCall string

	// Action URLs (for button callbacks)
	AlertURL string // deep link to alert in NightOwl UI
}

// EscalationMessage notifies about an escalation event.
type EscalationMessage struct {
	AlertID        string
	Title          string
	Severity       string
	Tier           int
	TierLabel      string // "Tier 1: On-Call Primary"
	TargetName     string
	TargetUserRef  string // platform user ref for @mention
	NotifyMethod   string // "messaging_dm", "messaging_channel", "phone", "sms"
	TimeoutMinutes int    // how long until next tier
	AlertURL       string
}

// HandoffMessage notifies about shift changes.
type HandoffMessage struct {
	RosterName     string
	OutgoingName   string
	OutgoingRef    string
	IncomingName   string
	IncomingRef    string
	OpenAlerts     int
	HandoffSummary string // markdown: key events from last shift
	WeekStart      string // "Mar 03, 2026"
}

// ResolutionPromptMessage asks the resolver to document the solution.
type ResolutionPromptMessage struct {
	AlertID     string
	Title       string
	ResolvedBy  string
	ResolverRef string // platform user ref
	Resolution  string // notes from resolve action
	AlertURL    string
}

// DirectMessage is a simple DM to a user.
type DirectMessage struct {
	Text    string
	Urgency string // "critical", "normal"
}

// IncomingCommand represents a slash command from any platform.
type IncomingCommand struct {
	Command    string // "search", "oncall", "ack", "resolve", "roster"
	Args       string // everything after the command
	UserRef    string // platform user identifier
	UserEmail  string // for mapping to NightOwl user
	ChannelID  string
	TenantSlug string // resolved from webhook config
}

// CommandResponse is what we send back to the user.
type CommandResponse struct {
	Text      string
	Ephemeral bool // only visible to the command user
	Sections  []ResponseSection
}

// ResponseSection is a block of content in a response.
type ResponseSection struct {
	Title  string
	Body   string // markdown
	Fields []Field
}

// Field is a key-value pair for display.
type Field struct {
	Label string
	Value string
	Short bool // display side-by-side (2-column)
}

// IncomingAction represents a button click or dialog submission.
type IncomingAction struct {
	ActionID   string // "ack_alert", "escalate_alert", "add_to_kb", etc.
	Value      string // alert ID or other context
	UserRef    string
	UserEmail  string
	ChannelID  string
	MessageID  string // for updating the original message
	TenantSlug string

	// Dialog/modal submission data (for KB creation form)
	FormData map[string]string
}

// InteractionResponse tells the platform what to do after an action.
type InteractionResponse struct {
	UpdateMessage *AlertMessage // update the original message (nil = no update)
	Ephemeral     string        // ephemeral reply to the user
	OpenForm      *FormDef      // open a modal/dialog (nil = don't)
}

// FormDef defines a modal/dialog form.
type FormDef struct {
	Title  string
	Fields []FormField
}

// FormField defines a form input.
type FormField struct {
	ID          string
	Label       string
	Type        string // "text", "textarea", "select"
	Placeholder string
	Required    bool
	Options     []FormOption // for select type
}

// FormOption is a dropdown option.
type FormOption struct {
	Label string
	Value string
}
