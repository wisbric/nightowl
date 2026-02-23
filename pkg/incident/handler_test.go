package incident

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func sampleRow() IncidentRow {
	return IncidentRow{
		ID:              uuid.New(),
		Title:           "Pod CrashLoopBackOff",
		Fingerprints:    []string{"fp1"},
		Severity:        "critical",
		Tags:            []string{"k8s"},
		Services:        []string{"payment-service"},
		Clusters:        []string{},
		Namespaces:      []string{},
		ErrorPatterns:   []string{},
		ResolutionCount: 0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func TestCreateIncident_Validation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing title",
			body:       `{"severity":"critical"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "title too short",
			body:       `{"title":"ab","severity":"warning"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid severity",
			body:       `{"title":"test incident","severity":"extreme"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "missing severity",
			body:       `{"title":"test incident"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid JSON",
			body:       `{bad}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
	}

	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestGetIncident_InvalidID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	r := httptest.NewRequest(http.MethodGet, "/incidents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateIncident_Validation(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing required fields",
			body:       `{}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid severity",
			body:       `{"title":"test","severity":"wrong"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			r := httptest.NewRequest(http.MethodPut, "/incidents/"+id.String(), strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestDeleteIncident_InvalidID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	r := httptest.NewRequest(http.MethodDelete, "/incidents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListHistory_InvalidID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	r := httptest.NewRequest(http.MethodGet, "/incidents/not-a-uuid/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSearch_MissingQuery(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	r := httptest.NewRequest(http.MethodGet, "/incidents/search", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSearch_InvalidLimit(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	tests := []struct {
		name  string
		query string
	}{
		{"negative", "/incidents/search?q=test&limit=-1"},
		{"zero", "/incidents/search?q=test&limit=0"},
		{"non-numeric", "/incidents/search?q=test&limit=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestFingerprint_EmptyFP(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	// chi requires a non-empty URL param, so /fingerprint/ results in 301 redirect
	// (trailing slash) or 404. The actual empty check is belt-and-suspenders.
	r := httptest.NewRequest(http.MethodGet, "/incidents/fingerprint/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	// chi returns 301 for trailing slash redirect
	if w.Code != http.StatusMovedPermanently && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 301/400/404", w.Code)
	}
}

func TestSearchResult_JSONSerialization(t *testing.T) {
	result := SearchResult{
		ID:                uuid.New(),
		Title:             "Test Incident",
		Severity:          "critical",
		Services:          []string{"svc-a"},
		Tags:              []string{"k8s"},
		Rank:              0.75,
		TitleHighlight:    "<mark>Test</mark> Incident",
		SymptomsHighlight: "",
		SolutionHighlight: "",
		ResolutionCount:   3,
		CreatedAt:         time.Now(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal SearchResult: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded["rank"].(float64) != float64(result.Rank) {
		t.Errorf("rank = %v, want %v", decoded["rank"], result.Rank)
	}
	if decoded["title_highlight"] != result.TitleHighlight {
		t.Errorf("title_highlight = %v, want %v", decoded["title_highlight"], result.TitleHighlight)
	}
}

func TestComputeDiff(t *testing.T) {
	old := IncidentRow{
		Title:    "Old Title",
		Severity: "warning",
		Tags:     []string{"a"},
	}
	new := IncidentRow{
		Title:    "New Title",
		Severity: "critical",
		Tags:     []string{"a", "b"},
	}

	diff := computeDiff(old, new)

	if _, ok := diff["title"]; !ok {
		t.Error("expected title in diff")
	}
	if _, ok := diff["severity"]; !ok {
		t.Error("expected severity in diff")
	}
	if _, ok := diff["tags"]; !ok {
		t.Error("expected tags in diff")
	}
	// Unchanged fields should not appear.
	if _, ok := diff["symptoms"]; ok {
		t.Error("symptoms should not be in diff (both nil)")
	}
}

func TestComputeDiff_NoDifference(t *testing.T) {
	row := sampleRow()
	diff := computeDiff(row, row)

	if len(diff) != 0 {
		t.Errorf("expected empty diff for identical rows, got %d entries", len(diff))
	}
}

func TestIncidentRowToResponse(t *testing.T) {
	row := sampleRow()
	resp := row.ToResponse()

	if resp.ID != row.ID {
		t.Errorf("ID = %v, want %v", resp.ID, row.ID)
	}
	if resp.Title != row.Title {
		t.Errorf("Title = %q, want %q", resp.Title, row.Title)
	}
	if resp.Severity != row.Severity {
		t.Errorf("Severity = %q, want %q", resp.Severity, row.Severity)
	}
	// Nil slices should become empty slices (not null in JSON).
	if resp.Clusters == nil {
		t.Error("Clusters should be [] not nil")
	}
	// Nullable UUID fields should be nil when not set.
	if resp.RunbookID != nil {
		t.Error("RunbookID should be nil")
	}
}

func TestEnsureSlice(t *testing.T) {
	if s := ensureSlice(nil); s == nil {
		t.Error("ensureSlice(nil) should return non-nil empty slice")
	}
	input := []string{"a", "b"}
	if s := ensureSlice(input); len(s) != 2 {
		t.Errorf("ensureSlice preserved slice, got len %d", len(s))
	}
}

func TestMerge_InvalidTargetID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	body := `{"source_id":"550e8400-e29b-41d4-a716-446655440000"}`
	r := httptest.NewRequest(http.MethodPost, "/incidents/not-a-uuid/merge", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestMerge_MissingSourceID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	id := uuid.New()
	body := `{}`
	r := httptest.NewRequest(http.MethodPost, "/incidents/"+id.String()+"/merge", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestMerge_InvalidSourceID(t *testing.T) {
	h := NewHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/incidents", h.Routes())

	id := uuid.New()
	body := `{"source_id":"not-valid"}`
	r := httptest.NewRequest(http.MethodPost, "/incidents/"+id.String()+"/merge", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestUnionSlice(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{"both empty", nil, nil, []string{}},
		{"a only", []string{"x", "y"}, nil, []string{"x", "y"}},
		{"b only", nil, []string{"a"}, []string{"a"}},
		{"no overlap", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"with overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"duplicates in a", []string{"a", "a"}, []string{"b"}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unionSlice(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBestSeverity(t *testing.T) {
	tests := []struct {
		a, b string
		want string
	}{
		{"info", "warning", "warning"},
		{"critical", "warning", "critical"},
		{"major", "critical", "critical"},
		{"warning", "warning", "warning"},
		{"info", "critical", "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			if got := bestSeverity(tt.a, tt.b); got != tt.want {
				t.Errorf("bestSeverity(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestBestText(t *testing.T) {
	short := "short"
	long := "this is a longer solution text"

	t.Run("both nil", func(t *testing.T) {
		if got := bestText(nil, nil); got != nil {
			t.Errorf("expected nil, got %q", *got)
		}
	})
	t.Run("a nil", func(t *testing.T) {
		got := bestText(nil, &short)
		if got == nil || *got != short {
			t.Errorf("expected %q", short)
		}
	})
	t.Run("b nil", func(t *testing.T) {
		got := bestText(&short, nil)
		if got == nil || *got != short {
			t.Errorf("expected %q", short)
		}
	})
	t.Run("b longer", func(t *testing.T) {
		got := bestText(&short, &long)
		if got == nil || *got != long {
			t.Errorf("expected %q, got %q", long, *got)
		}
	})
	t.Run("a longer", func(t *testing.T) {
		got := bestText(&long, &short)
		if got == nil || *got != long {
			t.Errorf("expected %q, got %q", long, *got)
		}
	})
}

func TestParseUUIDPtr(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result, err := ParseUUIDPtr(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Valid {
			t.Error("expected invalid UUID for nil input")
		}
	})

	t.Run("valid UUID", func(t *testing.T) {
		s := "550e8400-e29b-41d4-a716-446655440000"
		result, err := ParseUUIDPtr(&s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Valid {
			t.Error("expected valid UUID")
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		s := "not-a-uuid"
		_, err := ParseUUIDPtr(&s)
		if err == nil {
			t.Error("expected error for invalid UUID")
		}
	})
}
