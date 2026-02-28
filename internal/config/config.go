package config

import (
	coreconfig "github.com/wisbric/core/pkg/config"
)

// Config holds all NightOwl-specific configuration, embedding shared infra fields.
type Config struct {
	coreconfig.BaseConfig

	// Session
	SessionSecret string `env:"NIGHTOWL_SESSION_SECRET"`
	SessionMaxAge string `env:"NIGHTOWL_SESSION_MAX_AGE" envDefault:"24h"`

	// OIDC (NightOwl-specific: Authorization Code flow)
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL  string `env:"OIDC_REDIRECT_URL" envDefault:"http://localhost:5173/auth/callback"`

	// Metrics
	MetricsPath string `env:"METRICS_PATH" envDefault:"/metrics"`

	// Slack
	SlackBotToken      string `env:"SLACK_BOT_TOKEN"`
	SlackSigningSecret string `env:"SLACK_SIGNING_SECRET"`
	SlackAlertChannel  string `env:"SLACK_ALERT_CHANNEL"`

	// Admin
	AdminPassword string `env:"NIGHTOWL_ADMIN_PASSWORD"`

	// Mattermost
	MattermostURL              string `env:"MATTERMOST_URL"`
	MattermostBotToken         string `env:"MATTERMOST_BOT_TOKEN"`
	MattermostWebhookSecret    string `env:"MATTERMOST_WEBHOOK_SECRET"`
	MattermostDefaultChannelID string `env:"MATTERMOST_DEFAULT_CHANNEL_ID"`
}
