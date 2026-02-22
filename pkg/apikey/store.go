package apikey

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const apiKeyColumns = `id, tenant_id, key_hash, key_prefix, description, role, scopes, last_used, expires_at, created_at`

// Store provides database operations for API keys using the global pool.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates an API key Store backed by the given global connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateParams holds parameters for creating an API key.
type CreateParams struct {
	TenantID    uuid.UUID
	KeyHash     string
	KeyPrefix   string
	Description string
	Role        string
	Scopes      []string
	ExpiresAt   pgtype.Timestamptz
}

// scanApiKeyRow scans a pgx.Row into an ApiKeyRow.
func scanApiKeyRow(row pgx.Row) (ApiKeyRow, error) {
	var r ApiKeyRow
	err := row.Scan(
		&r.ID, &r.TenantID, &r.KeyHash, &r.KeyPrefix, &r.Description,
		&r.Role, &r.Scopes, &r.LastUsed, &r.ExpiresAt, &r.CreatedAt,
	)
	return r, err
}

// scanApiKeyRows scans multiple rows into ApiKeyRow slices.
func scanApiKeyRows(rows pgx.Rows) ([]ApiKeyRow, error) {
	defer rows.Close()
	var items []ApiKeyRow
	for rows.Next() {
		var r ApiKeyRow
		if err := rows.Scan(
			&r.ID, &r.TenantID, &r.KeyHash, &r.KeyPrefix, &r.Description,
			&r.Role, &r.Scopes, &r.LastUsed, &r.ExpiresAt, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning api key row: %w", err)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating api key rows: %w", err)
	}
	return items, nil
}

// List returns all API keys for the given tenant.
func (s *Store) List(ctx context.Context, tenantID uuid.UUID) ([]ApiKeyRow, error) {
	query := `SELECT ` + apiKeyColumns + ` FROM public.api_keys WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	return scanApiKeyRows(rows)
}

// Create inserts a new API key and returns the created row.
func (s *Store) Create(ctx context.Context, p CreateParams) (ApiKeyRow, error) {
	query := `INSERT INTO public.api_keys (tenant_id, key_hash, key_prefix, description, role, scopes, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING ` + apiKeyColumns

	row := s.pool.QueryRow(ctx, query,
		p.TenantID, p.KeyHash, p.KeyPrefix, p.Description, p.Role, p.Scopes, p.ExpiresAt,
	)
	return scanApiKeyRow(row)
}

// Delete permanently removes an API key by ID.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM public.api_keys WHERE id = $1`
	tag, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
