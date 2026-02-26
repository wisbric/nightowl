package pat

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for personal access token management.
type Handler struct {
	logger *slog.Logger
}

// NewHandler creates a PAT handler.
func NewHandler(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// Routes returns a chi.Router with PAT routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.handleCreate)
	r.Get("/", h.handleList)
	r.Delete("/{id}", h.handleDelete)
	return r
}

func (h *Handler) store(r *http.Request) *Store {
	conn := tenant.ConnFromContext(r.Context())
	return NewStore(conn)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !httpserver.DecodeAndValidate(w, r, &req) {
		return
	}

	id := auth.FromContext(r.Context())
	if id == nil || id.UserID == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "user identity required")
		return
	}

	// Generate random token: nwl_pat_ + 32 hex chars (16 random bytes).
	rawBytes := make([]byte, 16)
	if _, err := rand.Read(rawBytes); err != nil {
		h.logger.Error("generating token", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal", "failed to generate token")
		return
	}
	rawToken := TokenPrefix + hex.EncodeToString(rawBytes)
	prefix := rawToken[:len(TokenPrefix)+8] // nwl_pat_ + first 8 hex chars

	// Hash the full token for storage.
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	// Calculate expiry.
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = &t
	}

	store := h.store(r)
	token, err := store.Create(r.Context(), *id.UserID, req.Name, tokenHash, prefix, expiresAt)
	if err != nil {
		h.logger.Error("creating token", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal", "failed to create token")
		return
	}

	httpserver.Respond(w, http.StatusCreated, CreateResponse{
		Token:    *token,
		RawToken: rawToken,
	})
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil || id.UserID == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "user identity required")
		return
	}

	store := h.store(r)
	tokens, err := store.ListByUser(r.Context(), *id.UserID)
	if err != nil {
		h.logger.Error("listing tokens", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal", "failed to list tokens")
		return
	}

	if tokens == nil {
		tokens = []Token{}
	}

	httpserver.Respond(w, http.StatusOK, ListResponse{
		Tokens: tokens,
		Count:  len(tokens),
	})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := auth.FromContext(r.Context())
	if id == nil || id.UserID == nil {
		httpserver.RespondError(w, http.StatusUnauthorized, "unauthorized", "user identity required")
		return
	}

	tokenID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid token ID")
		return
	}

	store := h.store(r)
	if err := store.Delete(r.Context(), tokenID, *id.UserID); err != nil {
		h.logger.Error("deleting token", "error", err)
		httpserver.RespondError(w, http.StatusNotFound, "not_found", "token not found")
		return
	}

	httpserver.Respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// HashToken computes the SHA-256 hex digest of a raw PAT string.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// GenerateToken creates a new random PAT string and returns (rawToken, prefix, hash).
func GenerateToken() (raw, prefix, hash string, err error) {
	rawBytes := make([]byte, 16)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", "", "", fmt.Errorf("generating random bytes: %w", err)
	}
	raw = TokenPrefix + hex.EncodeToString(rawBytes)
	prefix = raw[:len(TokenPrefix)+8]
	hash = HashToken(raw)
	return raw, prefix, hash, nil
}
