package roster

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GenerateSchedule creates schedule entries for the given roster.
// It respects locked weeks and distributes primary/secondary duty fairly.
func (s *Service) GenerateSchedule(ctx context.Context, rosterID uuid.UUID, from time.Time, weeks int) ([]ScheduleEntry, error) {
	roster, err := s.store.GetRoster(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("getting roster: %w", err)
	}

	members, err := s.store.ListActiveMembers(ctx, rosterID)
	if err != nil {
		return nil, fmt.Errorf("listing active members: %w", err)
	}

	if len(members) == 0 {
		s.logger.Warn("no active members for schedule generation", "roster_id", rosterID)
		return []ScheduleEntry{}, nil
	}

	// Align 'from' to the roster's handoff day.
	weekStart := alignToHandoffDay(from, roster.HandoffDay)

	// Build duty counts from schedule entries BEFORE the generation window only.
	// This avoids inflated counts from rows we're about to overwrite.
	primaryCount := make(map[uuid.UUID]int)
	secondaryCount := make(map[uuid.UUID]int)
	for _, m := range members {
		pc, _ := s.store.CountPrimaryWeeksBefore(ctx, rosterID, m.UserID, weekStart)
		primaryCount[m.UserID] = pc
		sc, _ := s.store.CountSecondaryWeeksBefore(ctx, rosterID, m.UserID, weekStart)
		secondaryCount[m.UserID] = sc
	}

	// Load existing schedule for the range so we can skip locked weeks.
	endDate := weekStart.AddDate(0, 0, weeks*7)
	existing, err := s.store.ListSchedule(ctx, rosterID, weekStart, endDate)
	if err != nil {
		return nil, fmt.Errorf("listing existing schedule: %w", err)
	}
	existingMap := make(map[string]*ScheduleEntry, len(existing))
	for i := range existing {
		existingMap[existing[i].WeekStart] = &existing[i]
	}

	// Track consecutive primary assignments for max_consecutive enforcement.
	var lastPrimary *uuid.UUID
	consecutiveCount := 0

	// Look back at the week before 'from' to seed consecutive tracking.
	prevWeek := weekStart.AddDate(0, 0, -7)
	prevEntry, _ := s.store.GetScheduleWeek(ctx, rosterID, prevWeek)
	if prevEntry != nil && prevEntry.PrimaryUserID != nil {
		lastPrimary = prevEntry.PrimaryUserID
		consecutiveCount = 1
		// Count further back.
		for i := 2; i <= roster.MaxConsecutiveWeeks; i++ {
			pw := weekStart.AddDate(0, 0, -7*i)
			pe, _ := s.store.GetScheduleWeek(ctx, rosterID, pw)
			if pe != nil && pe.PrimaryUserID != nil && *pe.PrimaryUserID == *lastPrimary {
				consecutiveCount++
			} else {
				break
			}
		}
	}

	var generated []ScheduleEntry
	for i := 0; i < weeks; i++ {
		ws := weekStart.AddDate(0, 0, i*7)
		we := ws.AddDate(0, 0, 7)
		wsKey := ws.Format("2006-01-02")

		// Skip locked weeks — but track them for consecutive counting.
		if e, ok := existingMap[wsKey]; ok && e.IsLocked {
			if e.PrimaryUserID != nil {
				if lastPrimary != nil && *lastPrimary == *e.PrimaryUserID {
					consecutiveCount++
				} else {
					lastPrimary = e.PrimaryUserID
					consecutiveCount = 1
				}
			}
			generated = append(generated, *e)
			continue
		}

		// Pick primary: least served, not exceeding max consecutive.
		primary := pickPrimary(members, primaryCount, lastPrimary, consecutiveCount, roster.MaxConsecutiveWeeks)
		var secondary *uuid.UUID
		if len(members) > 1 && primary != nil {
			secondary = pickSecondary(members, primaryCount, secondaryCount, *primary)
		}

		entry, err := s.store.UpsertScheduleWeek(ctx, rosterID, ws, we,
			primary, secondary, false, true, nil)
		if err != nil {
			return nil, fmt.Errorf("upserting schedule week %s: %w", wsKey, err)
		}
		generated = append(generated, *entry)

		// Update tracking.
		if primary != nil {
			primaryCount[*primary]++
			if lastPrimary != nil && *lastPrimary == *primary {
				consecutiveCount++
			} else {
				lastPrimary = primary
				consecutiveCount = 1
			}
		}
		if secondary != nil {
			secondaryCount[*secondary]++
		}
	}

	return generated, nil
}

// alignToHandoffDay finds the most recent handoff day on or before the given date.
// handoffDay: 0=Sunday, 1=Monday, ..., 6=Saturday.
func alignToHandoffDay(t time.Time, handoffDay int) time.Time {
	// Normalize to start of day.
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	current := int(d.Weekday()) // 0=Sunday
	diff := current - handoffDay
	if diff < 0 {
		diff += 7
	}
	return d.AddDate(0, 0, -diff)
}

// pickPrimary selects the member with the fewest primary weeks served,
// respecting max consecutive weeks constraint.
func pickPrimary(members []MemberResponse, primaryCount map[uuid.UUID]int,
	lastPrimary *uuid.UUID, consecutiveCount, maxConsecutive int) *uuid.UUID {

	var best *uuid.UUID
	bestCount := -1

	for _, m := range members {
		// Skip if this would exceed max consecutive.
		if maxConsecutive > 0 && lastPrimary != nil && m.UserID == *lastPrimary && consecutiveCount >= maxConsecutive {
			continue
		}

		count := primaryCount[m.UserID]
		if best == nil || count < bestCount {
			id := m.UserID
			best = &id
			bestCount = count
		}
	}

	// If all members are blocked by consecutive constraint, relax and pick least served.
	if best == nil && len(members) > 0 {
		for _, m := range members {
			count := primaryCount[m.UserID]
			if best == nil || count < bestCount {
				id := m.UserID
				best = &id
				bestCount = count
			}
		}
	}

	return best
}

// pickSecondary selects the member with the fewest total duty weeks (primary + secondary),
// excluding the current week's primary.
func pickSecondary(members []MemberResponse, primaryCount, secondaryCount map[uuid.UUID]int, primaryUID uuid.UUID) *uuid.UUID {
	var best *uuid.UUID
	bestTotal := -1

	for _, m := range members {
		if m.UserID == primaryUID {
			continue
		}
		total := primaryCount[m.UserID] + secondaryCount[m.UserID]
		if best == nil || total < bestTotal {
			id := m.UserID
			best = &id
			bestTotal = total
		}
	}
	return best
}
