# NightOwl — Login, Local Admin & Profile UI (Phase 10)

> Aligns NightOwl's authentication and profile UI with BookOwl. References
> BookOwl's `docs/12-login-and-profile.md` for the shared patterns — implement
> the same approach in NightOwl rather than inventing a parallel system.

---

## 1. What Changes and Why

Three things are being added/changed:

1. **Shared Keycloak config page** — NightOwl's OIDC configuration uses the same
   admin UI pattern as BookOwl (issuer URL, client ID, client secret stored
   encrypted, hot reload, test connection). No reason to have two different UIs
   for the same Keycloak tenant.

2. **Local admin account** — break-glass account identical to BookOwl's. One per
   tenant, bcrypt password, `must_change=true` on creation, forced password change
   before app access.

3. **User info moved to bottom-left of sidebar** — the current layout puts
   `[user] [◐]` in the header top-right. Move it to the bottom of the sidebar,
   below the nav items, matching BookOwl's layout convention.

---

## 2. Spec Files That Need Updating

### 2.1 `02-architecture.md` — Authentication section

**Section 5.1 (Authentication Methods):** Add local admin row:

| Method | Use Case |
|--------|----------|
| OIDC/OAuth2 (JWT) | Web UI users, SSO via Keycloak/Dex |
| Local admin | Break-glass account, one per tenant, bcrypt password |
| API Key (header: `X-API-Key`) | Webhook senders, agent integrations |
| Slack signing secret | Slack bot event verification |
| Dev header (`X-Tenant-Slug`) | Development fallback |

**Authentication precedence:** JWT → Local admin session → API Key → Dev header

**New auth endpoints (public, no auth required):**
```
POST /auth/local           — local admin login
POST /auth/logout          — clear session
GET  /auth/oidc/login      — initiate OIDC redirect
GET  /auth/callback        — OIDC callback
POST /auth/change-password — forced password change (must_change flow)
```

**Session:** NightOwl issues its own session JWT (signed with `NIGHTOWL_SECRET_KEY`)
rather than forwarding Keycloak tokens. Same pattern as BookOwl. Cookie name:
`no_session` (HttpOnly, Secure, SameSite=Strict).

### 2.2 `03-data-model.md` — Global schema additions

Add to **Section 2 (Global Tables)**:

**2.3 local_admins**

