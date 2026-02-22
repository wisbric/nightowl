# NightOwl — Architecture Specification

## 1. System Overview

NightOwl is a monolithic Go API with a React frontend, deployed as a single Helm chart on Kubernetes. It avoids microservice complexity while maintaining clean internal domain boundaries.

```
┌──────────────────────────────────────────────────────────────────┐
│                        Ingress (TLS)                             │
│                     nightowl.example.com                         │
├──────────────────┬───────────────────────────────────────────────┤
│   Frontend SPA   │              API Server                       │
│   (React/Vite)   │           (Go / chi)                          │
│   nginx container├───────────────────────────────────────────────┤
│   or dev proxy   │  ┌─────────┐ ┌──────────┐ ┌──────────────┐  │
│                   │  │  Alert  │ │Knowledge │ │   Roster &   │  │
│                   │  │ Engine  │ │  Base    │ │  Escalation  │  │
│                   │  └────┬────┘ └─────┬────┘ └──────┬───────┘  │
│                   │       │            │             │           │
│                   │  ┌────┴────────────┴─────────────┴────┐     │
│                   │  │         PostgreSQL 16+              │     │
│                   │  │  (schema-per-tenant isolation)      │     │
│                   │  └────────────────────────────────────┘     │
│                   │       │                                      │
│                   │  ┌────┴────┐                                 │
│                   │  │  Redis  │  (pub/sub, alert dedup cache)  │
│                   │  └─────────┘                                 │
├───────────────────┴──────────────────────────────────────────────┤
│                    External Integrations                          │
│  ┌─────────┐  ┌──────────┐  ┌─────────┐  ┌────────────────┐    │
│  │  Slack  │  │  Twilio   │  │  Keep   │  │ SigNoz/Prom/   │    │
│  │  Bot    │  │  (phone)  │  │Webhooks │  │ Alertmanager   │    │
│  └─────────┘  └──────────┘  └─────────┘  └────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

## 2. Technology Decisions

### 2.1 Language: Go

| Factor | Decision |
|--------|----------|
| **Language** | Go 1.25+ (module: `github.com/wisbric/nightowl`) |
| **Rationale** | Single binary deployment, excellent Kubernetes ecosystem, low memory footprint, strong concurrency for webhook processing. Familiar in the CNCF ecosystem. |
| **Binary** | `cmd/nightowl` with `-mode` flag: `api`, `worker`, `seed`, `seed-demo` |

### 2.2 Framework & Libraries

| Component | Library | Notes |
|-----------|---------|-------|
| HTTP framework | `net/http` + `go-chi/chi/v5` | Lightweight, stdlib-compatible |
| Database | `jackc/pgx/v5` (driver) + `sqlc` (codegen) | Type-safe queries, no ORM overhead |
| Migrations | `golang-migrate/migrate/v4` | SQL-based, version controlled |
| Auth | `coreos/go-oidc/v3` + middleware | OIDC token validation + API key auth |
| Slack | `slack-go/slack` | Community SDK |
| Telephony | `twilio/twilio-go` | Phone/SMS callout |
| Redis | `redis/go-redis/v9` | Caching, pub/sub, dedup |
| Logging | `slog` (stdlib) | Structured JSON logging |
| Metrics | `prometheus/client_golang` | /metrics endpoint |
| Tracing | `go.opentelemetry.io/otel` | OTLP gRPC export |
| Config | `caarlos0/env/v11` | 12-factor env vars |
| UUIDs | `google/uuid` | UUID generation |

### 2.3 Frontend

| Component | Choice | Notes |
|-----------|--------|-------|
| Framework | React 19 + TypeScript 5.9 | Strict mode enabled |
| Build | Vite 7 | Dev server on port 3000, proxy to :8080 |
| UI Kit | shadcn/ui + Tailwind CSS 4 | Dark mode default, custom NightOwl theme |
| State | TanStack Query 5 | Server state management, caching |
| Routing | TanStack Router 1 | Type-safe routing, 18 routes |
| Forms | React Hook Form + Zod | Validation |
| Charts | Recharts 3 | Alert severity bar charts |
| Icons | lucide-react | Consistent icon set |
| Dates | date-fns | Date formatting |

### 2.4 Database: PostgreSQL 16+

Single database with schema-per-tenant isolation. The `public` schema holds global tables (tenants, API keys). Each tenant gets a `tenant_{slug}` schema with all domain tables.

### 2.5 Why Not Microservices

For a team of 1–5 engineers, a well-structured monolith with clean domain boundaries is faster to develop, deploy, debug, and operate. The internal domains (alerts, knowledge base, roster) communicate via function calls, not network hops. If a domain needs to scale independently later, it can be extracted.

## 3. Domain Architecture

### 3.1 Internal Domains

```
pkg/
├── alert/           # Alert ingestion, dedup, grouping, lifecycle
│   ├── handler.go   # HTTP handlers + Routes()
│   ├── service.go   # Business logic
│   ├── store.go     # Database queries
│   ├── alert.go     # Type definitions
│   ├── webhook.go   # Alertmanager/Keep/generic webhook handlers
│   ├── dedup.go     # Redis-backed deduplication with DB fallback
│   └── enrich.go    # KB enrichment (fingerprint + text match)
├── incident/        # Knowledge base, search, merge, history
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   └── incident.go
├── runbook/         # Runbooks CRUD + templates
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   ├── runbook.go
│   └── templates.go # Pre-seeded K8s runbook content
├── roster/          # On-call schedules, rotations, overrides, history
│   ├── handler.go
│   ├── service.go   # On-call calculation, primary/secondary, history
│   ├── store.go     # Raw SQL JOINs for display_name resolution
│   ├── roster.go    # Type definitions
│   └── ical.go      # iCal/ICS calendar export
├── escalation/      # Escalation policies, engine, dry-run
│   ├── handler.go
│   ├── store.go
│   ├── escalation.go
│   └── engine.go    # Background worker: 30s poll, tier progression
├── tenant/          # Multi-tenancy, schema provisioning
│   ├── tenant.go    # Info struct, context helpers
│   ├── middleware.go # search_path middleware
│   └── provisioner.go # Schema creation + migration
├── slack/           # Slack bot, slash commands, interactive messages
│   ├── handler.go   # Event/interaction/command handlers
│   ├── notifier.go  # Alert posting to channels
│   ├── verify.go    # Signing secret verification
│   ├── messages.go  # Block kit message builders
│   └── types.go
├── integration/     # External telephony
│   ├── callout.go   # CalloutService interface
│   └── twilio_handler.go # Twilio voice/SMS handlers
├── user/            # User CRUD
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   └── user.go
├── apikey/          # API key management
│   ├── handler.go
│   ├── service.go
│   ├── store.go
│   └── apikey.go
└── tenantconfig/    # Tenant settings CRUD
    ├── handler.go
    └── config.go

