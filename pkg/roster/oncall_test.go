package roster

import (
	"testing"
	"time"
)

func TestParseHandoffTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"09:00", false},
		{"00:00", false},
		{"23:59", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseHandoffTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHandoffTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("parseHandoffTime(%q) returned invalid time", tt.input)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2026-01-01", false},
		{"2026-12-31", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("parseDate(%q) returned invalid date", tt.input)
			}
		})
	}
}

func TestPgtypeTimeToString(t *testing.T) {
	ht, _ := parseHandoffTime("14:30")
	got := pgtypeTimeToString(ht)
	if got != "14:30" {
		t.Errorf("pgtypeTimeToString = %q, want 14:30", got)
	}
}

func TestIsInShiftWindow(t *testing.T) {
	svc := &Service{}
	roster := RosterResponse{
		Timezone:    "UTC",
		HandoffTime: "08:00",
	}

	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"within window", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), true},
		{"at start", time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC), true},
		{"before window", time.Date(2026, 1, 1, 7, 59, 0, 0, time.UTC), false},
		{"at end", time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC), false},
		{"after window", time.Date(2026, 1, 1, 21, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isInHandoffWindow(roster, tt.at)
			if got != tt.want {
				t.Errorf("isInShiftWindow(%v) = %v, want %v", tt.at, got, tt.want)
			}
		})
	}
}

func TestIsInShiftWindow_WrapsMidnight(t *testing.T) {
	svc := &Service{}
	roster := RosterResponse{
		Timezone:    "UTC",
		HandoffTime: "20:00",
	}

	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"within window before midnight", time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC), true},
		{"within window after midnight", time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC), true},
		{"before window", time.Date(2026, 1, 1, 19, 0, 0, 0, time.UTC), false},
		{"after window", time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isInHandoffWindow(roster, tt.at)
			if got != tt.want {
				t.Errorf("isInShiftWindow(%v) = %v, want %v", tt.at, got, tt.want)
			}
		})
	}
}
