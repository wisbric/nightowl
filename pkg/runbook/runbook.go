package runbook

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateRequest is the JSON body for POST /api/v1/runbooks.
type CreateRequest struct {
	Title    string   `json:"title" validate:"required,min=3"`
	Content  string   `json:"content" validate:"required"`
	Category *string  `json:"category"`
	Tags     []string `json:"tags"`
}

// UpdateRequest is the JSON body for PUT /api/v1/runbooks/:id.
type UpdateRequest struct {
	Title    string   `json:"title" validate:"required,min=3"`
	Content  string   `json:"content" validate:"required"`
	Category *string  `json:"category"`
	Tags     []string `json:"tags"`
}

// ListFilters holds optional filter parameters for listing runbooks.
type ListFilters struct {
	Category string
}

// Response is the JSON response for a single runbook.
type Response struct {
	ID         uuid.UUID  `json:"id"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Category   *string    `json:"category,omitempty"`
	IsTemplate bool       `json:"is_template"`
	Tags       []string   `json:"tags"`
	CreatedBy  *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// RunbookRow represents a row returned from the runbooks table.
type RunbookRow struct {
	ID         uuid.UUID
	Title      string
	Content    string
	Category   *string
	IsTemplate bool
	Tags       []string
	CreatedBy  pgtype.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ToResponse converts a RunbookRow to a Response DTO.
func (r *RunbookRow) ToResponse() Response {
	resp := Response{
		ID:         r.ID,
		Title:      r.Title,
		Content:    r.Content,
		Category:   r.Category,
		IsTemplate: r.IsTemplate,
		Tags:       ensureSlice(r.Tags),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.CreatedBy.Valid {
		uid := uuid.UUID(r.CreatedBy.Bytes)
		resp.CreatedBy = &uid
	}
	return resp
}

// ensureSlice returns s if non-nil, otherwise an empty slice.
func ensureSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// defaultSlice returns s if non-nil, otherwise an empty string slice.
func defaultSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
