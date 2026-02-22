package roster

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateICSFromSchedule_Empty(t *testing.T) {
	r := RosterResponse{
		ID:          uuid.New(),
		Name:        "Test Roster",
		Timezone:    "UTC",
		HandoffTime: "09:00",
	}

	ical := generateICSFromSchedule(r, nil, nil)
	if !strings.Contains(ical, "BEGIN:VCALENDAR") {
		t.Error("missing VCALENDAR start")
	}
	if !strings.Contains(ical, "END:VCALENDAR") {
		t.Error("missing VCALENDAR end")
	}
	if strings.Contains(ical, "BEGIN:VEVENT") {
		t.Error("expected no events for empty schedule")
	}
}

func TestGenerateICSFromSchedule_WithEntries(t *testing.T) {
	rosterID := uuid.New()
	primary := uuid.New()
	secondary := uuid.New()
	r := RosterResponse{
		ID:          rosterID,
		Name:        "Primary",
		Timezone:    "UTC",
		HandoffTime: "08:00",
	}

	schedule := []ScheduleEntry{
		{
			ID:                   uuid.New(),
			RosterID:             rosterID,
			WeekStart:            "2026-02-24",
			WeekEnd:              "2026-03-03",
			PrimaryUserID:        &primary,
			PrimaryDisplayName:   "Alice",
			SecondaryUserID:      &secondary,
			SecondaryDisplayName: "Bob",
		},
	}

	ical := generateICSFromSchedule(r, schedule, nil)

	if !strings.Contains(ical, "PRODID:-//NightOwl//Roster//EN") {
		t.Error("missing PRODID")
	}
	if !strings.Contains(ical, "BEGIN:VEVENT") {
		t.Error("expected events for schedule with entries")
	}
	if !strings.Contains(ical, "SUMMARY:On-Call: Alice") {
		t.Error("expected on-call summary with display name")
	}
	if !strings.Contains(ical, "Secondary: Bob") {
		t.Error("expected secondary in description")
	}
}

func TestGenerateICSFromSchedule_WithOverrides(t *testing.T) {
	rosterID := uuid.New()
	r := RosterResponse{
		ID:          rosterID,
		Name:        "Primary",
		Timezone:    "UTC",
		HandoffTime: "08:00",
	}

	reason := "vacation"
	overrides := []OverrideResponse{
		{
			ID:       uuid.New(),
			RosterID: rosterID,
			UserID:   uuid.New(),
			StartAt:  time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC),
			EndAt:    time.Date(2026, 2, 2, 8, 0, 0, 0, time.UTC),
			Reason:   &reason,
		},
	}

	ical := generateICSFromSchedule(r, nil, overrides)

	if !strings.Contains(ical, "SUMMARY:Override:") {
		t.Error("expected override event in calendar")
	}
	if !strings.Contains(ical, "vacation") {
		t.Error("expected override reason in calendar")
	}
}
