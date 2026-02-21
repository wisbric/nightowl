package integration

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/httpserver"
)

// TwilioHandler provides HTTP handlers for Twilio inbound webhooks.
type TwilioHandler struct {
	logger *slog.Logger
}

// NewTwilioHandler creates a TwilioHandler.
func NewTwilioHandler(logger *slog.Logger) *TwilioHandler {
	return &TwilioHandler{logger: logger}
}

// Routes returns a chi.Router with Twilio webhook routes.
func (h *TwilioHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/acknowledge", h.handleAcknowledge)
	r.Post("/escalate", h.handleEscalate)
	r.Post("/sms", h.handleSMS)
	return r
}

// handleAcknowledge handles the Twilio callback when digit 1 is pressed.
func (h *TwilioHandler) handleAcknowledge(w http.ResponseWriter, r *http.Request) {
	alertID := r.URL.Query().Get("alert_id")
	if _, err := uuid.Parse(alertID); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert_id")
		return
	}

	h.logger.Info("twilio acknowledge callback",
		"alert_id", alertID,
	)

	// TODO: Acknowledge the alert via the alert store when Twilio is fully integrated.
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say>Alert acknowledged. Thank you.</Say>
</Response>`))
}

// handleEscalate handles the Twilio callback when digit 2 is pressed.
func (h *TwilioHandler) handleEscalate(w http.ResponseWriter, r *http.Request) {
	alertID := r.URL.Query().Get("alert_id")
	if _, err := uuid.Parse(alertID); err != nil {
		httpserver.RespondError(w, http.StatusBadRequest, "bad_request", "invalid alert_id")
		return
	}

	h.logger.Info("twilio escalate callback",
		"alert_id", alertID,
	)

	// TODO: Trigger escalation via the escalation engine when Twilio is fully integrated.
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say>Alert escalated to the next tier. Thank you.</Say>
</Response>`))
}

// handleSMS handles inbound SMS replies (ACK or ESC).
func (h *TwilioHandler) handleSMS(w http.ResponseWriter, r *http.Request) {
	body := r.FormValue("Body")
	from := r.FormValue("From")

	h.logger.Info("twilio inbound sms",
		"from", from,
		"body", body,
	)

	// TODO: Parse ACK/ESC replies and route to alert store when Twilio is fully integrated.
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Message>Received. Processing your request.</Message>
</Response>`))
}
