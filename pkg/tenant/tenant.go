package tenant

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	coretenant "github.com/wisbric/core/pkg/tenant"
)

// Info is an alias for the core tenant Info type.
type Info = coretenant.Info

// SchemaName returns the PostgreSQL schema name for a tenant slug.
func SchemaName(slug string) string {
	return coretenant.SchemaName(slug)
}

// NewContext stores tenant info in the context.
func NewContext(ctx context.Context, info *Info) context.Context {
	return coretenant.NewContext(ctx, info)
}

// FromContext extracts the tenant info from the context.
// Returns nil if no tenant is set.
func FromContext(ctx context.Context) *Info {
	return coretenant.FromContext(ctx)
}

// NewConnContext stores a tenant-scoped database connection in the context.
func NewConnContext(ctx context.Context, conn *pgxpool.Conn) context.Context {
	return coretenant.NewConnContext(ctx, conn)
}

// ConnFromContext extracts the tenant-scoped database connection from the context.
// Returns nil if no connection is set.
func ConnFromContext(ctx context.Context) *pgxpool.Conn {
	return coretenant.ConnFromContext(ctx)
}
