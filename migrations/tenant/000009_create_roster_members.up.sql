CREATE TABLE roster_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id   UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    position    INTEGER NOT NULL,
    UNIQUE(roster_id, user_id),
    UNIQUE(roster_id, position)
);

CREATE INDEX idx_roster_members_roster ON roster_members(roster_id);
