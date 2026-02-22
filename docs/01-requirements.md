# NightOwl — Product Requirements

> Incident Knowledge Base, Alert Management, On-Call Roster & Escalation Platform for 24/7 Operations

## 1. Problem Statement

Managed service providers operating 24/7 across multiple time zones (e.g., NZ ↔ Germany, 11–13 hour offset) lack a unified, self-hosted platform that combines:

- Incident knowledge capture and retrieval (what broke, how it was fixed)
- Intelligent alert ingestion, deduplication, and enrichment
- On-call roster management with cross-timezone handoffs
- Escalation and callout for major incidents
- Integration with existing monitoring stacks (SigNoz, Keep, Prometheus, Alertmanager)

Current tooling is fragmented: PagerDuty/Opsgenie are SaaS (data sovereignty issues for KRITIS/public sector), Grafana OnCall is in maintenance mode, and most knowledge bases are disconnected from the alert pipeline.

## 2. Target Users

| Role | Description |
|------|-------------|
| **On-call engineer** | Receives alerts, investigates, resolves incidents, contributes solutions |
| **Team lead / SRE manager** | Manages rosters, reviews incident trends, defines escalation policies |
| **AI agent** | Automated systems that create alerts, attempt resolution, and log outcomes |
| **Platform admin** | Configures tenants, integrations, users, and RBAC |

## 3. Core Feature Requirements

### 3.1 Knowledge Base

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| KB-01 | Store incidents with: title, error patterns, affected services, symptoms, root cause, solution, runbook link, tags | Must | Done |
| KB-02 | Full-text search across all incident fields using PostgreSQL tsvector with weighted ranking and highlighting | Must | Done |
| KB-03 | Fingerprint-based exact matching for automated lookups (Keep/agent integration) | Must | Done |
| KB-04 | Merge duplicate/similar incidents — link related incidents, consolidate solutions | Must | Done |
| KB-05 | Resolution tracking: who resolved it (human or agent), when, what method | Must | Done |
| KB-06 | Runbook attachment: markdown runbooks stored inline with CRUD, linked to incidents | Must | Done |
| KB-07 | Pre-seeded runbook templates for common Kubernetes failure modes (pod crashloop, OOM, cert expiry, etcd issues, node not ready, PVC stuck, DNS resolution failure) | Should | Done |
| KB-08 | Version history on incident records — track edits to solutions over time via diff tracking | Should | Done |
| KB-09 | Severity classification: info, warning, critical, major incident | Must | Done |
| KB-10 | Service catalogue integration — map incidents to services/clusters/namespaces | Should | Done |
| KB-11 | Incident statistics and trend dashboards (MTTR, repeat incidents, top offenders) | Should | Partial (dashboard shows counts) |
| KB-12 | AI-assisted similar incident search (semantic matching beyond keyword) | Could | Not started |

### 3.2 Alert Ingestion & Management

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| AL-01 | Webhook receiver for Alertmanager/Prometheus format (compatible with SigNoz) | Must | Done |
| AL-02 | Webhook receiver for Keep alert format | Must | Done |
| AL-03 | Generic JSON webhook receiver with configurable field mapping | Should | Done |
| AL-04 | Alert deduplication by fingerprint with Redis cache (5min TTL) and DB fallback | Must | Done |
| AL-05 | Alert grouping — merge same-type alerts across clusters/namespaces | Must | Partial (dedup by fingerprint) |
| AL-06 | Auto-enrichment: match incoming alert against knowledge base, attach known solution | Must | Done |
| AL-07 | Alert lifecycle: firing → acknowledged → investigating → resolved | Must | Done |
| AL-08 | Agent-created alerts: automated systems can POST alerts with metadata about auto-remediation attempts | Must | Done |
| AL-09 | Alert → incident promotion: escalate grouped alerts to a formal incident | Must | Not started |
| AL-10 | Silence/inhibit rules: suppress known-noisy alerts during maintenance windows | Should | Not started |

### 3.3 Slack Integration

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| SL-01 | Post critical/major alerts to configurable Slack channels | Must | Done |
| SL-02 | Interactive Slack messages: acknowledge, escalate, resolve from Slack | Must | Done |
| SL-03 | Slack command: `/nightowl search <query>` to search knowledge base | Must | Done |
| SL-04 | Slack command: `/nightowl oncall` to show current on-call roster | Must | Done |
| SL-05 | Post incident resolution summaries to Slack with link to add/edit solution | Must | Done |
| SL-06 | Thread-based incident updates — all updates to an incident in one Slack thread | Should | Done (via message mappings) |
| SL-07 | Configurable per-tenant Slack workspaces | Must | Done (via tenant config) |

