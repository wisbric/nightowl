-- name: GetAPIKeyByHash :one
SELECT * FROM public.api_keys WHERE key_hash = $1;

-- name: ListAPIKeysByTenant :many
SELECT * FROM public.api_keys WHERE tenant_id = $1 ORDER BY created_at DESC;

-- name: CreateAPIKey :one
INSERT INTO public.api_keys (tenant_id, key_hash, key_prefix, description, role, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE public.api_keys SET last_used = now() WHERE id = $1;

-- name: DeleteAPIKey :exec
DELETE FROM public.api_keys WHERE id = $1;
