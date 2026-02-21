package audit

import (
	"log/slog"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")

	ip := clientIP(r)
	want := netip.MustParseAddr("203.0.113.50")
	if ip != want {
		t.Errorf("clientIP = %v, want %v", ip, want)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "198.51.100.23")

	ip := clientIP(r)
	want := netip.MustParseAddr("198.51.100.23")
	if ip != want {
		t.Errorf("clientIP = %v, want %v", ip, want)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:12345"

	ip := clientIP(r)
	want := netip.MustParseAddr("192.0.2.1")
	if ip != want {
		t.Errorf("clientIP = %v, want %v", ip, want)
	}
}

func TestClientIP_Precedence(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50")
	r.Header.Set("X-Real-IP", "198.51.100.23")
	r.RemoteAddr = "192.0.2.1:12345"

	ip := clientIP(r)
	want := netip.MustParseAddr("203.0.113.50")
	if ip != want {
		t.Errorf("clientIP = %v, want %v (X-Forwarded-For should take precedence)", ip, want)
	}
}

func TestClientIP_XRealIPFallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "198.51.100.23")
	r.RemoteAddr = "192.0.2.1:12345"

	ip := clientIP(r)
	want := netip.MustParseAddr("198.51.100.23")
	if ip != want {
		t.Errorf("clientIP = %v, want %v (X-Real-IP should take precedence over RemoteAddr)", ip, want)
	}
}

func TestClientIP_InvalidXFF(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	r.RemoteAddr = "192.0.2.1:12345"

	ip := clientIP(r)
	want := netip.MustParseAddr("192.0.2.1")
	if ip != want {
		t.Errorf("clientIP = %v, want %v (should fall back to RemoteAddr)", ip, want)
	}
}

func TestLog_DropsWhenFull(t *testing.T) {
	logger := slog.Default()
	w := NewWriter(nil, logger)
	// Don't start the background goroutine — nothing drains the channel.

	// Fill the buffer.
	for i := 0; i < bufferSize; i++ {
		w.Log(Entry{Action: "test", Resource: "test"})
	}

	// The next log should be dropped (non-blocking).
	w.Log(Entry{Action: "dropped", Resource: "dropped"})

	// Verify buffer is full.
	if len(w.entries) != bufferSize {
		t.Errorf("buffer size = %d, want %d", len(w.entries), bufferSize)
	}
}

func TestLogFromRequest_ExtractsFields(t *testing.T) {
	logger := slog.Default()
	w := NewWriter(nil, logger)
	// Don't start — we'll read from the channel directly.

	r := httptest.NewRequest("POST", "/api/v1/incidents", nil)
	r.Header.Set("User-Agent", "test-agent/1.0")
	r.Header.Set("X-Real-IP", "198.51.100.23")

	id := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	w.LogFromRequest(r, "create", "incident", id, nil)

	// Read the entry from the channel.
	entry := <-w.entries

	if entry.Action != "create" {
		t.Errorf("Action = %q, want %q", entry.Action, "create")
	}
	if entry.Resource != "incident" {
		t.Errorf("Resource = %q, want %q", entry.Resource, "incident")
	}
	if entry.IPAddress == nil {
		t.Fatal("IPAddress should not be nil")
	}
	if *entry.IPAddress != netip.MustParseAddr("198.51.100.23") {
		t.Errorf("IPAddress = %v, want 198.51.100.23", *entry.IPAddress)
	}
	if entry.UserAgent == nil || *entry.UserAgent != "test-agent/1.0" {
		t.Errorf("UserAgent = %v, want test-agent/1.0", entry.UserAgent)
	}
}
