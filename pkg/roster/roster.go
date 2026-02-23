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
	HandoffTime        string     `json:"handoff_time" validate:"required"` // HH:MM
	HandoffDay         int        `json:"handoff_day"`                      // 0=Sun..6=Sat, default 1=Mon
	ScheduleWeeksAhead int        `json:"schedule_weeks_ahead"`             // default 12
	MaxConsecutiveWeeks int       `json:"max_consecutive_weeks"`            // default 2
	IsFollowTheSun     bool       `json:"is_follow_the_sun"`
	LinkedRosterID     *uuid.UUID `json:"linked_roster_id"`
	ActiveHoursStart   *string    `json:"active_hours_start"` // HH:MM
	ActiveHoursEnd     *string    `json:"active_hours_end"`   // HH:MM
	EscalationPolicyID *uuid.UUID `json:"escalation_policy_id"`
	EndDate            *string    `json:"end_date"` // YYYY-MM-DD, optional
}

// UpdateRosterRequest is the JSON body for PUT /api/v1/rosters/:id.
type UpdateRosterRequest struct {
	Name                string  `json:"name" validate:"required,min=2"`
	Description         *string `json:"description"`
	Timezone            string  `json:"timezone" validate:"required"`
	HandoffTime         string  `json:"handoff_time" validate:"required"` // HH:MM
	HandoffDay          int     `json:"handoff_day"`
	ScheduleWeeksAhead  int     `json:"schedule_weeks_ahead"`
	MaxConsecutiveWeeks int     `json:"max_consecutive_weeks"`
	EndDate             *string `json:"end_date"`
	IsActive            *bool   `json:"is_active"`
}

