package runbook

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for runbooks.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates a runbook Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

const runbookColumns = `id, title, content, category, is_template, tags, created_by, created_at, updated_at`

// scanRunbookRow scans a pgx.Row into a RunbookRow.
func scanRunbookRow(row pgx.Row) (RunbookRow, error) {
	var r RunbookRow
	err := row.Scan(
		&r.ID, &r.Title, &r.Content, &r.Category, &r.IsTemplate,
		&r.Tags, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// scanRunbookRows scans multiple rows into RunbookRow slices.
func scanRunbookRows(rows pgx.Rows) ([]RunbookRow, error) {
	defer rows.Close()
	var items []RunbookRow
	for rows.Next() {
		var r RunbookRow
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Category, &r.IsTemplate,
			&r.Tags, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning runbook row: %w", err)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating runbook rows: %w", err)
	}
	return items, nil
}

// Get returns a single runbook by ID.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (RunbookRow, error) {
	query := `SELECT ` + runbookColumns + ` FROM runbooks WHERE id = $1`
	row := s.dbtx.QueryRow(ctx, query, id)
	return scanRunbookRow(row)
}

// CreateParams holds parameters for creating a runbook.
type CreateParams struct {
	Title      string
	Content    string
	Category   *string
	IsTemplate bool
	Tags       []string
	CreatedBy  pgtype.UUID
}

// Create inserts a new runbook.
func (s *Store) Create(ctx context.Context, p CreateParams) (RunbookRow, error) {
	query := `INSERT INTO runbooks (title, content, category, is_template, tags, created_by)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING ` + runbookColumns
	row := s.dbtx.QueryRow(ctx, query,
		p.Title, p.Content, p.Category, p.IsTemplate, p.Tags, p.CreatedBy,
	)
	return scanRunbookRow(row)
}

// UpdateParams holds parameters for updating a runbook.
type UpdateParams struct {
	ID       uuid.UUID
	Title    string
	Content  string
	Category *string
	Tags     []string
}

// Update updates all editable fields and returns the updated row.
func (s *Store) Update(ctx context.Context, p UpdateParams) (RunbookRow, error) {
	query := `UPDATE runbooks
	SET title = $2, content = $3, category = $4, tags = $5, updated_at = now()
	WHERE id = $1
	RETURNING ` + runbookColumns
	row := s.dbtx.QueryRow(ctx, query,
		p.ID, p.Title, p.Content, p.Category, p.Tags,
	)
	return scanRunbookRow(row)
}

// Delete permanently removes a runbook.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM runbooks WHERE id = $1`
	tag, err := s.dbtx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting runbook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// List returns runbooks with optional category filter and pagination.
func (s *Store) List(ctx context.Context, filters ListFilters, limit, offset int) ([]RunbookRow, error) {
	where := "1=1"
	args := []any{}
	argN := 1

	if filters.Category != "" {
		where += fmt.Sprintf(" AND category = $%d", argN)
		args = append(args, filters.Category)
		argN++
	}

	query := fmt.Sprintf(
		`SELECT %s FROM runbooks WHERE %s ORDER BY title ASC LIMIT $%d OFFSET $%d`,
		runbookColumns, where, argN, argN+1,
	)
	args = append(args, limit, offset)

	rows, err := s.dbtx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing runbooks: %w", err)
	}
	return scanRunbookRows(rows)
}

// Count returns the count of runbooks matching the given filters.
func (s *Store) Count(ctx context.Context, filters ListFilters) (int, error) {
	where := "1=1"
	args := []any{}
	argN := 1

	if filters.Category != "" {
		where += fmt.Sprintf(" AND category = $%d", argN)
		args = append(args, filters.Category)
	}

	query := fmt.Sprintf(`SELECT count(*) FROM runbooks WHERE %s`, where)
	var count int
	if err := s.dbtx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting runbooks: %w", err)
	}
	return count, nil
}

// ListTemplates returns only pre-seeded template runbooks.
func (s *Store) ListTemplates(ctx context.Context) ([]RunbookRow, error) {
	query := `SELECT ` + runbookColumns + ` FROM runbooks WHERE is_template = true ORDER BY title ASC`
	rows, err := s.dbtx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing template runbooks: %w", err)
	}
	return scanRunbookRows(rows)
}
