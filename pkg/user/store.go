package user

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for users.
type Store struct {
	dbtx db.DBTX
}

// NewStore creates a user Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{dbtx: dbtx}
}

const userColumns = `id, external_id, email, display_name, timezone, phone, slack_user_id, role, is_active, created_at, updated_at`

// UserRow represents a row returned from the users table.
type UserRow struct {
	ID          uuid.UUID
	ExternalID  string
	Email       string
	DisplayName string
	Timezone    string
	Phone       *string
	SlackUserID *string
	Role        string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ToResponse converts a UserRow to a Response DTO.
func (u *UserRow) ToResponse() Response {
	return Response{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		Timezone:    u.Timezone,
		Phone:       u.Phone,
		SlackUserID: u.SlackUserID,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// scanUserRow scans a pgx.Row into a UserRow.
func scanUserRow(row pgx.Row) (UserRow, error) {
	var u UserRow
	err := row.Scan(
		&u.ID, &u.ExternalID, &u.Email, &u.DisplayName, &u.Timezone,
		&u.Phone, &u.SlackUserID, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

// scanUserRows scans multiple rows into UserRow slices.
func scanUserRows(rows pgx.Rows) ([]UserRow, error) {
	defer rows.Close()
	var items []UserRow
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(
			&u.ID, &u.ExternalID, &u.Email, &u.DisplayName, &u.Timezone,
			&u.Phone, &u.SlackUserID, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user row: %w", err)
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user rows: %w", err)
	}
	return items, nil
}

// List returns all active users ordered by display name.
func (s *Store) List(ctx context.Context) ([]UserRow, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE is_active = true ORDER BY display_name`
	rows, err := s.dbtx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return scanUserRows(rows)
}

// Get returns a single user by ID.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (UserRow, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	row := s.dbtx.QueryRow(ctx, query, id)
	return scanUserRow(row)
}

// CreateUserParams holds parameters for creating a user.
type CreateUserParams struct {
	Email       string
	DisplayName string
	Role        string
	Timezone    string
	Phone       *string
	SlackUserID *string
}

// Create inserts a new user with a generated external_id.
func (s *Store) Create(ctx context.Context, p CreateUserParams) (UserRow, error) {
	externalID := uuid.New().String()
	query := `INSERT INTO users (external_id, email, display_name, role, timezone, phone, slack_user_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING ` + userColumns
	row := s.dbtx.QueryRow(ctx, query,
		externalID, p.Email, p.DisplayName, p.Role, p.Timezone, p.Phone, p.SlackUserID,
	)
	return scanUserRow(row)
}

// UpdateUserParams holds parameters for updating a user.
type UpdateUserParams struct {
	Email       string
	DisplayName string
	Role        string
	Timezone    string
	Phone       *string
	SlackUserID *string
}

// Update updates all editable fields and returns the updated row.
func (s *Store) Update(ctx context.Context, id uuid.UUID, p UpdateUserParams) (UserRow, error) {
	query := `UPDATE users
	SET email = $2, display_name = $3, role = $4, timezone = $5, phone = $6, slack_user_id = $7, updated_at = now()
	WHERE id = $1
	RETURNING ` + userColumns
	row := s.dbtx.QueryRow(ctx, query,
		id, p.Email, p.DisplayName, p.Role, p.Timezone, p.Phone, p.SlackUserID,
	)
	return scanUserRow(row)
}

// Deactivate soft-deletes a user by setting is_active to false.
// Also deactivates the user from all roster memberships and removes
// them from future unlocked schedule weeks.
func (s *Store) Deactivate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET is_active = false, updated_at = now() WHERE id = $1`
	tag, err := s.dbtx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deactivating user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	// Deactivate from all roster memberships.
	if _, err := s.dbtx.Exec(ctx,
		`UPDATE roster_members SET is_active = false, left_at = now() WHERE user_id = $1 AND is_active = true`,
		id); err != nil {
		return fmt.Errorf("deactivating roster memberships: %w", err)
	}

	// Remove from current and future unlocked schedule weeks (preserve locked/past weeks for history).
	if _, err := s.dbtx.Exec(ctx,
		`UPDATE roster_schedule SET primary_user_id = NULL WHERE primary_user_id = $1 AND is_locked = false AND week_start >= CURRENT_DATE`,
		id); err != nil {
		return fmt.Errorf("clearing primary schedule assignments: %w", err)
	}
	if _, err := s.dbtx.Exec(ctx,
		`UPDATE roster_schedule SET secondary_user_id = NULL WHERE secondary_user_id = $1 AND is_locked = false AND week_start >= CURRENT_DATE`,
		id); err != nil {
		return fmt.Errorf("clearing secondary schedule assignments: %w", err)
	}

	return nil
}
