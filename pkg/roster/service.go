package roster

import (
	"context"
	"fmt"
	"log/slog"
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
	return s.store.CreateRoster(ctx, req)
}

func (s *Service) GetRoster(ctx context.Context, id uuid.UUID) (RosterResponse, error) {
	return s.store.GetRoster(ctx, id)
}

func (s *Service) ListRosters(ctx context.Context) ([]RosterResponse, error) {
	return s.store.ListRosters(ctx)
}

func (s *Service) UpdateRoster(ctx context.Context, id uuid.UUID, req UpdateRosterRequest) (RosterResponse, error) {
	return s.store.UpdateRoster(ctx, id, req)
}

func (s *Service) DeleteRoster(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteRoster(ctx, id)
}

// --- Member CRUD ---

func (s *Service) ListMembers(ctx context.Context, rosterID uuid.UUID) ([]MemberResponse, error) {
	members, err := s.store.ListMembers(ctx, rosterID)
	if err != nil {
		return nil, err
	}
	// Enrich with primary weeks served.
	for i := range members {
		count, _ := s.store.CountPrimaryWeeks(ctx, rosterID, members[i].UserID)
		members[i].PrimaryWeeksServed = count
	}
	return members, nil
}

func (s *Service) AddMember(ctx context.Context, rosterID uuid.UUID, req AddMemberRequest) (MemberResponse, error) {
	member, err := s.store.AddMember(ctx, rosterID, req.UserID)
	if err != nil {
		return MemberResponse{}, err
	}
	// Regenerate unlocked future schedule to include the new member.
	// Runs synchronously — the store uses the per-request DB connection
	// which is released after the handler returns.
	if err := s.regenerateFuture(ctx, rosterID); err != nil {
		s.logger.Error("regenerating schedule after add member", "error", err, "roster_id", rosterID)
	}
	return member, nil
}

func (s *Service) DeactivateMember(ctx context.Context, rosterID, userID uuid.UUID) error {
	if err := s.store.DeactivateMember(ctx, rosterID, userID); err != nil {
		return err
	}
	if err := s.regenerateFuture(ctx, rosterID); err != nil {
		s.logger.Error("regenerating schedule after deactivate member", "error", err, "roster_id", rosterID)
	}
	return nil
}

func (s *Service) SetMemberActive(ctx context.Context, rosterID, userID uuid.UUID, active bool) error {
	if err := s.store.SetMemberActive(ctx, rosterID, userID, active); err != nil {
		return err
	}
	if err := s.regenerateFuture(ctx, rosterID); err != nil {
		s.logger.Error("regenerating schedule after toggle member", "error", err, "roster_id", rosterID)
	}
	return nil
}

// regenerateFuture regenerates unlocked future schedule weeks.
func (s *Service) regenerateFuture(ctx context.Context, rosterID uuid.UUID) error {
	roster, err := s.store.GetRoster(ctx, rosterID)
	if err != nil {
		return err
	}
	_, err = s.GenerateSchedule(ctx, rosterID, time.Now(), roster.ScheduleWeeksAhead)
	return err
}

// --- Schedule ---

func (s *Service) GetSchedule(ctx context.Context, rosterID uuid.UUID, from, to time.Time) ([]ScheduleEntry, error) {
	return s.store.ListSchedule(ctx, rosterID, from, to)
}

func (s *Service) GetScheduleWeek(ctx context.Context, rosterID uuid.UUID, weekStart time.Time) (*ScheduleEntry, error) {
	return s.store.GetScheduleWeek(ctx, rosterID, weekStart)
}

func (s *Service) UpdateScheduleWeek(ctx context.Context, rosterID uuid.UUID, weekStart time.Time, req UpdateScheduleWeekRequest) (*ScheduleEntry, error) {
	// Compute week_end.
	weekEnd := weekStart.AddDate(0, 0, 7)
	return s.store.UpsertScheduleWeek(ctx, rosterID, weekStart, weekEnd,
		req.PrimaryUserID, req.SecondaryUserID, true, false, req.Notes)
}

func (s *Service) UnlockScheduleWeek(ctx context.Context, rosterID uuid.UUID, weekStart time.Time) error {
	return s.store.UnlockScheduleWeek(ctx, rosterID, weekStart)
}

// --- Overrides ---

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
	return s.store.CreateOverride(ctx, rosterID, req.UserID, startAt, endAt, req.Reason, callerID)
}

func (s *Service) DeleteOverride(ctx context.Context, overrideID uuid.UUID) error {
	return s.store.DeleteOverride(ctx, overrideID)
}

// --- On-call resolution: override → schedule → unassigned ---

func (s *Service) GetOnCall(ctx context.Context, rosterID uuid.UUID, at time.Time) (*OnCallResponse, error) {
	roster, err := s.store.GetRoster(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("getting roster: %w", err)
	}

	// Always resolve from this roster's own schedule.
	// Follow-the-sun delegation is handled by the escalation engine,
	// not the per-roster on-call endpoint.
	return s.resolveOnCall(ctx, roster, at)
}

