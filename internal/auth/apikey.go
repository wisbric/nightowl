package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// APIKeyAuthenticator validates API keys against the database.
type APIKeyAuthenticator struct {
	DB db.DBTX
}

// APIKeyResult holds the resolved identity data from an API key lookup.
type APIKeyResult struct {
	APIKeyID  uuid.UUID
	TenantID  uuid.UUID
	KeyPrefix string
	Role      string
	Scopes    []string
}

// Authenticate hashes the raw key, looks it up in public.api_keys, and
// validates expiration.
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, rawKey string) (*APIKeyResult, error) {
	if rawKey == "" {
		return nil, fmt.Errorf("empty API key")
	}

	hash := HashAPIKey(rawKey)

	q := db.New(a.DB)
	key, err := q.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("looking up API key: %w", err)
	}

	// Check expiration.
	if key.ExpiresAt.Valid && key.ExpiresAt.Time.Before(time.Now()) {
		return nil, fmt.Errorf("API key expired at %s", key.ExpiresAt.Time)
	}

	// Update last_used asynchronously — fire and forget.
	go func() {
		_ = q.UpdateAPIKeyLastUsed(context.Background(), key.ID)
	}()

	role := key.Role
	if !IsValidRole(role) {
		role = RoleEngineer
	}

	return &APIKeyResult{
		APIKeyID:  key.ID,
		TenantID:  key.TenantID,
		KeyPrefix: key.KeyPrefix,
		Role:      role,
		Scopes:    key.Scopes,
	}, nil
}
