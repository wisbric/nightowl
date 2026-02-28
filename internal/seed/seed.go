package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang.org/x/crypto/bcrypt"

	"github.com/wisbric/core/pkg/auth"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// DevAPIKey is the raw API key seeded for development/testing.
// It is only created by the seed command and should never be used in production.
const DevAPIKey = "ow_dev_seed_key_do_not_use_in_production"

// Run provisions the "acme" development tenant and populates it with sample
// users and services. It is idempotent: re-running will ensure all resources
// exist without duplicating them.
func Run(ctx context.Context, pool *pgxpool.Pool, databaseURL, migrationsDir string, logger *slog.Logger, adminPassword string) error {
	prov := &tenant.Provisioner{
		DB:            pool,
		DatabaseURL:   databaseURL,
		MigrationsDir: migrationsDir,
		Logger:        logger,
	}

	q := db.New(pool)

	// Check if the tenant already exists.
	existing, err := q.GetTenantBySlug(ctx, "acme")
	if err == nil {
		logger.Info("seed: tenant 'acme' already exists, ensuring local admin")
		// Always ensure local admin exists even when tenant was already created.
		if err := ensureLocalAdmin(ctx, pool, existing.ID, logger, adminPassword); err != nil {
			return err
		}
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

	// Set password for admin user (bcrypt hash of "admin").
	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}
	if _, err := conn.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", string(adminHash), user1.ID); err != nil {
		return fmt.Errorf("setting admin password: %w", err)
	}
	logger.Info("seed: set password for admin user", "email", user1.Email)

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

	// Create local admin for this tenant.
	if err := ensureLocalAdmin(ctx, pool, info.ID, logger, adminPassword); err != nil {
		return err
	}

	logger.Info("seed: completed successfully",
		"tenant", info.Slug,
		"users", 2,
		"services", 2,
		"api_keys", 1,
	)
	return nil
}

// ensureLocalAdmin creates or updates the local admin account.
// When adminPassword is explicitly set, the password is always updated (ON CONFLICT DO UPDATE).
// When adminPassword is empty, the default is used and existing admins are left untouched.
func ensureLocalAdmin(ctx context.Context, pool *pgxpool.Pool, tenantID [16]byte, logger *slog.Logger, adminPassword string) error {
	localAdminPassword := adminPassword
	if localAdminPassword == "" {
		localAdminPassword = "nightowl-admin"
	}
	adminPasswordHash, err := bcrypt.GenerateFromPassword([]byte(localAdminPassword), 12)
	if err != nil {
		return fmt.Errorf("hashing local admin password: %w", err)
	}

	var query string
	if adminPassword != "" {
		// Explicit password: upsert so existing admin gets the configured password.
		query = "INSERT INTO public.local_admins (tenant_id, username, password_hash, must_change) VALUES ($1, 'admin', $2, true) ON CONFLICT (tenant_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, must_change = true"
	} else {
		query = "INSERT INTO public.local_admins (tenant_id, username, password_hash, must_change) VALUES ($1, 'admin', $2, true) ON CONFLICT (tenant_id) DO NOTHING"
	}

	tag, err := pool.Exec(ctx, query, tenantID, string(adminPasswordHash))
	if err != nil {
		return fmt.Errorf("creating local admin: %w", err)
	}

	if tag.RowsAffected() > 0 {
		logger.Info("seed: created/updated local admin", "username", "admin")
	} else {
		logger.Info("seed: local admin already exists")
	}
	return nil
}
