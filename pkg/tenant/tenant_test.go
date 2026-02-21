package tenant

import (
	"context"
	"testing"
)

func TestSchemaName(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"acme", "tenant_acme"},
		{"test_org", "tenant_test_org"},
		{"a1", "tenant_a1"},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := SchemaName(tt.slug)
			if got != tt.want {
				t.Errorf("SchemaName(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Without tenant set.
	if got := FromContext(ctx); got != nil {
		t.Fatalf("expected nil tenant, got %+v", got)
	}

	info := &Info{Slug: "acme", Schema: "tenant_acme"}
	ctx = NewContext(ctx, info)

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("expected tenant info, got nil")
	}
	if got.Slug != "acme" {
		t.Errorf("slug = %q, want %q", got.Slug, "acme")
	}
}

func TestConnContextNilWithout(t *testing.T) {
	ctx := context.Background()
	if got := ConnFromContext(ctx); got != nil {
		t.Fatalf("expected nil conn, got %v", got)
	}
}
