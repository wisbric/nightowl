# NightOwl — Data Model Specification

## 1. Schema Strategy

- `public` schema: global tables (tenants, API keys)
- `tenant_<slug>` schema: all tenant-specific data
- Migrations run per-schema using `golang-migrate`
- Global migrations: `migrations/global/` (2 migrations)
- Tenant migrations: `migrations/tenant/` (15 migrations)

## 2. Global Tables (public schema)

### 2.1 tenants

```sql
CREATE TABLE public.tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,  -- used as schema name: tenant_<slug>
    config      JSONB NOT NULL DEFAULT '{}',
    -- config contains: slack_workspace_id, slack_bot_token, twilio_config,
    -- default_timezone, retention_days_alerts, retention_days_incidents
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tenants_slug ON public.tenants(slug);
```

### 2.2 api_keys

```sql
CREATE TABLE public.api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    key_hash    TEXT NOT NULL UNIQUE,  -- SHA-256 of the raw key
    key_prefix  TEXT NOT NULL,         -- first 8 chars for identification
    description TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL DEFAULT 'engineer',  -- admin, manager, engineer, readonly
    scopes      TEXT[] NOT NULL DEFAULT '{}',
    last_used   TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ,          -- NULL = no expiry
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_tenant ON public.api_keys(tenant_id);
CREATE INDEX idx_api_keys_hash ON public.api_keys(key_hash);
```

## 3. Tenant Tables (per tenant_<slug> schema)

### 3.1 users

Migration: `000001_create_users`

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT NOT NULL UNIQUE,  -- OIDC subject claim
    email           TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',  -- IANA timezone
    phone           TEXT,                          -- E.164 format for callout
    slack_user_id   TEXT,                          -- Slack user ID for DMs
    role            TEXT NOT NULL DEFAULT 'engineer',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_external_id ON users(external_id);
```

### 3.2 services

Migration: `000002_create_services`

```sql
CREATE TABLE services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    cluster     TEXT,
    namespace   TEXT,
    description TEXT,
    owner_id    UUID REFERENCES users(id),
    tier        TEXT DEFAULT 'standard',    -- critical, standard, best-effort
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, cluster, namespace)
);
```

### 3.3 escalation_policies

Migration: `000003_create_escalation_policies`

```sql
CREATE TABLE escalation_policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    tiers       JSONB NOT NULL,
    -- tiers format:
    -- [
    --   { "tier": 1, "timeout_minutes": 5, "notify_via": ["slack_dm"], "targets": ["oncall_primary"] },
    --   { "tier": 2, "timeout_minutes": 10, "notify_via": ["slack_dm", "phone"], "targets": ["oncall_backup"] },
    --   { "tier": 3, "timeout_minutes": 15, "notify_via": ["phone", "slack_channel"], "targets": ["team_lead"] }
    -- ]
    repeat_count    INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.4 runbooks

Migration: `000004_create_runbooks`

```sql
CREATE TABLE runbooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,          -- markdown
    category    TEXT,
    is_template BOOLEAN DEFAULT false,
    tags        TEXT[] DEFAULT '{}',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_runbooks_category ON runbooks(category);
CREATE INDEX idx_runbooks_template ON runbooks(is_template) WHERE is_template = true;
```

### 3.5 incidents

Migration: `000005_create_incidents`

```sql
CREATE TABLE incidents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title               TEXT NOT NULL,
    fingerprints        TEXT[] NOT NULL DEFAULT '{}',
    severity            TEXT NOT NULL DEFAULT 'warning',
    category            TEXT,
    tags                TEXT[] NOT NULL DEFAULT '{}',
    services            TEXT[] NOT NULL DEFAULT '{}',
    clusters            TEXT[] NOT NULL DEFAULT '{}',
    namespaces          TEXT[] NOT NULL DEFAULT '{}',
    symptoms            TEXT,
    error_patterns      TEXT[] DEFAULT '{}',
    root_cause          TEXT,
    solution            TEXT,
    runbook_id          UUID REFERENCES runbooks(id),
    resolution_count    INTEGER NOT NULL DEFAULT 0,
    last_resolved_at    TIMESTAMPTZ,
    last_resolved_by    UUID REFERENCES users(id),
    avg_resolution_mins FLOAT,
    merged_into_id      UUID REFERENCES incidents(id),
    search_vector       TSVECTOR,
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incidents_fingerprints ON incidents USING GIN(fingerprints);
CREATE INDEX idx_incidents_search ON incidents USING GIN(search_vector);
CREATE INDEX idx_incidents_tags ON incidents USING GIN(tags);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_merged ON incidents(merged_into_id) WHERE merged_into_id IS NOT NULL;
```

