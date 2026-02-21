package roster

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- Roster types ---

// CreateRosterRequest is the JSON body for POST /api/v1/rosters.
type CreateRosterRequest struct {
	Name               string     `json:"name" validate:"required,min=2"`
	Description        *string    `json:"description"`
	Timezone           string     `json:"timezone" validate:"required"`
	RotationType       string     `json:"rotation_type" validate:"required,oneof=daily weekly custom"`
	RotationLength     int        `json:"rotation_length" validate:"required,min=1"`
	HandoffTime        string     `json:"handoff_time" validate:"required"` // HH:MM format
	IsFollowTheSun     bool       `json:"is_follow_the_sun"`
	LinkedRosterID     *uuid.UUID `json:"linked_roster_id"`
	EscalationPolicyID *uuid.UUID `json:"escalation_policy_id"`
	StartDate          string     `json:"start_date" validate:"required"` // YYYY-MM-DD
}

// UpdateRosterRequest is the JSON body for PUT /api/v1/rosters/:id.
type UpdateRosterRequest struct {
	Name           string  `json:"name" validate:"required,min=2"`
	Description    *string `json:"description"`
	Timezone       string  `json:"timezone" validate:"required"`
	RotationType   string  `json:"rotation_type" validate:"required,oneof=daily weekly custom"`
	RotationLength int     `json:"rotation_length" validate:"required,min=1"`
	HandoffTime    string  `json:"handoff_time" validate:"required"` // HH:MM format
}

// RosterResponse is the JSON response for a single roster.
type RosterResponse struct {
	ID                 uuid.UUID  `json:"id"`
	Name               string     `json:"name"`
	Description        *string    `json:"description,omitempty"`
	Timezone           string     `json:"timezone"`
	RotationType       string     `json:"rotation_type"`
	RotationLength     int32      `json:"rotation_length"`
	HandoffTime        string     `json:"handoff_time"`
	IsFollowTheSun     bool       `json:"is_follow_the_sun"`
	LinkedRosterID     *uuid.UUID `json:"linked_roster_id,omitempty"`
	EscalationPolicyID *uuid.UUID `json:"escalation_policy_id,omitempty"`
	StartDate          string     `json:"start_date"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// --- Member types ---

// AddMemberRequest is the JSON body for POST /api/v1/rosters/:id/members.
type AddMemberRequest struct {
	UserID   uuid.UUID `json:"user_id" validate:"required"`
	Position int32     `json:"position" validate:"min=0"`
}

// MemberResponse is the JSON response for a roster member.
type MemberResponse struct {
	ID       uuid.UUID `json:"id"`
	RosterID uuid.UUID `json:"roster_id"`
	UserID   uuid.UUID `json:"user_id"`
	Position int32     `json:"position"`
}

// --- Override types ---

// CreateOverrideRequest is the JSON body for POST /api/v1/rosters/:id/overrides.
type CreateOverrideRequest struct {
	UserID  uuid.UUID `json:"user_id" validate:"required"`
	StartAt string    `json:"start_at" validate:"required"` // RFC3339
	EndAt   string    `json:"end_at" validate:"required"`   // RFC3339
	Reason  *string   `json:"reason"`
}

// OverrideResponse is the JSON response for a roster override.
type OverrideResponse struct {
	ID        uuid.UUID  `json:"id"`
	RosterID  uuid.UUID  `json:"roster_id"`
	UserID    uuid.UUID  `json:"user_id"`
	StartAt   time.Time  `json:"start_at"`
	EndAt     time.Time  `json:"end_at"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// --- On-call types ---

// OnCallResponse describes who is currently on-call.
type OnCallResponse struct {
	UserID     uuid.UUID `json:"user_id"`
	RosterID   uuid.UUID `json:"roster_id"`
	RosterName string    `json:"roster_name"`
	IsOverride bool      `json:"is_override"`
	ShiftStart time.Time `json:"shift_start"`
	ShiftEnd   time.Time `json:"shift_end"`
}

// --- Helpers ---

func pgtypeUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func uuidToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgtypeTimeToString(t pgtype.Time) string {
	if !t.Valid {
		return "09:00"
	}
	us := t.Microseconds
	hours := us / 3600000000
	minutes := (us % 3600000000) / 60000000
	return time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
}

func pgtypeDateToString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}
