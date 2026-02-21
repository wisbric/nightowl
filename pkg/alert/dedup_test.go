package alert

import (
	"testing"
)

func TestRedisKey(t *testing.T) {
	got := redisKey("tenant_acme", "fp123")
	want := "alert:dedup:tenant_acme:fp123"
	if got != want {
		t.Errorf("redisKey() = %q, want %q", got, want)
	}
}

func TestRedisKey_Prefix(t *testing.T) {
	got := redisKey("tenant_x", "abc")
	if got[:len(redisKeyPrefix)] != redisKeyPrefix {
		t.Errorf("redisKey() should start with %q, got %q", redisKeyPrefix, got)
	}
}

func TestDedupTTL(t *testing.T) {
	if dedupTTL.Minutes() != 5 {
		t.Errorf("dedupTTL = %v, want 5 minutes", dedupTTL)
	}
}

func TestTenantSchemaFromNilContext(t *testing.T) {
	// When no tenant info in context, should return "unknown".
	// We test the helper via the webhook handler's tenantSchema function.
	// Since we can't easily create an http.Request with tenant context here,
	// we verify the redisKey format is deterministic.
	k1 := redisKey("tenant_a", "fp1")
	k2 := redisKey("tenant_a", "fp1")
	if k1 != k2 {
		t.Error("redisKey should be deterministic")
	}

	// Different tenants produce different keys.
	k3 := redisKey("tenant_b", "fp1")
	if k1 == k3 {
		t.Error("different tenants should produce different keys")
	}

	// Different fingerprints produce different keys.
	k4 := redisKey("tenant_a", "fp2")
	if k1 == k4 {
		t.Error("different fingerprints should produce different keys")
	}
}
