-- name: GetMessageMappingByAlert :one
SELECT * FROM message_mappings WHERE alert_id = $1 AND provider = $2 LIMIT 1;

-- name: GetMessageMappingByChannelMsg :one
SELECT * FROM message_mappings
WHERE channel_id = $1 AND message_id = $2
LIMIT 1;

-- name: CreateMessageMapping :one
INSERT INTO message_mappings (alert_id, incident_id, provider, channel_id, message_id, thread_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
