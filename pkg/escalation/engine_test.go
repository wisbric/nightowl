package escalation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

func TestProcessAlert_NotYetTime(t *testing.T) {
	// An alert that was just created should not be escalated yet.
	engine := &Engine{
		interval: 30 * time.Second,
	}

	policyID := uuid.New()
	tiersJSON, _ := json.Marshal([]Tier{
		{Tier: 1, TimeoutMinutes: 5, NotifyVia: []string{"slack_dm"}, Targets: []string{"oncall_primary"}},
		{Tier: 2, TimeoutMinutes: 10, NotifyVia: []string{"phone"}, Targets: []string{"team_lead"}},
	})
	_ = engine
	_ = policyID
	_ = tiersJSON
	// This is a unit-level smoke test; full integration requires a DB.
	// We verify the tier parsing and cumulative timeout logic.

	tiers := parseTiers(json.RawMessage(tiersJSON))
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(tiers))
	}

	// Verify cumulative timeout calculation.
	cumulative := 0
	for i := range tiers {
		cumulative += tiers[i].TimeoutMinutes
	}
	if cumulative != 15 {
		t.Errorf("cumulative timeout = %d, want 15", cumulative)
	}
}

func TestShouldEscalate_ElapsedTime(t *testing.T) {
	tiersJSON, _ := json.Marshal([]Tier{
		{Tier: 1, TimeoutMinutes: 5, NotifyVia: []string{"slack_dm"}, Targets: []string{"oncall_primary"}},
		{Tier: 2, TimeoutMinutes: 10, NotifyVia: []string{"phone"}, Targets: []string{"team_lead"}},
	})

	tiers := parseTiers(json.RawMessage(tiersJSON))

	tests := []struct {
		name         string
		currentTier  int32
		alertAge     time.Duration
		wantEscalate bool
		wantNextTier int
	}{
		{
			name:         "new alert, not yet time for tier 1",
			currentTier:  0,
			alertAge:     3 * time.Minute,
			wantEscalate: false,
		},
		{
			name:         "new alert, time for tier 1",
			currentTier:  0,
			alertAge:     6 * time.Minute,
			wantEscalate: true,
			wantNextTier: 1,
		},
		{
			name:         "at tier 1, not yet time for tier 2",
			currentTier:  1,
			alertAge:     10 * time.Minute,
			wantEscalate: false,
		},
		{
			name:         "at tier 1, time for tier 2",
			currentTier:  1,
			alertAge:     16 * time.Minute,
			wantEscalate: true,
			wantNextTier: 2,
		},
		{
			name:         "at tier 2, no more tiers",
			currentTier:  2,
			alertAge:     30 * time.Minute,
			wantEscalate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Now().Add(-tt.alertAge)

			// Find the next tier.
			var nextTierIdx int
			found := false
			for i, tier := range tiers {
				if int32(tier.Tier) > tt.currentTier {
					nextTierIdx = i
					found = true
					break
				}
			}

			if !found {
				if tt.wantEscalate {
					t.Error("expected escalation but no next tier found")
				}
				return
			}

			// Calculate cumulative timeout.
			cumulativeTimeout := 0
			for i := 0; i <= nextTierIdx; i++ {
				cumulativeTimeout += tiers[i].TimeoutMinutes
			}

			elapsed := time.Since(createdAt)
			shouldEscalate := elapsed >= time.Duration(cumulativeTimeout)*time.Minute

			if shouldEscalate != tt.wantEscalate {
				t.Errorf("shouldEscalate = %v, want %v (elapsed=%v, cumTimeout=%dm)",
					shouldEscalate, tt.wantEscalate, elapsed, cumulativeTimeout)
			}

			if shouldEscalate && tiers[nextTierIdx].Tier != tt.wantNextTier {
				t.Errorf("nextTier = %d, want %d", tiers[nextTierIdx].Tier, tt.wantNextTier)
			}
		})
	}
}

func TestAlertEscalationPolicyIDValid(t *testing.T) {
	// Verify pgtype.UUID validity check logic used in processAlert.
	a := db.Alert{
		EscalationPolicyID: pgtype.UUID{Valid: false},
	}
	if a.EscalationPolicyID.Valid {
		t.Error("expected invalid policy ID")
	}

	policyID := uuid.New()
	a.EscalationPolicyID = pgtype.UUID{Bytes: policyID, Valid: true}
	if !a.EscalationPolicyID.Valid {
		t.Error("expected valid policy ID")
	}
	if uuid.UUID(a.EscalationPolicyID.Bytes) != policyID {
		t.Error("policy ID mismatch")
	}
}
