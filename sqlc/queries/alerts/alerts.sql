-- name: GetAlert :one
SELECT * FROM alerts WHERE id = $1;

-- name: GetAlertByFingerprint :one
SELECT * FROM alerts
WHERE fingerprint = $1 AND status != 'resolved'
ORDER BY last_fired_at DESC LIMIT 1;

-- name: ListAlerts :many
SELECT * FROM alerts ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CreateAlert :one
INSERT INTO alerts (
    fingerprint, status, severity, source, title, description,
    labels, annotations, service_id, escalation_policy_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: AcknowledgeAlert :one
UPDATE alerts
SET status = 'acknowledged', acknowledged_by = $2, acknowledged_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ResolveAlert :one
UPDATE alerts
SET status = 'resolved', resolved_by = $2, resolved_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: IncrementAlertOccurrence :exec
UPDATE alerts
SET occurrence_count = occurrence_count + 1, last_fired_at = now(), updated_at = now()
WHERE id = $1;
