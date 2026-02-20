-- name: ListEscalationEvents :many
SELECT * FROM escalation_events
WHERE alert_id = $1
ORDER BY created_at;

-- name: CreateEscalationEvent :one
INSERT INTO escalation_events (alert_id, policy_id, tier, action, target_user_id, notify_method, notify_result)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