### 3.4 On-Call & Escalation

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| OC-01 | Roster management: define rotations with start/end times, timezone per roster | Must | Done |
| OC-02 | Cross-timezone roster support: handoff-aware scheduling (e.g., NZ morning → DE afternoon) | Must | Done |
| OC-03 | Follow-the-sun scheduling: auto-calculate handoff based on linked roster timezones | Must | Done |
| OC-04 | Override shifts: temporary swap without changing the rotation | Must | Done |
| OC-05 | Escalation policies: define tiers (L1 on-call → L2 backup → L3 manager) with timeout per tier | Must | Done |
| OC-06 | Callout for major incidents: trigger phone call/SMS via Twilio integration | Must | Done |
| OC-07 | Escalation timeout: if no acknowledgment within N minutes, escalate to next tier | Must | Done |
| OC-08 | Holiday/leave management: mark team members as unavailable, auto-adjust roster | Should | Not started |
| OC-09 | On-call handoff report: auto-generate summary of open incidents at shift change | Should | Not started |
| OC-10 | Roster calendar export (iCal/ICS format) | Should | Done |
| OC-11 | On-call analytics: hours per person, escalation frequency, response times | Should | Not started |
| OC-12 | Primary/secondary on-call display with display names | Must | Done |
| OC-13 | On-call history: last 10 completed rotation shifts | Should | Done |
| OC-14 | Roster active/ended status with optional end_date | Should | Done |
| OC-15 | User search dropdown for adding members and overrides | Should | Done |
| OC-16 | Escalation policy dry-run: simulate escalation path without triggering | Should | Done |

### 3.5 Multi-Tenancy & Data Sovereignty

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| MT-01 | Tenant isolation: each customer/team has isolated data (schema-per-tenant) | Must | Done |
| MT-02 | Tenant-specific configurations: Slack workspace, escalation policies, alert sources | Must | Done |
| MT-03 | Self-hosted deployment: all data stays on-premises or in customer-controlled infrastructure | Must | Done |
| MT-04 | No external SaaS dependencies for core functionality (phone/SMS callout is the exception) | Must | Done |
| MT-05 | RBAC: admin, manager, engineer, read-only roles per tenant | Must | Done |
| MT-06 | Audit log: all actions logged with user, timestamp, and tenant (async buffered writer) | Must | Done |
| MT-07 | Data retention policies: configurable per tenant | Should | Not started |

### 3.6 Admin & Operations (added post-spec)

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| AD-01 | User management: CRUD endpoints for tenant users with role assignment | Must | Done |
| AD-02 | API key management: create (returns raw key once), list (masked), revoke | Must | Done |
| AD-03 | Tenant configuration: Slack workspace URL, Twilio SID, default timezone settings | Must | Done |
| AD-04 | System status page: DB/Redis health, latency, uptime, version, last alert time | Should | Done |
| AD-05 | OpenAPI/Swagger documentation with embedded UI at `/api/docs` | Should | Done |
| AD-06 | Demo seed command: one-command full-stack demo with sample data | Should | Done |
| AD-07 | CSV export for alert and incident lists | Should | Done |
| AD-08 | Keyboard shortcuts on alert detail (K=acknowledge, R=resolve) | Should | Done |
| AD-09 | About page with build info (version, commit, uptime) | Should | Done |

## 4. Non-Functional Requirements

| Category | Requirement | Status |
|----------|-------------|--------|
| **Deployment** | Kubernetes-native (Helm chart), single namespace per instance | Done |
| **Database** | PostgreSQL 16+ (primary store), Redis for caching/pub-sub | Done |
| **Authentication** | OIDC/OAuth2 (Keycloak, Dex, or similar), API keys for integrations | Done |
| **TLS** | All endpoints TLS-terminated (cert-manager compatible) | Done (via ingress) |
| **Availability** | Stateless API servers, horizontally scalable behind ingress | Done |
| **Observability** | Prometheus metrics endpoint, structured JSON logging, OpenTelemetry traces | Done |
| **Performance** | Alert ingestion: < 500ms p99, KB search: < 1s p99, Slack notification: < 3s | Targets set |
| **Compliance** | GDPR-compatible data handling, audit trail for KRITIS requirements | Done (audit log) |
| **Backup** | PostgreSQL backup via standard tools (pg_dump, Velero, CNPG) | Done (via Helm) |

## 5. Out of Scope (v1)

- Native mobile app (web responsive is sufficient)
- Video/voice conferencing (use existing tools)
- Full ITSM ticketing (this is incident-focused, not a ServiceNow replacement)
- AI auto-remediation engine (agents can report their actions, but NightOwl doesn't execute remediations)
- ChatOps beyond Slack (Teams, Discord, etc. are future)

## 6. Success Metrics

- MTTR reduction: 30% within 3 months of adoption (measured by resolution timestamps)
- Knowledge base coverage: 80% of recurring alerts have a documented solution within 6 months
- On-call response: 95% of critical alerts acknowledged within 5 minutes
- Engineer satisfaction: reduced context-switching measured by fewer tools needed during incident response
