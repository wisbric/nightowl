-- Reverse roster v2 migration.

-- 1. Re-add dropped rotation columns.
ALTER TABLE rosters
    ADD COLUMN IF NOT EXISTS rotation_type TEXT NOT NULL DEFAULT 'weekly',
    ADD COLUMN IF NOT EXISTS rotation_length INTEGER NOT NULL DEFAULT 7,
    ADD COLUMN IF NOT EXISTS start_date DATE NOT NULL DEFAULT CURRENT_DATE;

-- 2. Drop roster v2 columns.
ALTER TABLE rosters
    DROP COLUMN IF EXISTS schedule_weeks_ahead,
    DROP COLUMN IF EXISTS max_consecutive_weeks,
    DROP COLUMN IF EXISTS handoff_day,
    DROP COLUMN IF EXISTS active_hours_start,
    DROP COLUMN IF EXISTS active_hours_end,
    DROP COLUMN IF EXISTS is_active;

-- 3. Drop schedule table.
DROP TABLE IF EXISTS roster_schedule;

-- 4. Drop new indexes and columns from roster_members.
DROP INDEX IF EXISTS idx_roster_members_active;
ALTER TABLE roster_members
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS joined_at,
    DROP COLUMN IF EXISTS left_at;

-- 5. Re-add position column.
ALTER TABLE roster_members
    ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE roster_members
    ADD CONSTRAINT roster_members_roster_id_position_key UNIQUE (roster_id, position);
