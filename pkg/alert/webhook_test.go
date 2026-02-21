package alert

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNormalizeSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"critical", "critical"},
		{"CRITICAL", "critical"},
		{"crit", "critical"},
		{"fatal", "critical"},
		{"emergency", "critical"},
		{"p1", "critical"},
		{"major", "major"},
		{"error", "major"},
		{"high", "major"},
		{"p2", "major"},
		{"warning", "warning"},
		{"warn", "warning"},
		{"medium", "warning"},
		{"p3", "warning"},
		{"info", "info"},
		{"informational", "info"},
		{"low", "info"},
		{"p4", "info"},
		{"p5", "info"},
		{"unknown", "warning"},
		{"", "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeSeverity(tt.input); got != tt.want {
				t.Errorf("normalizeSeverity(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"firing", "firing"},
		{"resolved", "resolved"},
		{"ok", "resolved"},
		{"inactive", "resolved"},
		{"RESOLVED", "resolved"},
		{"active", "firing"},
		{"", "firing"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeStatus(tt.input); got != tt.want {
				t.Errorf("normalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateFingerprint(t *testing.T) {
	fp1 := generateFingerprint("alert-a", json.RawMessage(`{"k":"v"}`))
	fp2 := generateFingerprint("alert-a", json.RawMessage(`{"k":"v"}`))
	fp3 := generateFingerprint("alert-b", json.RawMessage(`{"k":"v"}`))

	if fp1 != fp2 {
		t.Errorf("same inputs should produce same fingerprint: %q != %q", fp1, fp2)
	}
	if fp1 == fp3 {
		t.Error("different inputs should produce different fingerprints")
	}
	if len(fp1) != 32 {
		t.Errorf("fingerprint length = %d, want 32", len(fp1))
	}
}

func TestEnsureJSON(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := ensureJSON(nil)
		if string(got) != "{}" {
			t.Errorf("ensureJSON(nil) = %q, want {}", string(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := ensureJSON(json.RawMessage{})
		if string(got) != "{}" {
			t.Errorf("ensureJSON([]) = %q, want {}", string(got))
		}
	})

	t.Run("null input", func(t *testing.T) {
		got := ensureJSON(json.RawMessage("null"))
		if string(got) != "{}" {
			t.Errorf("ensureJSON(null) = %q, want {}", string(got))
		}
	})

	t.Run("valid input", func(t *testing.T) {
		input := json.RawMessage(`{"key":"value"}`)
		got := ensureJSON(input)
		if string(got) != `{"key":"value"}` {
			t.Errorf("ensureJSON = %q, want original", string(got))
		}
	})
}

func TestNormalizeAlertmanager(t *testing.T) {
	a := alertmanagerAlert{
		Status: "firing",
		Labels: map[string]string{
			"alertname": "PodCrashLoopBackOff",
			"severity":  "critical",
			"cluster":   "prod-eu-1",
			"namespace": "payments",
		},
		Annotations: map[string]string{
			"summary":     "Pod is in CrashLoopBackOff",
			"description": "Pod payment-gateway has been restarting",
		},
		Fingerprint: "abc123",
	}

	n := normalizeAlertmanager(a)

	if n.Title != "PodCrashLoopBackOff" {
		t.Errorf("Title = %q, want PodCrashLoopBackOff", n.Title)
	}
	if n.Severity != "critical" {
		t.Errorf("Severity = %q, want critical", n.Severity)
	}
	if n.Status != "firing" {
		t.Errorf("Status = %q, want firing", n.Status)
	}
	if n.Source != "alertmanager" {
		t.Errorf("Source = %q, want alertmanager", n.Source)
	}
	if n.Fingerprint != "abc123" {
		t.Errorf("Fingerprint = %q, want abc123", n.Fingerprint)
	}
	if n.Description == nil || *n.Description != "Pod is in CrashLoopBackOff" {
		t.Errorf("Description = %v, want summary annotation", n.Description)
	}
}

func TestNormalizeAlertmanager_MissingFields(t *testing.T) {
	a := alertmanagerAlert{
		Status: "firing",
		Labels: map[string]string{},
	}

	n := normalizeAlertmanager(a)

	if n.Title != "Unnamed Alertmanager Alert" {
		t.Errorf("Title = %q, want fallback title", n.Title)
	}
	if n.Severity != "warning" {
		t.Errorf("Severity = %q, want warning (default)", n.Severity)
	}
	if n.Fingerprint == "" {
		t.Error("Fingerprint should be auto-generated")
	}
	if n.Description != nil {
		t.Error("Description should be nil when no annotations")
	}
}

func TestNormalizeAlertmanager_Resolved(t *testing.T) {
	a := alertmanagerAlert{
		Status:      "resolved",
		Labels:      map[string]string{"alertname": "TestAlert"},
		Fingerprint: "fp1",
	}

	n := normalizeAlertmanager(a)

	if n.Status != "resolved" {
		t.Errorf("Status = %q, want resolved", n.Status)
	}
}

func TestNormalizeKeep(t *testing.T) {
	p := keepPayload{
		ID:          "keep-uuid-123",
		Name:        "HighMemoryUsage",
		Status:      "firing",
		Severity:    "warning",
		Source:      []string{"prometheus", "signoz"},
		Fingerprint: "keep-fp-456",
		Labels:      map[string]string{"env": "production"},
		Description: "Memory usage above 90%",
	}

	n := normalizeKeep(p)

	if n.Title != "HighMemoryUsage" {
		t.Errorf("Title = %q, want HighMemoryUsage", n.Title)
	}
	if n.Severity != "warning" {
		t.Errorf("Severity = %q, want warning", n.Severity)
	}
	if n.Source != "keep" {
		t.Errorf("Source = %q, want keep", n.Source)
	}
	if n.Fingerprint != "keep-fp-456" {
		t.Errorf("Fingerprint = %q, want keep-fp-456", n.Fingerprint)
	}
	if n.Description == nil || *n.Description != "Memory usage above 90%" {
		t.Errorf("Description = %v, want description text", n.Description)
	}

	// Verify annotations contain keep metadata.
	var ann map[string]any
	if err := json.Unmarshal(n.Annotations, &ann); err != nil {
		t.Fatalf("failed to unmarshal annotations: %v", err)
	}
	if ann["keep_id"] != "keep-uuid-123" {
		t.Errorf("annotations.keep_id = %v, want keep-uuid-123", ann["keep_id"])
	}
}

func TestNormalizeGeneric(t *testing.T) {
	p := genericPayload{
		Title:       "Disk space low on /var",
		Severity:    "major",
		Fingerprint: "disk-var-low",
		Description: "Disk usage above 95%",
		Labels:      map[string]string{"host": "web-01"},
		Source:      "monitoring-script",
	}

	n := normalizeGeneric(p)

	if n.Title != "Disk space low on /var" {
		t.Errorf("Title = %q", n.Title)
	}
	if n.Severity != "major" {
		t.Errorf("Severity = %q, want major", n.Severity)
	}
	if n.Source != "monitoring-script" {
		t.Errorf("Source = %q, want monitoring-script", n.Source)
	}
	if n.Fingerprint != "disk-var-low" {
		t.Errorf("Fingerprint = %q, want disk-var-low", n.Fingerprint)
	}
	if n.Status != "firing" {
		t.Errorf("Status = %q, want firing (generic always fires)", n.Status)
	}
}

func TestNormalizeGeneric_Defaults(t *testing.T) {
	p := genericPayload{
		Title: "Simple alert",
	}

	n := normalizeGeneric(p)

	if n.Source != "generic" {
		t.Errorf("Source = %q, want generic (default)", n.Source)
	}
	if n.Severity != "warning" {
		t.Errorf("Severity = %q, want warning (default)", n.Severity)
	}
	if n.Fingerprint == "" {
		t.Error("Fingerprint should be auto-generated")
	}
}

// --- Handler validation tests ---

func newTestRouter() (*WebhookHandler, chi.Router) {
	h := NewWebhookHandler(nil, nil)
	router := chi.NewRouter()
	router.Mount("/webhooks", h.Routes())
	return h, router
}

func TestAlertmanagerWebhook_EmptyBody(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAlertmanagerWebhook_InvalidJSON(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager", strings.NewReader("{bad"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAlertmanagerWebhook_NoAlerts(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager", strings.NewReader(`{"alerts":[]}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestKeepWebhook_EmptyBody(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/keep", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestKeepWebhook_MissingName(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/keep", strings.NewReader(`{"status":"firing"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestGenericWebhook_EmptyBody(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/generic", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGenericWebhook_MissingTitle(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/generic", strings.NewReader(`{"severity":"warning"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

func TestGenericWebhook_InvalidJSON(t *testing.T) {
	_, router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/webhooks/generic", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
