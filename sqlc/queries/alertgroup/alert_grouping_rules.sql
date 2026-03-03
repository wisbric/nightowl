-- name: CreateAlertGroupingRule :one
INSERT INTO alert_grouping_rules (name, description, position, is_enabled, matchers, group_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAlertGroupingRule :one
SELECT * FROM alert_grouping_rules WHERE id = $1;

-- name: ListAlertGroupingRules :many
SELECT * FROM alert_grouping_rules ORDER BY position, name;

-- name: ListEnabledAlertGroupingRules :many
SELECT * FROM alert_grouping_rules WHERE is_enabled = true ORDER BY position, name;

-- name: UpdateAlertGroupingRule :one
UPDATE alert_grouping_rules
SET name = $2, description = $3, position = $4, is_enabled = $5, matchers = $6, group_by = $7, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAlertGroupingRule :exec
DELETE FROM alert_grouping_rules WHERE id = $1;
