package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testPayload struct {
	Title    string `json:"title" validate:"required,min=3"`
	Severity string `json:"severity" validate:"required,oneof=info warning critical major"`
	Email    string `json:"email" validate:"omitempty,email"`
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid JSON",
			body:    `{"title":"test","severity":"warning"}`,
			wantErr: false,
		},
		{
			name:    "empty body",
			body:    "",
			wantErr: true,
			errMsg:  "request body is empty",
		},
		{
			name:    "invalid JSON",
			body:    `{invalid}`,
			wantErr: true,
			errMsg:  "invalid JSON",
		},
		{
			name:    "unknown field",
			body:    `{"title":"test","unknown":"field"}`,
			wantErr: true,
			errMsg:  "invalid JSON",
		},
		{
			name:    "trailing data",
			body:    `{"title":"test"}{"extra":true}`,
			wantErr: true,
			errMsg:  "request body must contain a single JSON object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			var p testPayload
			err := Decode(r, &p)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		payload   testPayload
		wantCount int
	}{
		{
			name:      "valid payload",
			payload:   testPayload{Title: "test title", Severity: "warning"},
			wantCount: 0,
		},
		{
			name:      "missing required fields",
			payload:   testPayload{},
			wantCount: 2, // title and severity
		},
		{
			name:      "title too short",
			payload:   testPayload{Title: "ab", Severity: "warning"},
			wantCount: 1,
		},
		{
			name:      "invalid severity",
			payload:   testPayload{Title: "test", Severity: "extreme"},
			wantCount: 1,
		},
		{
			name:      "invalid email",
			payload:   testPayload{Title: "test", Severity: "warning", Email: "not-an-email"},
			wantCount: 1,
		},
		{
			name:      "valid email",
			payload:   testPayload{Title: "test", Severity: "warning", Email: "user@example.com"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.payload)
			if len(errs) != tt.wantCount {
				t.Errorf("Validate() returned %d errors, want %d: %+v", len(errs), tt.wantCount, errs)
			}
		})
	}
}

func TestDecodeAndValidate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantOK     bool
		wantStatus int
	}{
		{
			name:   "valid request",
			body:   `{"title":"test title","severity":"warning"}`,
			wantOK: true,
		},
		{
			name:       "invalid JSON",
			body:       `{bad}`,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing required fields",
			body:       `{"title":"ab"}`,
			wantOK:     false,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var p testPayload
			ok := DecodeAndValidate(w, r, &p)
			if ok != tt.wantOK {
				t.Errorf("DecodeAndValidate() = %v, want %v", ok, tt.wantOK)
			}
			if !ok && w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Title", "title"},
		{"CreatedAt", "created_at"},
		{"ID", "i_d"},
		{"PageSize", "page_size"},
		{"lowercase", "lowercase"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := toSnakeCase(tt.in)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
