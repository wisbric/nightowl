-- name: GetTenant :one
SELECT * FROM public.tenants WHERE id = $1;

-- name: GetTenantBySlug :one
SELECT * FROM public.tenants WHERE slug = $1;

-- name: ListTenants :many
SELECT * FROM public.tenants ORDER BY name;

-- name: CreateTenant :one
INSERT INTO public.tenants (name, slug, config)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateTenant :one
UPDATE public.tenants
SET name = $2, config = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTenant :exec
DELETE FROM public.tenants WHERE id = $1;
