# OpsWatch — Product Requirements

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

| ID | Requirement | Priority |
|----|-------------|----------|
| KB-01 | Store incidents with: title, error patterns, affected services, symptoms, root cause, solution, runbook URL, tags | Must |
| KB-02 | Full-text search across all incident fields using PostgreSQL tsvector | Must |
| KB-03 | Fingerprint-based exact matching for automated lookups (Keep/agent integration) | Must |
| KB-04 | Merge duplicate/similar incidents — link related incidents, consolidate solutions | Must |
| KB-05 | Resolution tracking: who resolved it (human or agent), when, what method | Must |
| KB-06 | Runbook attachment: markdown runbooks stored inline or linked externally | Must |
| KB-07 | Pre-seeded runbook templates for common Kubernetes failure modes (pod crashloop, OOM, cert expiry, etcd issues, node not ready, PVC stuck, DNS resolution failure) | Should |
| KB-08 | Version history on incident records — track edits to solutions over time | Should |
| KB-09 | Severity classification: info, warning, critical, major incident | Must |
| KB-10 | Service catalogue integration — map incidents to services/clusters/namespaces | Should |
| KB-11 | Incident statistics and trend dashboards (MTTR, repeat incidents, top offenders) | Should |
| KB-12 | AI-assisted similar incident search (semantic matching beyond keyword) | Could |

### 3.2 Alert Ingestion & Management

| ID | Requirement | Priority |
|----|-------------|----------|
| AL-01 | Webhook receiver for Alertmanager/Prometheus format (compatible with SigNoz) | Must |
| AL-02 | Webhook receiver for Keep alert format | Must |
| AL-03 | Generic JSON webhook receiver with configurable field mapping | Should |
| AL-04 | Alert deduplication by fingerprint within configurable time window | Must |
| AL-05 | Alert grouping — merge same-type alerts across clusters/namespaces | Must |
| AL-06 | Auto-enrichment: match incoming alert against knowledge base, attach known solution | Must |
| AL-07 | Alert lifecycle: firing → acknowledged → investigating → resolved | Must |
| AL-08 | Agent-created alerts: automated systems can POST alerts with metadata about auto-remediation attempts | Must |
| AL-09 | Alert → incident promotion: escalate grouped alerts to a formal incident | Must |
| AL-10 | Silence/inhibit rules: suppress known-noisy alerts during maintenance windows | Should |

### 3.3 Slack Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| SL-01 | Post critical/major alerts to configurable Slack channels | Must |
| SL-02 | Interactive Slack messages: acknowledge, escalate, resolve from Slack | Must |
| SL-03 | Slack command: `/opswatch search <query>` to search knowledge base | Must |
| SL-04 | Slack command: `/opswatch oncall` to show current on-call roster | Must |
| SL-05 | Post incident resolution summaries to Slack with link to add/edit solution | Must |
| SL-06 | Thread-based incident updates — all updates to an incident in one Slack thread | Should |
| SL-07 | Configurable per-tenant Slack workspaces | Must |

### 3.4 On-Call & Escalation

| ID | Requirement | Priority |
|----|-------------|----------|
| OC-01 | Roster management: define rotations with start/end times, timezone per person | Must |
| OC-02 | Cross-timezone roster support: handoff-aware scheduling (e.g., NZ morning → DE afternoon) | Must |
| OC-03 | Follow-the-sun scheduling: auto-calculate handoff based on member timezones | Must |
| OC-04 | Override shifts: temporary swap without changing the rotation | Must |
| OC-05 | Escalation policies: define tiers (L1 on-call → L2 backup → L3 manager) with timeout per tier | Must |
| OC-06 | Callout for major incidents: trigger phone call/SMS via integration (Twilio/Vonage) | Must |
| OC-07 | Escalation timeout: if no acknowledgment within N minutes, escalate to next tier | Must |
| OC-08 | Holiday/leave management: mark team members as unavailable, auto-adjust roster | Should |
| OC-09 | On-call handoff report: auto-generate summary of open incidents at shift change | Should |
| OC-10 | Roster calendar export (iCal/ICS format) | Should |
| OC-11 | On-call analytics: hours per person, escalation frequency, response times | Should |

### 3.5 Multi-Tenancy & Data Sovereignty

| ID | Requirement | Priority |
|----|-------------|----------|
| MT-01 | Tenant isolation: each customer/team has isolated data (separate schema or row-level security) | Must |
| MT-02 | Tenant-specific configurations: Slack workspace, escalation policies, alert sources | Must |
| MT-03 | Self-hosted deployment: all data stays on-premises or in customer-controlled infrastructure | Must |
| MT-04 | No external SaaS dependencies for core functionality (phone/SMS callout is the exception) | Must |
| MT-05 | RBAC: admin, manager, engineer, read-only roles per tenant | Must |
| MT-06 | Audit log: all actions (create, update, delete, acknowledge, escalate) logged with user, timestamp, and tenant | Must |
| MT-07 | Data retention policies: configurable per tenant (e.g., 90 days alerts, 2 years incidents) | Should |

## 4. Non-Functional Requirements

| Category | Requirement |
|----------|-------------|
| **Deployment** | Kubernetes-native (Helm chart), single namespace per instance |
| **Database** | PostgreSQL 16+ (primary store), Redis for caching/pub-sub |
| **Authentication** | OIDC/OAuth2 (Keycloak, Dex, or similar), API keys for integrations |
| **TLS** | All endpoints TLS-terminated (cert-manager compatible) |
| **Availability** | Stateless API servers, horizontally scalable behind ingress |
| **Observability** | Prometheus metrics endpoint, structured JSON logging, OpenTelemetry traces |
| **Performance** | Alert ingestion: < 500ms p99, KB search: < 1s p99, Slack notification: < 3s |
| **Compliance** | GDPR-compatible data handling, audit trail for KRITIS requirements |
| **Backup** | PostgreSQL backup via standard tools (pg_dump, Velero, CNPG) |

## 5. Out of Scope (v1)

- Native mobile app (web responsive is sufficient)
- Video/voice conferencing (use existing tools)
- Full ITSM ticketing (this is incident-focused, not a ServiceNow replacement)
- AI auto-remediation engine (agents can report their actions, but OpsWatch doesn't execute remediations)
- ChatOps beyond Slack (Teams, Discord, etc. are future)

## 6. Success Metrics

- MTTR reduction: 30% within 3 months of adoption (measured by resolution timestamps)
- Knowledge base coverage: 80% of recurring alerts have a documented solution within 6 months
- On-call response: 95% of critical alerts acknowledged within 5 minutes
- Engineer satisfaction: reduced context-switching measured by fewer tools needed during incident response
