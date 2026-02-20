# OpsWatch — Architecture Specification

## 1. System Overview

OpsWatch is a monolithic API with a React frontend, deployed as a single Helm chart on Kubernetes. It avoids microservice complexity while maintaining clean internal domain boundaries.

```
┌──────────────────────────────────────────────────────────────────┐
│                        Ingress (TLS)                             │
│                     opswatch.example.com                         │
├──────────────────┬───────────────────────────────────────────────┤
│   Frontend SPA   │              API Server                       │
│   (React/Vite)   │         (Go or Python/FastAPI)                │
│   served by API  │                                               │
│   or separate    ├───────────────────────────────────────────────┤
│   static deploy  │  ┌─────────┐ ┌──────────┐ ┌──────────────┐  │
│                   │  │  Alert  │ │Knowledge │ │   Roster &   │  │
│                   │  │ Engine  │ │  Base    │ │  Escalation  │  │
│                   │  └────┬────┘ └─────┬────┘ └──────┬───────┘  │
│                   │       │            │             │           │
│                   │  ┌────┴────────────┴─────────────┴────┐     │
│                   │  │         PostgreSQL 16+              │     │
│                   │  │  (tenants, alerts, incidents,       │     │
│                   │  │   rosters, audit_log)               │     │
│                   │  └────────────────────────────────────┘     │
│                   │       │                                      │
│                   │  ┌────┴────┐                                 │
│                   │  │  Redis  │  (pub/sub, alert dedup cache,  │
│                   │  │         │   session cache, rate limiting) │
│                   │  └─────────┘                                 │
├───────────────────┴──────────────────────────────────────────────┤
│                    External Integrations                          │
│  ┌─────────┐  ┌──────────┐  ┌─────────┐  ┌────────────────┐    │
│  │  Slack  │  │  Twilio/  │  │  Keep   │  │ SigNoz/Prom/   │    │
│  │  Bot    │  │  Vonage   │  │Webhooks │  │ Alertmanager   │    │
│  └─────────┘  └──────────┘  └─────────┘  └────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

## 2. Technology Decisions

### 2.1 Language: Go

| Factor | Decision |
|--------|----------|
| **Language** | Go 1.23+ |
| **Rationale** | Single binary deployment, excellent Kubernetes ecosystem, low memory footprint, strong concurrency for webhook processing. Familiar in the CNCF ecosystem your customers already operate in. |
| **Alternative considered** | Python/FastAPI — faster prototyping but heavier runtime, less suited to high-throughput webhook processing |

### 2.2 Framework & Libraries

| Component | Library | Notes |
|-----------|---------|-------|
| HTTP framework | `net/http` + `chi` router | Lightweight, stdlib-compatible |
| Database | `pgx` (driver) + `sqlc` (codegen) | Type-safe queries, no ORM overhead |
| Migrations | `golang-migrate` | SQL-based, version controlled |
| Auth | `coreos/go-oidc` + middleware | OIDC token validation |
| Slack | `slack-go/slack` | Official community SDK |
| Telephony | `twilio/twilio-go` | Phone/SMS callout |
| Redis | `redis/go-redis/v9` | Caching, pub/sub |
| Logging | `slog` (stdlib) | Structured JSON logging |
| Metrics | `prometheus/client_golang` | /metrics endpoint |
| Tracing | `go.opentelemetry.io/otel` | OTLP export to SigNoz |
| Testing | `testing` + `testcontainers-go` | Integration tests with real PostgreSQL |
| Config | `caarlos0/env` + YAML | 12-factor env vars with optional config file |

### 2.3 Frontend

| Component | Choice | Notes |
|-----------|--------|-------|
| Framework | React 18 + TypeScript | Widely supported, good component ecosystem |
| Build | Vite | Fast builds, good DX |
| UI Kit | shadcn/ui + Tailwind | Professional look, accessible, customizable |
| State | TanStack Query | Server state management, caching |
| Routing | TanStack Router | Type-safe routing |
| Forms | React Hook Form + Zod | Validation |
| Tables | TanStack Table | Sortable, filterable data tables |
| Charts | Recharts | Incident trend dashboards |
| Calendar | react-big-calendar | Roster visualization |
| Markdown | @uiw/react-md-editor | Runbook editing |

### 2.4 Database: PostgreSQL 16+

Single database with schema-per-tenant isolation for simplicity. Row-level security (RLS) as an alternative for higher tenant density.

### 2.5 Why Not Microservices

For a team of 1–5 engineers, a well-structured monolith with clean domain boundaries is faster to develop, deploy, debug, and operate. The internal domains (alerts, knowledge base, roster) communicate via function calls, not network hops. If a domain needs to scale independently later, it can be extracted.

## 3. Domain Architecture

### 3.1 Internal Domains

```
pkg/
├── alert/          # Alert ingestion, dedup, grouping, lifecycle
│   ├── handler.go  # HTTP handlers (webhook receivers)
│   ├── service.go  # Business logic
│   ├── store.go    # Database queries (sqlc generated)
│   └── dedup.go    # Redis-backed deduplication
├── incident/       # Knowledge base, runbooks, resolution tracking
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   └── search.go   # Full-text search logic
├── roster/         # On-call schedules, rotations, overrides
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   └── timezone.go # Cross-timezone handoff calculations
├── escalation/     # Escalation policies, callout, notifications
│   ├── handler.go
│   ├── service.go
│   ├── engine.go   # Escalation timer and state machine
│   └── notify.go   # Slack, phone, SMS dispatch
├── tenant/         # Multi-tenancy, RBAC, audit
│   ├── handler.go
│   ├── service.go
│   ├── middleware.go # Tenant extraction from JWT/API key
│   └── audit.go     # Audit log writer
├── slack/          # Slack bot, slash commands, interactive messages
│   ├── handler.go   # Event/interaction handlers
│   ├── commands.go  # Slash command routing
│   └── messages.go  # Block kit message builders
└── integration/    # External webhook formatters
    ├── alertmanager.go
    ├── keep.go
    └── generic.go
