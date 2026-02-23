package pat

import (
	"time"

	"github.com/google/uuid"
)

// Prefix for all personal access tokens — identifiable in leaked credential scans.
const TokenPrefix = "nwl_pat_"

// Token represents a personal access token row.
type Token struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateRequest is the JSON body for creating a PAT.
type CreateRequest struct {
	Name      string `json:"name" validate:"required,min=1,max=100"`
	ExpiresIn *int   `json:"expires_in_days"` // optional: days until expiry
}

// CreateResponse includes the full token (shown only once).
type CreateResponse struct {
	Token
	RawToken string `json:"raw_token"`
}

// ListResponse wraps a list of tokens.
type ListResponse struct {
	Tokens []Token `json:"tokens"`
	Count  int     `json:"count"`
}
