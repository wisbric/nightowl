package roster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for rosters, members, overrides, and schedules.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates a roster Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

// =====================
// Roster operations
// =====================

func (s *Store) CreateRoster(ctx context.Context, r CreateRosterRequest) (RosterResponse, error) {
	handoffTime, err := parseHandoffTime(r.HandoffTime)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("invalid handoff_time: %w", err)
	}

	weeksAhead := r.ScheduleWeeksAhead
	if weeksAhead <= 0 {
		weeksAhead = 12
	}
	maxConsec := r.MaxConsecutiveWeeks
	if maxConsec <= 0 {
		maxConsec = 2
	}

	var endDate pgtype.Date
	if r.EndDate != nil && *r.EndDate != "" {
		endDate, err = parseDate(*r.EndDate)
		if err != nil {
			return RosterResponse{}, fmt.Errorf("invalid end_date: %w", err)
		}
	}

	query := `INSERT INTO rosters (name, description, timezone, handoff_time, handoff_day,
	           schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	           linked_roster_id, active_hours_start, active_hours_end,
	           escalation_policy_id, end_date, is_active)
	          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,true)
	          RETURNING id, name, description, timezone, handoff_time, handoff_day,
	           schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	           linked_roster_id, active_hours_start, active_hours_end,
	           escalation_policy_id, end_date, is_active, created_at, updated_at`

	return s.scanRoster(s.dbtx.QueryRow(ctx, query,
		r.Name, r.Description, r.Timezone, handoffTime, r.HandoffDay,
		weeksAhead, maxConsec, r.IsFollowTheSun,
		uuidToPgtype(r.LinkedRosterID),
		parseOptionalTime(r.ActiveHoursStart), parseOptionalTime(r.ActiveHoursEnd),
		uuidToPgtype(r.EscalationPolicyID), endDate,
	))
}

func (s *Store) GetRoster(ctx context.Context, id uuid.UUID) (RosterResponse, error) {
	query := `SELECT id, name, description, timezone, handoff_time, handoff_day,
	           schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	           linked_roster_id, active_hours_start, active_hours_end,
	           escalation_policy_id, end_date, is_active, created_at, updated_at
	          FROM rosters WHERE id = $1`
	return s.scanRoster(s.dbtx.QueryRow(ctx, query, id))
}

func (s *Store) ListRosters(ctx context.Context) ([]RosterResponse, error) {
	query := `SELECT id, name, description, timezone, handoff_time, handoff_day,
	           schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	           linked_roster_id, active_hours_start, active_hours_end,
	           escalation_policy_id, end_date, is_active, created_at, updated_at
	          FROM rosters ORDER BY created_at DESC`
	rows, err := s.dbtx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing rosters: %w", err)
	}
	defer rows.Close()

	var result []RosterResponse
	for rows.Next() {
		r, err := s.scanRosterFromRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	if result == nil {
		result = []RosterResponse{}
	}
	return result, nil
}

func (s *Store) UpdateRoster(ctx context.Context, id uuid.UUID, r UpdateRosterRequest) (RosterResponse, error) {
	handoffTime, err := parseHandoffTime(r.HandoffTime)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("invalid handoff_time: %w", err)
	}

	weeksAhead := r.ScheduleWeeksAhead
	if weeksAhead <= 0 {
		weeksAhead = 12
	}
	maxConsec := r.MaxConsecutiveWeeks
	if maxConsec <= 0 {
		maxConsec = 2
	}

	var endDate pgtype.Date
	if r.EndDate != nil && *r.EndDate != "" {
		endDate, err = parseDate(*r.EndDate)
		if err != nil {
			return RosterResponse{}, fmt.Errorf("invalid end_date: %w", err)
		}
	}

	query := `UPDATE rosters SET name=$2, description=$3, timezone=$4, handoff_time=$5,
	           handoff_day=$6, schedule_weeks_ahead=$7, max_consecutive_weeks=$8,
	           end_date=$9, updated_at=now()
	          WHERE id=$1
	          RETURNING id, name, description, timezone, handoff_time, handoff_day,
	           schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	           linked_roster_id, active_hours_start, active_hours_end,
	           escalation_policy_id, end_date, is_active, created_at, updated_at`

	return s.scanRoster(s.dbtx.QueryRow(ctx, query,
		id, r.Name, r.Description, r.Timezone, handoffTime,
		r.HandoffDay, weeksAhead, maxConsec, endDate,
	))
}

