package tenant

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeaderResolver_Resolve(t *testing.T) {
	resolver := HeaderResolver{}

	t.Run("returns slug from header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Tenant-Slug", "acme")

		slug, err := resolver.Resolve(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if slug != "acme" {
			t.Errorf("slug = %q, want %q", slug, "acme")
		}
	})

	t.Run("returns error when header missing", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		_, err := resolver.Resolve(r)
		if err == nil {
			t.Fatal("expected error for missing header")
		}
	})
}
