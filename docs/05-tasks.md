# NightOwl — Implementation Tasks

## Phase 1: Foundation (Weeks 1–2)

### 1.1 Project Scaffolding
- [x] Initialize Go module (`github.com/wisbric/nightowl`)
- [x] Set up project structure per architecture spec (`pkg/`, `cmd/`, `internal/`, `migrations/`)
- [x] Configure `sqlc` with PostgreSQL schema
- [x] Set up `golang-migrate` migration framework
- [x] Docker build (multi-stage: build → distroless)
- [x] Basic Makefile (build, test, migrate, lint, sqlc, seed, docker)
- [ ] CI: GitHub Actions for lint, test, build

### 1.2 Database & Multi-Tenancy
- [x] Create global schema migrations (tenants, api_keys)
- [x] Create tenant schema template migrations (users, services, escalation_policies, runbooks, incidents, incident_history, alerts, rosters, roster_members, roster_overrides, escalation_events, audit_log, slack_message_mappings)
- [x] Implement tenant provisioning: create schema + run migrations
- [x] Implement `search_path` middleware: extract tenant from auth → acquire connection → set schema
- [x] Seed development data: "acme" tenant, 4 users (Alice, Bob, Charlie, Diana), sample services
- [x] Dev API key: `ow_dev_seed_key_do_not_use_in_production`

### 1.3 Authentication & RBAC
- [x] OIDC middleware: validate JWT, extract claims (sub, email, tenant)
- [x] API key middleware: SHA-256 hash lookup, extract tenant + role
- [x] RBAC middleware: check role against required permission per endpoint
- [x] Dev fallback: `X-Tenant-Slug` header for development (no real auth, admin access)
- [x] Health endpoints: `/healthz` (liveness), `/readyz` (DB + Redis ping)

### 1.4 Core API Framework
- [x] Chi router setup with middleware chain (RequestID, logging, recovery, CORS, metrics, auth, tenant)
- [x] Structured JSON error responses via `httpserver.RespondError()`
- [x] Request validation via `httpserver.DecodeAndValidate()`
- [x] Pagination helper (offset-based)
- [x] Prometheus metrics middleware (`nightowl_api_request_duration_seconds`)

---

## Phase 2: Knowledge Base (Weeks 3–4)

### 2.1 Incidents CRUD
- [x] `POST /api/v1/incidents` — create incident with all fields
- [x] `GET /api/v1/incidents` — list with filters (severity, category, service, tags)
- [x] `GET /api/v1/incidents/:id` — detail view
- [x] `PUT /api/v1/incidents/:id` — update with diff tracking (incident_history)
- [x] `DELETE /api/v1/incidents/:id` — delete

### 2.2 Search
- [x] `GET /api/v1/incidents/search?q=<text>` — full-text search using tsvector
- [x] `GET /api/v1/incidents/fingerprint/:fp` — exact fingerprint lookup
- [x] Weighted ranking: title/error_patterns/category (A), symptoms/root_cause/services (B), solution/tags (C)
- [x] Search result highlighting (ts_headline)

### 2.3 Incident Merge
- [x] `POST /api/v1/incidents/:id/merge` — merge source into target
- [x] Merge logic: combine fingerprints, services, error_patterns; keep best solution
- [x] Update all alerts referencing merged incident

### 2.4 Incident History
- [x] Diff capture on update (store old/new values in incident_history)
- [x] `GET /api/v1/incidents/:id/history` — chronological change log

### 2.5 Runbooks
- [x] CRUD endpoints for runbooks (POST, GET, GET/:id, PUT, DELETE)
- [x] `GET /api/v1/runbooks/templates` — list pre-seeded templates
- [x] Pre-seed Kubernetes templates (pod crashloop, OOM, cert expiry, etcd, node not ready, PVC stuck, DNS failure)

### 2.6 Audit Logging
- [x] Async buffered audit log writer (channel-based, 256 capacity, flush at 32 entries or 2s)
- [x] `GET /api/v1/audit-log` — list endpoint with filtering
- [x] Log all CRUD operations with user/API key, action, resource, detail, IP, user agent

---

## Phase 3: Alert Engine (Weeks 5–6)

### 3.1 Webhook Receivers
- [x] `POST /api/v1/webhooks/alertmanager` — parse Alertmanager payload
- [x] `POST /api/v1/webhooks/keep` — parse Keep payload
- [x] `POST /api/v1/webhooks/generic` — parse generic JSON (lenient decoder)
- [x] Normalize all formats to internal alert struct
- [x] API key auth on all webhook endpoints