```

### 3.2 Cross-Domain Communication

Domains call each other through service interfaces (Go interfaces), never directly accessing another domain's store. This allows testing with mocks and future extraction if needed.

```go
// escalation/service.go calls roster/service.go
type RosterQuerier interface {
    GetCurrentOnCall(ctx context.Context, tenantID, scheduleID uuid.UUID) ([]OnCallMember, error)
}

// alert/service.go calls incident/service.go
type KnowledgeBaseSearcher interface {
    SearchByFingerprint(ctx context.Context, tenantID uuid.UUID, fingerprint string) (*Incident, error)
    SearchByText(ctx context.Context, tenantID uuid.UUID, query string, limit int) ([]Incident, error)
}
```

## 4. Data Flow

### 4.1 Alert Ingestion Flow

```
Alertmanager/SigNoz/Keep
        │
        ▼
  POST /api/v1/webhooks/alertmanager   (or /keep, /generic)
        │
        ▼
  ┌─ Parse & normalize to internal alert format
  │
  ├─ Deduplicate (Redis: fingerprint → last_seen, 5min window)
  │   └─ If duplicate: update counter, skip processing
  │
  ├─ Enrich from knowledge base (fingerprint match → attach solution)
  │
  ├─ Classify severity (from labels or rules)
  │
  ├─ Persist to PostgreSQL (alerts table)
  │
  ├─ If critical/major:
  │   ├─ Post to Slack channel (with solution if known)
  │   ├─ Look up current on-call from roster
  │   └─ Start escalation timer
  │
  └─ Publish event to Redis pub/sub (for real-time UI updates)
```

### 4.2 Escalation Flow

```
Alert fires (critical) → Escalation engine starts
        │
        ▼
  Tier 1: Notify on-call engineer (Slack DM + push)
        │
        ├─ Acknowledged within timeout? → Stop escalation
        │
        ▼ (timeout, e.g., 5 min)
  Tier 2: Notify backup on-call (Slack DM + phone call via Twilio)
        │
        ├─ Acknowledged within timeout? → Stop escalation
        │
        ▼ (timeout, e.g., 10 min)
  Tier 3: Notify team lead/manager (phone call + Slack channel alert)
        │
        └─ If still unacknowledged → post to emergency channel
