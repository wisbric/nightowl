-- name: GetRoster :one
SELECT * FROM rosters WHERE id = $1;

-- name: ListRosters :many
SELECT * FROM rosters ORDER BY name;

-- name: CreateRoster :one
INSERT INTO rosters (
    name, description, timezone,
    handoff_time, handoff_day, schedule_weeks_ahead, max_consecutive_weeks,
    is_follow_the_sun, linked_roster_id,
    escalation_policy_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateRoster :one
UPDATE rosters
SET name = $2, description = $3, timezone = $4,
    handoff_time = $5, handoff_day = $6, updated_at = now()
WHERE id = $1
RETURNING *;