### 3.2 Deduplication
- [x] Redis-based dedup: `alert:dedup:{schema}:{fingerprint}` with 5min TTL
- [x] On duplicate: increment occurrence_count, update last_fired_at
- [x] DB fallback if Redis unavailable
- [x] Metric: `nightowl_alerts_deduplicated_total`

### 3.3 Knowledge Base Enrichment
- [x] On new alert: fingerprint lookup in incidents table
- [x] If match: attach matched_incident_id, suggested_solution to alert
- [x] If no fingerprint match: attempt text search on alert title
- [x] Metric: `nightowl_kb_hits_total`

### 3.4 Alert Lifecycle
- [x] `PATCH /api/v1/alerts/:id/acknowledge` — set acknowledged_by/at
- [x] `PATCH /api/v1/alerts/:id/resolve` — set resolved_by/at
- [x] Alert list with filters: status, severity, fingerprint
- [x] Auto-resolve: if Alertmanager sends status=resolved, update alert

### 3.5 Agent Integration
- [x] Parse `agent_metadata` from generic webhook
- [x] If auto_resolved: create alert in resolved state, auto-create KB entry
- [x] Metric: `nightowl_alerts_agent_resolved_total`

### 3.6 Alert Metrics
- [x] `nightowl_alerts_received_total{source, severity}`
- [x] `nightowl_alert_processing_duration_seconds`
- [x] `nightowl_kb_hits_total` (enrichment success)

---

## Phase 4: Roster & Escalation (Weeks 7–9)

### 4.1 Roster Management
- [x] CRUD for rosters (with optional end_date, is_active computed field)
- [x] CRUD for roster_members (with position ordering)
- [x] CRUD for roster_overrides
- [x] `GET /api/v1/rosters/:id/oncall` — primary + secondary on-call with display names
- [x] `GET /api/v1/rosters/:id/oncall?at=<RFC3339>` — on-call at specific time
- [x] `GET /api/v1/rosters/:id/oncall/history` — last 10 completed shifts
- [x] Rotation calculation: daily/weekly based on start_date + rotation_length
- [x] Override takes precedence over calculated rotation

### 4.2 Follow-the-Sun
- [x] Linked roster support: two rosters covering different timezone windows
- [x] `is_follow_the_sun` flag: determines which sub-roster is active based on time
- [x] 12-hour shift window calculation per roster timezone
- [x] API: return active sub-roster member based on query time

### 4.3 Escalation Policies
- [x] CRUD for escalation policies (with JSONB tiers)
- [x] Tier definition: timeout_minutes, notification methods, targets
- [x] `POST /api/v1/escalation-policies/:id/dry-run` — simulate escalation path
- [x] `GET /api/v1/escalation-policies/:id/events/:alertID` — escalation events for alert

### 4.4 Escalation Engine (Worker)
- [x] Background process: `--mode=worker`, poll every 30s for unacknowledged firing alerts
- [x] Timer-based escalation: step through tiers based on alert age
- [x] Redis pub/sub: receive acknowledgment events (`nightowl:alert:ack` channel)
- [x] Persist escalation_events for audit trail
- [x] Metric: `nightowl_alerts_escalated_total{tier}`

### 4.5 Callout (Twilio)
- [x] Twilio REST API integration: make calls, send SMS
- [x] TwiML generation for voice calls (text-to-speech + digit gathering)
- [x] Inbound webhook handlers: `POST /api/v1/twilio/voice`, `POST /api/v1/twilio/sms`
- [x] NoopCaller fallback for environments without Twilio

### 4.6 Handoff Notifications
- [ ] Worker: at configured handoff times, generate handoff report
- [ ] Send outgoing summary (open incidents) to leaving on-call
- [ ] Send incoming briefing to new on-call
- [ ] Channel notification: "On-call has changed: <@new_person>"

### 4.7 Calendar Export
- [x] `GET /api/v1/rosters/:id/export.ics` — generate iCal feed
- [x] Include: rotation shifts (30 days), overrides, display names in summaries
- [x] Downloadable via API key auth

---

## Phase 5: Slack Integration (Weeks 9–10)

### 5.1 Slack App Setup
- [x] Slack signing secret verification middleware (`pkg/slack/verify.go`)
- [x] Event subscription handler (`/api/v1/slack/events`)
- [x] Interaction handler (`/api/v1/slack/interactions`)
- [x] Command handler (`/api/v1/slack/commands`)

### 5.2 Alert Notifications
- [x] Post critical/major alerts to configured channel
- [x] Block Kit message with: severity, cluster, namespace, service, on-call, solution
- [x] Action buttons: Acknowledge, View Runbook, Escalate
- [x] Update message when alert status changes
- [x] Thread-based updates via message mapping

