package slack

import (
	"bytes"
	"io"
	"net/http"

	goslack "github.com/slack-go/slack"
)

// VerifyMiddleware verifies the Slack signing secret on incoming requests.
// If signingSecret is empty, verification is skipped (dev mode).
func VerifyMiddleware(signingSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if signingSecret == "" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			sv, err := goslack.NewSecretsVerifier(r.Header, signingSecret)
			if err != nil {
				http.Error(w, "invalid signature headers", http.StatusUnauthorized)
				return
			}

			if _, err := sv.Write(body); err != nil {
				http.Error(w, "signature verification failed", http.StatusUnauthorized)
				return
			}

			if err := sv.Ensure(); err != nil {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
