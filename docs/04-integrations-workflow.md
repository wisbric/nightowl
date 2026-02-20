# OpsWatch — Integrations & Workflow Specification

## 1. Slack Integration

### 1.1 Slack App Configuration

OpsWatch operates as a Slack App (not a legacy bot) with the following scopes:

**Bot Token Scopes:**
- `chat:write` — post messages to channels
- `chat:write.customize` — post with custom username/icon
- `commands` — receive slash commands
- `im:write` — send DMs to on-call engineers
- `users:read` — resolve Slack user IDs to display names
- `users:read.email` — match Slack users to OpsWatch users

**Event Subscriptions:**
- `message.im` — receive DMs (for quick incident logging)
- `app_mention` — respond when @OpsWatch is mentioned

**Interactivity:**
- Request URL: `https://opswatch.example.com/api/v1/slack/interactions`
- Slash commands URL: `https://opswatch.example.com/api/v1/slack/commands`

### 1.2 Slash Commands

```
/opswatch search <query>
  → Searches knowledge base, returns top 3 results as Slack blocks
  → Each result has: title, severity, solution preview, "View Full" button

/opswatch oncall [roster-name]
  → Shows current on-call for specified roster (or all rosters if omitted)
  → Includes: name, timezone, local time, shift end time

/opswatch ack <alert-id>
  → Acknowledge an alert from Slack
  → Confirms with ephemeral message

/opswatch resolve <alert-id> [notes]
  → Resolve an alert with optional notes
  → Prompts to add to knowledge base if new issue

/opswatch roster [roster-name]
  → Shows upcoming rotation schedule (next 7 days)
  → Includes overrides
```

### 1.3 Alert Notification Message Format

```json
{
  "blocks": [
    {
      "type": "header",
      "text": { "type": "plain_text", "text": "🔴 CRITICAL: Pod CrashLoopBackOff" }
    },
    {
      "type": "section",
      "fields": [
        { "type": "mrkdwn", "text": "*Cluster:* production-de-01" },
        { "type": "mrkdwn", "text": "*Namespace:* customer-api" },
        { "type": "mrkdwn", "text": "*Service:* payment-gateway" },
        { "type": "mrkdwn", "text": "*On-Call:* <@U123456> (Stefan)" }
      ]
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "💡 *Known Solution:* OOM kill detected. Scale memory limit from 256Mi to 512Mi. See runbook."
      }
    },
    {
      "type": "actions",
      "elements": [
        { "type": "button", "text": { "type": "plain_text", "text": "✅ Acknowledge" }, "action_id": "ack_alert", "value": "alert_uuid" },
        { "type": "button", "text": { "type": "plain_text", "text": "📋 View Runbook" }, "action_id": "view_runbook", "url": "https://opswatch.example.com/runbooks/xyz" },
        { "type": "button", "text": { "type": "plain_text", "text": "🔼 Escalate" }, "action_id": "escalate_alert", "value": "alert_uuid", "style": "danger" }
      ]
    }
  ]
}
```

### 1.4 Resolution Prompt

When an alert is resolved and no matching KB entry exists:

```json
{
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "✅ Alert *Pod CrashLoopBackOff* resolved by <@U123456>.\n\nThis looks like a *new issue* not in the knowledge base."
      }
    },
    {
      "type": "actions",
      "elements": [
        {
          "type": "button",
          "text": { "type": "plain_text", "text": "📝 Add to Knowledge Base" },
          "action_id": "create_incident_modal",
          "value": "alert_uuid"
        },
        {
          "type": "button",
          "text": { "type": "plain_text", "text": "Skip" },
          "action_id": "skip_kb_entry",
          "value": "alert_uuid"
        }
      ]
    }
  ]
}
```

Clicking "Add to Knowledge Base" opens a Slack modal pre-filled with alert metadata.

## 2. Webhook Receivers

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
  "receiver": "opswatch",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "PodCrashLoopBackOff",
        "cluster": "production-de-01",
        "namespace": "customer-api",
        "pod": "payment-gateway-7f8b9c-xyz",
        "severity": "critical"
      },
      "annotations": {
        "summary": "Pod is in CrashLoopBackOff",
        "description": "Pod payment-gateway-7f8b9c-xyz has been restarting for 15 minutes",
        "runbook_url": "https://runbooks.example.com/pod-crashloop"
      },
      "startsAt": "2026-02-20T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "abc123def456"
    }
  ]
}
```

**Processing:**
1. Extract `fingerprint` from each alert
2. Map `severity` label to internal severity enum
3. Map `cluster`, `namespace` to service_id if matching service exists
4. Run through dedup → enrich → persist → notify pipeline

### 2.2 Keep Format

```
POST /api/v1/webhooks/keep
Header: X-API-Key: <tenant-api-key>
Content-Type: application/json