**Full-text search trigger** (updated by migration `000014` to include `category`):

```sql
CREATE OR REPLACE FUNCTION update_incident_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.category, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.symptoms, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.root_cause, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.solution, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.error_patterns, ' '), '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.services, ' '), '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_incidents_search_vector
    BEFORE INSERT OR UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION update_incident_search_vector();
```

### 3.6 incident_history

Migration: `000006_create_incident_history`

```sql
CREATE TABLE incident_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    changed_by  UUID REFERENCES users(id),
    change_type TEXT NOT NULL,  -- created, updated, merged, resolution_added
    diff        JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_history_incident ON incident_history(incident_id, created_at DESC);
```

### 3.7 alerts

Migration: `000007_create_alerts`

```sql
CREATE TABLE alerts (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint             TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'firing',
    severity                TEXT NOT NULL DEFAULT 'warning',
    source                  TEXT NOT NULL,
    title                   TEXT NOT NULL,
    description             TEXT,
    labels                  JSONB NOT NULL DEFAULT '{}',
    annotations             JSONB NOT NULL DEFAULT '{}',
    service_id              UUID REFERENCES services(id),
    matched_incident_id     UUID REFERENCES incidents(id),
    suggested_solution      TEXT,
    acknowledged_by         UUID REFERENCES users(id),
    acknowledged_at         TIMESTAMPTZ,
    resolved_by             UUID REFERENCES users(id),
    resolved_at             TIMESTAMPTZ,
    resolved_by_agent       BOOLEAN DEFAULT false,
    agent_resolution_notes  TEXT,
    occurrence_count        INTEGER NOT NULL DEFAULT 1,
    first_fired_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_fired_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    escalation_policy_id    UUID REFERENCES escalation_policies(id),
    current_escalation_tier INTEGER DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_status ON alerts(status) WHERE status != 'resolved';
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_created ON alerts(created_at DESC);
CREATE INDEX idx_alerts_service ON alerts(service_id);
CREATE INDEX idx_alerts_labels ON alerts USING GIN(labels);
```

### 3.8 rosters

Migration: `000008_create_rosters` + `000015_add_roster_end_date`

```sql
CREATE TABLE rosters (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 TEXT NOT NULL,
    description          TEXT,
    timezone             TEXT NOT NULL,
    rotation_type        TEXT NOT NULL,          -- daily, weekly, custom
    rotation_length      INTEGER NOT NULL DEFAULT 7,
    handoff_time         TIME NOT NULL DEFAULT '09:00',
    is_follow_the_sun    BOOLEAN DEFAULT false,
    linked_roster_id     UUID REFERENCES rosters(id),
    escalation_policy_id UUID REFERENCES escalation_policies(id),
    start_date           DATE NOT NULL,
    end_date             DATE,                   -- NULL = perpetual roster
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

The API computes `is_active` from `end_date`: active if `end_date` is NULL or >= today.

### 3.9 roster_members

Migration: `000009_create_roster_members`

```sql
CREATE TABLE roster_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    position    INTEGER NOT NULL,  -- order in rotation (0-indexed)

    UNIQUE(roster_id, user_id),
    UNIQUE(roster_id, position)
);

CREATE INDEX idx_roster_members_roster ON roster_members(roster_id);
```

The API JOINs with `users` to return `display_name` alongside each member.

### 3.10 roster_overrides

Migration: `000010_create_roster_overrides`

```sql
CREATE TABLE roster_overrides (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    start_at    TIMESTAMPTZ NOT NULL,
    end_at      TIMESTAMPTZ NOT NULL,
    reason      TEXT,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (end_at > start_at)
);

CREATE INDEX idx_roster_overrides_roster ON roster_overrides(roster_id);
CREATE INDEX idx_roster_overrides_active ON roster_overrides(roster_id, start_at, end_at);
```

### 3.11 escalation_events

Migration: `000011_create_escalation_events`

```sql
CREATE TABLE escalation_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id        UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    policy_id       UUID NOT NULL REFERENCES escalation_policies(id),
    tier            INTEGER NOT NULL,
    action          TEXT NOT NULL,
    target_user_id  UUID REFERENCES users(id),
    notify_method   TEXT,
    notify_result   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_escalation_events_alert ON escalation_events(alert_id, created_at);
