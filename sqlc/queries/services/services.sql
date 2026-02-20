-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: ListServices :many
SELECT * FROM services ORDER BY name;

-- name: CreateService :one
INSERT INTO services (name, cluster, namespace, description, owner_id, tier, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateService :one
UPDATE services
SET name = $2, cluster = $3, namespace = $4, description = $5,
    owner_id = $6, tier = $7, metadata = $8, updated_at = now()
WHERE id = $1
RETURNING *;
