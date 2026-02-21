# NightOwl — Implementation Tasks

## Phase 1: Foundation (Weeks 1–2)

### 1.1 Project Scaffolding
- [ ] Initialize Go module (`github.com/wisbric/nightowl`)
- [ ] Set up project structure per architecture spec (`pkg/`, `cmd/`, `internal/`, `migrations/`)
- [ ] Configure `sqlc` with PostgreSQL schema
- [ ] Set up `golang-migrate` migration framework
- [ ] Docker build (multi-stage: build → scratch/distroless)
- [ ] Basic Makefile (build, test, migrate, lint)
- [ ] CI: GitHub Actions for lint, test, build

### 1.2 Database & Multi-Tenancy
- [ ] Create global schema migrations (tenants, api_keys)
- [ ] Create tenant schema template migrations (all tenant tables from data model spec)
- [ ] Implement tenant provisioning: create schema + run migrations
- [ ] Implement `search_path` middleware: extract tenant from JWT/API key → set schema
- [ ] Seed development data: 1 tenant, 2 users, sample services

### 1.3 Authentication & RBAC
- [ ] OIDC middleware: validate JWT, extract claims (sub, email, tenant)
- [ ] API key middleware: hash lookup, extract tenant + role
- [ ] RBAC middleware: check role against required permission per endpoint
- [ ] Health endpoints: `/healthz`, `/readyz` (DB + Redis ping)

### 1.4 Core API Framework
- [ ] Chi router setup with middleware chain (logging, recovery, CORS, auth, tenant, request ID)
- [ ] Structured JSON error responses
- [ ] Request validation (Go struct tags or manual)
- [ ] Pagination helper (cursor-based for alerts, offset for KB)
- [ ] Prometheus metrics middleware (`nightowl_api_request_duration_seconds`)

---

## Phase 2: Knowledge Base (Weeks 3–4)

### 2.1 Incidents CRUD
- [ ] `POST /api/v1/incidents` — create incident with all fields
- [ ] `GET /api/v1/incidents` — list with filters (severity, category, service, tags)
- [ ] `GET /api/v1/incidents/:id` — detail view with history
- [ ] `PUT /api/v1/incidents/:id` — update with diff tracking
- [ ] `DELETE /api/v1/incidents/:id` — soft delete (mark merged or archived)

### 2.2 Search
- [ ] `GET /api/v1/incidents/search?q=<text>` — full-text search using tsvector
- [ ] `GET /api/v1/incidents/fingerprint/:fp` — exact fingerprint lookup
- [ ] Weighted ranking: title/error_patterns (A), symptoms/root_cause (B), solution (C)
- [ ] Search result highlighting (ts_headline)

### 2.3 Incident Merge
- [ ] `POST /api/v1/incidents/:id/merge` — merge source into target
- [ ] Merge logic: combine fingerprints, services, error_patterns; keep best solution
- [ ] Update all alerts referencing merged incident

### 2.4 Incident History
- [ ] Trigger-based diff capture on update (store old/new values)
- [ ] `GET /api/v1/incidents/:id/history` — chronological change log

### 2.5 Runbooks
- [ ] CRUD endpoints for runbooks
- [ ] Pre-seed Kubernetes templates (pod crashloop, OOM, cert expiry, etcd, node not ready, PVC stuck, DNS failure)
- [ ] Link runbooks to incidents

### 2.6 Audit Logging
- [ ] Audit log writer (async, buffered writes)
- [ ] Log all CRUD operations with user, action, resource, diff

---

## Phase 3: Alert Engine (Weeks 5–6)

### 3.1 Webhook Receivers
- [ ] `POST /api/v1/webhooks/alertmanager` — parse Alertmanager payload
- [ ] `POST /api/v1/webhooks/keep` — parse Keep payload
- [ ] `POST /api/v1/webhooks/generic` — parse generic JSON
- [ ] Normalize all formats to internal alert struct
- [ ] API key auth on all webhook endpoints

### 3.2 Deduplication
- [ ] Redis-based dedup: fingerprint → alert_id with TTL
- [ ] On duplicate: increment occurrence_count, update last_fired_at
- [ ] DB fallback if Redis unavailable
- [ ] Metric: `nightowl_alerts_deduplicated_total`

