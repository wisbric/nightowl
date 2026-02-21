# NightOwl — Data Model Specification

## 1. Schema Strategy

- `public` schema: global tables (tenants, API keys, system config)
- `tenant_<slug>` schema: all tenant-specific data
- Migrations run per-schema using `golang-migrate`

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
    key_hash    TEXT NOT NULL UNIQUE,  -- bcrypt or SHA-256 of the key
    key_prefix  TEXT NOT NULL,         -- first 8 chars for identification
    description TEXT NOT NULL DEFAULT '',
    role        TEXT NOT NULL DEFAULT 'engineer',  -- admin, manager, engineer, readonly, webhook
    scopes      TEXT[] NOT NULL DEFAULT '{}',       -- optional: restrict to specific endpoints
    last_used   TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ,          -- NULL = no expiry
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_keys_tenant ON public.api_keys(tenant_id);
CREATE INDEX idx_api_keys_hash ON public.api_keys(key_hash);
```

## 3. Tenant Tables (per tenant_<slug> schema)

### 3.1 users (tenant members)

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT NOT NULL UNIQUE,  -- OIDC subject claim
    email           TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',  -- IANA timezone (e.g., 'Pacific/Auckland', 'Europe/Berlin')
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

### 3.2 services (service catalogue)

```sql
CREATE TABLE services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,           -- e.g., "customer-api", "etcd", "ingress-controller"
    cluster     TEXT,                    -- Kubernetes cluster name
    namespace   TEXT,                    -- Kubernetes namespace
    description TEXT,
    owner_id    UUID REFERENCES users(id),  -- team/person responsible
    tier        TEXT DEFAULT 'standard',    -- critical, standard, best-effort
    metadata    JSONB DEFAULT '{}',         -- flexible: labels, annotations, links
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, cluster, namespace)
);
```

### 3.3 alerts

```sql
CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint     TEXT NOT NULL,         -- dedup key (hash of alert identity labels)
    status          TEXT NOT NULL DEFAULT 'firing',  -- firing, acknowledged, investigating, resolved
    severity        TEXT NOT NULL DEFAULT 'warning', -- info, warning, critical, major
    source          TEXT NOT NULL,          -- alertmanager, keep, generic, agent
    title           TEXT NOT NULL,
    description     TEXT,
    labels          JSONB NOT NULL DEFAULT '{}',     -- all alert labels
    annotations     JSONB NOT NULL DEFAULT '{}',     -- all alert annotations
    service_id      UUID REFERENCES services(id),
    
    -- Enrichment from knowledge base
    matched_incident_id UUID REFERENCES incidents(id),
    suggested_solution  TEXT,
    
    -- Resolution
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id),  -- NULL if resolved by source (auto-resolve)
    resolved_at     TIMESTAMPTZ,
    resolved_by_agent BOOLEAN DEFAULT false,     -- true if resolved by automated agent
    agent_resolution_notes TEXT,                  -- what the agent did
    
    -- Dedup tracking
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    first_fired_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_fired_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- Escalation
    escalation_policy_id UUID REFERENCES escalation_policies(id),
    current_escalation_tier INTEGER DEFAULT 0,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_status ON alerts(status) WHERE status != 'resolved';
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_created ON alerts(created_at DESC);
CREATE INDEX idx_alerts_service ON alerts(service_id);
CREATE INDEX idx_alerts_labels ON alerts USING GIN(labels);
```

### 3.4 incidents (knowledge base)

```sql
CREATE TABLE incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT NOT NULL,
    fingerprints    TEXT[] NOT NULL DEFAULT '{}',  -- one or more fingerprints that match this incident
    
    -- Classification
    severity        TEXT NOT NULL DEFAULT 'warning',
    category        TEXT,                           -- e.g., "networking", "storage", "compute", "application"
    tags            TEXT[] NOT NULL DEFAULT '{}',
    
    -- Affected scope
    services        TEXT[] NOT NULL DEFAULT '{}',   -- service names
    clusters        TEXT[] NOT NULL DEFAULT '{}',   -- cluster names
    namespaces      TEXT[] NOT NULL DEFAULT '{}',
    
    -- Knowledge
    symptoms        TEXT,              -- what the alert/issue looks like
    error_patterns  TEXT[] DEFAULT '{}', -- regex or exact strings that match
    root_cause      TEXT,
    solution        TEXT,              -- markdown
    runbook_id      UUID REFERENCES runbooks(id),
    
    -- Resolution tracking
    resolution_count    INTEGER NOT NULL DEFAULT 0,
    last_resolved_at    TIMESTAMPTZ,
    last_resolved_by    UUID REFERENCES users(id),
    avg_resolution_mins FLOAT,
    
    -- Merge tracking
    merged_into_id  UUID REFERENCES incidents(id),  -- NULL unless merged
    
    -- Full-text search
    search_vector   TSVECTOR,
    
    created_by      UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incidents_fingerprints ON incidents USING GIN(fingerprints);
CREATE INDEX idx_incidents_search ON incidents USING GIN(search_vector);
CREATE INDEX idx_incidents_tags ON incidents USING GIN(tags);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_merged ON incidents(merged_into_id) WHERE merged_into_id IS NOT NULL;

-- Auto-update search vector
CREATE OR REPLACE FUNCTION update_incident_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
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

### 3.5 incident_history (version tracking)

```sql
CREATE TABLE incident_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    changed_by  UUID REFERENCES users(id),
    change_type TEXT NOT NULL,  -- created, updated, merged, resolution_added
    diff        JSONB NOT NULL, -- { field: { old: ..., new: ... } }
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_history_incident ON incident_history(incident_id, created_at DESC);
```

