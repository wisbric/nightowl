-- name: GetIncident :one
SELECT * FROM incidents WHERE id = $1;

-- name: ListIncidents :many
SELECT * FROM incidents
WHERE merged_into_id IS NULL
ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: SearchIncidents :many
SELECT id, title, severity, symptoms, solution, runbook_id,
       ts_rank(search_vector, plainto_tsquery('english', $1)) AS rank
FROM incidents
WHERE search_vector @@ plainto_tsquery('english', $1)
  AND merged_into_id IS NULL
ORDER BY rank DESC
LIMIT $2;

-- name: GetIncidentByFingerprint :one
SELECT * FROM incidents
WHERE $1 = ANY(fingerprints)
  AND merged_into_id IS NULL
LIMIT 1;

-- name: CreateIncident :one
INSERT INTO incidents (
    title, fingerprints, severity, category, tags,
    services, clusters, namespaces, symptoms, error_patterns,
    root_cause, solution, runbook_id, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: UpdateIncident :one
UPDATE incidents
SET title = $2, severity = $3, category = $4, tags = $5,
    symptoms = $6, root_cause = $7, solution = $8,
    runbook_id = $9, updated_at = now()
WHERE id = $1
RETURNING *;
