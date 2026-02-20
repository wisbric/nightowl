CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    phone           TEXT,
    slack_user_id   TEXT,
    role            TEXT NOT NULL DEFAULT 'engineer',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    cluster     TEXT,
    namespace   TEXT,
    description TEXT,
    owner_id    UUID REFERENCES users(id),
    tier        TEXT DEFAULT 'standard',
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, cluster, namespace)
);

CREATE TABLE escalation_policies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    description     TEXT,
    tiers           JSONB NOT NULL,
    repeat_count    INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE runbooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    category    TEXT,
    is_template BOOLEAN DEFAULT false,
    tags        TEXT[] DEFAULT '{}',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

CREATE TABLE incident_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    changed_by  UUID REFERENCES users(id),
    change_type TEXT NOT NULL,
    diff        JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

CREATE TABLE rosters (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    description             TEXT,
    timezone                TEXT NOT NULL,
    rotation_type           TEXT NOT NULL,
    rotation_length         INTEGER NOT NULL DEFAULT 7,
    handoff_time            TIME NOT NULL DEFAULT '09:00',
    is_follow_the_sun       BOOLEAN DEFAULT false,
    linked_roster_id        UUID REFERENCES rosters(id),
    escalation_policy_id    UUID REFERENCES escalation_policies(id),
    start_date              DATE NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE roster_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    position    INTEGER NOT NULL,
    UNIQUE(roster_id, user_id),
    UNIQUE(roster_id, position)
);

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

CREATE TABLE slack_message_mappings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL,
    message_ts  TEXT NOT NULL,
    thread_ts   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