Body: Keep alert event format
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

Body: Any JSON
{
  "title": "Required: alert title",
  "severity": "critical",
  "fingerprint": "optional-dedup-key",
  "description": "Optional description",
  "labels": { "key": "value" },
  "source": "my-custom-system"
}
```

Field mapping configurable per API key in tenant config.

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
    "action_taken": "Increased memory limit from 256Mi to 512Mi and restarted pod",
    "action_result": "success",
    "auto_resolved": true,
    "confidence": 0.95
  }
}
```

If `agent_metadata.auto_resolved` is true, OpsWatch:
1. Creates the alert in resolved state
2. Auto-creates a KB entry with the agent's action as the solution
3. Posts a summary to Slack (informational, no escalation)

## 3. Telephony Integration (Twilio/Vonage)

### 3.1 Phone Callout Flow

```
Escalation engine triggers phone callout
        │
        ▼
  POST to Twilio REST API
  - From: configured Twilio number (per tenant)
  - To: on-call engineer's phone (E.164)
  - TwiML: text-to-speech message describing the alert
        │
        ▼
  TwiML script:
  "Critical alert for <service>. <summary>.
   Press 1 to acknowledge. Press 2 to escalate."
        │
        ├─ Digit 1: callback to /api/v1/twilio/acknowledge?alert_id=xxx
        │            → acknowledges alert, stops escalation
        │
        └─ Digit 2: callback to /api/v1/twilio/escalate?alert_id=xxx
                     → escalates to next tier
```

### 3.2 SMS Notification

Used as supplementary notification alongside phone calls:

```
"[OpsWatch] CRITICAL: PodCrashLoopBackOff in production-de-01/customer-api. 
Reply ACK to acknowledge or ESC to escalate. Alert ID: abc123"
```

Inbound SMS webhook processes replies.

## 4. Cross-Timezone Roster Workflow

### 4.1 Follow-the-Sun Example

```
Roster: "Global Primary"
├── Sub-roster: "APAC" (timezone: Pacific/Auckland)
│   ├── Handoff time: 08:00 NZST (covers 08:00-20:00 NZST)
│   └── Members: Stefan, Alice, Bob (rotation: weekly)
│
└── Sub-roster: "EMEA" (timezone: Europe/Berlin)
    ├── Handoff time: 08:00 CET (covers 08:00-20:00 CET)
    └── Members: Hans, Katja, Lars (rotation: weekly)

Timeline (UTC):
00:00 ──── 07:00 ──── 12:00 ──── 19:00 ──── 00:00
  │  APAC on-call  │  overlap  │  EMEA on-call  │  APAC
  │  (NZ evening)  │           │  (DE day)       │
  ├────────────────┤           ├────────────────┤
  19:00 NZST       08:00 CET  20:00 CET        08:00 NZST
```

### 4.2 Handoff Logic

```
GET /api/v1/rosters/:id/oncall?at=<timestamp>

1. Check override table for active override at given time → return if found
2. If follow-the-sun:
   a. Convert timestamp to each sub-roster's timezone
   b. Determine which sub-roster's shift covers this time
   c. Calculate rotation position within that sub-roster
3. If standard rotation:
   a. Calculate days since roster start_date
   b. Divide by rotation_length to get current cycle
   c. Current cycle modulo member count = position
4. Return on-call user with their timezone and shift boundaries
```

### 4.3 Handoff Notification

At each handoff time, the worker process sends:

**To outgoing on-call:**
```
"Your on-call shift is ending. Open incidents: 2 (1 critical, 1 warning).
[View Handoff Report]"
```

**To incoming on-call:**
```
"You are now on-call for Global Primary. Open incidents: 2.
- CRITICAL: etcd leader election failure (cluster prod-de-01) — acknowledged by Hans
- WARNING: High memory usage on ingress-controller (cluster prod-nz-01) — investigating
[View Dashboard]"
```

## 5. Alert → Incident Promotion

When grouped alerts indicate a major incident:

1. Engineer clicks "Promote to Incident" in UI or Slack
2. OpsWatch creates an incident record linked to all related alerts
3. A Slack thread is created as the incident war room
4. Escalation policy triggers if configured
5. All future updates to related alerts post to the incident thread
6. On resolution, engineer is prompted to fill in root cause and solution

## 6. Data Retention

Handled by `opswatch-cleanup` CronJob running daily:

```
For each tenant:
  - Delete resolved alerts older than retention_days_alerts (default: 90)
  - Archive audit_log entries older than retention_days_audit (default: 365)
  - Incidents/runbooks: never auto-deleted (knowledge is permanent)
  - Escalation events: follow alert retention
```
