# CLAUDE.md — NightOwl

## Project Overview

NightOwl (formerly OpsWatch) is an incident knowledge base, alert management, on-call roster, and escalation platform for 24/7 operations teams. It is designed for managed service providers running Kubernetes infrastructure across multiple time zones. NightOwl is a Wisbric product (wisbric.com).

## Specifications

All design decisions are captured in `docs/`. Always read the relevant spec before implementing:

- `docs/01-requirements.md` — Product requirements and feature matrix
- `docs/02-architecture.md` — System architecture, tech stack, domain structure
- `docs/03-data-model.md` — PostgreSQL schema, queries, multi-tenancy strategy
- `docs/04-integrations-workflow.md` — Slack, webhooks, Twilio, roster handoff flows
- `docs/05-tasks.md` — Implementation task breakdown by phase
- `docs/06-branding.md` — NightOwl branding, design system, color palette, typography
- `docs/07-deployment.md` — CI/CD pipeline, container builds, Helm deployment guide

## Branding

The product is called **NightOwl**. The frontend must follow the design system in `docs/06-branding.md`. Key points:
- Dark mode is the default
- Use the NightOwl color palette (navy primary, owl-gold accent, severity colors)
- Inter font for UI, JetBrains Mono for code/alerts
- Sidebar navigation layout
- Footer: "A Wisbric product"

## Tech Stack

- **Language:** Go 1.23+
- **Router:** chi
- **Database:** PostgreSQL 16+ via pgx + sqlc
- **Migrations:** golang-migrate (SQL files)
- **Cache:** Redis via go-redis/v9
- **Auth:** OIDC (coreos/go-oidc) + API keys
- **Slack:** slack-go/slack
- **Telephony:** twilio-go
- **Metrics:** prometheus/client_golang
- **Tracing:** OpenTelemetry (OTLP)
- **Logging:** slog (structured JSON)
- **Frontend:** React 18 + TypeScript + Vite + shadcn/ui + Tailwind
- **Testing:** testcontainers-go for integration tests

## Code Conventions

- Go code follows standard `gofmt` + `golangci-lint` with default rules
- Package names are single lowercase words matching directory name
- Use table-driven tests
- Errors: wrap with `fmt.Errorf("doing X: %w", err)` — never discard errors silently
- Context: always pass `context.Context` as first parameter
- SQL: all queries via sqlc-generated code, never raw string concatenation
- HTTP handlers return JSON; always use the shared `respond` and `respondError` helpers
- Frontend: functional components only, no class components
- Frontend: use TanStack Query for all API calls, never raw fetch in components

## Multi-Tenancy

Schema-per-tenant isolation. Every request must resolve a tenant (from JWT or API key). The middleware sets `search_path` before any query executes. Never reference tenant data without going through the tenant middleware.

## Testing

- Unit tests: mock dependencies via interfaces
- Integration tests: use testcontainers for real PostgreSQL and Redis
- Run `make test` before committing
- Run `make lint` before committing

## Commit Style

- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`
- One logical change per commit
- Reference task IDs from docs/05-tasks.md where applicable (e.g., `feat(alert): webhook receiver for Alertmanager [AL-01]`)
