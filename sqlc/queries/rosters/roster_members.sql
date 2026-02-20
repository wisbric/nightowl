-- name: ListRosterMembers :many
SELECT * FROM roster_members
WHERE roster_id = $1
ORDER BY position;

-- name: CreateRosterMember :one
INSERT INTO roster_members (roster_id, user_id, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteRosterMember :exec
DELETE FROM roster_members WHERE id = $1;

-- name: CountRosterMembers :one
SELECT count(*) FROM roster_members WHERE roster_id = $1;
