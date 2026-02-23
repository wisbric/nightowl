package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service encapsulates API key business logic.
type Service struct {
	store  *Store
	logger *slog.Logger
}

// NewService creates an API key Service backed by the given global pool.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{
		store:  NewStore(pool),
		logger: logger,
	}
}

// List returns all API keys for the given tenant.
func (s *Service) List(ctx context.Context, tenantID uuid.UUID) ([]Response, error) {
	rows, err := s.store.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}

	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToResponse())
	}
	return items, nil
}

// Create generates a new API key, stores its hash, and returns the raw key once.
func (s *Service) Create(ctx context.Context, tenantID uuid.UUID, req CreateRequest) (CreateResponse, error) {
	raw, hash, prefix := generateAPIKey()

	row, err := s.store.Create(ctx, CreateParams{
		TenantID:    tenantID,
		KeyHash:     hash,
		KeyPrefix:   prefix,
		Description: req.Description,
		Role:        req.Role,
		Scopes:      []string{},
		ExpiresAt:   pgtype.Timestamptz{},
	})
	if err != nil {
		return CreateResponse{}, fmt.Errorf("creating api key: %w", err)
	}

	return CreateResponse{
		Response: row.ToResponse(),
		RawKey:   raw,
	}, nil
}

// Delete permanently removes an API key.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	return nil
}

// generateAPIKey creates a random API key with prefix "ow_", its SHA-256 hash,
// and a short prefix for display.
func generateAPIKey() (raw, hash, prefix string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	raw = fmt.Sprintf("ow_%x", b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	prefix = raw[:10]
	return
}
