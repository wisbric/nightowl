package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"

	"github.com/wisbric/nightowl/internal/db"
)

// OIDCFlowHandler handles the OAuth2 Authorization Code flow.
type OIDCFlowHandler struct {
	oauth2Cfg  *oauth2.Config
	oidcAuth   *OIDCAuthenticator
	sessionMgr *SessionManager
	pool       *pgxpool.Pool
	redis      *redis.Client
	logger     *slog.Logger
}

// NewOIDCFlowHandler creates a handler for the full OIDC Authorization Code flow.
func NewOIDCFlowHandler(
	oauth2Cfg *oauth2.Config,
	oidcAuth *OIDCAuthenticator,
	sm *SessionManager,
	pool *pgxpool.Pool,
	rdb *redis.Client,
	logger *slog.Logger,
) *OIDCFlowHandler {
	return &OIDCFlowHandler{
		oauth2Cfg:  oauth2Cfg,
		oidcAuth:   oidcAuth,
		sessionMgr: sm,
		pool:       pool,
		redis:      rdb,
		logger:     logger,
	}
}

// HandleLogin redirects the user to the OIDC identity provider.
func (h *OIDCFlowHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "internal", "failed to generate state")
		return
	}

	// Store state in Redis with 10 minute TTL.
	if err := h.redis.Set(r.Context(), "oidc_state:"+state, "1", 10*time.Minute).Err(); err != nil {
		h.logger.Error("oidc: storing state in redis", "error", err)
		respondErr(w, http.StatusInternalServerError, "internal", "failed to store state")
		return
	}

	url := h.oauth2Cfg.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

// HandleCallback handles the IdP callback after authentication.
func (h *OIDCFlowHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state.
	state := r.URL.Query().Get("state")
	if state == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "missing state parameter")
		return
	}

	result, err := h.redis.GetDel(ctx, "oidc_state:"+state).Result()
	if err != nil || result == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "invalid or expired state")
		return
	}

	// Check for error from IdP.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		h.logger.Warn("oidc: IdP returned error", "error", errParam, "description", desc)
		respondErr(w, http.StatusUnauthorized, "unauthorized", "authentication failed: "+errParam)
		return
	}

	// Exchange code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		respondErr(w, http.StatusBadRequest, "bad_request", "missing code parameter")
		return
	}

	oauth2Token, err := h.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("oidc: code exchange failed", "error", err)
		respondErr(w, http.StatusUnauthorized, "unauthorized", "code exchange failed")
		return
	}

	// Extract and verify the ID token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		respondErr(w, http.StatusUnauthorized, "unauthorized", "no id_token in response")
		return
	}

	claims, err := h.oidcAuth.Authenticate(ctx, "Bearer "+rawIDToken)
	if err != nil {
		h.logger.Error("oidc: token verification failed", "error", err)
		respondErr(w, http.StatusUnauthorized, "unauthorized", "invalid id_token")
		return
	}

	// Look up or create the user in the tenant schema.
	userRow, tenantID, err := h.findOrCreateUser(ctx, claims)
	if err != nil {
		h.logger.Error("oidc: user lookup/create failed", "error", err)
		respondErr(w, http.StatusInternalServerError, "internal", "failed to resolve user")
		return
	}

	// Issue session JWT.
	token, err := h.sessionMgr.IssueToken(SessionClaims{
		Subject:    claims.Subject,
		Email:      claims.Email,
		Role:       claims.Role,
		TenantSlug: claims.TenantSlug,
		TenantID:   tenantID,
		UserID:     userRow.ID.String(),
		Method:     "oidc",
	})
	if err != nil {
		h.logger.Error("oidc: issuing session token", "error", err)
		respondErr(w, http.StatusInternalServerError, "internal", "failed to issue token")
		return
	}

	// Redirect to frontend with token.
	redirectURL := fmt.Sprintf("%s?token=%s", h.oauth2Cfg.RedirectURL, token)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// findOrCreateUser resolves an OIDC user to a database user row.
func (h *OIDCFlowHandler) findOrCreateUser(ctx context.Context, claims *OIDCClaims) (*db.User, string, error) {
	q := db.New(h.pool)

	// Resolve tenant.
	t, err := q.GetTenantBySlug(ctx, claims.TenantSlug)
	if err != nil {
		return nil, "", fmt.Errorf("looking up tenant %s: %w", claims.TenantSlug, err)
	}

	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	schema := fmt.Sprintf("tenant_%s", t.Slug)
	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		return nil, "", fmt.Errorf("setting search_path: %w", err)
	}

	tq := db.New(conn)

	// Try to find existing user by external_id.
	user, err := tq.GetUserByExternalID(ctx, claims.Subject)
	if err == nil {
		return &user, t.ID.String(), nil
	}

	// Create new user.
	user, err = tq.CreateUser(ctx, db.CreateUserParams{
		ExternalID:  claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.Email, // Default to email; can be updated later.
		Timezone:    "UTC",
		Role:        claims.Role,
	})
	if err != nil {
		return nil, "", fmt.Errorf("creating user: %w", err)
	}

	h.logger.Info("oidc: created new user", "user_id", user.ID, "email", claims.Email, "tenant", claims.TenantSlug)
	return &user, t.ID.String(), nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
