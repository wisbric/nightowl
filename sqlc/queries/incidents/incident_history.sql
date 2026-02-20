-- name: ListIncidentHistory :many
SELECT * FROM incident_history
WHERE incident_id = $1
ORDER BY created_at DESC;

-- name: CreateIncidentHistory :one
INSERT INTO incident_history (incident_id, changed_by, change_type, diff)
VALUES ($1, $2, $3, $4)
RETURNING *;
