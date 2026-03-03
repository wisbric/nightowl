# NightOwl Platform — Feature Specification

> Comprehensive feature inventory across NightOwl, BookOwl, and TicketOwl with implementation status.

This document extends the product requirements (`01-requirements.md`) into a full platform feature specification covering all capability domains. Each requirement is assessed against the current codebase.

**Status key:**
- **Done** — Fully implemented and functional
- **Partial** — Core logic exists but incomplete or missing edge cases
- **Not started** — No implementation exists

---

## 1. Core Alert Management

Webhook-based alert ingestion with deduplication, lifecycle management, and auto-enrichment.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| AM-01 | Alertmanager/Prometheus webhook receiver (compatible with SigNoz) | Must | Done |
| AM-02 | Keep alert format webhook receiver | Must | Done |
| AM-03 | Generic JSON webhook receiver with configurable field mapping | Should | Done |
| AM-04 | Alert deduplication by SHA-256 fingerprint (title + labels) with Redis cache (5min TTL) and DB fallback | Must | Done |
| AM-05 | Alert lifecycle: firing → acknowledged → resolved with user/timestamp tracking | Must | Done |
| AM-06 | Auto-enrichment: match incoming alert against KB by fingerprint, full-text search, and BookOwl runbook search | Must | Done |
| AM-07 | Agent-created alerts with metadata (agent_id, action_taken, confidence, auto_resolved flag) | Must | Done |
| AM-08 | Auto-resolve from Alertmanager status=resolved updates (fingerprint match) | Must | Done |
| AM-09 | Agent auto-resolve: create alert in resolved state, auto-generate KB entry from agent action | Must | Done |
| AM-10 | Alert → incident promotion: escalate grouped alerts to a formal incident | Must | Not started |
| AM-11 | Silence/inhibit rules: suppress known-noisy alerts during maintenance windows | Should | Not started |
| AM-12 | Severity mapping: critical/major/warning/info (handles P1–P5 notation) | Must | Done |
| AM-13 | Payload size limit (1 MiB) with structured error responses | Must | Done |
| AM-14 | Prometheus metrics: `nightowl_alerts_received_total{source,severity}`, `nightowl_alerts_deduplicated_total`, `nightowl_alerts_agent_resolved_total` | Should | Done |

**Key files:** `pkg/alert/webhook.go`, `pkg/alert/dedup.go`, `pkg/alert/enrich.go`, `pkg/alert/handler.go`

---

## 2. Noise Reduction & Correlation

Mechanisms for reducing alert fatigue through deduplication, grouping, and correlation.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| NR-01 | Fingerprint-based deduplication: SHA-256(title + labels), increment occurrence_count on duplicate | Must | Done |
| NR-02 | Redis hot-path dedup cache with 5min TTL, PostgreSQL cold-path fallback | Must | Done |
| NR-03 | Configurable alert grouping rules (group by labels like service, cluster, namespace) | Must | Not started |
| NR-04 | Time-based alert correlation (e.g., 3+ alerts within 5min → merge into incident) | Should | Not started |
| NR-05 | Maintenance window scheduling: suppress alerts for planned downtime periods | Should | Not started |
| NR-06 | AI-powered semantic correlation: similar incident matching beyond keyword search | Could | Not started |
| NR-07 | Group-level actions: acknowledge/resolve all alerts in a group at once | Should | Not started |

**Key files:** `pkg/alert/dedup.go`, `pkg/alert/webhook.go`

---

## 3. Workflow Automation