```

### 4.3 Incident Resolution & KB Update

```
Alert resolved (by human or agent)
        │
        ▼
  Resolution event received
        │
        ├─ Check: is this a known incident? (fingerprint match)
        │   ├─ Yes: link resolution to existing KB entry, update stats
        │   └─ No: prompt engineer to create KB entry
        │       ├─ Slack interactive message: "New issue resolved. Add to knowledge base?"
        │       └─ If agent resolved: auto-create KB entry with agent's resolution notes
        │
        └─ Post resolution summary to Slack thread
```

## 5. Authentication & Authorization

### 5.1 Authentication Methods

| Method | Use Case |
|--------|----------|
| OIDC/OAuth2 (JWT) | Web UI users, SSO via Keycloak/Dex |
| API Key (header: `X-API-Key`) | Webhook senders (Keep, Alertmanager), agent integrations |
| Slack signing secret | Slack bot event verification |

### 5.2 RBAC Model

```
Roles per tenant:
├── admin      — full access, manage users, configure integrations
├── manager    — manage rosters, escalation policies, view audit logs
├── engineer   — create/edit incidents, acknowledge alerts, manage own on-call
└── readonly   — view dashboards, search KB, view rosters
```

Permissions stored in `tenant_members` table with role enum. Middleware extracts tenant + role from JWT claims or API key lookup.

## 6. Multi-Tenancy

### 6.1 Isolation Strategy: Schema-per-Tenant

Each tenant gets a separate PostgreSQL schema. The API server sets `search_path` based on the authenticated tenant.

```sql
-- Tenant creation
CREATE SCHEMA tenant_adfinis;
SET search_path TO tenant_adfinis;
-- Run migrations within schema
```

**Why schema isolation over RLS:**
- Stronger data isolation (important for KRITIS compliance)
- Easier per-tenant backup/restore
- Cleaner data lifecycle management
- Slight operational overhead is acceptable at expected tenant count (< 50)

### 6.2 Shared Schema

The `public` schema holds only:
- `tenants` table (id, name, slug, config)
- `api_keys` table (key_hash, tenant_id, role, description)
- `global_config` (system-wide settings)

## 7. Deployment Architecture

### 7.1 Kubernetes Resources

```yaml
# Single Helm chart produces:
Deployment:  opswatch-api      (2+ replicas, stateless)
Deployment:  opswatch-worker   (1+ replicas, escalation timers, background jobs)
StatefulSet: opswatch-postgres (or use CNPG operator / external)
Deployment:  opswatch-redis    (or use external)
CronJob:     opswatch-cleanup  (data retention enforcement)
ConfigMap:   opswatch-config
Secret:      opswatch-secrets  (DB creds, Slack tokens, Twilio keys, API keys)
Ingress:     opswatch          (TLS via cert-manager)
Service:     opswatch-api
Service:     opswatch-postgres
Service:     opswatch-redis
ServiceMonitor: opswatch       (Prometheus scraping)
```

### 7.2 Worker Process

Separate deployment running the same binary with `--mode=worker` flag. Handles:
- Escalation timer ticks (check every 30s for unacknowledged alerts past timeout)
- Slack event processing (async)
- Data retention cleanup
- Roster handoff notifications

Uses Redis pub/sub to receive events from the API server.

### 7.3 Resource Estimates

| Component | CPU Request | Memory Request | Storage |
|-----------|-------------|----------------|---------|
| API (per replica) | 100m | 128Mi | — |
| Worker | 100m | 128Mi | — |
| PostgreSQL | 250m | 512Mi | 10Gi+ |
| Redis | 50m | 64Mi | — |
| **Total (minimal)** | **600m** | **960Mi** | **10Gi** |

## 8. API Design

RESTful JSON API. All endpoints under `/api/v1/`. Authentication required on all endpoints except webhook receivers (which use API key auth).

### 8.1 Key Endpoints

```
# Alerts
POST   /api/v1/webhooks/alertmanager     # Alertmanager webhook receiver
POST   /api/v1/webhooks/keep             # Keep webhook receiver
POST   /api/v1/webhooks/generic          # Generic JSON webhook
GET    /api/v1/alerts                    # List alerts (filterable)
GET    /api/v1/alerts/:id                # Get alert detail
PATCH  /api/v1/alerts/:id/acknowledge    # Acknowledge alert
PATCH  /api/v1/alerts/:id/resolve        # Resolve alert

