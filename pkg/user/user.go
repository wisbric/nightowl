package user

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the JSON body for POST /api/v1/users.
type CreateRequest struct {
	Email       string  `json:"email" validate:"required,email"`
	DisplayName string  `json:"display_name" validate:"required,min=2"`
	Role        string  `json:"role" validate:"required"`
	Timezone    string  `json:"timezone"`
	Phone       *string `json:"phone"`
	SlackUserID *string `json:"slack_user_id"`
}

// UpdateRequest is the JSON body for PUT /api/v1/users/:id.
type UpdateRequest struct {
	Email       string  `json:"email" validate:"required,email"`
	DisplayName string  `json:"display_name" validate:"required,min=2"`
	Role        string  `json:"role" validate:"required"`
	Timezone    string  `json:"timezone"`
	Phone       *string `json:"phone"`
	SlackUserID *string `json:"slack_user_id"`
}

// Response is the JSON response for a single user.
type Response struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Timezone    string    `json:"timezone"`
	Phone       *string   `json:"phone,omitempty"`
	SlackUserID *string   `json:"slack_user_id,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
