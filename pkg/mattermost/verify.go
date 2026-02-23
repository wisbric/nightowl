package mattermost

import "net/http"

// VerifyMiddleware verifies the Mattermost webhook token on incoming requests.
// If webhookSecret is empty, verification is skipped (dev mode).
func VerifyMiddleware(webhookSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if webhookSecret == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Mattermost sends the token as a form value or JSON field.
			// For slash commands it's a form value; for actions/dialogs we skip
			// token check since those come from the server directly.
			token := r.FormValue("token")
			if token == "" {
				// For JSON payloads (actions/dialogs), we trust that the
				// action URL is only known to the Mattermost server.
				next.ServeHTTP(w, r)
				return
			}

			if token != webhookSecret {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
