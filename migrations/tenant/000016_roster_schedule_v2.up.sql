-- Roster v2: explicit weekly schedule replaces calculated rotation.

-- 1. Add new columns to rosters.
ALTER TABLE rosters
    ADD COLUMN schedule_weeks_ahead INTEGER NOT NULL DEFAULT 12,
    ADD COLUMN max_consecutive_weeks INTEGER NOT NULL DEFAULT 2,
    ADD COLUMN handoff_day INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN active_hours_start TIME,
    ADD COLUMN active_hours_end TIME,
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true;

-- 2. Backfill is_active from end_date (active if no end_date or end_date >= today).
UPDATE rosters SET is_active = (end_date IS NULL OR end_date >= CURRENT_DATE);

-- 3. Modify roster_members: drop position, add tracking columns.
ALTER TABLE roster_members
    DROP CONSTRAINT IF EXISTS roster_members_roster_id_position_key,
    DROP COLUMN IF EXISTS position,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_roster_members_active ON roster_members(roster_id) WHERE is_active = true;

-- 4. Create roster_schedule table.
CREATE TABLE roster_schedule (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roster_id         UUID NOT NULL REFERENCES rosters(id) ON DELETE CASCADE,
    week_start        DATE NOT NULL,
    week_end          DATE NOT NULL,
    primary_user_id   UUID REFERENCES users(id),
    secondary_user_id UUID REFERENCES users(id),
    is_locked         BOOLEAN NOT NULL DEFAULT false,
    generated         BOOLEAN NOT NULL DEFAULT true,
    notes             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(roster_id, week_start),
    CHECK (primary_user_id IS DISTINCT FROM secondary_user_id),
    CHECK (week_end > week_start)
);

CREATE INDEX idx_roster_schedule_roster ON roster_schedule(roster_id, week_start);
CREATE INDEX idx_roster_schedule_current ON roster_schedule(roster_id, week_start, week_end);

-- 5. Drop old rotation columns from rosters (no longer needed).
ALTER TABLE rosters
    DROP COLUMN IF EXISTS rotation_type,
    DROP COLUMN IF EXISTS rotation_length,
    DROP COLUMN IF EXISTS start_date;