### 5.3 Slash Commands
- [x] `/nightowl search <query>` — KB search, top results
- [x] `/nightowl oncall [roster]` — current on-call display
- [x] `/nightowl ack <alert-id>` — acknowledge from Slack
- [x] `/nightowl resolve <alert-id> [notes]` — resolve with notes
- [x] `/nightowl roster [name]` — upcoming schedule

### 5.4 Interactive Flows
- [x] "Add to Knowledge Base" modal from resolved alerts
- [x] Modal submission → create incident via incidents service
- [x] Ephemeral responses for confirmations

### 5.5 Slack Message Tracking
- [x] Store channel_id + message_ts in slack_message_mappings table
- [x] Use for updating messages when status changes

---

## Phase 6: Frontend (Weeks 10–13)

### 6.1 Setup
- [x] Vite + React 19 + TypeScript scaffold
- [x] Tailwind CSS 4 + shadcn/ui component library
- [x] TanStack Router with 18 routes
- [x] TanStack Query client with `/api/v1` base URL
- [x] Auto-use dev API key in development mode
- [x] Dark mode default with theme toggle (localStorage persistence)

### 6.2 Dashboard
- [x] Active alerts by severity (cards + bar chart)
- [x] On-call widget: per-roster on-call display with display names (primary + secondary)
- [x] Recent activity feed (audit log entries)
- [x] Filter inactive rosters from on-call widget

### 6.3 Alert Views
- [x] Alert list: filterable table (status, severity) with CSV export
- [x] Alert detail: labels, annotations, linked incident, suggested solution
- [x] Acknowledge/resolve actions
- [x] Keyboard shortcuts: K=acknowledge, R=resolve

### 6.4 Knowledge Base Views
- [x] Search page with full-text search, highlighting
- [x] Incident list: filterable by severity, category, with CSV export
- [x] Incident detail: all fields, resolution history
- [x] Incident create/edit form
- [x] Merge UI: merge incidents via dialog

### 6.5 Roster Views
- [x] Roster list: table with primary/secondary on-call, active/ended status badges
- [x] Active rosters sorted first, inactive rows dimmed
- [x] Roster detail: configuration, members with display names, overrides
- [x] User search dropdown for adding members and overrides
- [x] On-call history section (last 10 shifts)
- [x] Create form with optional end_date
- [x] iCal export link

### 6.6 Escalation Policy Views
- [x] Policy list with tier summary
- [x] Policy editor: add/remove/reorder tiers, configure timeouts
- [x] Dry-run test button with results display

### 6.7 Admin Views
- [x] Admin hub with navigation cards
- [x] User management: CRUD with role assignment
- [x] API key management: create (copy raw key), list (masked), revoke
- [x] Tenant configuration: Slack workspace, Twilio SID, default timezone
- [x] Audit log viewer: filterable activity list

### 6.8 System Pages (added post-spec)
- [x] Status page: DB/Redis health, latency, uptime, version, last alert time
- [x] About page: build info, version, commit SHA
- [x] 404 Not Found page
- [x] Branded loading spinner (owl logo animation)
- [x] Branded empty states across all pages

---

## Phase 7: Deployment & Operations (Weeks 13–14)

### 7.1 Helm Chart
- [x] Chart structure: templates for API, worker, web deployments
- [x] values.yaml with sensible defaults
- [x] Configurable: replicas, resources, ingress, TLS, external DB/Redis
- [x] ServiceMonitor for Prometheus Operator

### 7.2 Documentation
- [x] README with quickstart
- [x] OpenAPI/Swagger spec with embedded UI at `/api/docs`
- [ ] Helm chart values documentation
- [ ] Slack app setup guide
- [ ] Twilio setup guide
- [ ] OIDC provider configuration guide

### 7.3 Testing
- [x] Unit tests for domain services (table-driven)
- [x] Integration test infrastructure (testcontainers pattern)
- [x] Webhook receiver tests: sample payload parsing
- [x] Escalation engine tests: tier progression
- [ ] E2E: alert → dedup → enrich → notify → acknowledge → resolve → KB prompt

### 7.4 Observability
- [x] Structured logging with `slog` (JSON/text configurable)
- [x] OpenTelemetry trace export (OTLP gRPC)
- [x] Grafana dashboard JSON in `deploy/grafana/`
- [ ] Alert rules: NightOwl self-monitoring (API errors, escalation failures)

### 7.5 Demo & Development (added post-spec)
- [x] `docker-compose.yml` for dev (PostgreSQL 16 + Redis 7)
- [x] `docker-compose.demo.yml` for one-command full-stack demo
- [x] `make seed` — idempotent dev tenant seeding
- [x] `make seed-demo` — destructive full demo data seeding
- [x] Frontend Dockerfile (multi-stage Node build + nginx serve)
- [x] GitHub release workflow
