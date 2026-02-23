package pat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// Store handles database operations for personal access tokens.
type Store struct {
	dbtx db.DBTX
}

// NewStore creates a PAT store with the given connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{dbtx: dbtx}
}

// Create inserts a new personal access token.
func (s *Store) Create(ctx context.Context, userID uuid.UUID, name, tokenHash, prefix string, expiresAt *time.Time) (*Token, error) {
	row := s.dbtx.QueryRow(ctx,
		`INSERT INTO personal_access_tokens (user_id, name, token_hash, prefix, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, name, prefix, expires_at, last_used_at, created_at`,
		userID, name, tokenHash, prefix, expiresAt,
	)

	var t Token
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting token: %w", err)
	}
	return &t, nil
}

// ListByUser returns all tokens for a user.
func (s *Store) ListByUser(ctx context.Context, userID uuid.UUID) ([]Token, error) {
	rows, err := s.dbtx.Query(ctx,
		`SELECT id, user_id, name, prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tokens: %w", err)
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// GetByID returns a token by ID, scoped to a user.
func (s *Store) GetByID(ctx context.Context, id, userID uuid.UUID) (*Token, error) {
	row := s.dbtx.QueryRow(ctx,
		`SELECT id, user_id, name, prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE id = $1 AND user_id = $2`,
		id, userID,
	)

	var t Token
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Prefix, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting token: %w", err)
	}
	return &t, nil
}

// Delete removes a token by ID, scoped to a user.
func (s *Store) Delete(ctx context.Context, id, userID uuid.UUID) error {
	result, err := s.dbtx.Exec(ctx,
		`DELETE FROM personal_access_tokens WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found")
	}
	return nil
}

// FindByPrefix returns the token hash and user info for a given prefix.
// Used by the auth middleware.
func (s *Store) FindByPrefix(ctx context.Context, prefix string) (tokenHash string, userID uuid.UUID, expiresAt *time.Time, err error) {
	row := s.dbtx.QueryRow(ctx,
		`SELECT token_hash, user_id, expires_at
		 FROM personal_access_tokens
		 WHERE prefix = $1`,
		prefix,
	)
	err = row.Scan(&tokenHash, &userID, &expiresAt)
	if err != nil {
		return "", uuid.Nil, nil, fmt.Errorf("finding token by prefix: %w", err)
	}
	return tokenHash, userID, expiresAt, nil
}

// UpdateLastUsed sets last_used_at to now.
func (s *Store) UpdateLastUsed(ctx context.Context, prefix string) {
	_, _ = s.dbtx.Exec(ctx,
		`UPDATE personal_access_tokens SET last_used_at = now() WHERE prefix = $1`,
		prefix,
	)
}
