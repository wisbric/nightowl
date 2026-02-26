package tenant

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
	coretenant "github.com/wisbric/core/pkg/tenant"
)

// sqlcStore implements coretenant.TenantStore using nightowl's sqlc queries.
// It captures the config to pass to CreateTenant.
type sqlcStore struct {
	pool   *pgxpool.Pool
	config json.RawMessage
}

func (s *sqlcStore) CreateTenant(ctx context.Context, name, slug string) (uuid.UUID, error) {
	cfg := s.config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}
	q := db.New(s.pool)
	t, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name:   name,
		Slug:   slug,
		Config: cfg,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return t.ID, nil
}

func (s *sqlcStore) DeleteTenant(ctx context.Context, id uuid.UUID) error {
	q := db.New(s.pool)
	return q.DeleteTenant(ctx, id)
}

// Provisioner handles creating and destroying tenant schemas.
// It wraps the core provisioner with nightowl-specific sqlc operations.
type Provisioner struct {
	DB            *pgxpool.Pool
	DatabaseURL   string
	MigrationsDir string
	Logger        *slog.Logger
}

// Provision creates a new tenant with optional config.
func (p *Provisioner) Provision(ctx context.Context, name, slug string, config json.RawMessage) (*Info, error) {
	store := &sqlcStore{pool: p.DB, config: config}
	cp := &coretenant.Provisioner{
		DB:            p.DB,
		Store:         store,
		DatabaseURL:   p.DatabaseURL,
		MigrationsDir: p.MigrationsDir,
		Logger:        p.Logger,
	}
	return cp.Provision(ctx, name, slug)
}

// Deprovision drops the tenant schema and removes the global record.
func (p *Provisioner) Deprovision(ctx context.Context, slug string) error {
	store := &sqlcStore{pool: p.DB}
	cp := &coretenant.Provisioner{
		DB:            p.DB,
		Store:         store,
		DatabaseURL:   p.DatabaseURL,
		MigrationsDir: p.MigrationsDir,
		Logger:        p.Logger,
	}
	return cp.Deprovision(ctx, slug)
}
