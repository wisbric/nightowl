-- name: GetEscalationPolicy :one
SELECT * FROM escalation_policies WHERE id = $1;

-- name: ListEscalationPolicies :many
SELECT * FROM escalation_policies ORDER BY name;

-- name: CreateEscalationPolicy :one
INSERT INTO escalation_policies (name, description, tiers, repeat_count)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateEscalationPolicy :one
UPDATE escalation_policies
SET name = $2, description = $3, tiers = $4, repeat_count = $5, updated_at = now()
WHERE id = $1
RETURNING *;
