package roster

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Service encapsulates roster business logic.
type Service struct {
	store  *Store
	logger *slog.Logger
}

// NewService creates a roster Service backed by the given database connection.
func NewService(dbtx db.DBTX, logger *slog.Logger) *Service {
	return &Service{
		store:  NewStore(dbtx),
		logger: logger,
	}
}

// --- Roster CRUD ---

func (s *Service) CreateRoster(ctx context.Context, req CreateRosterRequest) (RosterResponse, error) {
	handoffTime, err := parseHandoffTime(req.HandoffTime)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("invalid handoff_time: %w", err)
	}
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("invalid start_date: %w", err)
	}

	fts := req.IsFollowTheSun
	return s.store.CreateRoster(ctx, db.CreateRosterParams{
		Name:               req.Name,
		Description:        req.Description,
		Timezone:           req.Timezone,
		RotationType:       req.RotationType,
		RotationLength:     int32(req.RotationLength),
		HandoffTime:        handoffTime,
		IsFollowTheSun:     &fts,
		LinkedRosterID:     uuidToPgtype(req.LinkedRosterID),
		EscalationPolicyID: uuidToPgtype(req.EscalationPolicyID),
		StartDate:          startDate,
	})
}

func (s *Service) GetRoster(ctx context.Context, id uuid.UUID) (RosterResponse, error) {
	return s.store.GetRoster(ctx, id)
}

func (s *Service) ListRosters(ctx context.Context) ([]RosterResponse, error) {
	return s.store.ListRosters(ctx)
}

func (s *Service) UpdateRoster(ctx context.Context, id uuid.UUID, req UpdateRosterRequest) (RosterResponse, error) {
	handoffTime, err := parseHandoffTime(req.HandoffTime)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("invalid handoff_time: %w", err)
	}

	return s.store.UpdateRoster(ctx, db.UpdateRosterParams{
		ID:             id,
		Name:           req.Name,
		Description:    req.Description,
		Timezone:       req.Timezone,
		RotationType:   req.RotationType,
		RotationLength: int32(req.RotationLength),
		HandoffTime:    handoffTime,
	})
}

func (s *Service) DeleteRoster(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteRoster(ctx, id)
}

// --- Member CRUD ---

func (s *Service) ListMembers(ctx context.Context, rosterID uuid.UUID) ([]MemberResponse, error) {
	return s.store.ListMembers(ctx, rosterID)
}

func (s *Service) AddMember(ctx context.Context, rosterID uuid.UUID, req AddMemberRequest) (MemberResponse, error) {
	return s.store.AddMember(ctx, rosterID, req.UserID, req.Position)
}

func (s *Service) RemoveMember(ctx context.Context, memberID uuid.UUID) error {
	return s.store.RemoveMember(ctx, memberID)
}

// --- Override CRUD ---

func (s *Service) ListOverrides(ctx context.Context, rosterID uuid.UUID) ([]OverrideResponse, error) {
	return s.store.ListOverrides(ctx, rosterID)
}

func (s *Service) CreateOverride(ctx context.Context, rosterID uuid.UUID, req CreateOverrideRequest, callerID pgtype.UUID) (OverrideResponse, error) {
	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		return OverrideResponse{}, fmt.Errorf("invalid start_at: %w", err)
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		return OverrideResponse{}, fmt.Errorf("invalid end_at: %w", err)
	}
	if !endAt.After(startAt) {
		return OverrideResponse{}, fmt.Errorf("end_at must be after start_at")
	}

	return s.store.CreateOverride(ctx, db.CreateRosterOverrideParams{
		RosterID:  rosterID,
		UserID:    req.UserID,
		StartAt:   startAt,
		EndAt:     endAt,
		Reason:    req.Reason,
		CreatedBy: callerID,
	})
}

func (s *Service) DeleteOverride(ctx context.Context, overrideID uuid.UUID) error {
	return s.store.DeleteOverride(ctx, overrideID)
}

// --- On-call calculation ---

// GetOnCall returns who is on-call for a roster at the given time.
// It checks overrides first, then calculates the rotation position.
func (s *Service) GetOnCall(ctx context.Context, rosterID uuid.UUID, at time.Time) (*OnCallResponse, error) {
	roster, err := s.store.GetRoster(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("getting roster: %w", err)
	}

	// Follow-the-sun: delegate to the active sub-roster.
	if roster.IsFollowTheSun && roster.LinkedRosterID != nil {
		return s.getFollowTheSunOnCall(ctx, roster, at)
	}

	return s.calculateOnCall(ctx, roster, at)
}

