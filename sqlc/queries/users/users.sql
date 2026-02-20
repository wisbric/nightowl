-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByExternalID :one
SELECT * FROM users WHERE external_id = $1;

-- name: ListUsers :many
SELECT * FROM users WHERE is_active = true ORDER BY display_name;

-- name: CreateUser :one
INSERT INTO users (external_id, email, display_name, timezone, phone, slack_user_id, role)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET email = $2, display_name = $3, timezone = $4, phone = $5,
    slack_user_id = $6, role = $7, updated_at = now()
WHERE id = $1
RETURNING *;
