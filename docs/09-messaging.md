# NightOwl — Pluggable Messaging (Slack + Mattermost)

> This spec refactors the existing Slack integration into a provider-agnostic messaging interface, then adds Mattermost as a second provider. Future providers (Teams, Discord, etc.) become a single package implementation.

---

## 1. Architecture

### 1.1 Current State

Everything is in `pkg/slack/` with direct Slack API calls hardcoded throughout:
- Alert notifications → Slack Block Kit
- Slash commands → Slack command format
- Interactive buttons → Slack interaction payloads
- Message tracking → `slack_message_mappings` table
- Signing secret verification → Slack-specific HMAC

### 1.2 Target State

```
pkg/
├── messaging/
│   ├── messaging.go          # Provider interface + message types
│   ├── registry.go           # Provider registry (lookup by name)
│   ├── renderer.go           # Shared: severity emoji, alert summary text
│   └── types.go              # Platform-agnostic message structs
├── slack/
│   ├── provider.go           # implements messaging.Provider
│   ├── formatter.go          # converts Message → Slack Block Kit JSON
│   ├── commands.go           # slash command handler
│   ├── interactions.go       # interactive message handler
│   ├── verify.go             # signing secret middleware
│   └── client.go             # Slack API wrapper
├── mattermost/
│   ├── provider.go           # implements messaging.Provider
│   ├── formatter.go          # converts Message → Mattermost attachment JSON
│   ├── commands.go           # slash command handler
│   ├── interactions.go       # interactive dialog/action handler
│   ├── verify.go             # token/HMAC verification middleware
│   └── client.go             # Mattermost API wrapper
```

### 1.3 Provider Selection

Per-tenant configuration. Each tenant can use a different provider (or none):

```json
{
  "messaging_provider": "slack",       // "slack" | "mattermost" | "none"
  "slack": {
    "bot_token": "xoxb-...",
    "signing_secret": "...",
    "default_channel": "#ops-alerts"
  },
  "mattermost": {
    "url": "https://mattermost.example.com",
    "bot_token": "...",
    "webhook_secret": "...",
    "default_channel_id": "abc123"
  }
}
```

A tenant can even have both enabled (e.g., Slack for internal team, Mattermost for customer-facing). The escalation policy tiers specify which provider to notify through.

---

## 2. Messaging Interface

### 2.1 Core Interface

```go
// pkg/messaging/messaging.go

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
```

### 2.2 Message Types

```go
// pkg/messaging/types.go

package messaging

import "time"

// MessageRef identifies a sent message for future updates.
type MessageRef struct {
    Provider  string // "slack" or "mattermost"
    ChannelID string // platform channel identifier
    MessageID string // platform message identifier (Slack: ts, Mattermost: post_id)
}

// AlertMessage is the platform-agnostic alert notification.
type AlertMessage struct {
    AlertID       string
    Title         string
    Severity      string    // critical, warning, info
    Status        string    // firing, acknowledged, resolved
    Cluster       string
    Namespace     string
    Service       string
    Description   string
    FiredAt       time.Time
    AcknowledgedBy string  // display name, empty if not acked
    ResolvedBy     string  // display name, empty if not resolved

    // Enrichment from KB
    HasKBMatch     bool
    Solution       string   // plain text solution summary
    RunbookTitle   string   // linked runbook title (empty if none)
    RunbookURL     string   // deep link to runbook in NightOwl UI

    // On-call context
    PrimaryOnCall   string  // display name
    PrimaryUserRef  string  // platform user ref for @mention
    SecondaryOnCall string

    // Action URLs (for button callbacks)
    AlertURL       string   // deep link to alert in NightOwl UI
}

// EscalationMessage notifies about an escalation event.
type EscalationMessage struct {
    AlertID        string
    Title          string
    Severity       string
    Tier           int
    TierLabel      string   // "Tier 1: On-Call Primary"
    TargetName     string
    TargetUserRef  string   // platform user ref for @mention
    NotifyMethod   string   // "slack_dm", "mattermost_dm", "phone", "sms"
    TimeoutMinutes int      // how long until next tier
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
    HandoffSummary string   // markdown: key events from last shift
    WeekStart      string   // "Mar 03, 2026"
}

// ResolutionPromptMessage asks the resolver to document the solution.
type ResolutionPromptMessage struct {
    AlertID      string
    Title        string
    ResolvedBy   string
    ResolverRef  string   // platform user ref
    Resolution   string   // notes from resolve action
    AlertURL     string
}

// DirectMessage is a simple DM to a user.
type DirectMessage struct {
    Text     string
    Urgency  string // "critical", "normal"
}

// IncomingCommand represents a slash command from any platform.
type IncomingCommand struct {
    Command    string   // "search", "oncall", "ack", "resolve", "roster"
    Args       string   // everything after the command
    UserRef    string   // platform user identifier
    UserEmail  string   // for mapping to NightOwl user
    ChannelID  string
    TenantSlug string   // resolved from webhook config
}

// CommandResponse is what we send back to the user.
type CommandResponse struct {
    Text        string
    Ephemeral   bool     // only visible to the command user
    Sections    []ResponseSection
}

// ResponseSection is a block of content in a response.
type ResponseSection struct {
    Title  string
    Body   string    // markdown
    Fields []Field   // key-value pairs
}

// Field is a key-value pair for display.
type Field struct {
    Label string
    Value string
    Short bool   // display side-by-side (2-column)
}

// IncomingAction represents a button click or dialog submission.
type IncomingAction struct {
    ActionID   string   // "ack_alert", "escalate_alert", "add_to_kb", etc.
    Value      string   // alert ID or other context
    UserRef    string
    UserEmail  string
    ChannelID  string
    MessageID  string   // for updating the original message
    TenantSlug string

    // Dialog/modal submission data (for KB creation form)
    FormData   map[string]string
}

// InteractionResponse tells the platform what to do after an action.
type InteractionResponse struct {
    UpdateMessage *AlertMessage  // update the original message (nil = no update)
    Ephemeral     string         // ephemeral reply to the user
    OpenForm      *FormDef       // open a modal/dialog (nil = don't)
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
```

