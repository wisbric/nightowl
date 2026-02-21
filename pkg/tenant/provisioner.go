package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/internal/platform"
)

// slugPattern restricts tenant slugs to safe identifiers for schema names.
var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,62}$`)

// Provisioner handles creating and destroying tenant schemas.
type Provisioner struct {
	DB            *pgxpool.Pool
	DatabaseURL   string
	MigrationsDir string // path to tenant migration files
	Logger        *slog.Logger
}

// Provision creates a new tenant: inserts the global record, creates the
// PostgreSQL schema, and runs tenant migrations.
func (p *Provisioner) Provision(ctx context.Context, name, slug string, config json.RawMessage) (*Info, error) {
	if !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("invalid tenant slug %q: must match %s", slug, slugPattern.String())
	}

	if config == nil {
		config = json.RawMessage(`{}`)
	}

	q := db.New(p.DB)
	t, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name:   name,
		Slug:   slug,
		Config: config,
	})
	if err != nil {
		return nil, fmt.Errorf("inserting tenant record: %w", err)
	}

	schema := SchemaName(slug)

	// Create the tenant schema. The slug is validated above so this is safe.
	if _, err := p.DB.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema)); err != nil {
		// Best-effort cleanup of the tenant row.
		_ = q.DeleteTenant(ctx, t.ID)
		return nil, fmt.Errorf("creating schema %s: %w", schema, err)
	}

	// Run tenant migrations against the new schema.
	tenantURL, err := withSearchPath(p.DatabaseURL, schema)
	if err != nil {
		return nil, fmt.Errorf("building tenant database URL: %w", err)
	}

	if err := platform.RunTenantMigrations(tenantURL, p.MigrationsDir); err != nil {
		// Best-effort cleanup.
		_, _ = p.DB.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
		_ = q.DeleteTenant(ctx, t.ID)
		return nil, fmt.Errorf("running tenant migrations: %w", err)
	}

	p.Logger.Info("tenant provisioned",
		"tenant_id", t.ID,
		"slug", slug,
		"schema", schema,
	)

	return &Info{
		ID:     t.ID,
		Name:   t.Name,
		Slug:   t.Slug,
		Schema: schema,
	}, nil
}

// Deprovision drops the tenant schema and removes the global record.
func (p *Provisioner) Deprovision(ctx context.Context, slug string) error {
	schema := SchemaName(slug)

	if _, err := p.DB.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
		return fmt.Errorf("dropping schema %s: %w", schema, err)
	}

	q := db.New(p.DB)
	t, err := q.GetTenantBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("looking up tenant %q: %w", slug, err)
	}

	if err := q.DeleteTenant(ctx, t.ID); err != nil {
		return fmt.Errorf("deleting tenant record: %w", err)
	}

	p.Logger.Info("tenant deprovisioned", "slug", slug, "schema", schema)
	return nil
}

// withSearchPath appends search_path=<schema> to a PostgreSQL connection URL.
func withSearchPath(databaseURL, schema string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("parsing database URL: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