### 3.6 runbooks

```sql
CREATE TABLE runbooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,          -- markdown
    category    TEXT,                    -- e.g., "kubernetes", "database", "networking"
    is_template BOOLEAN DEFAULT false,  -- pre-seeded templates
    tags        TEXT[] DEFAULT '{}',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_runbooks_category ON runbooks(category);
CREATE INDEX idx_runbooks_template ON runbooks(is_template) WHERE is_template = true;
```

### 3.7 rosters (on-call schedules)

```sql
CREATE TABLE rosters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,          -- e.g., "EMEA Primary", "APAC Primary"
    description     TEXT,
    timezone        TEXT NOT NULL,          -- primary timezone for display
    rotation_type   TEXT NOT NULL,          -- daily, weekly, custom
    rotation_length INTEGER NOT NULL DEFAULT 7,  -- days per rotation
    handoff_time    TIME NOT NULL DEFAULT '09:00', -- local time in roster timezone
    
    -- Follow-the-sun config
    is_follow_the_sun BOOLEAN DEFAULT false,
    linked_roster_id  UUID REFERENCES rosters(id),  -- paired roster for follow-the-sun
    
    -- Escalation
    escalation_policy_id UUID REFERENCES escalation_policies(id),
    
    start_date      DATE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.8 roster_members

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

### 3.9 roster_overrides

```sql
CREATE TABLE roster_overrides (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),  -- who is covering
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

### 3.10 escalation_policies

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
    repeat_count    INTEGER DEFAULT 0,     -- 0 = no repeat, N = repeat N times before giving up
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.11 escalation_events (escalation audit trail)

```sql
CREATE TABLE escalation_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id        UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    policy_id       UUID NOT NULL REFERENCES escalation_policies(id),
    tier            INTEGER NOT NULL,
    action          TEXT NOT NULL,  -- notified, acknowledged, timeout, resolved, cancelled
    target_user_id  UUID REFERENCES users(id),
    notify_method   TEXT,           -- slack_dm, phone, sms, slack_channel
    notify_result   TEXT,           -- sent, failed, busy, voicemail
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_escalation_events_alert ON escalation_events(alert_id, created_at);
```

### 3.12 audit_log

```sql
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),
    api_key_id  UUID,                   -- if action via API key
    action      TEXT NOT NULL,           -- create, update, delete, acknowledge, escalate, resolve, merge, login
    resource    TEXT NOT NULL,           -- alert, incident, roster, escalation_policy, user, runbook
    resource_id UUID,
    detail      JSONB DEFAULT '{}',     -- contextual info (diff, reason, etc.)
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_resource ON audit_log(resource, resource_id);
CREATE INDEX idx_audit_log_user ON audit_log(user_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);

-- Partitioned by month for efficient retention
-- CREATE TABLE audit_log (...) PARTITION BY RANGE (created_at);
```

### 3.13 slack_message_mappings

```sql
-- Track which Slack messages correspond to which alerts/incidents
-- for interactive message updates (e.g., "Mark Resolved" button)
CREATE TABLE slack_message_mappings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL,
    message_ts  TEXT NOT NULL,     -- Slack message timestamp (acts as message ID)
    thread_ts   TEXT,              -- parent thread timestamp if in a thread
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    CHECK (alert_id IS NOT NULL OR incident_id IS NOT NULL)
);

CREATE INDEX idx_slack_mappings_alert ON slack_message_mappings(alert_id);
CREATE INDEX idx_slack_mappings_channel_ts ON slack_message_mappings(channel_id, message_ts);
```

## 4. Key Queries

### 4.1 Who is on-call right now?

```sql
-- Check overrides first, then calculate rotation position
WITH override_check AS (
    SELECT user_id
    FROM roster_overrides
    WHERE roster_id = $1
      AND now() BETWEEN start_at AND end_at
    LIMIT 1
),
rotation_calc AS (
    SELECT rm.user_id
    FROM roster_members rm
    JOIN rosters r ON r.id = rm.roster_id
    WHERE r.id = $1
      AND rm.position = (
          -- Calculate current position based on days since start
          EXTRACT(EPOCH FROM (now() - (r.start_date + r.handoff_time)::timestamptz))
          / (r.rotation_length * 86400)
      )::integer % (SELECT COUNT(*) FROM roster_members WHERE roster_id = $1)
)
SELECT COALESCE(
    (SELECT user_id FROM override_check),
    (SELECT user_id FROM rotation_calc)
) AS on_call_user_id;
```

### 4.2 Full-text knowledge base search

```sql
SELECT id, title, severity, symptoms, solution, runbook_id,
       ts_rank(search_vector, query) AS rank
FROM incidents,
     plainto_tsquery('english', $1) query
WHERE search_vector @@ query
  AND merged_into_id IS NULL  -- exclude merged incidents
ORDER BY rank DESC
LIMIT $2;
```

### 4.3 Alert dedup check

```sql
-- Redis first (hot path), fallback to DB
-- Redis key: alert:dedup:<tenant_id>:<fingerprint>
-- Redis value: alert_id, TTL = dedup_window (e.g., 300s)

-- DB fallback:
SELECT id, occurrence_count
FROM alerts
WHERE fingerprint = $1
  AND status != 'resolved'
  AND last_fired_at > now() - INTERVAL '5 minutes'
ORDER BY last_fired_at DESC
LIMIT 1;
```