### 3.3 Knowledge Base Enrichment
- [ ] On new alert: fingerprint lookup in incidents table
- [ ] If match: attach matched_incident_id, suggested_solution to alert
- [ ] If no fingerprint match: attempt text search on alert title/description

### 3.4 Alert Lifecycle
- [ ] `PATCH /api/v1/alerts/:id/acknowledge` — set acknowledged_by/at
- [ ] `PATCH /api/v1/alerts/:id/resolve` — set resolved, prompt KB update
- [ ] Alert list with filters: status, severity, service, source, time range
- [ ] Auto-resolve: if Alertmanager sends status=resolved, update alert

### 3.5 Agent Integration
- [ ] Parse `agent_metadata` from generic webhook
- [ ] If auto_resolved: create alert in resolved state, auto-create KB entry
- [ ] Metric: `nightowl_alerts_agent_resolved_total`

### 3.6 Alert Metrics
- [ ] `nightowl_alerts_received_total{source, severity}`
- [ ] `nightowl_alert_processing_duration_seconds{source}`
- [ ] `nightowl_kb_hits_total` (enrichment success)

---

## Phase 4: Roster & Escalation (Weeks 7–9)

### 4.1 Roster Management
- [ ] CRUD for rosters (schedules)
- [ ] CRUD for roster_members (with position ordering)
- [ ] CRUD for roster_overrides
- [ ] `GET /api/v1/rosters/:id/oncall` — who is on-call now
- [ ] `GET /api/v1/rosters/:id/oncall?at=<ISO8601>` — who is on-call at time X
- [ ] Rotation calculation: daily/weekly based on start_date + rotation_length
- [ ] Override takes precedence over calculated rotation

### 4.2 Follow-the-Sun
- [ ] Linked roster support: two rosters covering different timezone windows
- [ ] `is_follow_the_sun` flag: determines which sub-roster is active based on time
- [ ] Handoff time calculation across timezone boundaries
- [ ] API: return active sub-roster member based on query time

### 4.3 Escalation Policies
- [ ] CRUD for escalation policies
- [ ] Tier definition: timeout, notification methods, targets
- [ ] Dry-run endpoint: simulate escalation without triggering

### 4.4 Escalation Engine (Worker)
- [ ] Background goroutine: poll for unacknowledged critical alerts
- [ ] Timer-based escalation: check every 30s
- [ ] State machine: alert created → tier 1 notified → timeout → tier 2 → ...
- [ ] Redis pub/sub: receive acknowledgment events from API server
- [ ] Persist escalation_events for audit trail
- [ ] Metric: `nightowl_alerts_escalated_total{tier}`

### 4.5 Callout (Twilio)
- [ ] Twilio REST API integration: make calls, send SMS
- [ ] TwiML generation for voice calls (text-to-speech + digit gathering)
- [ ] Inbound webhook handlers: `/api/v1/twilio/acknowledge`, `/api/v1/twilio/escalate`
- [ ] SMS inbound: parse ACK/ESC replies
- [ ] Fallback: if Twilio fails, retry once then log error and escalate

### 4.6 Handoff Notifications
- [ ] Worker: at configured handoff times, generate handoff report
- [ ] Send outgoing summary (open incidents) to leaving on-call
- [ ] Send incoming briefing to new on-call
- [ ] Channel notification: "On-call has changed: <@new_person>"

### 4.7 Calendar Export
- [ ] `GET /api/v1/rosters/:id/export.ics` — generate iCal feed
- [ ] Include: rotation shifts, overrides, handoff times
- [ ] Subscribable URL (with API key auth)

---

## Phase 5: Slack Integration (Weeks 9–10)

### 5.1 Slack App Setup
- [ ] Slack signing secret verification middleware
- [ ] Event subscription handler (`/api/v1/slack/events`)
- [ ] Interaction handler (`/api/v1/slack/interactions`)
- [ ] Command handler (`/api/v1/slack/commands`)

