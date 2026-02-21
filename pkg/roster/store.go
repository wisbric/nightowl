package roster

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for rosters, members, and overrides.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates a roster Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

// --- Roster operations ---

func rosterToResponse(r db.Roster) RosterResponse {
	resp := RosterResponse{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		Timezone:       r.Timezone,
		RotationType:   r.RotationType,
		RotationLength: r.RotationLength,
		HandoffTime:    pgtypeTimeToString(r.HandoffTime),
		StartDate:      pgtypeDateToString(r.StartDate),
		LinkedRosterID: pgtypeUUIDToPtr(r.LinkedRosterID),
		EscalationPolicyID: pgtypeUUIDToPtr(r.EscalationPolicyID),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.IsFollowTheSun != nil {
		resp.IsFollowTheSun = *r.IsFollowTheSun
	}
	return resp
}

func (s *Store) CreateRoster(ctx context.Context, p db.CreateRosterParams) (RosterResponse, error) {
	row, err := s.q.CreateRoster(ctx, p)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("creating roster: %w", err)
	}
	return rosterToResponse(row), nil
}

func (s *Store) GetRoster(ctx context.Context, id uuid.UUID) (RosterResponse, error) {
	row, err := s.q.GetRoster(ctx, id)
	if err != nil {
		return RosterResponse{}, err
	}
	return rosterToResponse(row), nil
}

func (s *Store) ListRosters(ctx context.Context) ([]RosterResponse, error) {
	rows, err := s.q.ListRosters(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing rosters: %w", err)
	}
	result := make([]RosterResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, rosterToResponse(r))
	}
	return result, nil
}

func (s *Store) UpdateRoster(ctx context.Context, p db.UpdateRosterParams) (RosterResponse, error) {
	row, err := s.q.UpdateRoster(ctx, p)
	if err != nil {
		return RosterResponse{}, err
	}
	return rosterToResponse(row), nil
}

func (s *Store) DeleteRoster(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM rosters WHERE id = $1`
	tag, err := s.dbtx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting roster: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// --- Member operations ---

func memberToResponse(m db.RosterMember) MemberResponse {
	return MemberResponse{
		ID:       m.ID,
		RosterID: m.RosterID,
		UserID:   m.UserID,
		Position: m.Position,
	}
}

func (s *Store) ListMembers(ctx context.Context, rosterID uuid.UUID) ([]MemberResponse, error) {
	rows, err := s.q.ListRosterMembers(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing roster members: %w", err)
	}
	result := make([]MemberResponse, 0, len(rows))
	for _, m := range rows {
		result = append(result, memberToResponse(m))
	}
	return result, nil
}

func (s *Store) AddMember(ctx context.Context, rosterID, userID uuid.UUID, position int32) (MemberResponse, error) {
	row, err := s.q.CreateRosterMember(ctx, db.CreateRosterMemberParams{
		RosterID: rosterID,
		UserID:   userID,
		Position: position,
	})
	if err != nil {
		return MemberResponse{}, fmt.Errorf("adding roster member: %w", err)
	}
	return memberToResponse(row), nil
}

func (s *Store) RemoveMember(ctx context.Context, memberID uuid.UUID) error {
	s.q.DeleteRosterMember(ctx, memberID)
	return nil
}

func (s *Store) CountMembers(ctx context.Context, rosterID uuid.UUID) (int64, error) {
	return s.q.CountRosterMembers(ctx, rosterID)
}

// --- Override operations ---

func overrideToResponse(o db.RosterOverride) OverrideResponse {
	return OverrideResponse{
		ID:        o.ID,
		RosterID:  o.RosterID,
		UserID:    o.UserID,
		StartAt:   o.StartAt,
		EndAt:     o.EndAt,
		Reason:    o.Reason,
		CreatedBy: pgtypeUUIDToPtr(o.CreatedBy),
		CreatedAt: o.CreatedAt,
	}
}

func (s *Store) ListOverrides(ctx context.Context, rosterID uuid.UUID) ([]OverrideResponse, error) {
	rows, err := s.q.ListRosterOverrides(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing roster overrides: %w", err)
	}
	result := make([]OverrideResponse, 0, len(rows))
	for _, o := range rows {
		result = append(result, overrideToResponse(o))
	}
	return result, nil
}

func (s *Store) CreateOverride(ctx context.Context, p db.CreateRosterOverrideParams) (OverrideResponse, error) {
	row, err := s.q.CreateRosterOverride(ctx, p)
	if err != nil {
		return OverrideResponse{}, fmt.Errorf("creating roster override: %w", err)
	}
	return overrideToResponse(row), nil
}

func (s *Store) DeleteOverride(ctx context.Context, overrideID uuid.UUID) error {
	s.q.DeleteRosterOverride(ctx, overrideID)
	return nil
}

func (s *Store) GetActiveOverride(ctx context.Context, rosterID uuid.UUID, at time.Time) (*OverrideResponse, error) {
	row, err := s.q.GetActiveOverride(ctx, db.GetActiveOverrideParams{
		RosterID: rosterID,
		Column2:  at,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("checking active override: %w", err)
	}
	resp := overrideToResponse(row)
	return &resp, nil
}
