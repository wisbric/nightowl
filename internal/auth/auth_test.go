package auth

import (
	"context"
	"testing"
)

func TestHashAPIKey(t *testing.T) {
	// Deterministic: same input → same hash.
	h1 := HashAPIKey("test-key-123")
	h2 := HashAPIKey("test-key-123")
	if h1 != h2 {
		t.Fatalf("same key produced different hashes: %q vs %q", h1, h2)
	}

	// Different input → different hash.
	h3 := HashAPIKey("different-key")
	if h1 == h3 {
		t.Fatal("different keys produced the same hash")
	}

	// SHA-256 produces 64-char hex string.
	if len(h1) != 64 {
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
}

func TestIsValidRole(t *testing.T) {
	tests := []struct {
		role  string
		valid bool
	}{
		{RoleAdmin, true},
		{RoleManager, true},
		{RoleEngineer, true},
		{RoleReadonly, true},
		{"superadmin", false},
		{"", false},
		{"Admin", false}, // case-sensitive
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := IsValidRole(tt.role)
			if got != tt.valid {
				t.Errorf("IsValidRole(%q) = %v, want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestIdentityContext(t *testing.T) {
	ctx := context.Background()

	// No identity yet.
	if id := FromContext(ctx); id != nil {
		t.Fatalf("expected nil, got %+v", id)
	}

	// Store and retrieve.
	identity := &Identity{
		Subject:    "user-123",
		Email:      "test@example.com",
		Role:       RoleEngineer,
		TenantSlug: "acme",
		Method:     MethodOIDC,
	}
	ctx = NewContext(ctx, identity)

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("expected identity, got nil")
	}
	if got.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user-123")
	}
	if got.Role != RoleEngineer {
		t.Errorf("Role = %q, want %q", got.Role, RoleEngineer)
	}
	if got.TenantSlug != "acme" {
		t.Errorf("TenantSlug = %q, want %q", got.TenantSlug, "acme")
	}
}
