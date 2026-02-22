# NightOwl — Integrations & Workflow Specification

## 1. Slack Integration

### 1.1 Slack App Configuration

NightOwl operates as a Slack App with the following scopes:

**Bot Token Scopes:**
- `chat:write` — post messages to channels
- `chat:write.customize` — post with custom username/icon
- `commands` — receive slash commands
- `im:write` — send DMs to on-call engineers
- `users:read` — resolve Slack user IDs to display names
- `users:read.email` — match Slack users to NightOwl users

**Event Subscriptions:**
- `message.im` — receive DMs (for quick incident logging)
- `app_mention` — respond when @NightOwl is mentioned

**Interactivity:**
- Request URL: `https://nightowl.example.com/api/v1/slack/interactions`
- Slash commands URL: `https://nightowl.example.com/api/v1/slack/commands`
- Events URL: `https://nightowl.example.com/api/v1/slack/events`

**Implementation:** `pkg/slack/` — handler.go, notifier.go, verify.go, messages.go, types.go

**Configuration:** Set via environment variables:
- `SLACK_BOT_TOKEN` — Bot user OAuth token
- `SLACK_SIGNING_SECRET` — Request signing secret for verification
- `SLACK_ALERT_CHANNEL` — Default channel for alert notifications

Slack endpoints are mounted outside the standard API auth chain at `/api/v1/slack/*` and verified using the Slack signing secret instead.

### 1.2 Slash Commands

```
/nightowl search <query>
  → Searches knowledge base, returns top 3 results as Slack blocks
  → Each result has: title, severity, solution preview, "View Full" button

/nightowl oncall [roster-name]
  → Shows current on-call for specified roster (or all rosters if omitted)
  → Includes: name, timezone, local time, shift end time

/nightowl ack <alert-id>
  → Acknowledge an alert from Slack
  → Confirms with ephemeral message

/nightowl resolve <alert-id> [notes]
  → Resolve an alert with optional notes
  → Prompts to add to knowledge base if new issue

/nightowl roster [roster-name]
  → Shows upcoming rotation schedule (next 7 days)
  → Includes overrides
```

### 1.3 Alert Notification Message Format

Block Kit messages posted to the configured alert channel with:
- Header: severity emoji + alert title
- Fields: cluster, namespace, service, on-call person
- Known solution section (if KB match found via enrichment)
- Action buttons: Acknowledge, View Runbook, Escalate

### 1.4 Message Tracking

Slack message timestamps (`message_ts`, `thread_ts`) are stored in `slack_message_mappings` table, linked to alert/incident IDs. This enables:
- Updating messages when alert status changes
- Thread-based follow-ups to original alert messages
- Bidirectional linking between Slack and NightOwl

### 1.5 Resolution Prompt

When an alert is resolved and no matching KB entry exists, a Slack message offers to create a KB entry with pre-filled metadata from the alert.

## 2. Webhook Receivers

All webhook receivers are implemented in `pkg/alert/webhook.go` and mounted at `/api/v1/webhooks/`. They require API key authentication via `X-API-Key` header.

### 2.1 Alertmanager Format

```
POST /api/v1/webhooks/alertmanager
Header: X-API-Key: <tenant-api-key>
Content-Type: application/json

Body: Standard Alertmanager webhook payload
{
  "version": "4",
  "groupKey": "...",
  "status": "firing",
  "receiver": "nightowl",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "PodCrashLoopBackOff",
        "cluster": "production-de-01",
        "namespace": "customer-api",
        "severity": "critical"
      },
      "annotations": {
        "summary": "Pod is in CrashLoopBackOff",
        "description": "Pod has been restarting for 15 minutes"
      },
      "startsAt": "2026-02-20T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "abc123def456"
    }
  ]
}
```

**Processing pipeline:**
1. Extract `fingerprint` from each alert
2. Map `severity` label to internal severity enum
3. Run through dedup (Redis 5min TTL, DB fallback) → enrich (KB fingerprint + text match) → persist → return
4. Record Prometheus metrics (`alerts_received_total`, `alert_processing_duration_seconds`)

### 2.2 Keep Format

```
POST /api/v1/webhooks/keep
Header: X-API-Key: <tenant-api-key>
Content-Type: application/json

Body:
{
  "id": "keep-alert-uuid",
  "name": "PodCrashLoopBackOff",
  "status": "firing",
  "severity": "critical",
  "source": ["prometheus", "signoz"],
  "fingerprint": "abc123def456",
  "labels": { ... },
  "description": "...",
  "lastReceived": "2026-02-20T10:00:00Z"
}
```

### 2.3 Generic Webhook

```
POST /api/v1/webhooks/generic
Header: X-API-Key: <tenant-api-key>
Content-Type: application/json

Body:
{
  "title": "Required: alert title",
  "severity": "critical",
  "fingerprint": "optional-dedup-key",
  "description": "Optional description",
  "labels": { "key": "value" },
  "source": "my-custom-system"
}
```

All webhook handlers use a lenient JSON decoder (no `DisallowUnknownFields`) to accept payloads with extra fields.

### 2.4 Agent-Created Alerts

Agents (automated remediation systems) use the generic webhook with additional fields:

```json
{
  "title": "OOM Kill - payment-gateway",
  "severity": "warning",
  "fingerprint": "oom-payment-gateway-prod",
  "source": "remediation-agent",
  "agent_metadata": {
    "agent_id": "k8s-healer-01",
    "action_taken": "Increased memory limit from 256Mi to 512Mi",
    "action_result": "success",
    "auto_resolved": true
  }
}
```