func (s *Store) SetRosterActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := s.dbtx.Exec(ctx, `UPDATE rosters SET is_active=$2, updated_at=now() WHERE id=$1`, id, active)
	return err
}

func (s *Store) DeleteRoster(ctx context.Context, id uuid.UUID) error {
	tag, err := s.dbtx.Exec(ctx, `DELETE FROM rosters WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting roster: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) scanRoster(row pgx.Row) (RosterResponse, error) {
	var r RosterResponse
	var handoffTime pgtype.Time
	var linkedRosterID, escalationPolicyID pgtype.UUID
	var activeHoursStart, activeHoursEnd pgtype.Time
	var endDate pgtype.Date
	var fts *bool

	err := row.Scan(
		&r.ID, &r.Name, &r.Description, &r.Timezone, &handoffTime, &r.HandoffDay,
		&r.ScheduleWeeksAhead, &r.MaxConsecutiveWeeks, &fts,
		&linkedRosterID, &activeHoursStart, &activeHoursEnd,
		&escalationPolicyID, &endDate, &r.IsActive, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return RosterResponse{}, err
	}
	r.HandoffTime = pgtypeTimeToString(handoffTime)
	r.LinkedRosterID = pgtypeUUIDToPtr(linkedRosterID)
	r.EscalationPolicyID = pgtypeUUIDToPtr(escalationPolicyID)
	r.EndDate = pgtypeDateToPtr(endDate)
	if activeHoursStart.Valid {
		s := pgtypeTimeToString(activeHoursStart)
		r.ActiveHoursStart = &s
	}
	if activeHoursEnd.Valid {
		s := pgtypeTimeToString(activeHoursEnd)
		r.ActiveHoursEnd = &s
	}
	if fts != nil {
		r.IsFollowTheSun = *fts
	}
	return r, nil
}

func (s *Store) scanRosterFromRows(rows pgx.Rows) (RosterResponse, error) {
	var r RosterResponse
	var handoffTime pgtype.Time
	var linkedRosterID, escalationPolicyID pgtype.UUID
	var activeHoursStart, activeHoursEnd pgtype.Time
	var endDate pgtype.Date
	var fts *bool

	err := rows.Scan(
		&r.ID, &r.Name, &r.Description, &r.Timezone, &handoffTime, &r.HandoffDay,
		&r.ScheduleWeeksAhead, &r.MaxConsecutiveWeeks, &fts,
		&linkedRosterID, &activeHoursStart, &activeHoursEnd,
		&escalationPolicyID, &endDate, &r.IsActive, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return RosterResponse{}, fmt.Errorf("scanning roster row: %w", err)
	}
	r.HandoffTime = pgtypeTimeToString(handoffTime)
	r.LinkedRosterID = pgtypeUUIDToPtr(linkedRosterID)
	r.EscalationPolicyID = pgtypeUUIDToPtr(escalationPolicyID)
	r.EndDate = pgtypeDateToPtr(endDate)
	if activeHoursStart.Valid {
		s := pgtypeTimeToString(activeHoursStart)
		r.ActiveHoursStart = &s
	}
	if activeHoursEnd.Valid {
		s := pgtypeTimeToString(activeHoursEnd)
		r.ActiveHoursEnd = &s
	}
	if fts != nil {
		r.IsFollowTheSun = *fts
	}
	return r, nil
}

// =====================
// Member operations
// =====================

func (s *Store) ListMembers(ctx context.Context, rosterID uuid.UUID) ([]MemberResponse, error) {
	query := `SELECT rm.id, rm.roster_id, rm.user_id, COALESCE(u.display_name, rm.user_id::text),
	                 rm.is_active, rm.joined_at, rm.left_at
	          FROM roster_members rm
	          LEFT JOIN users u ON u.id = rm.user_id
	          WHERE rm.roster_id = $1
	          ORDER BY rm.is_active DESC, rm.joined_at`
	rows, err := s.dbtx.Query(ctx, query, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing roster members: %w", err)
	}
	defer rows.Close()

	var result []MemberResponse
	for rows.Next() {
		var m MemberResponse
		if err := rows.Scan(&m.ID, &m.RosterID, &m.UserID, &m.DisplayName,
			&m.IsActive, &m.JoinedAt, &m.LeftAt); err != nil {
			return nil, fmt.Errorf("scanning member row: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []MemberResponse{}
	}
	return result, nil
}

func (s *Store) ListActiveMembers(ctx context.Context, rosterID uuid.UUID) ([]MemberResponse, error) {
	query := `SELECT rm.id, rm.roster_id, rm.user_id, COALESCE(u.display_name, rm.user_id::text),
	                 rm.is_active, rm.joined_at, rm.left_at
	          FROM roster_members rm
	          LEFT JOIN users u ON u.id = rm.user_id
	          WHERE rm.roster_id = $1 AND rm.is_active = true
	          ORDER BY rm.joined_at`
	rows, err := s.dbtx.Query(ctx, query, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing active roster members: %w", err)
	}
	defer rows.Close()

	var result []MemberResponse
	for rows.Next() {
		var m MemberResponse
		if err := rows.Scan(&m.ID, &m.RosterID, &m.UserID, &m.DisplayName,
			&m.IsActive, &m.JoinedAt, &m.LeftAt); err != nil {
			return nil, fmt.Errorf("scanning member row: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []MemberResponse{}
	}
	return result, nil
}

func (s *Store) AddMember(ctx context.Context, rosterID, userID uuid.UUID) (MemberResponse, error) {
	query := `INSERT INTO roster_members (roster_id, user_id, is_active, joined_at)
	          VALUES ($1, $2, true, now())
	          ON CONFLICT (roster_id, user_id) DO UPDATE SET is_active = true, left_at = NULL
	          RETURNING id, roster_id, user_id, is_active, joined_at, left_at`
	var m MemberResponse
	err := s.dbtx.QueryRow(ctx, query, rosterID, userID).Scan(
		&m.ID, &m.RosterID, &m.UserID, &m.IsActive, &m.JoinedAt, &m.LeftAt,
	)
	if err != nil {
		return MemberResponse{}, fmt.Errorf("adding roster member: %w", err)
	}
	displayName, _ := s.GetUserDisplayName(ctx, userID)
	m.DisplayName = displayName
	return m, nil
}

func (s *Store) DeactivateMember(ctx context.Context, rosterID, userID uuid.UUID) error {
	_, err := s.dbtx.Exec(ctx,
		`UPDATE roster_members SET is_active = false, left_at = now() WHERE roster_id = $1 AND user_id = $2`,
		rosterID, userID)
	return err
}

func (s *Store) SetMemberActive(ctx context.Context, rosterID, userID uuid.UUID, active bool) error {
	if active {
		_, err := s.dbtx.Exec(ctx,
			`UPDATE roster_members SET is_active = true, left_at = NULL WHERE roster_id = $1 AND user_id = $2`,
			rosterID, userID)
		return err
	}
	return s.DeactivateMember(ctx, rosterID, userID)
}

// GetUserDisplayName fetches the display_name for a single user.
func (s *Store) GetUserDisplayName(ctx context.Context, userID uuid.UUID) (string, error) {
	var name string
	err := s.dbtx.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1`, userID).Scan(&name)
	if err != nil {
		return userID.String(), err
	}
	return name, nil
}

