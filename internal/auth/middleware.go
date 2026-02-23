package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// MethodSession indicates authentication via self-issued session JWT.
const MethodSession = "session"

// Middleware returns an HTTP middleware that authenticates the caller via
// session JWT, OIDC JWT, API key, or dev header and stores the resulting
// Identity in the request context.
//
// Authentication precedence:
//  1. Authorization: Bearer <jwt>  →  Session JWT (HMAC) → OIDC validation
//  2. X-API-Key: <raw-key>        →  API key hash lookup
//  3. X-Tenant-Slug: <slug>       →  Development-only fallback (no real auth)
//
// If none succeed, the request is rejected with 401.
func Middleware(sessionMgr *SessionManager, oidcAuth *OIDCAuthenticator, patAuth *PATAuthenticator, pool db.DBTX, logger *slog.Logger) func(http.Handler) http.Handler {
	apikeyAuth := &APIKeyAuthenticator{DB: pool}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var identity *Identity

			// 1. Try Bearer token: PAT → session JWT → OIDC JWT.
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "bearer ") {
				rawToken := strings.TrimPrefix(authHeader, "Bearer ")
				rawToken = strings.TrimPrefix(rawToken, "bearer ")
				rawToken = strings.TrimSpace(rawToken)

				// 1a. Try personal access token (nwl_pat_ prefix).
				if strings.HasPrefix(rawToken, PATPrefix) && patAuth != nil {
					result, err := patAuth.Authenticate(r.Context(), rawToken)
					if err != nil {
						logger.Warn("PAT authentication failed", "error", err)
						respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid personal access token")
						return
					}

					identity = &Identity{
						Subject:    result.DisplayName,
						Email:      result.Email,
						Role:       result.Role,
						TenantSlug: result.TenantSlug,
						TenantID:   result.TenantID,
						UserID:     &result.UserID,
						Method:     MethodPAT,
					}

					logger.Debug("authenticated via PAT",
						"email", result.Email,
						"tenant_slug", result.TenantSlug,
					)
				}

				// 1b. Try session JWT (HMAC-signed).
				if identity == nil && sessionMgr != nil {
					claims, err := sessionMgr.ValidateToken(rawToken)
					if err == nil {
						userID, _ := uuid.Parse(claims.UserID)
						tenantID, _ := uuid.Parse(claims.TenantID)
						identity = &Identity{
							Subject:    claims.Subject,
							Email:      claims.Email,
							Role:       claims.Role,
							TenantSlug: claims.TenantSlug,
							TenantID:   tenantID,
							UserID:     &userID,
							Method:     MethodSession,
						}

						logger.Debug("authenticated via session JWT",
							"sub", claims.Subject,
							"email", claims.Email,
							"tenant_slug", claims.TenantSlug,
						)
					}
				}

				// 1c. Fall through to OIDC JWT if session validation failed.
				if identity == nil {
					if oidcAuth == nil {
						logger.Warn("JWT presented but OIDC is not configured")
						respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid token")
						return
					}

					claims, err := oidcAuth.Authenticate(r.Context(), authHeader)
					if err != nil {
						logger.Warn("OIDC authentication failed", "error", err)
						respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid token")
						return
					}

					identity = &Identity{
						Subject:    claims.Subject,
						Email:      claims.Email,
						Role:       claims.Role,
						TenantSlug: claims.TenantSlug,
						Method:     MethodOIDC,
					}

					logger.Debug("authenticated via OIDC",
						"sub", claims.Subject,
						"email", claims.Email,
						"tenant_slug", claims.TenantSlug,
					)
				}
			}

			// 2. Try API key.
			if identity == nil {
				if rawKey := r.Header.Get("X-API-Key"); rawKey != "" {
					result, err := apikeyAuth.Authenticate(r.Context(), rawKey)
					if err != nil {
						logger.Warn("API key authentication failed", "error", err)
						respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
						return
					}

					// Look up tenant slug from tenant ID.
					q := db.New(pool)
					t, err := q.GetTenant(r.Context(), result.TenantID)
					if err != nil {
						logger.Error("tenant lookup for API key failed", "tenant_id", result.TenantID, "error", err)
						respondErr(w, http.StatusUnauthorized, "unauthorized", "tenant not found")
						return
					}

					identity = &Identity{
						Subject:    fmt.Sprintf("apikey:%s", result.KeyPrefix),
						Role:       result.Role,
						TenantSlug: t.Slug,
						TenantID:   t.ID,
						APIKeyID:   &result.APIKeyID,
						Method:     MethodAPIKey,
					}

					logger.Debug("authenticated via API key",
						"key_prefix", result.KeyPrefix,
						"tenant_slug", t.Slug,
						"role", result.Role,
					)
				}
			}

			// 3. Dev-mode fallback: X-Tenant-Slug header (no real authentication).
			if identity == nil {
				if slug := r.Header.Get("X-Tenant-Slug"); slug != "" {
					devID := uuid.Nil
					identity = &Identity{
						Subject:    "dev:anonymous",
						Email:      "dev@localhost",
						Role:       RoleAdmin,
						TenantSlug: slug,
						TenantID:   devID,
						UserID:     &devID,
						Method:     MethodDev,
					}

					// Try to resolve a real admin user so user-scoped
					// operations (e.g. PAT management) work in dev mode.
					if pool != nil {
						q := db.New(pool)
						if t, err := q.GetTenantBySlug(r.Context(), slug); err == nil {
							identity.TenantID = t.ID
							schema := fmt.Sprintf("tenant_%s", slug)
							var userID uuid.UUID
							var email, displayName string
							err := pool.QueryRow(r.Context(),
								fmt.Sprintf("SELECT id, email, display_name FROM %s.users WHERE role = 'admin' AND is_active = true LIMIT 1", schema),
							).Scan(&userID, &email, &displayName)
							if err == nil {
								identity.UserID = &userID
								identity.Email = email
								identity.Subject = displayName
							}
						}
					}

					logger.Debug("dev-mode authentication", "tenant_slug", slug)
				}
			}

			if identity == nil {
				respondErr(w, http.StatusUnauthorized, "unauthorized", "no valid authentication provided")
				return
			}

			ctx := NewContext(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondErr(w http.ResponseWriter, status int, errStr, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errStr,
		"message": message,
	})
}