internal/
├── app/             # Application orchestrator (modes: api, worker, seed, seed-demo)
├── auth/            # OIDC + API key + RBAC middleware
├── audit/           # Async buffered audit log writer + list handler
├── config/          # Env-based config (caarlos0/env)
├── db/              # sqlc-generated models and queries
├── docs/            # OpenAPI/Swagger UI handler
├── httpserver/      # Chi server, middleware, response helpers, pagination
├── platform/        # PostgreSQL pool, Redis client, migration runner
├── seed/            # Dev seed + demo seed data
├── telemetry/       # Logger, metrics registration, tracing setup
└── version/         # Version + commit (set via ldflags)
```

### 3.2 Cross-Domain Communication

Domains are loosely coupled. Each domain package creates a `Service` from the tenant-scoped database connection on each request:

```go
func (h *Handler) service(r *http.Request) *Service {
    conn := tenant.ConnFromContext(r.Context())
    return NewService(conn, h.logger)
}
```

The alert enricher queries incidents directly via the store layer within the same request-scoped connection.

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
  ├─ Deduplicate (Redis: fingerprint → last_seen, 5min TTL)
  │   └─ If duplicate: update occurrence_count + last_fired_at, skip
  │
  ├─ Enrich from knowledge base (fingerprint match → attach solution)
  │
  ├─ Persist to PostgreSQL (alerts table)
  │
  ├─ Record Prometheus metrics (received, dedup, processing duration)
  │
  └─ Return 200 with alert ID
```

### 4.2 Escalation Flow

```
Alert fires (critical) → Escalation engine starts (worker mode, 30s poll)
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
        └─ If still unacknowledged → log event, repeat if policy allows
```

The escalation engine runs as a separate `--mode=worker` process. It polls for unacknowledged `status='firing'` alerts, steps through tiers based on elapsed time, creates `escalation_events` records, and listens for ack events via Redis pub/sub (`nightowl:alert:ack` channel).

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
        │       └─ If agent resolved: auto-create KB entry with agent notes
        │
        └─ Post resolution summary to Slack thread