// =====================
// Schedule operations
// =====================

func (s *Store) ListSchedule(ctx context.Context, rosterID uuid.UUID, from, to time.Time) ([]ScheduleEntry, error) {
	query := `SELECT rs.id, rs.roster_id, rs.week_start, rs.week_end,
	                 rs.primary_user_id, COALESCE(up.display_name, ''),
	                 rs.secondary_user_id, COALESCE(us.display_name, ''),
	                 rs.is_locked, rs.generated, rs.notes,
	                 rs.created_at, rs.updated_at
	          FROM roster_schedule rs
	          LEFT JOIN users up ON up.id = rs.primary_user_id
	          LEFT JOIN users us ON us.id = rs.secondary_user_id
	          WHERE rs.roster_id = $1 AND rs.week_start >= $2 AND rs.week_start <= $3
	          ORDER BY rs.week_start`
	rows, err := s.dbtx.Query(ctx, query, rosterID, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing schedule: %w", err)
	}
	defer rows.Close()

	var result []ScheduleEntry
	for rows.Next() {
		var e ScheduleEntry
		var weekStart, weekEnd time.Time
		var primaryUID, secondaryUID pgtype.UUID
		if err := rows.Scan(&e.ID, &e.RosterID, &weekStart, &weekEnd,
			&primaryUID, &e.PrimaryDisplayName,
			&secondaryUID, &e.SecondaryDisplayName,
			&e.IsLocked, &e.Generated, &e.Notes,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning schedule row: %w", err)
		}
		e.WeekStart = weekStart.Format("2006-01-02")
		e.WeekEnd = weekEnd.Format("2006-01-02")
		e.PrimaryUserID = pgtypeUUIDToPtr(primaryUID)
		e.SecondaryUserID = pgtypeUUIDToPtr(secondaryUID)
		result = append(result, e)
	}
	if result == nil {
		result = []ScheduleEntry{}
	}
	return result, nil
}

