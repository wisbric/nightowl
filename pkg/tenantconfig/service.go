package tenantconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
)

// Service encapsulates business logic for tenant configuration.
type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewService creates a tenant config Service backed by the global pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

// Get returns the current tenant configuration.
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID) (*ConfigResponse, error) {
	q := db.New(s.pool)
	t, err := q.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("fetching tenant: %w", err)
	}

	var cfg TenantConfig
	if len(t.Config) > 0 {
		if err := json.Unmarshal(t.Config, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling config: %w", err)
		}
	}

	provider := cfg.MessagingProvider
	if provider == "" {
		provider = "none"
	}

	return &ConfigResponse{
		MessagingProvider:          provider,
		SlackWorkspaceURL:          cfg.SlackWorkspaceURL,
		SlackChannel:               cfg.SlackChannel,
		MattermostURL:              cfg.MattermostURL,
		MattermostDefaultChannelID: cfg.MattermostDefaultChannelID,
		TwilioSID:                  cfg.TwilioSID,
		TwilioPhoneNumber:          cfg.TwilioPhoneNumber,
		DefaultTimezone:            cfg.DefaultTimezone,
		BookOwlAPIURL:              cfg.BookOwlAPIURL,
		BookOwlAPIKey:              cfg.BookOwlAPIKey,
		UpdatedAt:                  t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// Update replaces the tenant configuration with the given values.
func (s *Service) Update(ctx context.Context, tenantID uuid.UUID, req UpdateRequest) (*ConfigResponse, error) {
	q := db.New(s.pool)

	// Fetch current tenant to preserve the name.
	t, err := q.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("fetching tenant: %w", err)
	}

	provider := req.MessagingProvider
	if provider == "" {
		provider = "none"
	}

	cfg := TenantConfig{
		MessagingProvider:          provider,
		SlackWorkspaceURL:          req.SlackWorkspaceURL,
		SlackChannel:               req.SlackChannel,
		MattermostURL:              req.MattermostURL,
		MattermostDefaultChannelID: req.MattermostDefaultChannelID,
		TwilioSID:                  req.TwilioSID,
		TwilioPhoneNumber:          req.TwilioPhoneNumber,
		DefaultTimezone:            req.DefaultTimezone,
		BookOwlAPIURL:              req.BookOwlAPIURL,
		BookOwlAPIKey:              req.BookOwlAPIKey,
	}

	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshalling config: %w", err)
	}

	updated, err := q.UpdateTenant(ctx, db.UpdateTenantParams{
		ID:     tenantID,
		Name:   t.Name,
		Config: configBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("updating tenant: %w", err)
	}

	return &ConfigResponse{
		MessagingProvider:          provider,
		SlackWorkspaceURL:          cfg.SlackWorkspaceURL,
		SlackChannel:               cfg.SlackChannel,
		MattermostURL:              cfg.MattermostURL,
		MattermostDefaultChannelID: cfg.MattermostDefaultChannelID,
		TwilioSID:                  cfg.TwilioSID,
		TwilioPhoneNumber:          cfg.TwilioPhoneNumber,
		DefaultTimezone:            cfg.DefaultTimezone,
		BookOwlAPIURL:              cfg.BookOwlAPIURL,
		BookOwlAPIKey:              cfg.BookOwlAPIKey,
		UpdatedAt:                  updated.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
