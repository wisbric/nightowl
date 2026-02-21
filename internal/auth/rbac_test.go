package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("rejects unauthenticated", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		RequireAuth(okHandler).ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("passes authenticated", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := NewContext(r.Context(), &Identity{Subject: "user", Role: RoleEngineer})
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()

		RequireAuth(okHandler).ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}

func TestRequireRole(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireRole(RoleAdmin, RoleManager)

	tests := []struct {
		name     string
		role     string
		wantCode int
	}{
		{"admin allowed", RoleAdmin, http.StatusOK},
		{"manager allowed", RoleManager, http.StatusOK},
		{"engineer rejected", RoleEngineer, http.StatusForbidden},
		{"readonly rejected", RoleReadonly, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			ctx := NewContext(r.Context(), &Identity{Subject: "u", Role: tt.role})
			r = r.WithContext(ctx)
			w := httptest.NewRecorder()

			mw(okHandler).ServeHTTP(w, r)

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestRequireMinRole(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireMinRole(RoleManager) // manager or above

	tests := []struct {
		name     string
		role     string
		wantCode int
	}{
		{"admin passes", RoleAdmin, http.StatusOK},
		{"manager passes", RoleManager, http.StatusOK},
		{"engineer rejected", RoleEngineer, http.StatusForbidden},
		{"readonly rejected", RoleReadonly, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			ctx := NewContext(r.Context(), &Identity{Subject: "u", Role: tt.role})
			r = r.WithContext(ctx)
			w := httptest.NewRecorder()

			mw(okHandler).ServeHTTP(w, r)

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestRequireAuth_NoIdentity(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := RequireMinRole(RoleEngineer)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	mw(okHandler).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
