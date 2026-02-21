package alert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NormalizedAlert is the internal representation that all webhook formats
// normalize to before persisting.
type NormalizedAlert struct {
	Fingerprint          string
	Status               string // firing, resolved
	Severity             string // info, warning, major, critical
	Source               string // alertmanager, keep, generic, or custom
	Title                string
	Description          *string
	Labels               json.RawMessage
	Annotations          json.RawMessage
	ResolvedByAgent      bool
	AgentResolutionNotes string
}

// Response is the API response for an alert.
type Response struct {
	ID                uuid.UUID       `json:"id"`
	Fingerprint       string          `json:"fingerprint"`
	Status            string          `json:"status"`
	Severity          string          `json:"severity"`
	Source            string          `json:"source"`
	Title             string          `json:"title"`
	Description       *string         `json:"description,omitempty"`
	Labels            json.RawMessage `json:"labels"`
	Annotations       json.RawMessage `json:"annotations"`
	MatchedIncidentID *uuid.UUID      `json:"matched_incident_id,omitempty"`
	SuggestedSolution *string         `json:"suggested_solution,omitempty"`
	OccurrenceCount   int32           `json:"occurrence_count"`
	FirstFiredAt      time.Time       `json:"first_fired_at"`
	LastFiredAt       time.Time       `json:"last_fired_at"`
	CreatedAt         time.Time       `json:"created_at"`
}

// BatchResponse is the response for webhook endpoints that process multiple alerts.
type BatchResponse struct {
	AlertsProcessed int        `json:"alerts_processed"`
	Alerts          []Response `json:"alerts"`
}

// normalizeSeverity maps external severity strings to internal values.
func normalizeSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical", "crit", "fatal", "emergency", "p1":
		return "critical"
	case "major", "error", "high", "p2":
		return "major"
	case "warning", "warn", "medium", "p3":
		return "warning"
	case "info", "informational", "low", "p4", "p5":
		return "info"
	default:
		return "warning"
	}
}

// normalizeStatus maps external status values to internal ones.
func normalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "resolved", "ok", "inactive":
		return "resolved"
	default:
		return "firing"
	}
}

// generateFingerprint creates a deterministic fingerprint from the alert title and labels.
func generateFingerprint(title string, labels json.RawMessage) string {
	data := fmt.Sprintf("%s:%s", title, string(labels))
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:16])
}

// ensureJSON returns raw if it contains a valid JSON object/array,
// otherwise returns "{}". This handles nil, empty, and literal "null" values.
func ensureJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage(`{}`)
	}
	return raw
}