### 5.2 Alert Notifications
- [ ] Post critical/major alerts to configured channel
- [ ] Block Kit message with: severity, cluster, namespace, service, on-call, solution
- [ ] Action buttons: Acknowledge, View Runbook, Escalate
- [ ] Update message when alert status changes (ack'd, resolved)
- [ ] Thread-based updates: follow-up to original alert message

### 5.3 Slash Commands
- [ ] `/nightowl search <query>` — KB search, top 3 results
- [ ] `/nightowl oncall [roster]` — current on-call display
- [ ] `/nightowl ack <alert-id>` — acknowledge from Slack
- [ ] `/nightowl resolve <alert-id> [notes]` — resolve with notes
- [ ] `/nightowl roster [name]` — upcoming schedule

### 5.4 Interactive Flows
- [ ] "Add to Knowledge Base" modal: pre-filled from alert data
- [ ] Modal submission → create incident via incidents service
- [ ] Ephemeral responses for confirmations

### 5.5 Slack Message Tracking
- [ ] Store channel_id + message_ts for each alert notification
- [ ] Use for updating messages when status changes

---

## Phase 6: Frontend (Weeks 10–13)

### 6.1 Setup
- [ ] Vite + React + TypeScript scaffold
- [ ] Tailwind + shadcn/ui configuration
- [ ] TanStack Router with auth guard
- [ ] TanStack Query client with API base URL config
- [ ] OIDC login flow (redirect to Keycloak/Dex)

### 6.2 Dashboard
- [ ] Alert overview: active alerts by severity (cards/counters)
- [ ] Recent incidents: latest KB entries
- [ ] Current on-call: who's on for each roster
- [ ] MTTR trend chart (Recharts)
- [ ] Top recurring alerts chart

### 6.3 Alert Views
- [ ] Alert list: sortable, filterable table (TanStack Table)
- [ ] Alert detail: labels, annotations, linked incident, escalation history
- [ ] Acknowledge/resolve actions with confirmation

### 6.4 Knowledge Base Views
- [ ] Search page with full-text search input
- [ ] Incident list: filterable by severity, category, service, tags
- [ ] Incident detail: all fields, linked alerts, resolution history
- [ ] Incident edit: form with markdown editor for solution/runbook
- [ ] Merge UI: select source and target incidents

### 6.5 Roster Views
- [ ] Calendar view: react-big-calendar showing rotation schedule
- [ ] Current on-call display with timezone context
- [ ] Override management: add/remove shift swaps
- [ ] Roster configuration: members, rotation type, handoff time

### 6.6 Escalation Policy Views
- [ ] Policy list with tier summary
- [ ] Policy editor: add/remove/reorder tiers, configure timeouts
- [ ] Dry-run test button

### 6.7 Admin Views
- [ ] User management: invite, role assignment, deactivation
- [ ] Tenant config: Slack workspace, Twilio, default timezone
- [ ] API key management: create, revoke, view usage
- [ ] Audit log viewer: filterable, searchable

---

## Phase 7: Deployment & Operations (Weeks 13–14)

### 7.1 Helm Chart
- [ ] Chart structure: templates for all Kubernetes resources
- [ ] values.yaml with sensible defaults
- [ ] Configurable: replicas, resources, ingress, TLS, external DB/Redis
- [ ] CNPG PostgreSQL cluster option
- [ ] Redis Sentinel option for HA
- [ ] ServiceMonitor for Prometheus Operator

### 7.2 Documentation
- [ ] README with quickstart
- [ ] Helm chart values documentation
- [ ] API documentation (OpenAPI/Swagger generated from code)
- [ ] Slack app setup guide
- [ ] Twilio setup guide
- [ ] OIDC provider configuration guide

### 7.3 Testing
- [ ] Unit tests for all domain services
- [ ] Integration tests with testcontainers (real PostgreSQL + Redis)
- [ ] Webhook receiver tests: send sample payloads, verify processing
- [ ] Escalation engine test: verify timer-based progression
- [ ] E2E: alert → dedup → enrich → notify → acknowledge → resolve → KB prompt

### 7.4 Observability
- [ ] Structured logging with `slog` (JSON output)
- [ ] OpenTelemetry trace export to SigNoz
- [ ] Grafana dashboard JSON for NightOwl metrics
- [ ] Alert rules: NightOwl itself (API errors, escalation failures, Slack delivery failures)
