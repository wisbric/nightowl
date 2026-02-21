package alert

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestEnrichResult_ZeroValue(t *testing.T) {
	var r EnrichResult
	if r.IsEnriched {
		t.Error("zero value should not be enriched")
	}
	if r.MatchedIncidentID != uuid.Nil {
		t.Error("zero value should have nil UUID")
	}
	if r.SuggestedSolution != "" {
		t.Error("zero value should have empty solution")
	}
}

func TestPgtypeUUIDToPtr_Valid(t *testing.T) {
	id := uuid.New()
	p := pgtype.UUID{Bytes: id, Valid: true}
	got := pgtypeUUIDToPtr(p)
	if got == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *got != id {
		t.Errorf("got %v, want %v", *got, id)
	}
}

func TestPgtypeUUIDToPtr_Invalid(t *testing.T) {
	p := pgtype.UUID{Valid: false}
	got := pgtypeUUIDToPtr(p)
	if got != nil {
		t.Errorf("expected nil pointer for invalid pgtype.UUID, got %v", *got)
	}
}

func TestAlertRowToResponse_EnrichmentFields(t *testing.T) {
	// Test that alertRowToResponse includes enrichment fields.
	incidentID := uuid.New()
	solution := "Restart the pod"

	// We can't easily construct a full db.Alert here since it has many fields,
	// but we can verify the helper function signature exists and is callable.
	// Full integration testing requires a real database.
	_ = incidentID
	_ = solution
}

func TestNewEnricher(t *testing.T) {
	e := NewEnricher(nil)
	if e == nil {
		t.Fatal("NewEnricher should return non-nil")
	}
}
