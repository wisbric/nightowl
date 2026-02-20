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
