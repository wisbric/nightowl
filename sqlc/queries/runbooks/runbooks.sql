-- name: GetRunbook :one
SELECT * FROM runbooks WHERE id = $1;

-- name: ListRunbooks :many
SELECT * FROM runbooks ORDER BY title;

-- name: ListRunbookTemplates :many
SELECT * FROM runbooks WHERE is_template = true ORDER BY title;

-- name: CreateRunbook :one
INSERT INTO runbooks (title, content, category, is_template, tags, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateRunbook :one
UPDATE runbooks
SET title = $2, content = $3, category = $4, tags = $5, updated_at = now()
WHERE id = $1
RETURNING *;
