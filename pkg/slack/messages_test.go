package slack

import (
	"testing"
)

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		severity string
		want     string
	}{
		{"critical", "🔴"},
		{"major", "🟠"},
		{"warning", "🟡"},
		{"info", "🔵"},
		{"unknown", "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := SeverityEmoji(tt.severity)
			if got != tt.want {
				t.Errorf("SeverityEmoji(%q) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long string", 10, "this is..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestAlertNotificationBlocks(t *testing.T) {
	alert := AlertInfo{
		AlertID:           "test-alert-id",
		Title:             "Pod CrashLoopBackOff",
		Severity:          "critical",
		Description:       "Pod is crashing repeatedly",
		Cluster:           "prod-de-01",
		Namespace:         "customer-api",
		Service:           "payment-gateway",
		OnCallUser:        "<@U123> (Stefan)",
		SuggestedSolution: "Increase memory limits",
	}

	blocks := AlertNotificationBlocks(alert)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// Should have: header + fields section + description + solution + actions = 5 blocks
	if len(blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(blocks))
	}
}

func TestAlertNotificationBlocks_Minimal(t *testing.T) {
	alert := AlertInfo{
		AlertID:  "test-id",
		Title:    "Test",
		Severity: "info",
	}

	blocks := AlertNotificationBlocks(alert)
	if len(blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// header + actions minimum
	if len(blocks) < 2 {
		t.Errorf("expected at least 2 blocks, got %d", len(blocks))
	}
}

func TestAlertAcknowledgedBlocks(t *testing.T) {
	blocks := AlertAcknowledgedBlocks("Pod CrashLoop", "<@U123>")
	if len(blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(blocks))
	}
}

func TestAlertResolvedBlocks_WithKBEntry(t *testing.T) {
	blocks := AlertResolvedBlocks("Pod CrashLoop", "<@U123>", true)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block (no KB prompt), got %d", len(blocks))
	}
}

func TestAlertResolvedBlocks_WithoutKBEntry(t *testing.T) {
	blocks := AlertResolvedBlocks("Pod CrashLoop", "<@U123>", false)
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks (with KB prompt), got %d", len(blocks))
	}
}

func TestSearchResultBlocks_Empty(t *testing.T) {
	blocks := SearchResultBlocks("test query", nil)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block for empty results, got %d", len(blocks))
	}
}

func TestSearchResultBlocks_WithResults(t *testing.T) {
	results := []SearchResult{
		{ID: "1", Title: "OOM Kill", Severity: "critical", Solution: "Increase memory"},
		{ID: "2", Title: "Disk Full", Severity: "warning", Solution: "Clean up logs"},
	}

	blocks := SearchResultBlocks("oom", results)
	if len(blocks) < 3 {
		t.Errorf("expected at least 3 blocks, got %d", len(blocks))
	}
}

func TestOnCallBlocks_Empty(t *testing.T) {
	blocks := OnCallBlocks(nil)
	if len(blocks) != 1 {
		t.Errorf("expected 1 block for empty entries, got %d", len(blocks))
	}
}

func TestOnCallBlocks_WithEntries(t *testing.T) {
	entries := []OnCallEntry{
		{RosterName: "Primary", UserDisplay: "Stefan", Timezone: "Pacific/Auckland"},
		{RosterName: "Backup", UserDisplay: "Hans", Timezone: "Europe/Berlin", IsOverride: true},
	}

	blocks := OnCallBlocks(entries)
	// header + 2 entries
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
}

func TestCreateIncidentModal(t *testing.T) {
	modal := CreateIncidentModal("alert-123", "Pod Crash", "crashing repeatedly", "critical")

	if modal.CallbackID != "create_incident_submit" {
		t.Errorf("callback_id = %q, want create_incident_submit", modal.CallbackID)
	}
	if len(modal.Blocks.BlockSet) != 4 {
		t.Errorf("expected 4 input blocks, got %d", len(modal.Blocks.BlockSet))
	}
}
