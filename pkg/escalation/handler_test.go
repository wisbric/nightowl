package escalation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter() chi.Router {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/escalation-policies", h.Routes())
	return router
}

func TestCreatePolicy_EmptyBody(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/escalation-policies/", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreatePolicy_MissingName(t *testing.T) {
	router := newTestRouter()

	body := `{"tiers":[{"tier":1,"timeout_minutes":5,"notify_via":["slack_dm"],"targets":["oncall_primary"]}]}`
	r := httptest.NewRequest(http.MethodPost, "/escalation-policies/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestCreatePolicy_MissingTiers(t *testing.T) {
	router := newTestRouter()

	body := `{"name":"Test Policy"}`
	r := httptest.NewRequest(http.MethodPost, "/escalation-policies/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestGetPolicy_InvalidID(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodGet, "/escalation-policies/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDryRun_InvalidID(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/escalation-policies/not-a-uuid/dry-run", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestParseTiers(t *testing.T) {
	raw := json.RawMessage(`[
		{"tier":1,"timeout_minutes":5,"notify_via":["slack_dm"],"targets":["oncall_primary"]},
		{"tier":2,"timeout_minutes":10,"notify_via":["phone"],"targets":["team_lead"]}
	]`)

	tiers := parseTiers(raw)
	if len(tiers) != 2 {
		t.Fatalf("got %d tiers, want 2", len(tiers))
	}
	if tiers[0].TimeoutMinutes != 5 {
		t.Errorf("tier 1 timeout = %d, want 5", tiers[0].TimeoutMinutes)
	}
	if tiers[1].Tier != 2 {
		t.Errorf("tier 2 number = %d, want 2", tiers[1].Tier)
	}
}

func TestParseTiers_Invalid(t *testing.T) {
	raw := json.RawMessage(`not json`)
	tiers := parseTiers(raw)
	if len(tiers) != 0 {
		t.Errorf("expected empty tiers for invalid JSON, got %d", len(tiers))
	}
}