Event-driven automation for alert processing, routing, and response.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| WA-01 | Rules engine: configurable if-this-then-that rules for alert processing | Should | Not started |
| WA-02 | Auto-acknowledge rules: suppress acknowledgment prompts for known low-severity patterns | Should | Not started |
| WA-03 | Auto-resolve from upstream source (Alertmanager resolved status) | Must | Done |
| WA-04 | Outbound webhooks: fire HTTP callbacks on alert events (created, ack'd, resolved, escalated) | Should | Not started |
| WA-05 | Slack interactive actions: acknowledge, escalate, resolve, create KB entry from Slack messages | Must | Done |
| WA-06 | Escalation-based automation: timeout-driven tier progression with notification dispatch | Must | Done |
| WA-07 | Redis pub/sub internal event bus (`nightowl:alert:ack`, `nightowl:alert:escalated`) | Must | Done |
| WA-08 | Agent auto-remediation reporting: agents POST results, NightOwl records outcomes and creates KB entries | Must | Done |
| WA-09 | Conditional alert routing: route alerts to specific teams/channels based on labels or severity | Should | Not started |

**Key files:** `pkg/escalation/engine.go`, `pkg/alert/webhook.go`, `pkg/slack/handler.go`

---

## 4. Incident Management

Knowledge base for capturing incident data, solutions, and resolution patterns.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| IM-01 | Incident CRUD: title, error patterns, affected services, symptoms, root cause, solution, tags | Must | Done |
| IM-02 | Full-text search with PostgreSQL tsvector: weighted ranking (title > symptoms > solution) and ts_headline highlighting | Must | Done |
| IM-03 | Fingerprint-based exact matching for automated lookups | Must | Done |
| IM-04 | Incident merge: combine fingerprints, services, tags (union); select best severity, longest text fields; reassign linked alerts | Must | Done |
| IM-05 | Resolution tracking: resolution_count, last_resolved_at/by, avg_resolution_mins | Must | Done |
| IM-06 | Severity classification: info, warning, major, critical with validation | Must | Done |
| IM-07 | Service catalog: services/clusters/namespaces arrays, filterable | Should | Done |
| IM-08 | Version history: incident_history table with JSON diffs, change_type (created/updated/merged/archived) | Should | Done |
| IM-09 | Post-mortem URL support with dedicated setter endpoint and history tracking | Should | Done |
| IM-10 | Soft-delete via merged_into_id self-reference (preserves audit trail) | Must | Done |
| IM-11 | Pre-seeded runbook templates for common Kubernetes failure modes | Should | Done |
| IM-12 | Incident statistics dashboard: severity breakdown, recent alerts, activity feed | Should | Partial |
| IM-13 | MTTR trending and repeat-incident analytics | Should | Not started |

**Key files:** `pkg/incident/handler.go`, `pkg/incident/service.go`, `pkg/incident/store.go`

---

## 5. Roster & On-Call Scheduling

Explicit weekly schedule management with cross-timezone support and override shifts.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| RS-01 | Roster CRUD: name, description, timezone (IANA), handoff_time, handoff_day, active status | Must | Done |
| RS-02 | Member management: add/deactivate members, display names, primary/secondary weeks_served counters | Must | Done |
| RS-03 | Explicit weekly schedule (v2): primary + secondary per week, locked/unlocked weeks, auto-regeneration | Must | Done |
| RS-04 | Fair rotation algorithm: least-served-first with max_consecutive_weeks constraint | Must | Done |
| RS-05 | Cross-timezone support: IANA timezone per roster with proper time conversion | Must | Done |
| RS-06 | Follow-the-sun: is_follow_the_sun flag, linked_roster_id, active_hours_start/end windows | Must | Done |
| RS-07 | Override shifts: start_at/end_at with reason, created_by tracking; overrides take priority in on-call resolution | Must | Done |
| RS-08 | On-call resolution: active override → scheduled assignment → unassigned (with historical query via `?at=`) | Must | Done |
| RS-09 | Coverage heatmap: time-slot coverage across all rosters with configurable resolution and gap detection | Should | Done |
| RS-10 | iCal export: RFC 5545 calendar with schedule entries and overrides, timezone-aware | Should | Done |
| RS-11 | Handoff notifications: worker detects incoming/outgoing transitions at handoff_time | Should | Partial |
| RS-12 | Holiday/leave management: explicit unavailability calendar with auto-roster adjustment | Should | Not started |
| RS-13 | On-call analytics: hours per person, escalation frequency, response times | Should | Not started |
| RS-14 | Handoff report: auto-generated summary of open incidents at shift change | Should | Not started |

**Key files:** `pkg/roster/handler.go`, `pkg/roster/service.go`, `pkg/roster/scheduler.go`, `pkg/roster/handoff.go`, `pkg/roster/ical.go`

---

## 6. Escalation Engine

Timer-based escalation through configurable policy tiers with multi-channel notification.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| ES-01 | Escalation policy CRUD: name, description, ordered tiers (JSONB), optional repeat_count | Must | Done |
| ES-02 | Tiered escalation: each tier defines timeout_minutes, notify_via methods, target roles/users | Must | Done |
| ES-03 | Escalation targets: oncall_primary, oncall_backup, team_lead, user:\<uuid\> | Must | Done |
| ES-04 | Notification channels per tier: slack_dm, slack_channel, phone, sms | Must | Done |
| ES-05 | Background engine: 30-second polling worker evaluates all firing unacknowledged alerts per tenant | Must | Done |
| ES-06 | Cumulative timeout evaluation: escalate when elapsed time ≥ sum of tier timeouts | Must | Done |
| ES-07 | De-escalation on acknowledgment: Redis pub/sub listener, next poll skips acknowledged alerts | Must | Done |
| ES-08 | Repeat escalation: restart from tier 1 when max tier reached and repeat_count > 0 | Should | Done |
| ES-09 | Escalation event audit trail: tier, action, target_user, notify_method, notify_result | Must | Done |
| ES-10 | Dry-run simulation: preview full escalation path with cumulative timeout display | Should | Done |
| ES-11 | Phone callout via Twilio: REST API for voice calls, TwiML for text-to-speech, digit gathering for ack | Must | Done |
| ES-12 | SMS via Twilio: inbound/outbound SMS webhook handling | Must | Done |
| ES-13 | Prometheus metric: `nightowl_alerts_escalated_total{tier}` | Should | Done |

**Key files:** `pkg/escalation/handler.go`, `pkg/escalation/engine.go`, `pkg/escalation/store.go`

---

## 7. Runbook Integration (BookOwl)

Integration between NightOwl alert/incident pipeline and BookOwl's collaborative knowledge base.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| RB-01 | BookOwl runbook search from NightOwl alert enrichment pipeline | Must | Done |
| RB-02 | Runbook URL attachment on alerts (suggested_solution + runbook_url fields) | Must | Done |
| RB-03 | Post-mortem creation: NightOwl incident data → BookOwl document with template substitution | Should | Partial |
| RB-04 | Integration spec: full REST contract defined in `bookowl/docs/05-nightowl-integration.md` | Must | Done |
| RB-05 | Slack "View Runbook" button on alert notifications linking to BookOwl | Should | Done |
| RB-06 | BookOwl live context blocks: embed NightOwl on-call data, service status, active alerts in runbooks | Could | Not started |
| RB-07 | Runbook migration command: bulk import existing runbooks into BookOwl | Could | Not started |

**Key files:** `pkg/alert/enrich.go`, `bookowl/docs/05-nightowl-integration.md`, `bookowl/internal/integration/`

---

## 8. Ticketing Integration (TicketOwl)

TicketOwl bridges NightOwl incidents to Zammad ticketing with SLA tracking and bi-directional sync.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| TI-01 | Zammad REST client: typed API wrapper with retry logic, OTel tracing, HMAC webhook validation | Must | Done |
| TI-02 | Auto-ticket creation: NightOwl incident webhooks trigger Zammad ticket creation via configurable rules | Must | Done |
| TI-03 | SLA engine: response/resolution targets per priority, tiered escalation, breach detection | Must | Done |
| TI-04 | SLA breach alerts: fire alerts to NightOwl when SLA thresholds are exceeded | Must | Done |
| TI-05 | Bi-directional sync: NightOwl → TicketOwl (incident webhooks), Zammad → TicketOwl (ticket webhooks) | Must | Done |
| TI-06 | Ticket ↔ incident linking: incident_links table mapping tickets to NightOwl incidents | Must | Done |
| TI-07 | State/priority caching: Redis-backed Zammad state/priority cache with 1h TTL | Should | Done |
| TI-08 | BookOwl article suggestions: search BookOwl for relevant articles when viewing a ticket | Should | Not started |
| TI-09 | SLA compliance dashboard: visualize SLA breaches and compliance rates over time | Should | Not started |

**Key files:** `ticketowl/internal/zammad/`, `ticketowl/internal/sla/`, `ticketowl/internal/nightowl/`, `ticketowl/internal/bookowl/`

---

## 9. Dashboards & Observability

Operational dashboards, metrics, and monitoring capabilities.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| DO-01 | NightOwl dashboard: active alert count, severity breakdown chart, recent alerts, activity feed, on-call display | Must | Done |
| DO-02 | Grafana dashboard JSON: pre-built dashboard for Prometheus metrics | Should | Done |
| DO-03 | Prometheus metrics endpoint (`/metrics`): HTTP latency histograms, alert/dedup/escalation counters | Must | Done |
| DO-04 | Structured JSON logging via slog across all services | Must | Done |
| DO-05 | OpenTelemetry tracing: OTLP gRPC export for distributed request tracing | Should | Done |
| DO-06 | System status page: DB/Redis health, latency, uptime, version, last alert time | Should | Done |
| DO-07 | MTTR trending: mean time to resolution over configurable time windows | Should | Not started |
| DO-08 | Alert volume trending: time-series charts of alert frequency by source/severity | Should | Not started |
| DO-09 | On-call analytics dashboard: hours per person, escalation frequency, response times | Should | Not started |
| DO-10 | Repeat-incident analytics: identify frequently recurring incidents and top offenders | Should | Not started |

**Key files:** `web/src/pages/dashboard.tsx`, `deploy/grafana/nightowl-dashboard.json`, `core/pkg/telemetry/`

---

## 10. Provider & Integration Framework

Pluggable provider architecture for messaging, notifications, and external integrations.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| PF-01 | Generic messaging provider interface: PostAlert, UpdateAlert, PostEscalation, PostHandoff, SendDM, LookupUser | Must | Done |
| PF-02 | Provider registry: manage multiple messaging providers per tenant | Must | Done |
| PF-03 | Slack provider: full implementation of messaging interface with Block Kit, interactive actions, slash commands | Must | Done |
| PF-04 | Mattermost provider: full implementation of messaging interface (parallel to Slack) | Should | Done |
| PF-05 | Slack slash commands: `/nightowl search`, `/nightowl oncall`, `/nightowl ack`, `/nightowl resolve`, `/nightowl roster` | Must | Done |
| PF-06 | Slack message tracking: `slack_message_mappings` table for alert → channel/message updates | Must | Done |
| PF-07 | Email notification provider | Should | Not started |
| PF-08 | SMS notification provider (generic, beyond Twilio escalation) | Should | Not started |
| PF-09 | Custom webhook provider: send notifications to arbitrary HTTP endpoints | Could | Not started |
| PF-10 | OpenAPI/Swagger documentation with embedded UI at `/api/docs` | Should | Done |

**Key files:** `pkg/messaging/`, `pkg/slack/`, `pkg/mattermost/`

---

## 11. Platform Foundations

Multi-tenancy, authentication, authorization, deployment, and operational infrastructure.

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| PL-01 | Schema-per-tenant isolation: middleware resolves tenant, acquires connection, sets search_path | Must | Done |
| PL-02 | Tenant-specific configuration: Slack workspace, Twilio SID, default timezone, escalation defaults | Must | Done |
| PL-03 | OIDC authentication: Keycloak/Dex/Auth0 via coreos/go-oidc/v3 | Must | Done |
| PL-04 | Cookie sessions: `wisbric_session` (HttpOnly, Secure, SameSite=Strict) shared across all services | Must | Done |
| PL-05 | Silent token refresh: auto-refresh when cookie has <2h remaining | Must | Done |
| PL-06 | Local admin break-glass login: `POST /auth/local` with forced password change on first login | Must | Done |
| PL-07 | Auth middleware precedence: Cookie → PAT → Session JWT → OIDC JWT → API Key → Dev header | Must | Done |
| PL-08 | RBAC: Admin (40), Manager (30), Engineer (20), ReadOnly (10) with hierarchical role checks | Must | Done |
| PL-09 | OIDC group-to-role mapping per service | Must | Done |
| PL-10 | API key management: SHA-256 hashed storage, create (returns raw key once), list (masked), revoke | Must | Done |
| PL-11 | Audit logging: async buffered writer, captures user/action/resource/IP/user-agent, queryable via API | Must | Done |
| PL-12 | Helm umbrella chart: NightOwl + BookOwl + TicketOwl + PostgreSQL + Redis + Keycloak + Zammad + MinIO | Must | Done |
| PL-13 | ArgoCD integration: Application manifests for GitOps deployment | Should | Done |
| PL-14 | Environment-specific Helm values: dev, lab, production | Should | Done |
| PL-15 | Self-hosted deployment: no external SaaS dependencies for core functionality | Must | Done |
| PL-16 | Data retention policies: configurable per-tenant retention windows | Should | Not started |
| PL-17 | User management: CRUD endpoints with role assignment per tenant | Must | Done |
| PL-18 | Demo seed command: one-command full-stack demo with sample data | Should | Done |

**Key files:** `core/pkg/auth/`, `core/pkg/tenant/`, `core/pkg/config/`, `umbrella-owl/`

---

## Coverage Summary

| Domain | Items | Done | Partial | Not Started | Coverage |
|--------|------:|-----:|--------:|------------:|---------:|
| Core Alert Management | 14 | 11 | 0 | 3 | 79% |
| Noise Reduction & Correlation | 7 | 2 | 0 | 5 | 29% |
| Workflow Automation | 9 | 5 | 0 | 4 | 56% |
| Incident Management | 13 | 11 | 1 | 1 | 88% |
| Roster & On-Call | 14 | 10 | 1 | 3 | 75% |
| Escalation Engine | 13 | 13 | 0 | 0 | 100% |
| Runbook Integration | 7 | 4 | 1 | 2 | 64% |
| Ticketing Integration | 9 | 7 | 0 | 2 | 78% |
| Dashboards & Observability | 10 | 6 | 0 | 4 | 60% |
| Provider Framework | 10 | 7 | 0 | 3 | 70% |
| Platform Foundations | 18 | 17 | 0 | 1 | 94% |
| **Total** | **124** | **93** | **3** | **28** | **76%** |

### Coverage calculation

Coverage = (Done + 0.5 × Partial) / Total items, rounded to nearest percent.

### Key gaps by priority

**Must-have gaps (blocking production readiness):**
- AM-10: Alert → incident promotion
- NR-03: Configurable alert grouping rules

**Should-have gaps (high value, next priorities):**
- AM-11: Silence/inhibit rules
- NR-04: Time-based alert correlation
- NR-05: Maintenance windows
- WA-01: Rules engine
- WA-04: Outbound event webhooks
- RS-12: Holiday/leave management
- DO-07: MTTR trending
- PF-07: Email notification provider

**Could-have gaps (future roadmap):**
- NR-06: AI-powered semantic correlation
- RB-06: BookOwl live context blocks
- PF-09: Custom webhook provider
