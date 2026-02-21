package runbook

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

func TestCreateRunbook_Validation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing title",
			body:       `{"content":"some content"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "title too short",
			body:       `{"title":"ab","content":"some content"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "missing content",
			body:       `{"title":"valid title"}`,
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

	h := NewHandler(nil)
	router := chi.NewRouter()
	router.Mount("/runbooks", h.Routes())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/runbooks", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestGetRunbook_InvalidID(t *testing.T) {
	h := NewHandler(nil)
	router := chi.NewRouter()
	router.Mount("/runbooks", h.Routes())

	r := httptest.NewRequest(http.MethodGet, "/runbooks/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateRunbook_Validation(t *testing.T) {
	h := NewHandler(nil)
	router := chi.NewRouter()
	router.Mount("/runbooks", h.Routes())

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
			name:       "title too short",
			body:       `{"title":"ab","content":"some content"}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			r := httptest.NewRequest(http.MethodPut, "/runbooks/"+id.String(), strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestDeleteRunbook_InvalidID(t *testing.T) {
	h := NewHandler(nil)
	router := chi.NewRouter()
	router.Mount("/runbooks", h.Routes())

	r := httptest.NewRequest(http.MethodDelete, "/runbooks/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunbookRowToResponse(t *testing.T) {
	cat := "kubernetes"
	row := RunbookRow{
		ID:         uuid.New(),
		Title:      "Pod CrashLoopBackOff",
		Content:    "## Steps\n1. Check logs",
		Category:   &cat,
		IsTemplate: true,
		Tags:       []string{"k8s", "pod"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	resp := row.ToResponse()

	if resp.ID != row.ID {
		t.Errorf("ID = %v, want %v", resp.ID, row.ID)
	}
	if resp.Title != row.Title {
		t.Errorf("Title = %q, want %q", resp.Title, row.Title)
	}
	if resp.Content != row.Content {
		t.Errorf("Content = %q, want %q", resp.Content, row.Content)
	}
	if !resp.IsTemplate {
		t.Error("IsTemplate should be true")
	}
	if resp.CreatedBy != nil {
		t.Error("CreatedBy should be nil")
	}
	if len(resp.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(resp.Tags))
	}
}

func TestRunbookResponse_JSONSerialization(t *testing.T) {
	cat := "kubernetes"
	resp := Response{
		ID:         uuid.New(),
		Title:      "Test Runbook",
		Content:    "# Steps",
		Category:   &cat,
		IsTemplate: false,
		Tags:       []string{"test"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded["title"] != resp.Title {
		t.Errorf("title = %v, want %v", decoded["title"], resp.Title)
	}
	if decoded["is_template"] != false {
		t.Errorf("is_template = %v, want false", decoded["is_template"])
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

func TestTemplateRunbooks(t *testing.T) {
	templates := TemplateRunbooks()

	if len(templates) != 7 {
		t.Errorf("expected 7 template runbooks, got %d", len(templates))
	}

	titles := make(map[string]bool)
	for _, tmpl := range templates {
		if tmpl.Title == "" {
			t.Error("template has empty title")
		}
		if tmpl.Content == "" {
			t.Error("template has empty content")
		}
		if tmpl.Category == nil {
			t.Error("template has nil category")
		}
		if titles[tmpl.Title] {
			t.Errorf("duplicate template title: %s", tmpl.Title)
		}
		titles[tmpl.Title] = true
	}
}
