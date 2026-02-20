-- name: GetActiveOverride :one
SELECT * FROM roster_overrides
WHERE roster_id = $1
  AND $2::timestamptz BETWEEN start_at AND end_at
LIMIT 1;

-- name: ListRosterOverrides :many
SELECT * FROM roster_overrides
WHERE roster_id = $1
ORDER BY start_at;

-- name: CreateRosterOverride :one
INSERT INTO roster_overrides (roster_id, user_id, start_at, end_at, reason, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteRosterOverride :exec
DELETE FROM roster_overrides WHERE id = $1;