If `agent_metadata.auto_resolved` is true, NightOwl:
1. Creates the alert in resolved state
2. Auto-creates a KB entry with the agent's action as the solution
3. Records `nightowl_alerts_agent_resolved_total` metric

### 2.5 Deduplication

Implemented in `pkg/alert/dedup.go`:

1. **Redis check** (hot path): Key `alert:dedup:{schema}:{fingerprint}`, 5min TTL
2. If found: increment `occurrence_count` + `last_fired_at`, record `alerts_deduplicated_total` metric, skip further processing
3. **DB fallback** (if Redis unavailable): Query alerts by fingerprint where status != resolved and last_fired_at within 5 minutes
4. If new: set Redis key, proceed to enrichment + persist

### 2.6 Knowledge Base Enrichment

Implemented in `pkg/alert/enrich.go`:

1. Fingerprint lookup in `incidents.fingerprints` array
2. If match: set `matched_incident_id` and `suggested_solution` on the alert
3. If no fingerprint match: attempt full-text search on alert title
4. Record `kb_hits_total` metric on match

## 3. Telephony Integration (Twilio)

Implemented in `pkg/integration/` with a `CalloutService` interface and `TwilioHandler` implementation.

### 3.1 Phone Callout Flow

```
Escalation engine triggers phone callout
        │
        ▼
  POST to Twilio REST API
  - From: configured Twilio number (per tenant)
  - To: on-call engineer's phone (E.164 from users table)
  - TwiML: text-to-speech message describing the alert
        │
        ▼
  TwiML script:
  "Critical alert for <service>. <summary>.
   Press 1 to acknowledge. Press 2 to escalate."
        │
        ├─ Digit 1: callback to POST /api/v1/twilio/voice
        │            → acknowledges alert, stops escalation
        │
        └─ Digit 2: callback to POST /api/v1/twilio/voice
                     → escalates to next tier
```

### 3.2 SMS Notification

```
POST /api/v1/twilio/sms — inbound SMS webhook

"[NightOwl] CRITICAL: PodCrashLoopBackOff in production-de-01.
Reply ACK to acknowledge or ESC to escalate."
```

### 3.3 Noop Fallback

A `NoopCaller` stub is provided for environments without Twilio configuration. It logs callout requests without sending.

## 4. Cross-Timezone Roster Workflow

### 4.1 Follow-the-Sun

Two rosters can be linked via `linked_roster_id` with `is_follow_the_sun = true`. Each roster covers a 12-hour window starting from its `handoff_time`.

```
Roster: "Global Primary"
├── Sub-roster: "APAC" (timezone: Pacific/Auckland)
│   ├── Handoff time: 08:00 NZST (covers 08:00-20:00 NZST)
│   └── Members: Stefan, Alice, Bob (rotation: weekly)
│
└── Sub-roster: "EMEA" (timezone: Europe/Berlin)
    ├── Handoff time: 08:00 CET (covers 08:00-20:00 CET)
    └── Members: Hans, Katja, Lars (rotation: weekly)
```

### 4.2 Handoff Logic

Implemented in `pkg/roster/service.go`:

```
GET /api/v1/rosters/:id/oncall?at=<timestamp>

1. Check override table for active override at given time → if found:
   - Primary = override user (with display_name)
   - Secondary = who would normally be on-call
2. If follow-the-sun:
   a. Convert timestamp to each sub-roster's timezone
   b. Determine which sub-roster's 12-hour window covers this time
   c. Calculate rotation position within that sub-roster
3. If standard rotation:
   a. Calculate days since roster start_date
   b. Divide by rotation_length to get current cycle
   c. position = current_cycle % member_count → primary
   d. (position + 1) % member_count → secondary
4. Return primary + secondary on-call with display_name, shift boundaries
```

### 4.3 On-Call History

`GET /api/v1/rosters/:id/oncall/history` returns the last 10 completed rotation shifts by walking backwards through rotation cycles from the current time.

### 4.4 Roster Active Status

Rosters have an optional `end_date`. The API computes `is_active` (true if end_date is NULL or >= today). Inactive rosters are dimmed in the frontend list and filtered from the dashboard on-call widget.

### 4.5 iCal Export

`GET /api/v1/rosters/:id/export.ics` generates an iCal feed with:
- Rotation shift events for the next 30 days (using display names)
- Override events as separate calendar entries
- Subscribable via any calendar client

## 5. Escalation Engine

Implemented in `pkg/escalation/engine.go`, runs as a separate process (`--mode=worker`).

### 5.1 Engine Loop

- Polls every 30 seconds for unacknowledged `status='firing'` alerts
- Steps through escalation policy tiers based on alert age
- Creates `escalation_events` records for audit trail
- Publishes notifications via Redis pub/sub
- Listens for acknowledgment events on `nightowl:alert:ack` channel

### 5.2 Dry-Run

`POST /api/v1/escalation-policies/:id/dry-run` simulates the full escalation path without triggering notifications. Returns the sequence of tiers, timeouts, and cumulative time.

## 6. Audit Logging

All mutating operations (create, update, delete, acknowledge, resolve, merge) are logged via the async audit writer (`internal/audit/`).

- Non-blocking: channel capacity 256, batch flush at 32 entries or 2 second timeout
- Entries dropped (with warning log) if buffer is full
- Captures: user/API key ID, action, resource type, resource ID, detail JSON, IP, user agent
- Queryable via `GET /api/v1/audit-log` with filtering
