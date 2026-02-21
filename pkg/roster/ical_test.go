package roster

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateICS_Empty(t *testing.T) {
	r := RosterResponse{
		ID:           uuid.New(),
		Name:         "Test Roster",
		Timezone:     "UTC",
		HandoffTime:  "09:00",
		StartDate:    "2026-01-01",
		RotationType: "weekly",
	}

	ical := generateICS(r, nil, nil)
	if !strings.Contains(ical, "BEGIN:VCALENDAR") {
		t.Error("missing VCALENDAR start")
	}
	if !strings.Contains(ical, "END:VCALENDAR") {
		t.Error("missing VCALENDAR end")
	}
	if strings.Contains(ical, "BEGIN:VEVENT") {
		t.Error("expected no events for empty member list")
	}
}

func TestGenerateICS_WithMembers(t *testing.T) {
	rosterID := uuid.New()
	r := RosterResponse{
		ID:             rosterID,
		Name:           "Primary",
		Timezone:       "UTC",
		HandoffTime:    "08:00",
		StartDate:      "2026-01-01",
		RotationType:   "weekly",
		RotationLength: 7,
	}

	members := []MemberResponse{
		{ID: uuid.New(), RosterID: rosterID, UserID: uuid.New(), Position: 0},
		{ID: uuid.New(), RosterID: rosterID, UserID: uuid.New(), Position: 1},
	}

	ical := generateICS(r, members, nil)

	if !strings.Contains(ical, "PRODID:-//NightOwl//Roster//EN") {
		t.Error("missing PRODID")
	}
	if !strings.Contains(ical, "BEGIN:VEVENT") {
		t.Error("expected events for roster with members")
	}
	if !strings.Contains(ical, "SUMMARY:On-Call:") {
		t.Error("expected on-call summary in events")
	}
}

func TestGenerateICS_WithOverrides(t *testing.T) {
	rosterID := uuid.New()
	r := RosterResponse{
		ID:             rosterID,
		Name:           "Primary",
		Timezone:       "UTC",
		HandoffTime:    "08:00",
		StartDate:      "2026-01-01",
		RotationType:   "weekly",
		RotationLength: 7,
	}

	members := []MemberResponse{
		{ID: uuid.New(), RosterID: rosterID, UserID: uuid.New(), Position: 0},
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

	ical := generateICS(r, members, overrides)

	if !strings.Contains(ical, "SUMMARY:Override:") {
		t.Error("expected override event in calendar")
	}
	if !strings.Contains(ical, "vacation") {
		t.Error("expected override reason in calendar")
	}
}
