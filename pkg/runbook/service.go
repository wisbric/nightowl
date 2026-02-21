package runbook

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Service encapsulates runbook business logic.
type Service struct {
	store  *Store
	logger *slog.Logger
}

// NewService creates a runbook Service backed by the given database connection.
func NewService(dbtx db.DBTX, logger *slog.Logger) *Service {
	return &Service{
		store:  NewStore(dbtx),
		logger: logger,
	}
}

// Create creates a new runbook.
func (s *Service) Create(ctx context.Context, req CreateRequest, userID pgtype.UUID) (Response, error) {
	row, err := s.store.Create(ctx, CreateParams{
		Title:    req.Title,
		Content:  req.Content,
		Category: req.Category,
		Tags:     defaultSlice(req.Tags),
		CreatedBy: userID,
	})
	if err != nil {
		return Response{}, fmt.Errorf("creating runbook: %w", err)
	}
	return row.ToResponse(), nil
}

// Get returns a single runbook by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Response, error) {
	row, err := s.store.Get(ctx, id)
	if err != nil {
		return Response{}, fmt.Errorf("getting runbook: %w", err)
	}
	return row.ToResponse(), nil
}

// List returns a paginated, filtered list of runbooks.
func (s *Service) List(ctx context.Context, filters ListFilters, limit, offset int) ([]Response, int, error) {
	rows, err := s.store.List(ctx, filters, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing runbooks: %w", err)
	}

	count, err := s.store.Count(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("counting runbooks: %w", err)
	}

	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToResponse())
	}
	return items, count, nil
}

// Update updates a runbook.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	updated, err := s.store.Update(ctx, UpdateParams{
		ID:       id,
		Title:    req.Title,
		Content:  req.Content,
		Category: req.Category,
		Tags:     defaultSlice(req.Tags),
	})
	if err != nil {
		return Response{}, fmt.Errorf("updating runbook: %w", err)
	}
	return updated.ToResponse(), nil
}

// Delete permanently removes a runbook.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting runbook: %w", err)
	}
	return nil
}

// ListTemplates returns all pre-seeded template runbooks.
func (s *Service) ListTemplates(ctx context.Context) ([]Response, error) {
	rows, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing template runbooks: %w", err)
	}

	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToResponse())
	}
	return items, nil
}