// RosterResponse is the JSON response for a single roster.
type RosterResponse struct {
	ID                  uuid.UUID  `json:"id"`
	Name                string     `json:"name"`
	Description         *string    `json:"description,omitempty"`
	Timezone            string     `json:"timezone"`
	HandoffTime         string     `json:"handoff_time"`
	HandoffDay          int        `json:"handoff_day"`
	ScheduleWeeksAhead  int        `json:"schedule_weeks_ahead"`
	MaxConsecutiveWeeks int        `json:"max_consecutive_weeks"`
	IsFollowTheSun      bool       `json:"is_follow_the_sun"`
	LinkedRosterID      *uuid.UUID `json:"linked_roster_id,omitempty"`
	ActiveHoursStart    *string    `json:"active_hours_start,omitempty"`
	ActiveHoursEnd      *string    `json:"active_hours_end,omitempty"`
	EscalationPolicyID  *uuid.UUID `json:"escalation_policy_id,omitempty"`
	EndDate             *string    `json:"end_date,omitempty"`
	IsActive            bool       `json:"is_active"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// --- Member types ---

// AddMemberRequest is the JSON body for POST /api/v1/rosters/:id/members.
type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

// UpdateMemberRequest is the JSON body for PUT /api/v1/rosters/:id/members/:userId.
type UpdateMemberRequest struct {
	IsActive bool `json:"is_active"`
}

// MemberResponse is the JSON response for a roster member.
type MemberResponse struct {
	ID                  uuid.UUID  `json:"id"`
	RosterID            uuid.UUID  `json:"roster_id"`
	UserID              uuid.UUID  `json:"user_id"`
	DisplayName         string     `json:"display_name"`
	IsActive            bool       `json:"is_active"`
	JoinedAt            time.Time  `json:"joined_at"`
	LeftAt              *time.Time `json:"left_at,omitempty"`
	PrimaryWeeksServed   int        `json:"primary_weeks_served"`
	SecondaryWeeksServed int        `json:"secondary_weeks_served"`
}

// --- Schedule types ---

// ScheduleEntry represents one week in the roster schedule.
type ScheduleEntry struct {
	ID                   uuid.UUID  `json:"id"`
	RosterID             uuid.UUID  `json:"roster_id"`
	WeekStart            string     `json:"week_start"` // YYYY-MM-DD
	WeekEnd              string     `json:"week_end"`
	PrimaryUserID        *uuid.UUID `json:"primary_user_id"`
	PrimaryDisplayName   string     `json:"primary_display_name,omitempty"`
	SecondaryUserID      *uuid.UUID `json:"secondary_user_id"`
	SecondaryDisplayName string     `json:"secondary_display_name,omitempty"`
	IsLocked             bool       `json:"is_locked"`
	Generated            bool       `json:"generated"`
	Notes                *string    `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// UpdateScheduleWeekRequest is the JSON body for PUT /api/v1/rosters/:id/schedule/:weekStart.
type UpdateScheduleWeekRequest struct {
	PrimaryUserID   *uuid.UUID `json:"primary_user_id"`
	SecondaryUserID *uuid.UUID `json:"secondary_user_id"`
	Notes           *string    `json:"notes"`
}

// GenerateScheduleRequest is the JSON body for POST /api/v1/rosters/:id/schedule/generate.
type GenerateScheduleRequest struct {
	From  *string `json:"from"`  // YYYY-MM-DD, default: current week
	Weeks *int    `json:"weeks"` // default: roster.schedule_weeks_ahead
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
	ID          uuid.UUID  `json:"id"`
	RosterID    uuid.UUID  `json:"roster_id"`
	UserID      uuid.UUID  `json:"user_id"`
	DisplayName string     `json:"display_name"`
	StartAt     time.Time  `json:"start_at"`
	EndAt       time.Time  `json:"end_at"`
	Reason      *string    `json:"reason,omitempty"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// --- On-call types ---

// OnCallResponse describes who is currently on-call.
type OnCallResponse struct {
	RosterID       uuid.UUID    `json:"roster_id"`
	RosterName     string       `json:"roster_name"`
	QueriedAt      time.Time    `json:"queried_at"`
	Source         string       `json:"source"` // "override" | "schedule" | "unassigned"
	Primary        *OnCallEntry `json:"primary"`
	Secondary      *OnCallEntry `json:"secondary"`
	WeekStart      *string      `json:"week_start,omitempty"`
	ActiveOverride *OverrideResponse `json:"active_override,omitempty"`
}

// OnCallEntry describes a single on-call person.
type OnCallEntry struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
}

// --- Coverage types ---

// CoverageRequest holds parameters for the coverage endpoint.
type CoverageRequest struct {
	From       time.Time
	To         time.Time
	Resolution int // minutes per slot, default 60
}

// CoverageResponse is the JSON response for GET /rosters/coverage.
type CoverageResponse struct {
	From              time.Time        `json:"from"`
	To                time.Time        `json:"to"`
	ResolutionMinutes int              `json:"resolution_minutes"`
	Rosters           []CoverageRoster `json:"rosters"`
	Slots             []CoverageSlot   `json:"slots"`
	GapSummary        GapSummary       `json:"gap_summary"`
}

// CoverageRoster summarises a roster for coverage context.
type CoverageRoster struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Timezone         string    `json:"timezone"`
	ActiveHoursStart *string   `json:"active_hours_start,omitempty"`
	ActiveHoursEnd   *string   `json:"active_hours_end,omitempty"`
	IsFollowTheSun   bool      `json:"is_follow_the_sun"`
}

// CoverageSlot is one time slot in the coverage heatmap.
type CoverageSlot struct {
	Time     time.Time              `json:"time"`
	Coverage []CoverageSlotRoster   `json:"coverage"`
	Gap      bool                   `json:"gap"`
}

// CoverageSlotRoster describes a roster's coverage during a slot.
type CoverageSlotRoster struct {
	RosterID   uuid.UUID `json:"roster_id"`
	RosterName string    `json:"roster_name"`
	Primary    string    `json:"primary"`
	Secondary  string    `json:"secondary,omitempty"`
	Source     string    `json:"source"`
}

// GapSummary describes coverage gaps.
type GapSummary struct {
	TotalGapHours float64   `json:"total_gap_hours"`
	Gaps          []GapInfo `json:"gaps"`
}

// GapInfo describes a single coverage gap.
type GapInfo struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	DurationHours float64   `json:"duration_hours"`
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

func pgtypeDateToPtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

func parseHandoffTime(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	us := int64(t.Hour())*3600000000 + int64(t.Minute())*60000000
	return pgtype.Time{Microseconds: us, Valid: true}, nil
}

func parseOptionalTime(s *string) pgtype.Time {
	if s == nil || *s == "" {
		return pgtype.Time{}
	}
	t, err := parseHandoffTime(*s)
	if err != nil {
		return pgtype.Time{}
	}
	return t
}

func parseDate(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}
