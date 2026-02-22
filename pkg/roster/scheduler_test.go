package roster

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAlignToHandoffDay(t *testing.T) {
	tests := []struct {
		name       string
		date       time.Time
		handoffDay int // 0=Sun, 1=Mon
		wantDate   string
	}{
		{"Monday already on Monday", time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC), 1, "2026-02-23"},
		{"Wednesday aligns back to Monday", time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC), 1, "2026-02-23"},
		{"Sunday aligns back to Monday", time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), 1, "2026-02-23"},
		{"Saturday aligns back to Monday", time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC), 1, "2026-02-23"},
		{"Tuesday aligns back to Sunday", time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC), 0, "2026-02-22"},
		{"Sunday aligns to Sunday", time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC), 0, "2026-02-22"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignToHandoffDay(tt.date, tt.handoffDay)
			gotStr := got.Format("2006-01-02")
			if gotStr != tt.wantDate {
				t.Errorf("alignToHandoffDay(%v, %d) = %s, want %s", tt.date.Format("2006-01-02"), tt.handoffDay, gotStr, tt.wantDate)
			}
		})
	}
}

func TestPickPrimary_Fairness(t *testing.T) {
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	members := []MemberResponse{
		{UserID: u1, DisplayName: "A"},
		{UserID: u2, DisplayName: "B"},
		{UserID: u3, DisplayName: "C"},
	}

	// All start at 0.
	counts := map[uuid.UUID]int{u1: 0, u2: 0, u3: 0}
	got := pickPrimary(members, counts, nil, 0, 2)
	if got == nil {
		t.Fatal("expected non-nil primary")
	}
	// Should pick the first one (all equal, first in slice wins).
	if *got != u1 {
		t.Errorf("expected first member (all equal), got %s", *got)
	}

	// After u1 has served once, should pick u2.
	counts[u1] = 1
	got = pickPrimary(members, counts, &u1, 1, 2)
	if got == nil || *got != u2 {
		t.Errorf("expected u2 (least served), got %v", got)
	}
}

func TestPickPrimary_MaxConsecutive(t *testing.T) {
	u1, u2 := uuid.New(), uuid.New()
	members := []MemberResponse{
		{UserID: u1, DisplayName: "A"},
		{UserID: u2, DisplayName: "B"},
	}

	// u1 has served 0 weeks but has been consecutive for 2 weeks (max is 2).
	counts := map[uuid.UUID]int{u1: 2, u2: 2}
	got := pickPrimary(members, counts, &u1, 2, 2)
	if got == nil {
		t.Fatal("expected non-nil primary")
	}
	// Should pick u2 since u1 hit max consecutive.
	if *got != u2 {
		t.Errorf("expected u2 (u1 at max consecutive), got %s", *got)
	}
}

func TestPickSecondary_ExcludesPrimary(t *testing.T) {
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	members := []MemberResponse{
		{UserID: u1, DisplayName: "A"},
		{UserID: u2, DisplayName: "B"},
		{UserID: u3, DisplayName: "C"},
	}
	counts := map[uuid.UUID]int{u1: 0, u2: 0, u3: 0}

	got := pickSecondary(members, counts, u1)
	if got == nil {
		t.Fatal("expected non-nil secondary")
	}
	if *got == u1 {
		t.Error("secondary should not be the same as primary")
	}
}

func TestPickPrimary_SingleMember(t *testing.T) {
	u1 := uuid.New()
	members := []MemberResponse{{UserID: u1, DisplayName: "A"}}
	counts := map[uuid.UUID]int{u1: 5}

	got := pickPrimary(members, counts, &u1, 5, 2)
	if got == nil {
		t.Fatal("expected non-nil primary even with max consecutive (relaxed)")
	}
	if *got != u1 {
		t.Errorf("expected sole member, got %s", *got)
	}
}