```sql
CREATE TABLE public.local_admins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL UNIQUE REFERENCES public.tenants(id) ON DELETE CASCADE,
    username        TEXT NOT NULL DEFAULT 'admin',
    password_hash   TEXT NOT NULL,          -- bcrypt cost 12
    must_change     BOOLEAN NOT NULL DEFAULT true,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

One row per tenant. Created automatically when a tenant is provisioned.

**Default password:**
- Dev seed: `nightowl-admin`
- Production: random 16-char alphanumeric OR `NIGHTOWL_ADMIN_PASSWORD` env var
- Printed to stdout once on tenant creation, never again
- `must_change=true` always set on creation

**2.4 oidc_config** (per tenant, in tenant schema)

```sql
CREATE TABLE oidc_config (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer_url      TEXT NOT NULL,
    client_id       TEXT NOT NULL,
    client_secret   TEXT NOT NULL,  -- AES-256-GCM encrypted, key from NIGHTOWL_SECRET_KEY
    enabled         BOOLEAN NOT NULL DEFAULT false,
    tested_at       TIMESTAMPTZ,    -- last successful test connection
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

This mirrors BookOwl's OIDC config storage. Hot reload: after saving, the OIDC
provider is re-initialized without pod restart (same mechanism as BookOwl).

### 2.3 `05-branding.md` — Layout section

**Section 5 (Layout) — Dashboard:**

Move user controls from header to sidebar bottom. Updated layout:

```
┌─────────────────────────────────────────────────────┐
│ 🦉 NightOwl          [tenant: acme]            [◐] │  ← dark mode toggle stays in header
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│ Dashboard│        (main content)                   │
│ Alerts   │                                          │
│ KB       │                                          │
│ Rosters  │                                          │
│ Escalate │                                          │
│ Admin    │                                          │
│          │                                          │
│ ──────── │                                          │
│ [SK]     │                                          │
│ Stefan K.│                                          │
│ engineer │                                          │
│ ──────── │                                          │
│ Admin    │                                          │
│ Sign out │                                          │
└──────────┴──────────────────────────────────────────┘
```

**Bottom-left sidebar section:**
- Circular avatar with user initials (same color-hash logic as BookOwl)
- Display name on first line
- Role badge on second line (`admin` / `manager` / `engineer` / `readonly`)
- Separator above
- "Admin" link (only shown to admin role)
- "Sign out" link → calls `POST /auth/logout`

Dark mode toggle moves from the avatar dropdown to a standalone icon button in
the header (top-right, replacing the old `[user]` area).

---

## 3. New: OIDC Configuration Page

### 3.1 Location

Admin → Authentication (new tab in the existing admin hub).

This is the **same UI and same backend pattern as BookOwl's** `docs/11-oidc-admin.md`.
Implement identically — same form fields, same test connection flow, same encrypted
storage, same hot reload. The only differences are:

- NightOwl uses `NIGHTOWL_SECRET_KEY` for AES-256-GCM encryption (not `BOOKOWL_SECRET_KEY`)
- Cookie name: `no_session` (not `bw_session`)
- Routes: `/api/v1/admin/oidc/config` (GET/PUT), `/api/v1/admin/oidc/test` (POST)

### 3.2 Admin Authentication Tab UI

```
┌─── Admin — Authentication ──────────────────────────────────────────┐
│                                                                      │
│  OIDC / Keycloak                                                     │
│                                                                      │
│  Issuer URL                                                          │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ https://keycloak.example.com/realms/nightowl                   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Client ID                                                           │
│  ┌─────────────────────────┐                                         │
│  │ nightowl                │                                         │
│  └─────────────────────────┘                                         │
│                                                                      │
│  Client Secret                                              [Reveal] │
│  ┌──────────────────────────────────────────────────┐               │
│  │ ••••••••••••••••••••••••••••••••••••••••         │               │
│  └──────────────────────────────────────────────────┘               │
│                                                                      │
│  Role Mapping (Keycloak Group → NightOwl Role)                      │
│  ┌──────────────────────┬──────────────────────────┐               │
│  │ /nightowl-admins     │ admin                    │               │
│  │ /nightowl-engineers  │ engineer                 │               │
│  │ /nightowl-readonly   │ readonly                 │               │
│  └──────────────────────┴──────────────────────────┘               │
│                                                                      │
│  [ Test Connection ]  ✅ Connected · Last tested Feb 24 14:32       │
│                                                                      │
│  [ Cancel ]                                       [ Save & Reload ] │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  LOCAL ADMIN ACCOUNT                                                 │
│                                                                      │
│  Username: admin                                                     │
│  Last login: Feb 24, 2026 at 11:42                                  │
│                                                                      │
│  [ Reset Password ]   ← generates new random password, prints once  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.3 API Endpoints

```
GET  /api/v1/admin/oidc/config     — get current OIDC config (secret masked)
PUT  /api/v1/admin/oidc/config     — save config, hot-reload OIDC provider
POST /api/v1/admin/oidc/test       — test connection, returns diagnostics
POST /api/v1/admin/local-admin/reset — reset local admin password (admin only)
```

---

## 4. Login Page

Route: `/login` — public, no auth required.

Same structure as BookOwl's login page:

```
┌─────────────────────────────────────────┐
│                                         │
│           🦉 NightOwl                   │
│                                         │
│   [ Sign in with Keycloak ]             │
│   (greyed out if OIDC not configured)   │
│                                         │
│   ────────── or ──────────              │
│                                         │
│   Username  [ admin              ]      │
│   Password  [ ••••••••••••••     ]      │
│                                         │
│   [ Sign in ]                           │
│                                         │
│   Rate limit: 10 attempts/15 min/IP     │
│                                         │
│   ─────────────────────────────────     │
│   Powered by Wisbric                    │
│                                         │
└─────────────────────────────────────────┘
```

**Behaviour:**
- OIDC button: visible always, greyed with tooltip "OIDC not configured" if no config saved
- Local admin form: always visible (break-glass)
- Rate limiting: 10 failed attempts per IP per 15 minutes via Redis INCR + EXPIRE → 429 with countdown timer
- `must_change=true`: after login, redirect to `/change-password` before any app access

### 4.1 Change Password Page

Route: `/change-password` — authenticated but gated (only accessible if `must_change=true`).

Same as BookOwl: password requirements (≥12 chars, upper+lower, number or symbol),
validates, updates `password_hash`, clears `must_change`, redirects to `/`.

---

## 5. Session JWT Format

```json
{
  "sub": "user-uuid-or-local-admin",
  "tenant": "acme",
  "role": "admin",
  "auth_method": "oidc",    // "oidc" | "local"
  "iat": 1740384000,
  "exp": 1740427200
}
```

Signed with `NIGHTOWL_SECRET_KEY`. TTL: `NIGHTOWL_SESSION_TTL` (default 12h).
Silent refresh: new cookie issued when <2h remaining.

---

## 6. Updated Auth Middleware

NightOwl's auth middleware currently handles: JWT → API Key → Dev header.

Updated order:

1. `no_session` cookie (OIDC or local admin session JWT) → validate, extract claims
2. `X-API-Key` header → SHA-256 hash lookup in `api_keys` table
3. Dev fallback: `X-Tenant-Slug` + `X-Dev-User` headers (dev mode only)

The existing API key flow is unchanged. Webhook senders continue using `X-API-Key`.

---

## 7. Configuration

```bash
# Session
NIGHTOWL_SECRET_KEY=<32-byte hex>    # used for JWT signing AND AES-256-GCM encryption
NIGHTOWL_SESSION_TTL=12h

# Local admin (seed)
NIGHTOWL_ADMIN_PASSWORD=             # optional: set initial password, else random
```

`NIGHTOWL_SECRET_KEY` already exists in the architecture spec for JWT signing.
Reuse it for OIDC client secret encryption (same as BookOwl reuses `BOOKOWL_SECRET_KEY`).

---

## 8. Tasks

Add to implementation backlog as Phase 10:

### Backend
- [ ] Global migration: `public.local_admins` table
- [ ] Tenant migration: `oidc_config` table
- [ ] Seed: create local admin on tenant provision, print password once, `must_change=true`
- [ ] `POST /auth/local` — bcrypt verify, rate limit (Redis), issue session JWT, set `no_session` cookie
- [ ] `POST /auth/logout` — clear `no_session` cookie
- [ ] `GET /auth/oidc/login` — initiate OIDC redirect (reads from `oidc_config` table, not env var)
- [ ] `GET /auth/callback` — validate state, exchange code, upsert user, map groups → role, issue session JWT
- [ ] `POST /auth/change-password` — bcrypt new password, clear `must_change`
- [ ] Session JWT issue + validate (sign with `NIGHTOWL_SECRET_KEY`)
- [ ] Silent session refresh (<2h remaining)
- [ ] Rate limiter: 10 fails/IP/15min via Redis INCR + EXPIRE
- [ ] `GET /api/v1/admin/oidc/config` — return config (secret masked)
- [ ] `PUT /api/v1/admin/oidc/config` — save, encrypt secret, hot-reload OIDC provider
- [ ] `POST /api/v1/admin/oidc/test` — test connection, return diagnostics
- [ ] `POST /api/v1/admin/local-admin/reset` — admin-only, reset password
- [ ] Update auth middleware: session cookie → API key → dev header
- [ ] Unit tests: bcrypt, rate limiter, session JWT, group→role mapping

### Frontend
- [ ] `/login` page — Keycloak button + local admin form, rate limit countdown
- [ ] `/change-password` page — must_change flow with password requirements
- [ ] OIDC callback route — loading spinner, redirect on success
- [ ] Auth context provider — `useAuth()` hook with current user
- [ ] Redirect unauthenticated → `/login?return=<path>`
- [ ] Move user info from header to **bottom-left of sidebar**: avatar initials, display name, role badge
- [ ] Remove user avatar from header top-right; keep dark mode toggle as standalone icon
- [ ] "Sign out" in sidebar bottom section → `POST /auth/logout`
- [ ] "Admin" link in sidebar bottom section (admin role only)
- [ ] Admin → Authentication tab: OIDC config form, test connection button, local admin reset
- [ ] Auth method indicator in sidebar user section ("via Keycloak" / "local admin")
