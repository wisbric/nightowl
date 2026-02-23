package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// SessionClaims are the claims embedded in a self-issued session JWT.
type SessionClaims struct {
	Subject    string `json:"sub"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	TenantSlug string `json:"tenant_slug"`
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id"`
	Method     string `json:"method"` // "oidc" or "local"
}

// SessionManager issues and validates self-signed session JWTs using HMAC-SHA256.
type SessionManager struct {
	signingKey []byte
	maxAge     time.Duration
}

// NewSessionManager creates a session manager. The secret must be at least 32 bytes.
func NewSessionManager(secret string, maxAge time.Duration) (*SessionManager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("session secret must be at least 32 bytes, got %d", len(secret))
	}
	return &SessionManager{
		signingKey: []byte(secret),
		maxAge:     maxAge,
	}, nil
}

// GenerateDevSecret generates a random 32-byte hex-encoded secret for dev mode.
func GenerateDevSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("reading random bytes: %v", err))
	}
	return hex.EncodeToString(b)
}

// IssueToken creates a signed JWT with the given claims.
func (sm *SessionManager) IssueToken(claims SessionClaims) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: sm.signingKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", fmt.Errorf("creating signer: %w", err)
	}

	now := time.Now()
	registered := jwt.Claims{
		Subject:   claims.Subject,
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(sm.maxAge)),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "nightowl",
	}

	token, err := jwt.Signed(signer).Claims(registered).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return token, nil
}

// ValidateToken verifies the JWT signature and expiry and returns the claims.
func (sm *SessionManager) ValidateToken(raw string) (*SessionClaims, error) {
	tok, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	var registered jwt.Claims
	var custom SessionClaims
	if err := tok.Claims(sm.signingKey, &registered, &custom); err != nil {
		return nil, fmt.Errorf("verifying token: %w", err)
	}

	if err := registered.ValidateWithLeeway(jwt.Expected{
		Issuer: "nightowl",
		Time:   time.Now(),
	}, 5*time.Second); err != nil {
		return nil, fmt.Errorf("validating claims: %w", err)
	}

	return &custom, nil
}