### 2.3 Provider Registry

```go
// pkg/messaging/registry.go

package messaging

import "fmt"

// Registry holds all available messaging providers.
type Registry struct {
    providers map[string]Provider
}

func NewRegistry() *Registry {
    return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
    r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, error) {
    p, ok := r.providers[name]
    if !ok {
        return nil, fmt.Errorf("messaging provider %q not registered", name)
    }
    return p, nil
}

func (r *Registry) All() []Provider {
    result := make([]Provider, 0, len(r.providers))
    for _, p := range r.providers {
        result = append(result, p)
    }
    return result
}
```

---

## 3. Slack Provider

### 3.1 Overview

Refactor existing `pkg/slack/` to implement the `messaging.Provider` interface. Most of the existing code stays — it just gets wrapped.

### 3.2 Mapping

| messaging.Provider method | Slack implementation |
|---|---|
| `PostAlert` | `chat.postMessage` with Block Kit |
| `UpdateAlert` | `chat.update` with updated blocks |
| `PostEscalation` | `chat.postMessage` to channel + DM |
| `PostHandoff` | `chat.postMessage` to channel |
| `PostResolutionPrompt` | `chat.postMessage` as DM with modal trigger |
| `SendDM` | `conversations.open` + `chat.postMessage` |
| `LookupUser` | `users.lookupByEmail` |

### 3.3 Commands

| NightOwl command | Slack slash command |
|---|---|
| `search <query>` | `/nightowl search <query>` |
| `oncall [roster]` | `/nightowl oncall [roster]` |
| `ack <alert-id>` | `/nightowl ack <alert-id>` |
| `resolve <alert-id>` | `/nightowl resolve <alert-id>` |
| `roster [name]` | `/nightowl roster [name]` |

### 3.4 Webhook Routes

```
POST /api/v1/slack/events         # Event subscriptions (unchanged)
POST /api/v1/slack/interactions   # Button clicks, modal submissions (unchanged)
POST /api/v1/slack/commands       # Slash commands (unchanged)
```

Signing secret verification middleware stays Slack-specific in `pkg/slack/verify.go`.

---

## 4. Mattermost Provider

### 4.1 Mattermost API Overview

Mattermost provides:
- **REST API v4** — `POST /api/v4/posts` to send messages, full CRUD on posts
- **Bot Accounts** — create a bot, get access token, use like Slack bot
- **Slash Commands** — custom commands with outgoing webhooks
- **Interactive Messages** — buttons and menus on posts, with action URLs
- **Interactive Dialogs** — modal forms (similar to Slack modals)
- **Incoming Webhooks** — simple message posting (limited formatting)
- **Outgoing Webhooks** — triggered on keywords or channels

