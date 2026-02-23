package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
)

// PATPrefix identifies personal access tokens.
const PATPrefix = "nwl_pat_"

// MethodPAT indicates authentication via personal access token.
const MethodPAT = "pat"

// PATAuthResult holds resolved identity data from a PAT lookup.
type PATAuthResult struct {
	UserID     uuid.UUID
	Email      string
	DisplayName string
	Role       string
	TenantSlug string
	TenantID   uuid.UUID
}

// PATAuthenticator validates personal access tokens across tenant schemas.
type PATAuthenticator struct {
	pool *pgxpool.Pool
}

// NewPATAuthenticator creates a PAT authenticator.
func NewPATAuthenticator(pool *pgxpool.Pool) *PATAuthenticator {
	return &PATAuthenticator{pool: pool}
}

// Authenticate validates a raw PAT string by looking up its prefix across tenants,
// verifying the hash, and checking expiry. Returns the resolved identity.
func (a *PATAuthenticator) Authenticate(ctx context.Context, rawToken string) (*PATAuthResult, error) {
	if len(rawToken) < len(PATPrefix)+8 {
		return nil, fmt.Errorf("token too short")
	}

	prefix := rawToken[:len(PATPrefix)+8]
	expectedHash := hashPAT(rawToken)

	q := db.New(a.pool)
	tenants, err := q.ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}

	for _, t := range tenants {
		conn, err := a.pool.Acquire(ctx)
		if err != nil {
			return nil, fmt.Errorf("acquiring connection: %w", err)
		}

		schema := fmt.Sprintf("tenant_%s", t.Slug)
		_, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		if err != nil {
			conn.Release()
			continue
		}

		var tokenHash string
		var userID uuid.UUID
		var expiresAt *time.Time
		err = conn.QueryRow(ctx,
			"SELECT token_hash, user_id, expires_at FROM personal_access_tokens WHERE prefix = $1",
			prefix,
		).Scan(&tokenHash, &userID, &expiresAt)

		if err != nil {
			conn.Release()
			continue
		}

		// Verify hash.
		if tokenHash != expectedHash {
			conn.Release()
			return nil, fmt.Errorf("invalid token")
		}

		// Check expiry.
		if expiresAt != nil && expiresAt.Before(time.Now()) {
			conn.Release()
			return nil, fmt.Errorf("token expired at %s", expiresAt)
		}

		// Look up user.
		var email, displayName, role string
		err = conn.QueryRow(ctx,
			"SELECT email, display_name, role FROM users WHERE id = $1 AND is_active = true",
			userID,
		).Scan(&email, &displayName, &role)
		conn.Release()

		if err != nil {
			return nil, fmt.Errorf("looking up user for PAT: %w", err)
		}

		// Update last_used_at asynchronously.
		go func() {
			c, err := a.pool.Acquire(context.Background())
			if err != nil {
				return
			}
			defer c.Release()
			_, _ = c.Exec(context.Background(),
				fmt.Sprintf("SET search_path TO %s, public", schema))
			_, _ = c.Exec(context.Background(),
				"UPDATE personal_access_tokens SET last_used_at = now() WHERE prefix = $1", prefix)
		}()

		return &PATAuthResult{
			UserID:      userID,
			Email:       email,
			DisplayName: displayName,
			Role:        role,
			TenantSlug:  t.Slug,
			TenantID:    t.ID,
		}, nil
	}

	return nil, fmt.Errorf("token not found")
}

func hashPAT(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
