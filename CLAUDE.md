# CLAUDE.md — NightOwl

## Project Overview

NightOwl (formerly OpsWatch) is an incident knowledge base, alert management, on-call roster, and escalation platform for 24/7 operations teams. It is designed for managed service providers running Kubernetes infrastructure across multiple time zones. NightOwl is a Wisbric product (wisbric.com).

## Specifications

All design decisions are captured in `docs/`. Always read the relevant spec before implementing:

- `docs/01-requirements.md` — Product requirements with implementation status
- `docs/02-architecture.md` — System architecture, tech stack, domain structure, API endpoints
- `docs/03-data-model.md` — PostgreSQL schema (15 tenant migrations + 2 global), queries, multi-tenancy
- `docs/04-integrations-workflow.md` — Slack, webhooks, Twilio, roster handoff, audit logging
- `docs/05-tasks.md` — Implementation task breakdown with completion status
- `docs/06-branding.md` — NightOwl branding, design system, color palette, typography
- `docs/07-deployment.md` — CI/CD pipeline, container builds, Helm deployment guide

## Branding

The product is called **NightOwl**. The frontend follows the design system in `docs/06-branding.md`. Key points:
- Dark mode is the default (theme toggle available)
- NightOwl color palette with severity/status colors
- Sidebar navigation layout with owl logo
- Branded loading spinners and empty states
- Footer: "A Wisbric product"

## Tech Stack

- **Language:** Go 1.25+ (module: `github.com/wisbric/nightowl`)
- **Binary:** `cmd/nightowl` with modes: api, worker, seed, seed-demo
- **Router:** go-chi/chi/v5
- **Database:** PostgreSQL 16+ via jackc/pgx/v5 + sqlc
- **Migrations:** golang-migrate (SQL files in `migrations/`)
- **Cache:** Redis 7 via redis/go-redis/v9
- **Auth:** Cookie sessions (`wisbric_session`) + OIDC (coreos/go-oidc/v3) + API keys (SHA-256) — auth logic lives in `core/pkg/auth`
- **Slack:** slack-go/slack
- **Telephony:** twilio/twilio-go
- **Metrics:** prometheus/client_golang (namespace: `nightowl`)
- **Tracing:** OpenTelemetry (OTLP gRPC)
- **Logging:** slog (structured JSON)
- **Config:** caarlos0/env/v11
- **Frontend:** React 19 + TypeScript 5.9 + Vite 7 + shadcn/ui + Tailwind CSS 4
- **Frontend state:** TanStack Query 5 + TanStack Router 1
- **Testing:** testcontainers-go for integration tests

## Code Conventions

- Go code follows standard `gofmt` + `golangci-lint` with default rules
- Package names are single lowercase words matching directory name
- Use table-driven tests
- Errors: wrap with `fmt.Errorf("doing X: %w", err)` — never discard errors silently
- Context: always pass `context.Context` as first parameter
- SQL: prefer sqlc-generated code; raw SQL for JOINs and columns not in sqlc schema
- HTTP handlers return JSON; always use `httpserver.Respond()` and `httpserver.RespondError()`
- Domain packages follow handler.go / service.go / store.go / {domain}.go pattern
- Per-request store creation from `tenant.ConnFromContext(r.Context())`
- Frontend: functional components only, no class components
- Frontend: use TanStack Query for all API calls, never raw fetch in components

## Multi-Tenancy

Schema-per-tenant isolation. Every request must resolve a tenant (from session cookie, JWT, API key, or dev header). The middleware acquires a pooled connection and sets `search_path` before any query executes. Never reference tenant data without going through the tenant middleware.

## Authentication

Auth is handled by the shared `core/pkg/auth` package. Middleware precedence: Cookie → PAT → Session JWT (Bearer) → OIDC JWT (Bearer) → API Key → Dev header.

- **Cookie sessions:** `wisbric_session` HttpOnly cookie set on login, validated by `core/pkg/auth/session.go`
- **Local admin:** Break-glass login at `POST /auth/local`, forced password change on first login
- **OIDC:** Optional, via Keycloak/Dex/Auth0 — set `OIDC_ISSUER_URL` and `OIDC_CLIENT_ID`
- **API keys:** `X-API-Key` header for service-to-service calls
- **Storage adapter:** `internal/authadapter/` implements `core/pkg/auth.Storage` for NightOwl's DB schema

## Development

```bash
docker compose up -d          # PostgreSQL + Redis
make seed                     # Create "acme" dev tenant (idempotent)
go run ./cmd/nightowl         # API on :8080
cd web && npm run dev         # Frontend on :3000 (proxies /api to :8080)
```

- Dev API key: `ow_dev_seed_key_do_not_use_in_production`
- Local admin: username `admin`, password `nightowl-admin` (dev mode only; forced password change on first login)
- Login URL: `http://localhost:3000/login`
- Env vars prefix: `NIGHTOWL_` (e.g., `NIGHTOWL_MODE`, `NIGHTOWL_HOST`, `NIGHTOWL_PORT`)
- DB credentials (dev): `nightowl:nightowl@localhost:5432/nightowl`
- Docker image: `nightowl:dev` (via `make docker`)

## Testing

- Unit tests: mock dependencies via interfaces, table-driven
- Integration tests: use testcontainers for real PostgreSQL and Redis
- Run `make test` before committing
- Run `make lint` before committing

## Commit Style

- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`
- One logical change per commit
- Reference task IDs from docs/05-tasks.md where applicable (e.g., `feat(alert): webhook receiver for Alertmanager [3.1]`)