```

## 5. Authentication & Authorization

### 5.1 Authentication Methods

| Method | Use Case |
|--------|----------|
| OIDC/OAuth2 (JWT) | Web UI users, SSO via Keycloak/Dex |
| API Key (header: `X-API-Key`) | Webhook senders, agent integrations, frontend dev mode |
| Slack signing secret | Slack bot event verification |
| Dev header (`X-Tenant-Slug`) | Development fallback (no real auth, grants admin) |

**Authentication precedence:** JWT → API Key → Dev header (dev only)

### 5.2 RBAC Model

```
Roles per tenant:
├── admin      — full access, manage users, configure integrations
├── manager    — manage rosters, escalation policies, view audit logs
├── engineer   — create/edit incidents, acknowledge alerts, manage own on-call
└── readonly   — view dashboards, search KB, view rosters
```

Role is stored per user in the `users` table and per API key in the `api_keys` table. Middleware extracts tenant + role from JWT claims or API key lookup.

## 6. Multi-Tenancy

### 6.1 Isolation Strategy: Schema-per-Tenant

Each tenant gets a separate PostgreSQL schema. The middleware sets `search_path` on each request.

```
Request → auth middleware → resolve tenant slug → acquire pooled connection
→ SET search_path TO tenant_{slug}, public → domain handler → release connection
```

**Why schema isolation over RLS:**
- Stronger data isolation (important for KRITIS compliance)
- Easier per-tenant backup/restore
- Cleaner data lifecycle management
- Slight operational overhead is acceptable at expected tenant count (< 50)

### 6.2 Shared Schema

The `public` schema holds only:
- `tenants` table (id, name, slug, config JSON)
- `api_keys` table (key_hash, tenant_id, role, scopes, description)

## 7. Deployment Architecture

### 7.1 Kubernetes Resources

```yaml
# Single Helm chart produces:
Deployment:  nightowl-api      (2+ replicas, stateless)
Deployment:  nightowl-worker   (1 replica, escalation engine)
Deployment:  nightowl-web      (nginx serving React SPA)
ConfigMap:   nightowl-config
Secret:      nightowl-secrets  (DB creds, Slack tokens, Twilio keys)
Ingress:     nightowl          (TLS via cert-manager)
Service:     nightowl-api
Service:     nightowl-web
ServiceMonitor: nightowl       (Prometheus scraping)
```

PostgreSQL and Redis are expected as external services (or provisioned via operators like CNPG).

### 7.2 Application Modes

The same Go binary runs in different modes via `-mode` flag or `NIGHTOWL_MODE` env var:

| Mode | Purpose |
|------|---------|
| `api` | HTTP server with all API endpoints |
| `worker` | Escalation engine (30s poll for unacknowledged alerts) |
| `seed` | Create dev tenant "acme" with sample users/services (idempotent) |
| `seed-demo` | Destructive: drop + recreate "acme" with full demo data |

### 7.3 Docker Images

| Image | Contents | Purpose |
|-------|----------|---------|
| `ghcr.io/wisbric/nightowl` | Go binary + migrations | API server, worker, seed |
| `ghcr.io/wisbric/nightowl-web` | Nginx + React build | Frontend SPA |

### 7.4 Development Setup

```bash
docker compose up -d          # PostgreSQL 16 + Redis 7
make seed                     # Create dev tenant with sample data
go run ./cmd/nightowl         # API on :8080
cd web && npm run dev         # Frontend on :3000 (proxies /api to :8080)
```

Dev API key: `ow_dev_seed_key_do_not_use_in_production` (auto-used by frontend in dev mode)

### 7.5 Resource Estimates

| Component | CPU Request | Memory Request | Storage |
|-----------|-------------|----------------|---------|
| API (per replica) | 100m | 128Mi | — |
| Worker | 100m | 128Mi | — |
| Web (per replica) | 50m | 64Mi | — |
| PostgreSQL | 250m | 512Mi | 10Gi+ |
| Redis | 50m | 64Mi | — |
| **Total (minimal)** | **650m** | **1Gi** | **10Gi** |

## 8. API Design

RESTful JSON API. All endpoints under `/api/v1/` require authentication (API key or JWT) except health checks and Slack webhooks.

### 8.1 Endpoints

```
# Health & System (unauthenticated)
GET    /healthz                                    # Liveness probe
GET    /readyz                                     # Readiness (DB + Redis ping)
GET    /metrics                                    # Prometheus metrics
GET    /api/docs                                   # Swagger UI
GET    /api/docs/openapi.yaml                      # OpenAPI spec

# System (authenticated)
GET    /api/v1/status                              # System health, latency, uptime
GET    /api/v1/ping                                # Debug: returns tenant/role info

# Alerts
GET    /api/v1/alerts                              # List (filters: status, severity, fingerprint)
GET    /api/v1/alerts/:id                          # Detail
PATCH  /api/v1/alerts/:id/acknowledge              # Acknowledge
PATCH  /api/v1/alerts/:id/resolve                  # Resolve

# Webhooks (API key auth)
POST   /api/v1/webhooks/alertmanager               # Alertmanager format
POST   /api/v1/webhooks/keep                       # Keep format
POST   /api/v1/webhooks/generic                    # Generic JSON

