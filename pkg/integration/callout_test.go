package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
)

func TestNoopCaller_Call(t *testing.T) {
	caller := &NoopCaller{Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	result, err := caller.Call(context.Background(), CalloutRequest{
		AlertID:  uuid.New(),
		UserID:   uuid.New(),
		Phone:    "+1234567890",
		Title:    "Test Alert",
		Severity: "critical",
		Method:   "phone",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Method != "phone" {
		t.Errorf("method = %q, want phone", result.Method)
	}
}

func TestNoopCaller_SendSMS(t *testing.T) {
	caller := &NoopCaller{Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	result, err := caller.SendSMS(context.Background(), CalloutRequest{
		AlertID:  uuid.New(),
		UserID:   uuid.New(),
		Phone:    "+1234567890",
		Title:    "Test Alert",
		Severity: "critical",
		Method:   "sms",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Method != "sms" {
		t.Errorf("method = %q, want sms", result.Method)
	}
}

func TestCallerInterface(t *testing.T) {
	// Verify NoopCaller implements Caller interface.
	var _ Caller = (*NoopCaller)(nil)
}
