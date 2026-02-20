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