# Knowledge Base (Incidents)
POST   /api/v1/incidents                           # Create
GET    /api/v1/incidents                           # List (filters: severity, category, service, tags)
GET    /api/v1/incidents/:id                       # Detail
PUT    /api/v1/incidents/:id                       # Update
DELETE /api/v1/incidents/:id                       # Delete
GET    /api/v1/incidents/search                    # Full-text search with highlighting
GET    /api/v1/incidents/fingerprint/:fp           # Exact fingerprint lookup
POST   /api/v1/incidents/:id/merge                 # Merge incidents
GET    /api/v1/incidents/:id/history               # Change history

# Runbooks
POST   /api/v1/runbooks                           # Create
GET    /api/v1/runbooks                           # List
GET    /api/v1/runbooks/:id                       # Detail
PUT    /api/v1/runbooks/:id                       # Update
DELETE /api/v1/runbooks/:id                       # Delete
GET    /api/v1/runbooks/templates                 # List templates

# Rosters
POST   /api/v1/rosters                            # Create (with optional end_date)
GET    /api/v1/rosters                            # List (includes is_active status)
GET    /api/v1/rosters/:id                        # Detail
PUT    /api/v1/rosters/:id                        # Update
DELETE /api/v1/rosters/:id                        # Delete
GET    /api/v1/rosters/:id/oncall                 # Primary + secondary on-call
GET    /api/v1/rosters/:id/oncall?at=<RFC3339>    # On-call at specific time
GET    /api/v1/rosters/:id/oncall/history         # Last 10 completed shifts
GET    /api/v1/rosters/:id/members                # List members (with display_name)
POST   /api/v1/rosters/:id/members                # Add member
DELETE /api/v1/rosters/:id/members/:mid            # Remove member
GET    /api/v1/rosters/:id/overrides              # List overrides
POST   /api/v1/rosters/:id/overrides              # Create override
DELETE /api/v1/rosters/:id/overrides/:oid          # Delete override
GET    /api/v1/rosters/:id/export.ics             # iCal calendar export

# Escalation Policies
POST   /api/v1/escalation-policies                # Create
GET    /api/v1/escalation-policies                # List
GET    /api/v1/escalation-policies/:id            # Detail
PUT    /api/v1/escalation-policies/:id            # Update
DELETE /api/v1/escalation-policies/:id            # Delete
POST   /api/v1/escalation-policies/:id/dry-run    # Simulate escalation path
GET    /api/v1/escalation-policies/:id/events/:alertID  # Escalation events for alert

# Users
POST   /api/v1/users                              # Create
GET    /api/v1/users                              # List
GET    /api/v1/users/:id                          # Detail
PUT    /api/v1/users/:id                          # Update
DELETE /api/v1/users/:id                          # Deactivate

# API Keys
POST   /api/v1/api-keys                           # Create (returns raw key once)
GET    /api/v1/api-keys                           # List (masked)
DELETE /api/v1/api-keys/:id                       # Revoke

# Admin
GET    /api/v1/admin/config                       # Get tenant config
PUT    /api/v1/admin/config                       # Update tenant config

# Audit Log
GET    /api/v1/audit-log                          # List (filterable)

# Slack (verified by signing secret, not API key auth)
POST   /api/v1/slack/events                       # Event subscriptions
POST   /api/v1/slack/interactions                  # Interactive messages
POST   /api/v1/slack/commands                      # Slash commands

# Twilio
POST   /api/v1/twilio/voice                       # Voice call webhook
POST   /api/v1/twilio/sms                         # SMS webhook
```

## 9. Observability

### 9.1 Metrics (Prometheus)

Namespace: `nightowl`. All registered in `internal/telemetry/metrics.go`.

```
nightowl_api_request_duration_seconds{method, path, status}  # HTTP latency histogram
nightowl_alerts_received_total{source, severity}              # Webhook receipt counter
nightowl_alerts_deduplicated_total                            # Dedup counter
nightowl_alerts_agent_resolved_total                          # Agent resolution counter
nightowl_alert_processing_duration_seconds                    # Webhook processing latency
nightowl_kb_hits_total                                        # KB enrichment match counter
nightowl_alerts_escalated_total{tier}                         # Escalation tier counter
nightowl_slack_notifications_total{type}                      # Slack notification counter
```

### 9.2 Logging

Structured JSON via `slog` (configurable text format for development). Every log line includes `request_id`, contextual fields per domain.

### 9.3 Tracing

OpenTelemetry with OTLP gRPC exporter. Configured via `OTEL_EXPORTER_OTLP_ENDPOINT` env var. Disabled if endpoint not set.

### 9.4 Grafana

Pre-built Grafana dashboard JSON in `deploy/grafana/` for NightOwl metrics visualization.