func (s *Store) GetScheduleWeek(ctx context.Context, rosterID uuid.UUID, weekStart time.Time) (*ScheduleEntry, error) {
	query := `SELECT rs.id, rs.roster_id, rs.week_start, rs.week_end,
	                 rs.primary_user_id, COALESCE(up.display_name, ''),
	                 rs.secondary_user_id, COALESCE(us.display_name, ''),
	                 rs.is_locked, rs.generated, rs.notes,
	                 rs.created_at, rs.updated_at
	          FROM roster_schedule rs
	          LEFT JOIN users up ON up.id = rs.primary_user_id
	          LEFT JOIN users us ON us.id = rs.secondary_user_id
	          WHERE rs.roster_id = $1 AND rs.week_start = $2`
	var e ScheduleEntry
	var ws, we time.Time
	var primaryUID, secondaryUID pgtype.UUID
	err := s.dbtx.QueryRow(ctx, query, rosterID, weekStart).Scan(
		&e.ID, &e.RosterID, &ws, &we,
		&primaryUID, &e.PrimaryDisplayName,
		&secondaryUID, &e.SecondaryDisplayName,
		&e.IsLocked, &e.Generated, &e.Notes,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting schedule week: %w", err)
	}
	e.WeekStart = ws.Format("2006-01-02")
	e.WeekEnd = we.Format("2006-01-02")
	e.PrimaryUserID = pgtypeUUIDToPtr(primaryUID)
	e.SecondaryUserID = pgtypeUUIDToPtr(secondaryUID)
	return &e, nil
}

// GetScheduleForTime finds the schedule entry covering a specific timestamp.
func (s *Store) GetScheduleForTime(ctx context.Context, rosterID uuid.UUID, at time.Time) (*ScheduleEntry, error) {
	query := `SELECT rs.id, rs.roster_id, rs.week_start, rs.week_end,
	                 rs.primary_user_id, COALESCE(up.display_name, ''),
	                 rs.secondary_user_id, COALESCE(us.display_name, ''),
	                 rs.is_locked, rs.generated, rs.notes,
	                 rs.created_at, rs.updated_at
	          FROM roster_schedule rs
	          LEFT JOIN users up ON up.id = rs.primary_user_id
	          LEFT JOIN users us ON us.id = rs.secondary_user_id
	          WHERE rs.roster_id = $1 AND rs.week_start <= $2 AND rs.week_end > $2`
	var e ScheduleEntry
	var ws, we time.Time
	var primaryUID, secondaryUID pgtype.UUID
	err := s.dbtx.QueryRow(ctx, query, rosterID, at).Scan(
		&e.ID, &e.RosterID, &ws, &we,
		&primaryUID, &e.PrimaryDisplayName,
		&secondaryUID, &e.SecondaryDisplayName,
		&e.IsLocked, &e.Generated, &e.Notes,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting schedule for time: %w", err)
	}
	e.WeekStart = ws.Format("2006-01-02")
	e.WeekEnd = we.Format("2006-01-02")
	e.PrimaryUserID = pgtypeUUIDToPtr(primaryUID)
	e.SecondaryUserID = pgtypeUUIDToPtr(secondaryUID)
	return &e, nil
}