# Knowledge Base (Incidents)
GET    /api/v1/incidents                 # List/search incidents
POST   /api/v1/incidents                 # Create incident record
GET    /api/v1/incidents/:id             # Get incident detail
PUT    /api/v1/incidents/:id             # Update incident
GET    /api/v1/incidents/search          # Full-text search
GET    /api/v1/incidents/fingerprint/:fp # Exact fingerprint lookup
POST   /api/v1/incidents/:id/merge       # Merge duplicate incidents
GET    /api/v1/incidents/:id/history     # Version history

# Runbooks
GET    /api/v1/runbooks                  # List runbooks
POST   /api/v1/runbooks                  # Create runbook
GET    /api/v1/runbooks/:id              # Get runbook (markdown)
PUT    /api/v1/runbooks/:id              # Update runbook
GET    /api/v1/runbooks/templates        # List pre-seeded templates

# Rosters
GET    /api/v1/rosters                   # List schedules
POST   /api/v1/rosters                   # Create schedule
GET    /api/v1/rosters/:id               # Get schedule detail
PUT    /api/v1/rosters/:id               # Update schedule
GET    /api/v1/rosters/:id/oncall        # Who is on-call now?
GET    /api/v1/rosters/:id/oncall?at=<ISO8601>  # Who is on-call at time X?
POST   /api/v1/rosters/:id/overrides     # Add override shift
DELETE /api/v1/rosters/:id/overrides/:oid # Remove override
GET    /api/v1/rosters/:id/export.ics    # iCal export

# Escalation Policies
GET    /api/v1/escalation-policies
POST   /api/v1/escalation-policies
PUT    /api/v1/escalation-policies/:id
GET    /api/v1/escalation-policies/:id/test  # Dry-run escalation

# Slack
POST   /api/v1/slack/events              # Slack event subscription
POST   /api/v1/slack/interactions         # Slack interactive messages
POST   /api/v1/slack/commands             # Slash commands

# Admin
GET    /api/v1/admin/tenants
POST   /api/v1/admin/tenants
GET    /api/v1/admin/audit-log
GET    /api/v1/admin/stats                # System-wide statistics

# Health
GET    /healthz                           # Liveness
GET    /readyz                            # Readiness (DB + Redis check)
GET    /metrics                           # Prometheus metrics
```

## 9. Observability

### 9.1 Metrics (Prometheus)

```
opswatch_alerts_received_total{source, severity, tenant}
opswatch_alerts_deduplicated_total{tenant}
opswatch_alerts_acknowledged_total{tenant}
opswatch_alerts_escalated_total{tenant, tier}
opswatch_alert_processing_duration_seconds{source}
opswatch_kb_searches_total{tenant, type}  # fingerprint vs text
opswatch_kb_search_duration_seconds{}
opswatch_kb_hits_total{tenant}            # search returned results
opswatch_escalation_response_time_seconds{tenant, tier}
opswatch_slack_notifications_total{tenant, type}
opswatch_api_request_duration_seconds{method, path, status}
```

### 9.2 Logging

Structured JSON via `slog`. Every log line includes: `tenant_id`, `request_id`, `user_id` (if authenticated), `domain` (alert/incident/roster/escalation).

### 9.3 Tracing

OpenTelemetry spans for:
- Webhook ingestion (parse → dedup → enrich → persist → notify)
- Escalation engine ticks
- Slack interactions
- KB searches

Export via OTLP to SigNoz.
