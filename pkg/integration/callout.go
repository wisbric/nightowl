package integration

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// CalloutRequest describes a phone/SMS callout to an on-call engineer.
type CalloutRequest struct {
	AlertID  uuid.UUID
	UserID   uuid.UUID
	Phone    string // E.164 format
	Title    string
	Severity string
	Summary  string
	Method   string // "phone" or "sms"
}

// CalloutResult describes the outcome of a callout attempt.
type CalloutResult struct {
	Success bool
	Method  string
	Detail  string
}

// Caller is the interface for making phone/SMS callouts.
// Implementations include Twilio, Vonage, or a noop stub.
type Caller interface {
	Call(ctx context.Context, req CalloutRequest) (CalloutResult, error)
	SendSMS(ctx context.Context, req CalloutRequest) (CalloutResult, error)
}

// NoopCaller is a stub implementation that logs but does not actually call.
type NoopCaller struct {
	Logger *slog.Logger
}

// Call logs the callout request and returns success (noop).
func (n *NoopCaller) Call(ctx context.Context, req CalloutRequest) (CalloutResult, error) {
	n.Logger.Info("noop callout: phone call",
		"alert_id", req.AlertID,
		"user_id", req.UserID,
		"phone", req.Phone,
		"title", req.Title,
	)
	return CalloutResult{
		Success: true,
		Method:  "phone",
		Detail:  "noop: call simulated",
	}, nil
}

// SendSMS logs the SMS request and returns success (noop).
func (n *NoopCaller) SendSMS(ctx context.Context, req CalloutRequest) (CalloutResult, error) {
	n.Logger.Info("noop callout: sms",
		"alert_id", req.AlertID,
		"user_id", req.UserID,
		"phone", req.Phone,
		"title", req.Title,
	)
	return CalloutResult{
		Success: true,
		Method:  "sms",
		Detail:  "noop: sms simulated",
	}, nil
}