### 4.2 Mattermost Message Format

Mattermost uses **attachments** (similar to Slack's legacy attachments) rather than Block Kit:

```json
{
  "channel_id": "abc123",
  "message": "",
  "props": {
    "attachments": [
      {
        "fallback": "CRITICAL: Pod CrashLoopBackOff",
        "color": "#DC2626",
        "title": "🔴 CRITICAL: Pod CrashLoopBackOff",
        "fields": [
          { "short": true, "title": "Cluster", "value": "production-de-01" },
          { "short": true, "title": "Namespace", "value": "customer-api" },
          { "short": true, "title": "Service", "value": "payment-gateway" },
          { "short": true, "title": "On-Call", "value": "@stefan.k" }
        ],
        "text": "💡 **Known Solution:** OOM kill detected. Scale memory limit from 256Mi to 512Mi.",
        "actions": [
          {
            "id": "ack_alert",
            "name": "✅ Acknowledge",
            "integration": {
              "url": "https://nightowl.example.com/api/v1/mattermost/actions",
              "context": { "action": "ack", "alert_id": "uuid" }
            }
          },
          {
            "id": "view_runbook",
            "name": "📋 View Runbook",
            "type": "button",
            "integration": {
              "url": "https://nightowl.example.com/runbooks/xyz"
            }
          },
          {
            "id": "escalate_alert",
            "name": "🔼 Escalate",
            "style": "danger",
            "integration": {
              "url": "https://nightowl.example.com/api/v1/mattermost/actions",
              "context": { "action": "escalate", "alert_id": "uuid" }
            }
          }
        ]
      }
    ]
  }
}
```

### 4.3 Mapping

| messaging.Provider method | Mattermost implementation |
|---|---|
| `PostAlert` | `POST /api/v4/posts` with attachments + actions |
| `UpdateAlert` | `PUT /api/v4/posts/{post_id}` with updated attachments |
| `PostEscalation` | `POST /api/v4/posts` to channel + DM |
| `PostHandoff` | `POST /api/v4/posts` to channel |
| `PostResolutionPrompt` | DM post with "Add to KB" button → opens dialog |
| `SendDM` | `POST /api/v4/channels/direct` + `POST /api/v4/posts` |
| `LookupUser` | `GET /api/v4/users/email/{email}` |

### 4.4 Commands

Mattermost slash commands work via outgoing webhook. NightOwl registers one command:

```
/nightowl <subcommand> [args]
```

The Mattermost server sends a POST to NightOwl's command endpoint. The payload is simpler than Slack's:

```json
{
  "channel_id": "abc123",
  "channel_name": "ops-alerts",
  "command": "/nightowl",
  "text": "search pod crashloop",
  "token": "verification_token",
  "user_id": "user123",
  "user_name": "stefan.k"
}
```

NightOwl parses the subcommand and args, routes through the same `CommandHandler` logic, and returns a response formatted for Mattermost:

```json
{
  "response_type": "ephemeral",
  "text": "### KB Search Results\n| # | Title | Severity |\n|---|---|---|\n| 1 | Pod CrashLoopBackOff | Critical |"
}
```

### 4.5 Interactive Actions

When a user clicks a button, Mattermost POSTs to the action URL:

```json
{
  "user_id": "user123",
  "user_name": "stefan.k",
  "channel_id": "abc123",
  "post_id": "post456",
  "context": {
    "action": "ack",
    "alert_id": "uuid"
  }
}
```

NightOwl processes through `InteractionHandler`, acknowledges the alert, and returns an update:

```json
{
  "update": {
    "message": "",
    "props": { "attachments": [ /* updated attachment with ack status */ ] }
  },
  "ephemeral_text": "✅ Alert acknowledged"
}
```

### 4.6 Interactive Dialogs (KB Creation)

When the user clicks "Add to Knowledge Base", NightOwl opens a Mattermost dialog:

```json
POST /api/v4/actions/dialogs/open
{
  "trigger_id": "from_action_payload",
  "url": "https://nightowl.example.com/api/v1/mattermost/dialogs",
  "dialog": {
    "title": "Add to Knowledge Base",
    "submit_label": "Create Incident",
    "elements": [
      { "display_name": "Title", "name": "title", "type": "text", "default": "Pod CrashLoopBackOff" },
      { "display_name": "Severity", "name": "severity", "type": "select",
        "options": [
          { "text": "Critical", "value": "critical" },
          { "text": "Warning", "value": "warning" },
          { "text": "Info", "value": "info" }
        ]
      },
      { "display_name": "Solution", "name": "solution", "type": "textarea", "default": "" },
      { "display_name": "Category", "name": "category", "type": "text" }
    ]
  }
}
```

### 4.7 Webhook Routes

```
POST /api/v1/mattermost/commands     # Slash commands
POST /api/v1/mattermost/actions      # Button clicks
POST /api/v1/mattermost/dialogs      # Dialog submissions
```

Verification: Mattermost sends a token with each request. Verify against the configured `webhook_secret`.

### 4.8 Mattermost Bot Setup Guide

Include in README / docs:

1. Go to **System Console → Integrations → Bot Accounts** → Enable
2. Go to **Integrations → Bot Accounts** → Create bot `nightowl`
3. Copy the access token
4. Go to **Integrations → Slash Commands** → Create:
   - Command: `/nightowl`
   - Request URL: `https://nightowl.example.com/api/v1/mattermost/commands`
   - Request Method: POST
   - Autocomplete: enabled
5. Configure NightOwl tenant: set `messaging_provider: mattermost`, paste bot token and server URL

---

## 5. Data Model Changes

### 5.1 Rename Table

```sql
ALTER TABLE slack_message_mappings RENAME TO message_mappings;
```

### 5.2 Update Schema

```sql
CREATE TABLE message_mappings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,         -- "slack" or "mattermost"
    channel_id  TEXT NOT NULL,         -- platform channel identifier
    message_id  TEXT NOT NULL,         -- Slack: message ts, Mattermost: post ID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(alert_id, provider)
);

CREATE INDEX idx_message_mappings_alert ON message_mappings(alert_id);
```

### 5.3 Tenant Config Update

Add to tenant configuration:

```sql
ALTER TABLE tenants
    ADD COLUMN messaging_provider TEXT NOT NULL DEFAULT 'none',  -- 'slack', 'mattermost', 'none'
    ADD COLUMN mattermost_url TEXT,
    ADD COLUMN mattermost_bot_token TEXT,
    ADD COLUMN mattermost_webhook_secret TEXT,
    ADD COLUMN mattermost_default_channel_id TEXT;
```

Existing Slack columns remain (`slack_bot_token`, `slack_signing_secret`, `slack_default_channel`).

---

## 6. Escalation Policy Update

The escalation tiers already specify `notify_via`. Extend to support both platforms:

```json
{
  "tiers": [
    {
      "tier": 1,
      "timeout_minutes": 5,
      "notify_via": ["messaging_dm"],
      "targets": ["oncall_primary"]
    },
    {
      "tier": 2,
      "timeout_minutes": 10,
      "notify_via": ["messaging_dm", "phone"],
      "targets": ["oncall_secondary"]
    },
    {
      "tier": 3,
      "timeout_minutes": 15,
      "notify_via": ["phone", "messaging_channel"],
      "targets": ["team_lead"]
    }
  ]
}
```

**Notify method mapping:**

| Tier method | Old (Slack-specific) | New (provider-agnostic) |
|---|---|---|
| DM the target | `slack_dm` | `messaging_dm` |
| Post to channel | `slack_channel` | `messaging_channel` |
| Phone call | `phone` | `phone` (unchanged) |
| SMS | `sms` | `sms` (unchanged) |

The escalation engine resolves `messaging_dm` → calls `provider.SendDM()` on whichever provider the tenant uses. `messaging_channel` → calls `provider.PostEscalation()`.

Backward compatibility: existing `slack_dm` and `slack_channel` values are treated as aliases for `messaging_dm` and `messaging_channel`.

---

## 7. Frontend Changes

### 7.1 Tenant Configuration Page

Add messaging provider selection:

```
┌─── Messaging Configuration ──────────────────────────┐
│                                                       │
│  Provider:  ( ) None  (●) Slack  ( ) Mattermost      │
│                                                       │
│  ── Slack Settings ────────────────────────────────── │
│  Bot Token:       [ xoxb-******************** ]       │
│  Signing Secret:  [ ************************* ]       │
│  Default Channel: [ #ops-alerts               ]       │
│  [Test Connection]  ✅ Connected                      │
│                                                       │
│  ── Mattermost Settings (disabled) ───────────────── │
│  Server URL:      [ _________________________ ]       │
│  Bot Token:       [ _________________________ ]       │
│  Webhook Secret:  [ _________________________ ]       │
│  Default Channel: [ _________________________ ]       │
│  [Test Connection]                                    │
│                                                       │
│                              [Cancel]  [Save]         │
└───────────────────────────────────────────────────────┘
```

### 7.2 Escalation Policy Editor

Update notify method dropdowns:

```
Before: "Slack DM" | "Slack Channel" | "Phone" | "SMS"
After:  "Message (DM)" | "Message (Channel)" | "Phone" | "SMS"
```

The label adapts to show the active provider: "Message via Slack (DM)" or "Message via Mattermost (DM)".

### 7.3 Status Page

Add messaging health check:

```
Messaging: Slack ✅ connected (workspace: Wisbric)
```
or
```
Messaging: Mattermost ✅ connected (server: mattermost.example.com)
```

---

## 8. Test Connection Endpoint

```
POST /api/v1/admin/messaging/test
Body: { "provider": "mattermost", "url": "...", "bot_token": "..." }

→ Attempts to connect and post a test message
→ Returns: { "ok": true, "workspace": "Wisbric", "bot_name": "nightowl" }
→ Or: { "ok": false, "error": "invalid token" }
```

Works for both Slack and Mattermost. Used by the frontend "Test Connection" button.

---

## 9. Migration Plan

### 9.1 Implementation Order

1. **Create `pkg/messaging/`** — interface, types, registry (pure Go, no external deps)
2. **Refactor `pkg/slack/`** — implement `messaging.Provider`, extract Slack-specific formatting
3. **Rename `slack_message_mappings` → `message_mappings`** — add `provider` column
4. **Update escalation engine** — use `messaging.Provider` instead of direct Slack calls
5. **Update alert pipeline** — use provider for notifications
6. **Update worker** — handoff notifications via provider
7. **Create `pkg/mattermost/`** — implement `messaging.Provider` for Mattermost
8. **Add Mattermost webhook routes** — commands, actions, dialogs
9. **Update tenant config** — add messaging provider selection + Mattermost fields
10. **Update frontend** — config page, escalation editor, status page
11. **Update tenant config API** — add Mattermost fields to GET/PUT
12. **Add test connection endpoint**
13. **Update demo seed** — default to Slack (existing behavior)
14. **Documentation** — Mattermost setup guide

### 9.2 Backward Compatibility

- Existing Slack-only tenants continue working — `messaging_provider` defaults to `slack` if `slack_bot_token` is set, `none` otherwise
- Existing `slack_dm` / `slack_channel` escalation tier values are treated as aliases
- Existing `slack_message_mappings` data migrated to `message_mappings` with `provider='slack'`

---

## 10. Implementation Prompt for Claude Code

```
Read docs/09-messaging.md and implement the pluggable messaging system:

1. Create pkg/messaging/ with the Provider interface, message types, 
   and provider registry

2. Refactor pkg/slack/ to implement messaging.Provider — extract 
   the interface, keep Slack-specific formatting in the package

3. Database migration: rename slack_message_mappings to message_mappings, 
   add provider column, add Mattermost config columns to tenants

4. Create pkg/mattermost/ implementing messaging.Provider — 
   REST API v4 client, message formatting, slash commands, 
   interactive actions, dialog support

5. Add Mattermost webhook routes: /api/v1/mattermost/commands, 
   /api/v1/mattermost/actions, /api/v1/mattermost/dialogs

6. Update escalation engine: use messaging.Provider instead of 
   direct Slack calls. Support messaging_dm and messaging_channel 
   notify methods.

7. Update alert notification pipeline to use the provider registry

8. Update tenant config API and frontend: messaging provider 
   selection, Mattermost settings, test connection button

9. Update escalation policy editor: provider-agnostic notify labels

10. Add POST /api/v1/admin/messaging/test endpoint

11. Build, test, fix, and commit when green.
```

---

## 11. Future Providers

Adding a new provider (e.g., Microsoft Teams, Google Chat, Discord) requires:

1. Create `pkg/<provider>/provider.go` implementing `messaging.Provider`
2. Create `pkg/<provider>/formatter.go` for platform-specific message format
3. Add webhook routes for the platform's callbacks
4. Register in the provider registry at startup
5. Add tenant config fields
6. Add to frontend provider selector

The core alert pipeline, escalation engine, and command routing remain unchanged.
