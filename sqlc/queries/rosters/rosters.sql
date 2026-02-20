-- name: GetRoster :one
SELECT * FROM rosters WHERE id = $1;

-- name: ListRosters :many
SELECT * FROM rosters ORDER BY name;

-- name: CreateRoster :one
INSERT INTO rosters (
    name, description, timezone, rotation_type, rotation_length,
    handoff_time, is_follow_the_sun, linked_roster_id,
    escalation_policy_id, start_date
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateRoster :one
UPDATE rosters
SET name = $2, description = $3, timezone = $4, rotation_type = $5,
    rotation_length = $6, handoff_time = $7, updated_at = now()
WHERE id = $1
RETURNING *;
