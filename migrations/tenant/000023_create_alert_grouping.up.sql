-- Alert grouping rules (configuration)
CREATE TABLE alert_grouping_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    position    INTEGER NOT NULL DEFAULT 0,
    is_enabled  BOOLEAN NOT NULL DEFAULT true,
    matchers    JSONB NOT NULL DEFAULT '[]',
    group_by    TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Computed alert groups
CREATE TABLE alert_groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id          UUID NOT NULL REFERENCES alert_grouping_rules(id) ON DELETE CASCADE,
    group_key_hash   TEXT NOT NULL,
    group_key_labels JSONB NOT NULL DEFAULT '{}',
    status           TEXT NOT NULL DEFAULT 'active',
    title            TEXT NOT NULL,
    alert_count      INTEGER NOT NULL DEFAULT 0,
    max_severity     TEXT NOT NULL DEFAULT 'info',
    first_alert_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_alert_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(rule_id, group_key_hash)
);

CREATE INDEX idx_alert_groups_status ON alert_groups(status);

-- FK on alerts table
ALTER TABLE alerts ADD COLUMN alert_group_id UUID REFERENCES alert_groups(id);
CREATE INDEX idx_alerts_alert_group_id ON alerts(alert_group_id);
