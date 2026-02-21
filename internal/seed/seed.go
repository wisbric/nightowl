package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/runbook"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// DevAPIKey is the raw API key seeded for development/testing.
// It is only created by the seed command and should never be used in production.
const DevAPIKey = "ow_dev_seed_key_do_not_use_in_production"

// Run provisions the "acme" development tenant and populates it with sample
// users and services. It is idempotent: if the tenant already exists it logs
// a message and returns nil.
func Run(ctx context.Context, pool *pgxpool.Pool, databaseURL, migrationsDir string, logger *slog.Logger) error {
	prov := &tenant.Provisioner{
		DB:            pool,
		DatabaseURL:   databaseURL,
		MigrationsDir: migrationsDir,
		Logger:        logger,
	}

	// Check if the tenant already exists.
	q := db.New(pool)
	if _, err := q.GetTenantBySlug(ctx, "acme"); err == nil {
		logger.Info("seed: tenant 'acme' already exists, skipping")
		return nil
	}

	info, err := prov.Provision(ctx, "Acme Corp", "acme", json.RawMessage(`{"timezone":"Europe/Berlin"}`))
	if err != nil {
		return fmt.Errorf("provisioning seed tenant: %w", err)
	}
	logger.Info("seed: provisioned tenant", "tenant_id", info.ID, "slug", info.Slug)

	// Acquire a connection scoped to the new tenant schema.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", info.Schema)); err != nil {
		return fmt.Errorf("setting search_path: %w", err)
	}

	tq := db.New(conn)

	// Create users.
	phone1 := "+4915112345678"
	user1, err := tq.CreateUser(ctx, db.CreateUserParams{
		ExternalID:  "oidc|alice",
		Email:       "alice@acme.example.com",
		DisplayName: "Alice Engineer",
		Timezone:    "Europe/Berlin",
		Phone:       &phone1,
		Role:        "admin",
	})
	if err != nil {
		return fmt.Errorf("creating user alice: %w", err)
	}
	logger.Info("seed: created user", "user", user1.DisplayName, "id", user1.ID)

	phone2 := "+14155551234"
	user2, err := tq.CreateUser(ctx, db.CreateUserParams{
		ExternalID:  "oidc|bob",
		Email:       "bob@acme.example.com",
		DisplayName: "Bob SRE",
		Timezone:    "America/New_York",
		Phone:       &phone2,
		Role:        "engineer",
	})
	if err != nil {
		return fmt.Errorf("creating user bob: %w", err)
	}
	logger.Info("seed: created user", "user", user2.DisplayName, "id", user2.ID)

	// Create services.
	cluster := "prod-eu-1"
	ns1 := "payments"
	desc1 := "Payment processing microservice"
	tier1 := "critical"
	svc1, err := tq.CreateService(ctx, db.CreateServiceParams{
		Name:        "payment-service",
		Cluster:     &cluster,
		Namespace:   &ns1,
		Description: &desc1,
		OwnerID:     pgtype.UUID{Bytes: user1.ID, Valid: true},
		Tier:        &tier1,
		Metadata:    []byte(`{"team":"platform","language":"go"}`),
	})
	if err != nil {
		return fmt.Errorf("creating service payment-service: %w", err)
	}
	logger.Info("seed: created service", "service", svc1.Name, "id", svc1.ID)

	ns2 := "monitoring"
	desc2 := "Kubernetes ingress controller"
	tier2 := "standard"
	svc2, err := tq.CreateService(ctx, db.CreateServiceParams{
		Name:        "ingress-nginx",
		Cluster:     &cluster,
		Namespace:   &ns2,
		Description: &desc2,
		OwnerID:     pgtype.UUID{Bytes: user2.ID, Valid: true},
		Tier:        &tier2,
		Metadata:    []byte(`{"team":"infra","language":"helm"}`),
	})
	if err != nil {
		return fmt.Errorf("creating service ingress-nginx: %w", err)
	}
	logger.Info("seed: created service", "service", svc2.Name, "id", svc2.ID)

	// Seed runbook templates.
	rbStore := runbook.NewStore(conn)
	templates := runbook.TemplateRunbooks()
	for _, tmpl := range templates {
		if _, err := rbStore.Create(ctx, runbook.CreateParams{
			Title:      tmpl.Title,
			Content:    tmpl.Content,
			Category:   tmpl.Category,
			IsTemplate: true,
			Tags:       tmpl.Tags,
		}); err != nil {
			return fmt.Errorf("seeding runbook template %q: %w", tmpl.Title, err)
		}
	}
	logger.Info("seed: created runbook templates", "count", len(templates))

	// Create a development API key (uses the global queries, not tenant-scoped).
	apiKeyHash := auth.HashAPIKey(DevAPIKey)
	apiKey, err := q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		TenantID:    info.ID,
		KeyHash:     apiKeyHash,
		KeyPrefix:   DevAPIKey[:16],
		Description: "Development seed API key",
		Role:        "admin",
		Scopes:      []string{"*"},
	})
	if err != nil {
		return fmt.Errorf("creating seed API key: %w", err)
	}
	logger.Info("seed: created API key",
		"id", apiKey.ID,
		"prefix", apiKey.KeyPrefix,
		"raw_key", DevAPIKey,
	)

	logger.Info("seed: completed successfully",
		"tenant", info.Slug,
		"users", 2,
		"services", 2,
		"api_keys", 1,
	)
	return nil
}
