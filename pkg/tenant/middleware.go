package tenant

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
	coretenant "github.com/wisbric/core/pkg/tenant"
)

// Ensure sqlcLookup satisfies the interface at compile time.
var _ coretenant.TenantLookup = (*sqlcLookup)(nil)

// Resolver identifies the tenant for the current request.
type Resolver = coretenant.Resolver

// HeaderResolver resolves the tenant from the X-Tenant-Slug header.
// Intended for development and testing; production should use JWT/API-key resolvers.
type HeaderResolver struct{}

func (HeaderResolver) Resolve(r *http.Request) (string, error) {
	slug := r.Header.Get("X-Tenant-Slug")
	if slug == "" {
		return "", fmt.Errorf("missing X-Tenant-Slug header")
	}
	return slug, nil
}

// sqlcLookup implements core tenant.TenantLookup using nightowl's sqlc queries.
type sqlcLookup struct {
	pool *pgxpool.Pool
}

func (l *sqlcLookup) LookupBySlug(ctx context.Context, slug string) (uuid.UUID, string, error) {
	q := db.New(l.pool)
	t, err := q.GetTenantBySlug(ctx, slug)
	if err != nil {
		return uuid.Nil, "", err
	}
	return t.ID, t.Name, nil
}

// Middleware returns the core tenant middleware using nightowl's sqlc-based
// tenant lookup instead of raw SQL.
func Middleware(pool *pgxpool.Pool, resolver Resolver, logger *slog.Logger) func(http.Handler) http.Handler {
	return coretenant.MiddlewareWithLookup(pool, &sqlcLookup{pool: pool}, resolver, logger)
}
