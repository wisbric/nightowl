package tenant

import (
	"testing"
)

func TestSlugValidation(t *testing.T) {
	tests := []struct {
		slug  string
		valid bool
	}{
		{"acme", true},
		{"test_org", true},
		{"a1", true},
		{"ab", true},
		{"a_very_long_slug_that_is_still_valid_abcdef", true},
		{"", false},
		{"A", false},         // uppercase
		{"1abc", false},      // starts with digit
		{"-abc", false},      // starts with dash
		{"a", false},         // too short (min 2)
		{"has space", false}, // contains space
		{"has-dash", false},  // contains dash
		{"UPPERCASE", false}, // all uppercase
		{"a.b", false},       // contains dot
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := slugPattern.MatchString(tt.slug)
			if got != tt.valid {
				t.Errorf("slugPattern.MatchString(%q) = %v, want %v", tt.slug, got, tt.valid)
			}
		})
	}
}

func TestWithSearchPath(t *testing.T) {
	tests := []struct {
		name    string
		dbURL   string
		schema  string
		wantErr bool
	}{
		{
			name:   "adds search_path to URL without params",
			dbURL:  "postgres://user:pass@localhost:5432/db?sslmode=disable",
			schema: "tenant_acme",
		},
		{
			name:   "replaces existing search_path",
			dbURL:  "postgres://user:pass@localhost:5432/db?sslmode=disable&search_path=public",
			schema: "tenant_test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := withSearchPath(tt.dbURL, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Fatalf("withSearchPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got == "" {
				t.Fatal("expected non-empty URL")
			}
			// Verify the URL contains the schema.
			if !contains(got, "search_path="+tt.schema) {
				t.Errorf("URL %q does not contain search_path=%s", got, tt.schema)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
