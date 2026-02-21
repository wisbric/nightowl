package slack

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newTestRouter() chi.Router {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewHandler(
		NewNotifier("", "", logger),
		nil,
		logger,
		"", // no signing secret (dev mode)
		"devco",
	)
	router := chi.NewRouter()
	router.Mount("/slack", h.Routes())
	return router
}

func TestEvents_URLVerification(t *testing.T) {
	router := newTestRouter()

	body := `{"type":"url_verification","challenge":"test_challenge_token"}`
	r := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["challenge"] != "test_challenge_token" {
		t.Errorf("challenge = %q, want test_challenge_token", resp["challenge"])
	}
}

func TestEvents_InvalidJSON(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCommands_NoSubcommand(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["response_type"] != "ephemeral" {
		t.Errorf("response_type = %q, want ephemeral", resp["response_type"])
	}
	if !strings.Contains(resp["text"], "Usage") {
		t.Errorf("expected usage text, got %q", resp["text"])
	}
}

func TestCommands_UnknownSubcommand(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=foobar&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["text"], "Unknown command") {
		t.Errorf("expected unknown command text, got %q", resp["text"])
	}
}

func TestCommands_AckMissingID(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=ack&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["text"], "Usage") {
		t.Errorf("expected usage text, got %q", resp["text"])
	}
}

func TestCommands_AckInvalidID(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=ack+not-a-uuid&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["text"], "Invalid alert ID") {
		t.Errorf("expected invalid ID text, got %q", resp["text"])
	}
}

func TestCommands_ResolveMissingID(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=resolve&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["text"], "Usage") {
		t.Errorf("expected usage text, got %q", resp["text"])
	}
}

func TestCommands_SearchMissingQuery(t *testing.T) {
	router := newTestRouter()

	body := "command=%2Fnightowl&text=search&user_id=U123&channel_id=C456"
	r := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["text"], "Usage") {
		t.Errorf("expected usage text, got %q", resp["text"])
	}
}

func TestInteractions_MissingPayload(t *testing.T) {
	router := newTestRouter()

	r := httptest.NewRequest(http.MethodPost, "/slack/interactions", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
