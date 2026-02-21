# NightOwl

**The wise one that watches while you sleep.**

NightOwl is an incident knowledge base, alert management, on-call roster, and escalation platform built for 24/7 operations teams running Kubernetes infrastructure across multiple time zones. It is a [Wisbric](https://wisbric.com) product.

---

## Features

- **Alert management** with deduplication and knowledge base enrichment
- **Incident knowledge base** with full-text search
- **On-call roster management** with timezone support
- **Timer-based escalation engine** with phone/SMS callout
- **Slack integration** (slash commands, interactive messages, notifications)
- **Multi-tenant schema isolation** (schema-per-tenant)
- **OIDC + API key authentication** with role-based access control
- **Prometheus metrics + OpenTelemetry tracing**

## Tech Stack

Go 1.23+ | chi router | PostgreSQL 16+ (pgx + sqlc) | Redis (go-redis/v9) | React 18 + TypeScript + Vite + Tailwind + shadcn/ui

---

## Quick Start

```bash
# Prerequisites: Go 1.23+, Docker, Node.js 20+

# Start infrastructure
docker compose up -d

# Run migrations and seed data
go run ./cmd/nightowl -mode seed

# Start the API server
go run ./cmd/nightowl

# Start the frontend dev server (separate terminal)
cd web && npm install && npm run dev

# API is at http://localhost:8080, Frontend at http://localhost:3000
# Dev API key: ow_dev_seed_key_do_not_use_in_production
```

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/webhooks/alertmanager` | Alertmanager webhook receiver |
| `POST` | `/api/v1/webhooks/keep` | Keep webhook receiver |
| `POST` | `/api/v1/webhooks/generic` | Generic JSON webhook |
| `GET` | `/api/v1/alerts` | List alerts |
| `GET` | `/api/v1/alerts/:id` | Alert detail |
| `PATCH` | `/api/v1/alerts/:id/acknowledge` | Acknowledge alert |
| `PATCH` | `/api/v1/alerts/:id/resolve` | Resolve alert |
| `GET` | `/api/v1/incidents` | List incidents |
| `POST` | `/api/v1/incidents` | Create incident |
| `GET` | `/api/v1/incidents/:id` | Incident detail |
| `PUT` | `/api/v1/incidents/:id` | Update incident |
| `GET` | `/api/v1/incidents/search?q=` | Full-text search |
| `GET` | `/api/v1/runbooks` | List runbooks |
| `POST` | `/api/v1/runbooks` | Create runbook |
| `GET` | `/api/v1/rosters` | List rosters |
| `GET` | `/api/v1/rosters/:id/oncall` | Current on-call |
| `POST` | `/api/v1/rosters/:id/overrides` | Add override |
| `GET` | `/api/v1/rosters/:id/export.ics` | iCal export |
| `GET` | `/api/v1/escalation-policies` | List policies |
| `POST` | `/api/v1/escalation-policies/:id/dry-run` | Test escalation |
| `GET` | `/api/v1/audit-log` | Audit log |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/metrics` | Prometheus metrics |

---

## Configuration

| Variable | Example | Description |
|----------|---------|-------------|
| `NIGHTOWL_MODE` | `api\|worker\|seed` | Runtime mode (default: `api`) |
| `NIGHTOWL_HOST` | `0.0.0.0` | Bind address |
| `NIGHTOWL_PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://...` | PostgreSQL connection string |
| `REDIS_URL` | `redis://...` | Redis connection string |
| `LOG_LEVEL` | `info` | Log level (`debug`/`info`/`warn`/`error`) |
| `LOG_FORMAT` | `json` | Log format (`json`/`text`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://...` | OpenTelemetry collector |
| `OIDC_ISSUER_URL` | `https://...` | OIDC provider URL |
| `OIDC_CLIENT_ID` | `nightowl` | OIDC client ID |
| `SLACK_BOT_TOKEN` | `xoxb-...` | Slack bot token |
| `SLACK_SIGNING_SECRET` | `...` | Slack signing secret |
| `SLACK_ALERT_CHANNEL` | `#alerts` | Slack alert channel |
| `CORS_ALLOWED_ORIGINS` | `*` | CORS origins (comma-separated) |

---

## Kubernetes Deployment

```bash
helm install nightowl deploy/helm/nightowl \
  --set secrets.databaseUrl="postgres://..." \
  --set secrets.redisUrl="redis://..." \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=nightowl.example.com
```

---

## Slack App Setup

1. Create a Slack app at [api.slack.com](https://api.slack.com).
2. Enable the **Events API** and subscribe to: `app_mention`, `message.channels`.
3. Add a slash command: `/nightowl`.
4. Enable **Interactivity** with the request URL: `https://your-domain/api/v1/slack/interactions`.
5. Set the event subscription URL: `https://your-domain/api/v1/slack/events`.
6. Add bot token scopes: `chat:write`, `commands`, `channels:read`, `users:read`.
7. Set the following environment variables:
   - `SLACK_BOT_TOKEN`
   - `SLACK_SIGNING_SECRET`
   - `SLACK_ALERT_CHANNEL`

---

## OIDC Setup

NightOwl is compatible with **Keycloak**, **Dex**, **Auth0**, or any standard OIDC provider.

1. Set `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` environment variables.
2. The JWT issued by the provider must include the following claims: `sub`, `email`, `tenant_slug`.
3. When OIDC is not configured, NightOwl falls back to API key authentication.

---

## Development

```bash
make build          # Build binary
make test           # Run tests
make lint           # Run linter
make sqlc           # Regenerate sqlc code
make migrate-up     # Run global migrations
make seed           # Seed development data
make docker         # Build Docker image
```

---

## Project Structure

```
cmd/nightowl/           Application entry point
internal/
  app/                   Application bootstrap
  auth/                  OIDC + API key authentication
  audit/                 Audit log writer
  config/                Configuration loading
  db/                    sqlc generated code
  httpserver/            HTTP server + middleware
  platform/              Database + Redis clients
  seed/                  Development seed data
  telemetry/             Logging, metrics, tracing
pkg/
  alert/                 Alert engine (webhooks, dedup, enrichment, lifecycle)
  escalation/            Escalation policies and engine
  incident/              Knowledge base CRUD + search
  integration/           Twilio callout stubs
  roster/                On-call schedules, overrides, iCal
  runbook/               Runbook templates
  slack/                 Slack bot integration
  tenant/                Multi-tenancy middleware
web/                     React frontend (Vite + TypeScript)
deploy/
  helm/nightowl/         Helm chart
  grafana/               Grafana dashboard
migrations/
  global/                Global schema migrations
  tenant/                Per-tenant schema migrations
```

---

## License

Copyright Wisbric. All rights reserved.
