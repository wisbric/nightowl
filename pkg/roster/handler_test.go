package roster

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter() chi.Router {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/rosters", h.Routes())
	return router
}

func TestCreateRoster_EmptyBody(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/rosters/", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRoster_InvalidJSON(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/rosters/", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateRoster_MissingName(t *testing.T) {
	router := newTestRouter()

	body := `{"timezone":"UTC","rotation_type":"weekly","rotation_length":7,"handoff_time":"09:00","start_date":"2026-01-01"}`
	r := httptest.NewRequest(http.MethodPost, "/rosters/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestGetRoster_InvalidID(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodGet, "/rosters/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetOnCall_InvalidTimestamp(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodGet, "/rosters/00000000-0000-0000-0000-000000000001/oncall?at=not-a-timestamp", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestAddMember_EmptyBody(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/rosters/00000000-0000-0000-0000-000000000001/members", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateOverride_EmptyBody(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/rosters/00000000-0000-0000-0000-000000000001/overrides", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