// calculateOnCall computes who is on-call using the rotation schedule.
func (s *Service) calculateOnCall(ctx context.Context, roster RosterResponse, at time.Time) (*OnCallResponse, error) {
	// 1. Check for active override.
	override, err := s.store.GetActiveOverride(ctx, roster.ID, at)
	if err != nil {
		return nil, fmt.Errorf("checking override: %w", err)
	}
	if override != nil {
		return &OnCallResponse{
			UserID:     override.UserID,
			RosterID:   roster.ID,
			RosterName: roster.Name,
			IsOverride: true,
			ShiftStart: override.StartAt,
			ShiftEnd:   override.EndAt,
		}, nil
	}

	// 2. Calculate rotation position.
	members, err := s.store.ListMembers(ctx, roster.ID)
	if err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	if len(members) == 0 {
		return nil, nil // No members, no one on-call.
	}

	loc, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %q: %w", roster.Timezone, err)
	}

	startDate, err := time.Parse("2006-01-02", roster.StartDate)
	if err != nil {
		return nil, fmt.Errorf("parsing start_date: %w", err)
	}

	handoffTime, err := time.Parse("15:04", roster.HandoffTime)
	if err != nil {
		return nil, fmt.Errorf("parsing handoff_time: %w", err)
	}

	// Build the rotation start point in the roster's timezone.
	rosterStart := time.Date(
		startDate.Year(), startDate.Month(), startDate.Day(),
		handoffTime.Hour(), handoffTime.Minute(), 0, 0, loc,
	)

	// Days since roster start.
	elapsed := at.Sub(rosterStart)
	if elapsed < 0 {
		// Before roster start — first member is on-call.
		shiftEnd := rosterStart
		shiftStart := shiftEnd.Add(-time.Duration(roster.RotationLength) * 24 * time.Hour)
		return &OnCallResponse{
			UserID:     members[0].UserID,
			RosterID:   roster.ID,
			RosterName: roster.Name,
			ShiftStart: shiftStart,
			ShiftEnd:   shiftEnd,
		}, nil
	}

	rotationDays := float64(roster.RotationLength)
	daysSinceStart := elapsed.Hours() / 24.0
	currentCycle := int(math.Floor(daysSinceStart / rotationDays))
	position := currentCycle % len(members)

	shiftStart := rosterStart.Add(time.Duration(currentCycle) * time.Duration(roster.RotationLength) * 24 * time.Hour)
	shiftEnd := shiftStart.Add(time.Duration(roster.RotationLength) * 24 * time.Hour)

	return &OnCallResponse{
		UserID:     members[position].UserID,
		RosterID:   roster.ID,
		RosterName: roster.Name,
		ShiftStart: shiftStart,
		ShiftEnd:   shiftEnd,
	}, nil
}

// getFollowTheSunOnCall determines which sub-roster is active at the given time
// and returns the on-call from that sub-roster.
func (s *Service) getFollowTheSunOnCall(ctx context.Context, roster RosterResponse, at time.Time) (*OnCallResponse, error) {
	linkedRoster, err := s.store.GetRoster(ctx, *roster.LinkedRosterID)
	if err != nil {
		return nil, fmt.Errorf("getting linked roster: %w", err)
	}

	// Determine which roster covers this time based on handoff hours in their respective timezones.
	if s.isInShiftWindow(roster, at) {
		return s.calculateOnCall(ctx, roster, at)
	}
	if s.isInShiftWindow(linkedRoster, at) {
		return s.calculateOnCall(ctx, linkedRoster, at)
	}

	// Fallback: use the primary roster.
	return s.calculateOnCall(ctx, roster, at)
}

// isInShiftWindow checks if the given time falls within a roster's shift window.
// Each roster covers a 12-hour window starting from its handoff time.
func (s *Service) isInShiftWindow(roster RosterResponse, at time.Time) bool {
	loc, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		return false
	}

	localTime := at.In(loc)
	handoff, err := time.Parse("15:04", roster.HandoffTime)
	if err != nil {
		return false
	}

	handoffHour := handoff.Hour()*60 + handoff.Minute()
	currentMin := localTime.Hour()*60 + localTime.Minute()

	// 12-hour shift window from handoff time.
	shiftEnd := handoffHour + 12*60
	if shiftEnd >= 24*60 {
		// Wraps past midnight.
		return currentMin >= handoffHour || currentMin < (shiftEnd-24*60)
	}
	return currentMin >= handoffHour && currentMin < shiftEnd
}

// --- Parsing helpers ---

func parseHandoffTime(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	us := int64(t.Hour())*3600000000 + int64(t.Minute())*60000000
	return pgtype.Time{Microseconds: us, Valid: true}, nil
}

func parseDate(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}
