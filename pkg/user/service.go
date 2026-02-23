package user

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// Service encapsulates user business logic.
type Service struct {
	store  *Store
	logger *slog.Logger
}

// NewService creates a user Service backed by the given database connection.
func NewService(dbtx db.DBTX, logger *slog.Logger) *Service {
	return &Service{
		store:  NewStore(dbtx),
		logger: logger,
	}
}

// List returns all active users.
func (s *Service) List(ctx context.Context) ([]Response, error) {
	rows, err := s.store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToResponse())
	}
	return items, nil
}

// Get returns a single user by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Response, error) {
	row, err := s.store.Get(ctx, id)
	if err != nil {
		return Response{}, fmt.Errorf("getting user: %w", err)
	}
	return row.ToResponse(), nil
}

// Create creates a new user.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Response, error) {
	row, err := s.store.Create(ctx, CreateUserParams(req))
	if err != nil {
		return Response{}, fmt.Errorf("creating user: %w", err)
	}
	return row.ToResponse(), nil
}

// Update updates a user.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Response, error) {
	row, err := s.store.Update(ctx, id, UpdateUserParams(req))
	if err != nil {
		return Response{}, fmt.Errorf("updating user: %w", err)
	}
	return row.ToResponse(), nil
}

// Deactivate soft-deletes a user.
func (s *Service) Deactivate(ctx context.Context, id uuid.UUID) error {
	if err := s.store.Deactivate(ctx, id); err != nil {
		return fmt.Errorf("deactivating user: %w", err)
	}
	return nil
}
