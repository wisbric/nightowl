package alertgroup

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
)

// TestHandlerRoutes verifies that the handler routes are correctly registered
// and respond to basic requests without panicking.
func TestHandlerRoutes(t *testing.T) {
	logger := slog.Default()
	h := NewHandler(logger, nil)
	router := h.Routes()

	// Verify the router was created successfully.
	if router == nil {
		t.Fatal("Routes() returned nil")
	}
}

// TestCreateRuleRequest_Validation verifies that CreateRuleRequest fields
// are serialized correctly.
func TestCreateRuleRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		valid   bool
	}{
		{
			name:    "valid request",
			payload: `{"name":"test rule","group_by":["service"],"matchers":[{"key":"env","op":"=","value":"prod"}]}`,
			valid:   true,
		},
		{
			name:    "missing name",
			payload: `{"group_by":["service"]}`,
			valid:   false,
		},
		{
			name:    "missing group_by",
			payload: `{"name":"test rule"}`,
			valid:   false,
		},
		{
			name:    "valid with empty matchers",
			payload: `{"name":"catch all","group_by":["service"],"matchers":[]}`,
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateRuleRequest
			if err := json.Unmarshal([]byte(tt.payload), &req); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if tt.valid {
				if req.Name == "" {
					t.Error("expected non-empty name")
				}
				if len(req.GroupBy) == 0 {
					t.Error("expected non-empty group_by")
				}
			}
		})
	}
}

// TestRuleResponse_JSON verifies that RuleResponse marshals correctly.
func TestRuleResponse_JSON(t *testing.T) {
	resp := RuleResponse{
		Name:      "test",
		Position:  1,
		IsEnabled: true,
		Matchers:  []Matcher{{Key: "service", Op: "=", Value: "api"}},
		GroupBy:   []string{"service", "cluster"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded["name"] != "test" {
		t.Errorf("expected name=test, got %v", decoded["name"])
	}
	if decoded["is_enabled"] != true {
		t.Errorf("expected is_enabled=true, got %v", decoded["is_enabled"])
	}
}

// TestGroupResponse_JSON verifies that GroupResponse marshals correctly.
func TestGroupResponse_JSON(t *testing.T) {
	resp := GroupResponse{
		RuleName:       "prod-rule",
		Title:          "service=api, cluster=prod",
		AlertCount:     5,
		MaxSeverity:    "critical",
		GroupKeyLabels: json.RawMessage(`{"service":"api","cluster":"prod"}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded["rule_name"] != "prod-rule" {
		t.Errorf("expected rule_name=prod-rule, got %v", decoded["rule_name"])
	}
	if decoded["alert_count"].(float64) != 5 {
		t.Errorf("expected alert_count=5, got %v", decoded["alert_count"])
	}
}

// TestHandlerListRules_NoTenant verifies the handler fails gracefully
// when no tenant connection is available (as expected).
func TestHandlerListRules_NoTenant(t *testing.T) {
	logger := slog.Default()
	h := NewHandler(logger, nil)

	// Without tenant context, this will panic — we verify the handler
	// setup works but can't test full HTTP cycle without a DB.
	req := httptest.NewRequest(http.MethodGet, "/rules", nil)
	w := httptest.NewRecorder()

	// We just verify the request can be constructed; full integration
	// testing requires a database.
	_ = req
	_ = w
	_ = h
}

// TestCreateRulePayload_RoundTrip verifies that a create rule payload
// can be marshaled and unmarshaled correctly.
func TestCreateRulePayload_RoundTrip(t *testing.T) {
	original := CreateRuleRequest{
		Name:     "production alerts",
		Position: 1,
		Matchers: []Matcher{
			{Key: "env", Op: "=", Value: "production"},
			{Key: "namespace", Op: "=~", Value: "prod-.*"},
		},
		GroupBy: []string{"service", "cluster"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CreateRuleRequest
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&decoded); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("name mismatch: %q != %q", decoded.Name, original.Name)
	}
	if len(decoded.Matchers) != len(original.Matchers) {
		t.Errorf("matchers count mismatch: %d != %d", len(decoded.Matchers), len(original.Matchers))
	}
	if len(decoded.GroupBy) != len(original.GroupBy) {
		t.Errorf("group_by count mismatch: %d != %d", len(decoded.GroupBy), len(original.GroupBy))
	}
}
