package alertgroup

import (
	"testing"
)

func TestMatchAlert(t *testing.T) {
	tests := []struct {
		name     string
		matchers []Matcher
		labels   map[string]string
		want     bool
	}{
		{
			name:     "empty matchers match all",
			matchers: []Matcher{},
			labels:   map[string]string{"service": "api", "cluster": "prod"},
			want:     true,
		},
		{
			name:     "exact match",
			matchers: []Matcher{{Key: "service", Op: "=", Value: "api"}},
			labels:   map[string]string{"service": "api"},
			want:     true,
		},
		{
			name:     "exact match failure",
			matchers: []Matcher{{Key: "service", Op: "=", Value: "api"}},
			labels:   map[string]string{"service": "web"},
			want:     false,
		},
		{
			name:     "not equal match",
			matchers: []Matcher{{Key: "env", Op: "!=", Value: "dev"}},
			labels:   map[string]string{"env": "prod"},
			want:     true,
		},
		{
			name:     "not equal failure",
			matchers: []Matcher{{Key: "env", Op: "!=", Value: "dev"}},
			labels:   map[string]string{"env": "dev"},
			want:     false,
		},
		{
			name:     "regex match",
			matchers: []Matcher{{Key: "namespace", Op: "=~", Value: "prod-.*"}},
			labels:   map[string]string{"namespace": "prod-us-east"},
			want:     true,
		},
		{
			name:     "regex match failure",
			matchers: []Matcher{{Key: "namespace", Op: "=~", Value: "prod-.*"}},
			labels:   map[string]string{"namespace": "staging-eu"},
			want:     false,
		},
		{
			name:     "regex not match",
			matchers: []Matcher{{Key: "namespace", Op: "!~", Value: "test-.*"}},
			labels:   map[string]string{"namespace": "prod-us"},
			want:     true,
		},
		{
			name:     "regex not match failure",
			matchers: []Matcher{{Key: "namespace", Op: "!~", Value: "test-.*"}},
			labels:   map[string]string{"namespace": "test-local"},
			want:     false,
		},
		{
			name: "AND logic - all must match",
			matchers: []Matcher{
				{Key: "service", Op: "=", Value: "api"},
				{Key: "cluster", Op: "=", Value: "prod"},
			},
			labels: map[string]string{"service": "api", "cluster": "prod"},
			want:   true,
		},
		{
			name: "AND logic - one fails",
			matchers: []Matcher{
				{Key: "service", Op: "=", Value: "api"},
				{Key: "cluster", Op: "=", Value: "prod"},
			},
			labels: map[string]string{"service": "api", "cluster": "staging"},
			want:   false,
		},
		{
			name:     "missing label key - exact match",
			matchers: []Matcher{{Key: "team", Op: "=", Value: "sre"}},
			labels:   map[string]string{"service": "api"},
			want:     false,
		},
		{
			name:     "missing label key - not equal",
			matchers: []Matcher{{Key: "team", Op: "!=", Value: "sre"}},
			labels:   map[string]string{"service": "api"},
			want:     true,
		},
		{
			name:     "unknown operator",
			matchers: []Matcher{{Key: "x", Op: ">", Value: "1"}},
			labels:   map[string]string{"x": "2"},
			want:     false,
		},
		{
			name:     "invalid regex",
			matchers: []Matcher{{Key: "x", Op: "=~", Value: "[invalid"}},
			labels:   map[string]string{"x": "anything"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchAlert(tt.matchers, tt.labels)
			if got != tt.want {
				t.Errorf("matchAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeGroupKeyHash_Deterministic(t *testing.T) {
	labels1 := map[string]string{"service": "api", "cluster": "prod"}
	labels2 := map[string]string{"cluster": "prod", "service": "api"} // same labels, different order

	hash1 := computeGroupKeyHash(labels1)
	hash2 := computeGroupKeyHash(labels2)

	if hash1 != hash2 {
		t.Errorf("hashes should be equal for same labels regardless of order: %s != %s", hash1, hash2)
	}

	// Different labels should produce different hashes.
	labels3 := map[string]string{"service": "web", "cluster": "prod"}
	hash3 := computeGroupKeyHash(labels3)
	if hash1 == hash3 {
		t.Error("hashes should be different for different labels")
	}
}

func TestComputeGroupTitle(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "single label",
			labels: map[string]string{"service": "api"},
			want:   "service=api",
		},
		{
			name:   "multiple labels sorted",
			labels: map[string]string{"service": "api", "cluster": "prod"},
			want:   "cluster=prod, service=api",
		},
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   "(all)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeGroupTitle(tt.labels)
			if got != tt.want {
				t.Errorf("computeGroupTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractGroupByLabels(t *testing.T) {
	labels := map[string]string{"service": "api", "cluster": "prod", "namespace": "default"}

	result := extractGroupByLabels([]string{"service", "cluster"}, labels)

	if result["service"] != "api" {
		t.Errorf("expected service=api, got %s", result["service"])
	}
	if result["cluster"] != "prod" {
		t.Errorf("expected cluster=prod, got %s", result["cluster"])
	}
	if _, ok := result["namespace"]; ok {
		t.Error("namespace should not be in result")
	}

	// Missing key should produce empty string.
	result2 := extractGroupByLabels([]string{"team"}, labels)
	if result2["team"] != "" {
		t.Errorf("expected empty string for missing key, got %q", result2["team"])
	}
}

func TestSeverityRanking(t *testing.T) {
	tests := []struct {
		a, b string
		want string
	}{
		{"critical", "info", "critical"},
		{"info", "critical", "critical"},
		{"major", "warning", "major"},
		{"warning", "info", "warning"},
		{"info", "info", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := maxSeverityOf(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("maxSeverityOf(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseMatchers(t *testing.T) {
	raw := []byte(`[{"key":"service","op":"=","value":"api"},{"key":"env","op":"!=","value":"dev"}]`)
	matchers := parseMatchers(raw)

	if len(matchers) != 2 {
		t.Fatalf("expected 2 matchers, got %d", len(matchers))
	}
	if matchers[0].Key != "service" || matchers[0].Op != "=" || matchers[0].Value != "api" {
		t.Errorf("unexpected first matcher: %+v", matchers[0])
	}
	if matchers[1].Key != "env" || matchers[1].Op != "!=" || matchers[1].Value != "dev" {
		t.Errorf("unexpected second matcher: %+v", matchers[1])
	}

	// Invalid JSON should return empty slice.
	empty := parseMatchers([]byte("invalid"))
	if len(empty) != 0 {
		t.Errorf("expected 0 matchers for invalid JSON, got %d", len(empty))
	}
}

func TestMarshalMatchers(t *testing.T) {
	matchers := []Matcher{{Key: "x", Op: "=", Value: "y"}}
	raw := marshalMatchers(matchers)
	parsed := parseMatchers(raw)

	if len(parsed) != 1 || parsed[0].Key != "x" {
		t.Errorf("roundtrip failed: %+v", parsed)
	}

	// Nil matchers should marshal to empty array.
	raw2 := marshalMatchers(nil)
	if string(raw2) != "[]" {
		t.Errorf("nil matchers should marshal to [], got %s", string(raw2))
	}
}
