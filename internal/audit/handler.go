package audit

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Handler provides HTTP handlers for the audit log API.
type Handler struct {
	logger *slog.Logger
}

// NewHandler creates an audit log Handler.
func NewHandler(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// Routes returns a chi.Router with audit log routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.handleList)
	return r
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	params, err := httpserver.ParseOffsetParams(r)
	if err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	conn := tenant.ConnFromContext(r.Context())
	q := db.New(conn)

	entries, err := q.ListAuditLog(r.Context(), db.ListAuditLogParams{
		Limit:  int32(params.PageSize),
		Offset: int32(params.Offset),
	})
	if err != nil {
		h.logger.Error("listing audit log", "error", err)
		httpserver.RespondError(w, http.StatusInternalServerError, "internal_error", "failed to list audit log")
		return
	}

	httpserver.Respond(w, http.StatusOK, entries)
}
