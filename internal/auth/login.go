package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/wisbric/nightowl/internal/db"
)

// LoginRequest is the JSON body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the JSON response for a successful login.
type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// UserInfo is the public user information returned in auth responses.
type UserInfo struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// AuthConfigResponse tells the frontend which auth methods are available.
type AuthConfigResponse struct {
	OIDCEnabled  bool   `json:"oidc_enabled"`
	OIDCName     string `json:"oidc_name"`
	LocalEnabled bool   `json:"local_enabled"`
}

// LoginHandler handles local email/password login and auth discovery.
type LoginHandler struct {
	sessionMgr  *SessionManager
	pool        *pgxpool.Pool
	logger      *slog.Logger
	oidcEnabled bool
}

// NewLoginHandler creates a new login handler.
func NewLoginHandler(sm *SessionManager, pool *pgxpool.Pool, logger *slog.Logger, oidcEnabled bool) *LoginHandler {
	return &LoginHandler{
		sessionMgr:  sm,
		pool:        pool,
		logger:      logger,
		oidcEnabled: oidcEnabled,
	}
}

// HandleLogin authenticates a user with email/password and returns a session JWT.
func (h *LoginHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Email == "" || req.Password == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "email and password are required")
		return
	}

	// Look up the user across all tenant schemas.
	userRow, tenantSlug, tenantID, err := h.findUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Warn("login: user lookup failed", "email", req.Email, "error", err)
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	// Verify password.
	if userRow.PasswordHash == nil || *userRow.PasswordHash == "" {
		h.logger.Warn("login: user has no password set", "email", req.Email)
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*userRow.PasswordHash), []byte(req.Password)); err != nil {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	// Issue session token.
	token, err := h.sessionMgr.IssueToken(SessionClaims{
		Subject:    userRow.DisplayName,
		Email:      userRow.Email,
		Role:       userRow.Role,
		TenantSlug: tenantSlug,
		TenantID:   tenantID,
		UserID:     userRow.ID.String(),
		Method:     "local",
	})
	if err != nil {
		h.logger.Error("login: issuing token", "error", err)
		respondErr(w, http.StatusInternalServerError, "internal", "failed to issue token")
		return
	}

	respondJSON(w, http.StatusOK, LoginResponse{
		Token: token,
		User: UserInfo{
			ID:          userRow.ID.String(),
			Email:       userRow.Email,
			DisplayName: userRow.DisplayName,
			Role:        userRow.Role,
		},
	})
}

// HandleAuthConfig returns the available authentication methods.
func (h *LoginHandler) HandleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, AuthConfigResponse{
		OIDCEnabled:  h.oidcEnabled,
		OIDCName:     "Sign in with SSO",
		LocalEnabled: true,
	})
}

// HandleMe returns the current user's info from a session token.
func (h *LoginHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) < 8 {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "no token provided")
		return
	}

	token := authHeader[7:] // strip "Bearer "
	claims, err := h.sessionMgr.ValidateToken(token)
	if err != nil {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":           claims.UserID,
		"email":        claims.Email,
		"display_name": claims.Subject,
		"role":         claims.Role,
		"tenant_slug":  claims.TenantSlug,
	})
}

// HandleLogout is a no-op endpoint for future server-side session revocation.
func (h *LoginHandler) HandleLogout(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// userWithPassword extends the sqlc-generated User row with the password_hash column.
type userWithPassword struct {
	db.User
	PasswordHash *string
}

// findUserByEmail searches across all tenant schemas for a user with the given email.
func (h *LoginHandler) findUserByEmail(ctx context.Context, email string) (*userWithPassword, string, string, error) {
	q := db.New(h.pool)
	tenants, err := q.ListTenants(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("listing tenants: %w", err)
	}

	for _, t := range tenants {
		conn, err := h.pool.Acquire(ctx)
		if err != nil {
			return nil, "", "", fmt.Errorf("acquiring connection: %w", err)
		}

		_, err = conn.Exec(ctx, fmt.Sprintf("SET search_path TO tenant_%s, public", t.Slug))
		if err != nil {
			conn.Release()
			continue
		}

		var u userWithPassword
		err = conn.QueryRow(ctx,
			"SELECT id, external_id, email, display_name, timezone, phone, slack_user_id, role, is_active, created_at, updated_at, password_hash FROM users WHERE email = $1 AND is_active = true",
			email,
		).Scan(
			&u.ID, &u.ExternalID, &u.Email, &u.DisplayName, &u.Timezone,
			&u.Phone, &u.SlackUserID, &u.Role, &u.IsActive,
			&u.CreatedAt, &u.UpdatedAt, &u.PasswordHash,
		)
		conn.Release()

		if err == nil {
			return &u, t.Slug, t.ID.String(), nil
		}
	}

	return nil, "", "", fmt.Errorf("user not found")
}