func (s *Service) resolveOnCall(ctx context.Context, roster RosterResponse, at time.Time) (*OnCallResponse, error) {
	resp := &OnCallResponse{
		RosterID:   roster.ID,
		RosterName: roster.Name,
		QueriedAt:  at,
	}

	// 1. Check for active override.
	override, err := s.store.GetActiveOverride(ctx, roster.ID, at)
	if err != nil {
		return nil, fmt.Errorf("checking override: %w", err)
	}

	if override != nil {
		resp.Source = "override"
		resp.Primary = &OnCallEntry{
			UserID:      override.UserID,
			DisplayName: override.DisplayName,
		}
		resp.ActiveOverride = override

		// Still look up scheduled secondary.
		sched, _ := s.store.GetScheduleForTime(ctx, roster.ID, at)
		if sched != nil {
			resp.WeekStart = &sched.WeekStart
			if sched.SecondaryUserID != nil {
				resp.Secondary = &OnCallEntry{
					UserID:      *sched.SecondaryUserID,
					DisplayName: sched.SecondaryDisplayName,
				}
			}
		}
		return resp, nil
	}

	// 2. Check schedule for current time.
	sched, err := s.store.GetScheduleForTime(ctx, roster.ID, at)
	if err != nil {
		return nil, fmt.Errorf("getting schedule: %w", err)
	}

	if sched != nil {
		resp.Source = "schedule"
		resp.WeekStart = &sched.WeekStart
		if sched.PrimaryUserID != nil {
			resp.Primary = &OnCallEntry{
				UserID:      *sched.PrimaryUserID,
				DisplayName: sched.PrimaryDisplayName,
			}
		}
		if sched.SecondaryUserID != nil {
			resp.Secondary = &OnCallEntry{
				UserID:      *sched.SecondaryUserID,
				DisplayName: sched.SecondaryDisplayName,
			}
		}
		return resp, nil
	}

	// 3. Unassigned.
	resp.Source = "unassigned"
	return resp, nil
}

// getFollowTheSunOnCall determines which sub-roster is active based on active hours.
// The response always carries the *requested* roster's identity, even when
// delegated to the linked roster for on-call resolution.
func (s *Service) getFollowTheSunOnCall(ctx context.Context, roster RosterResponse, at time.Time) (*OnCallResponse, error) {
	if s.isInActiveHours(roster, at) {
		return s.resolveOnCall(ctx, roster, at)
	}
	if roster.LinkedRosterID != nil {
		linkedRoster, err := s.store.GetRoster(ctx, *roster.LinkedRosterID)
		if err != nil {
			return nil, fmt.Errorf("getting linked roster: %w", err)
		}
		if s.isInActiveHours(linkedRoster, at) {
			resp, err := s.resolveOnCall(ctx, linkedRoster, at)
			if err != nil {
				return nil, err
			}
			// Preserve the originally requested roster's identity.
			resp.RosterID = roster.ID
			resp.RosterName = roster.Name
			return resp, nil
		}
	}
	// Fallback: use this roster.
	return s.resolveOnCall(ctx, roster, at)
}

func (s *Service) isInActiveHours(roster RosterResponse, at time.Time) bool {
	if roster.ActiveHoursStart == nil || roster.ActiveHoursEnd == nil {
		// No active hours configured — use handoff-based 12-hour window.
		return s.isInHandoffWindow(roster, at)
	}

	loc, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		return false
	}
	localTime := at.In(loc)
	start, err := time.Parse("15:04", *roster.ActiveHoursStart)
	if err != nil {
		return false
	}
	end, err := time.Parse("15:04", *roster.ActiveHoursEnd)
	if err != nil {
		return false
	}

	startMin := start.Hour()*60 + start.Minute()
	endMin := end.Hour()*60 + end.Minute()
	currentMin := localTime.Hour()*60 + localTime.Minute()

	if endMin > startMin {
		return currentMin >= startMin && currentMin < endMin
	}
	// Wraps past midnight.
	return currentMin >= startMin || currentMin < endMin
}

func (s *Service) isInHandoffWindow(roster RosterResponse, at time.Time) bool {
	loc, err := time.LoadLocation(roster.Timezone)
	if err != nil {
		return false
	}
	localTime := at.In(loc)
	handoff, err := time.Parse("15:04", roster.HandoffTime)
	if err != nil {
		return false
	}

	handoffMin := handoff.Hour()*60 + handoff.Minute()
	currentMin := localTime.Hour()*60 + localTime.Minute()

	shiftEnd := handoffMin + 12*60
	if shiftEnd >= 24*60 {
		return currentMin >= handoffMin || currentMin < (shiftEnd-24*60)
	}
	return currentMin >= handoffMin && currentMin < shiftEnd
}
