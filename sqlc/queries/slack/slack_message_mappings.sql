-- name: GetSlackMappingByAlert :one
SELECT * FROM slack_message_mappings WHERE alert_id = $1 LIMIT 1;

-- name: GetSlackMappingByChannelTS :one
SELECT * FROM slack_message_mappings
WHERE channel_id = $1 AND message_ts = $2
LIMIT 1;

-- name: CreateSlackMapping :one
INSERT INTO slack_message_mappings (alert_id, incident_id, channel_id, message_ts, thread_ts)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