```

### 3.12 audit_log

Migration: `000012_create_audit_log`

```sql
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),
    api_key_id  UUID,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id UUID,
    detail      JSONB DEFAULT '{}',
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_resource ON audit_log(resource, resource_id);
CREATE INDEX idx_audit_log_user ON audit_log(user_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);
```

Audit entries are written asynchronously via a buffered channel (capacity 256, flush every 2s or at 32 entries).

### 3.13 slack_message_mappings

Migration: `000013_create_slack_message_mappings`

```sql
CREATE TABLE slack_message_mappings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL,
    message_ts  TEXT NOT NULL,
    thread_ts   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (alert_id IS NOT NULL OR incident_id IS NOT NULL)
);

CREATE INDEX idx_slack_mappings_alert ON slack_message_mappings(alert_id);
CREATE INDEX idx_slack_mappings_channel_ts ON slack_message_mappings(channel_id, message_ts);
```

## 4. Migration History

| # | Name | Description |
|---|------|-------------|
| Global 001 | `create_tenants` | Tenants table with slug index |
| Global 002 | `create_api_keys` | API keys with tenant FK, hash index |
| Tenant 001 | `create_users` | Users with external_id, email, role |
| Tenant 002 | `create_services` | Service catalogue with cluster/namespace |
| Tenant 003 | `create_escalation_policies` | Policies with JSONB tiers |
| Tenant 004 | `create_runbooks` | Runbooks with category, template flag |
| Tenant 005 | `create_incidents` | Incidents with FTS trigger |
| Tenant 006 | `create_incident_history` | Incident changelog |
| Tenant 007 | `create_alerts` | Alerts with indexes |
| Tenant 008 | `create_rosters` | On-call rosters |
| Tenant 009 | `create_roster_members` | Roster membership |
| Tenant 010 | `create_roster_overrides` | Time-based overrides |
| Tenant 011 | `create_escalation_events` | Escalation audit trail |
| Tenant 012 | `create_audit_log` | Audit log |
| Tenant 013 | `create_slack_message_mappings` | Slack message tracking |
| Tenant 014 | `add_category_to_search_vector` | Add category to FTS trigger |
| Tenant 015 | `add_roster_end_date` | Add end_date column to rosters |

## 5. Key Queries

### 5.1 On-call calculation (in Go, not SQL)

The on-call calculation is performed in `pkg/roster/service.go`:

1. Check for active override: `SELECT ... FROM roster_overrides WHERE roster_id = $1 AND start_at <= $2 AND end_at > $2`
2. If override active: primary = override user, secondary = scheduled rotation member
3. Calculate rotation position: `days_since_start / rotation_length % member_count`
4. Primary = member at calculated position, secondary = member at `(position + 1) % count`
5. Member queries JOIN with `users` table to resolve `display_name`

### 5.2 Full-text knowledge base search

```sql
SELECT id, title, severity, symptoms, solution,
       ts_rank(search_vector, query) AS rank,
       ts_headline('english', title, query) AS title_highlight,
       ts_headline('english', COALESCE(symptoms, ''), query) AS symptoms_highlight,
       ts_headline('english', COALESCE(solution, ''), query) AS solution_highlight
FROM incidents,
     plainto_tsquery('english', $1) query
WHERE search_vector @@ query
  AND merged_into_id IS NULL
ORDER BY rank DESC
LIMIT $2 OFFSET $3;
```

### 5.3 Alert dedup check

```
Redis (hot path): GET alert:dedup:{schema}:{fingerprint} → alert_id (5min TTL)

DB fallback:
SELECT id, occurrence_count
FROM alerts
WHERE fingerprint = $1
  AND status != 'resolved'
  AND last_fired_at > now() - INTERVAL '5 minutes'
ORDER BY last_fired_at DESC
LIMIT 1;
```

## 6. sqlc Configuration

Queries are defined in `sqlc/queries/` organized by domain. Schema templates in `sqlc/schema/`. Generated Go code in `internal/db/`. Run `make sqlc` to regenerate.

Note: Some queries (particularly in the roster package) use raw SQL instead of sqlc-generated code for JOINs and columns not in the original sqlc schema (e.g., `display_name` from users, `end_date`).
