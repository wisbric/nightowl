package apikey

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateRequest is the JSON body for POST /api/v1/apikeys.
type CreateRequest struct {
	Description string `json:"description" validate:"required"`
	Role        string `json:"role" validate:"required"`
}

// Response is the JSON response for a single API key (without the raw key).
type Response struct {
	ID          uuid.UUID  `json:"id"`
	KeyPrefix   string     `json:"key_prefix"`
	Description string     `json:"description"`
	Role        string     `json:"role"`
	Scopes      []string   `json:"scopes"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateResponse includes the raw key (only shown once at creation).
type CreateResponse struct {
	Response
	RawKey string `json:"raw_key"`
}

// ApiKeyRow represents a row returned from the public.api_keys table.
type ApiKeyRow struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	KeyHash     string
	KeyPrefix   string
	Description string
	Role        string
	Scopes      []string
	LastUsed    pgtype.Timestamptz
	ExpiresAt   pgtype.Timestamptz
	CreatedAt   time.Time
}

// ToResponse converts an ApiKeyRow to a Response DTO.
func (r *ApiKeyRow) ToResponse() Response {
	resp := Response{
		ID:          r.ID,
		KeyPrefix:   r.KeyPrefix,
		Description: r.Description,
		Role:        r.Role,
		Scopes:      ensureSlice(r.Scopes),
		CreatedAt:   r.CreatedAt,
	}
	if r.LastUsed.Valid {
		t := r.LastUsed.Time
		resp.LastUsed = &t
	}
	if r.ExpiresAt.Valid {
		t := r.ExpiresAt.Time
		resp.ExpiresAt = &t
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
