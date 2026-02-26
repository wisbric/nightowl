package tenant

import (
	"testing"

	coretenant "github.com/wisbric/core/pkg/tenant"
)

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
			got, err := coretenant.WithSearchPath(tt.dbURL, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Fatalf("WithSearchPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got == "" {
				t.Fatal("expected non-empty URL")
			}
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