func (s *Store) UpsertScheduleWeek(ctx context.Context, rosterID uuid.UUID,
	weekStart, weekEnd time.Time,
	primaryUID, secondaryUID *uuid.UUID,
	isLocked, generated bool,
	notes *string) (*ScheduleEntry, error) {

	query := `INSERT INTO roster_schedule (roster_id, week_start, week_end,
	           primary_user_id, secondary_user_id, is_locked, generated, notes)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	          ON CONFLICT (roster_id, week_start) DO UPDATE SET
	           primary_user_id = EXCLUDED.primary_user_id,
	           secondary_user_id = EXCLUDED.secondary_user_id,
	           is_locked = EXCLUDED.is_locked,
	           generated = EXCLUDED.generated,
	           notes = EXCLUDED.notes,
	           updated_at = now()
	          RETURNING id, created_at, updated_at`

	var pUID, sUID pgtype.UUID
	if primaryUID != nil {
		pUID = pgtype.UUID{Bytes: *primaryUID, Valid: true}
	}
	if secondaryUID != nil {
		sUID = pgtype.UUID{Bytes: *secondaryUID, Valid: true}
	}

	var id uuid.UUID
	var createdAt, updatedAt time.Time
	err := s.dbtx.QueryRow(ctx, query,
		rosterID, weekStart, weekEnd, pUID, sUID, isLocked, generated, notes,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting schedule week: %w", err)
	}

	// Resolve display names.
	pName, sName := "", ""
	if primaryUID != nil {
		pName, _ = s.GetUserDisplayName(ctx, *primaryUID)
	}
	if secondaryUID != nil {
		sName, _ = s.GetUserDisplayName(ctx, *secondaryUID)
	}

	return &ScheduleEntry{
		ID:                   id,
		RosterID:             rosterID,
		WeekStart:            weekStart.Format("2006-01-02"),
		WeekEnd:              weekEnd.Format("2006-01-02"),
		PrimaryUserID:        primaryUID,
		PrimaryDisplayName:   pName,
		SecondaryUserID:      secondaryUID,
		SecondaryDisplayName: sName,
		IsLocked:             isLocked,
		Generated:            generated,
		Notes:                notes,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

func (s *Store) UnlockScheduleWeek(ctx context.Context, rosterID uuid.UUID, weekStart time.Time) error {
	tag, err := s.dbtx.Exec(ctx,
		`UPDATE roster_schedule SET is_locked = false, updated_at = now() WHERE roster_id = $1 AND week_start = $2`,
		rosterID, weekStart)
	if err != nil {
		return fmt.Errorf("unlocking schedule week: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CountPrimaryWeeks counts how many weeks a user has been primary for a roster.
func (s *Store) CountPrimaryWeeks(ctx context.Context, rosterID, userID uuid.UUID) (int, error) {
	var count int
	err := s.dbtx.QueryRow(ctx,
		`SELECT COUNT(*) FROM roster_schedule WHERE roster_id = $1 AND primary_user_id = $2`,
		rosterID, userID).Scan(&count)
	return count, err
}

// CountSecondaryWeeks counts how many weeks a user has been secondary for a roster.
func (s *Store) CountSecondaryWeeks(ctx context.Context, rosterID, userID uuid.UUID) (int, error) {
	var count int
	err := s.dbtx.QueryRow(ctx,
		`SELECT COUNT(*) FROM roster_schedule WHERE roster_id = $1 AND secondary_user_id = $2`,
		rosterID, userID).Scan(&count)
	return count, err
}

// ListOverridesInRange lists overrides for a roster within a time range.
func (s *Store) ListOverridesInRange(ctx context.Context, rosterID uuid.UUID, from, to time.Time) ([]OverrideResponse, error) {
	query := `SELECT ro.id, ro.roster_id, ro.user_id, COALESCE(u.display_name, ro.user_id::text),
	                 ro.start_at, ro.end_at, ro.reason, ro.created_by, ro.created_at
	          FROM roster_overrides ro
	          LEFT JOIN users u ON u.id = ro.user_id
	          WHERE ro.roster_id = $1 AND ro.end_at > $2 AND ro.start_at < $3
	          ORDER BY ro.start_at`
	rows, err := s.dbtx.Query(ctx, query, rosterID, from, to)
	if err != nil {
		return nil, fmt.Errorf("listing overrides in range: %w", err)
	}
	defer rows.Close()

	var result []OverrideResponse
	for rows.Next() {
		var o OverrideResponse
		var createdBy pgtype.UUID
		if err := rows.Scan(&o.ID, &o.RosterID, &o.UserID, &o.DisplayName,
			&o.StartAt, &o.EndAt, &o.Reason, &createdBy, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning override row: %w", err)
		}
		o.CreatedBy = pgtypeUUIDToPtr(createdBy)
		result = append(result, o)
	}
	return result, nil
}

// =====================
// Override operations (unchanged)
// =====================

func (s *Store) ListOverrides(ctx context.Context, rosterID uuid.UUID) ([]OverrideResponse, error) {
	query := `SELECT ro.id, ro.roster_id, ro.user_id, COALESCE(u.display_name, ro.user_id::text),
	                 ro.start_at, ro.end_at, ro.reason, ro.created_by, ro.created_at
	          FROM roster_overrides ro
	          LEFT JOIN users u ON u.id = ro.user_id
	          WHERE ro.roster_id = $1
	          ORDER BY ro.start_at DESC`
	rows, err := s.dbtx.Query(ctx, query, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing roster overrides: %w", err)
	}
	defer rows.Close()

	var result []OverrideResponse
	for rows.Next() {
		var o OverrideResponse
		var createdBy pgtype.UUID
		if err := rows.Scan(&o.ID, &o.RosterID, &o.UserID, &o.DisplayName,
			&o.StartAt, &o.EndAt, &o.Reason, &createdBy, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning override row: %w", err)
		}
		o.CreatedBy = pgtypeUUIDToPtr(createdBy)
		result = append(result, o)
	}
	if result == nil {
		result = []OverrideResponse{}
	}
	return result, nil
}

func (s *Store) CreateOverride(ctx context.Context, rosterID, userID uuid.UUID,
	startAt, endAt time.Time, reason *string, createdBy pgtype.UUID) (OverrideResponse, error) {

	query := `INSERT INTO roster_overrides (roster_id, user_id, start_at, end_at, reason, created_by)
	          VALUES ($1, $2, $3, $4, $5, $6)
	          RETURNING id, roster_id, user_id, start_at, end_at, reason, created_by, created_at`
	var o OverrideResponse
	var cb pgtype.UUID
	err := s.dbtx.QueryRow(ctx, query, rosterID, userID, startAt, endAt, reason, createdBy).Scan(
		&o.ID, &o.RosterID, &o.UserID, &o.StartAt, &o.EndAt, &o.Reason, &cb, &o.CreatedAt,
	)
	if err != nil {
		return OverrideResponse{}, fmt.Errorf("creating roster override: %w", err)
	}
	o.DisplayName, _ = s.GetUserDisplayName(ctx, userID)
	o.CreatedBy = pgtypeUUIDToPtr(cb)
	return o, nil
}

func (s *Store) DeleteOverride(ctx context.Context, overrideID uuid.UUID) error {
	_, err := s.dbtx.Exec(ctx, `DELETE FROM roster_overrides WHERE id = $1`, overrideID)
	return err
}

func (s *Store) GetActiveOverride(ctx context.Context, rosterID uuid.UUID, at time.Time) (*OverrideResponse, error) {
	query := `SELECT ro.id, ro.roster_id, ro.user_id, COALESCE(u.display_name, ro.user_id::text),
	                 ro.start_at, ro.end_at, ro.reason, ro.created_by, ro.created_at
	          FROM roster_overrides ro
	          LEFT JOIN users u ON u.id = ro.user_id
	          WHERE ro.roster_id = $1 AND ro.start_at <= $2 AND ro.end_at > $2
	          LIMIT 1`
	var o OverrideResponse
	var createdBy pgtype.UUID
	err := s.dbtx.QueryRow(ctx, query, rosterID, at).Scan(
		&o.ID, &o.RosterID, &o.UserID, &o.DisplayName,
		&o.StartAt, &o.EndAt, &o.Reason, &createdBy, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking active override: %w", err)
	}
	o.CreatedBy = pgtypeUUIDToPtr(createdBy)
	return &o, nil
}
