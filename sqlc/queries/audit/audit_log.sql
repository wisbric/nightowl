-- name: ListAuditLog :many
SELECT * FROM audit_log
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAuditLogByResource :many
SELECT * FROM audit_log
WHERE resource = $1 AND resource_id = $2
ORDER BY created_at DESC;

-- name: CreateAuditLogEntry :one
INSERT INTO audit_log (user_id, api_key_id, action, resource, resource_id, detail, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;
