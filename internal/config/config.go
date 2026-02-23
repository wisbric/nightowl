package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration, loaded from environment variables.
type Config struct {
	// Mode selects the runtime mode: "api" or "worker".
	Mode string `env:"NIGHTOWL_MODE" envDefault:"api"`

	// Server
	Host string `env:"NIGHTOWL_HOST" envDefault:"0.0.0.0"`
	Port int    `env:"NIGHTOWL_PORT" envDefault:"8080"`

	// Database
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://nightowl:nightowl@localhost:5432/nightowl?sslmode=disable"`

	// Redis
	RedisURL string `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	// Telemetry
	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	MetricsPath  string `env:"METRICS_PATH" envDefault:"/metrics"`

	// Migrations
	MigrationsGlobalDir string `env:"MIGRATIONS_GLOBAL_DIR" envDefault:"migrations/global"`
	MigrationsTenantDir string `env:"MIGRATIONS_TENANT_DIR" envDefault:"migrations/tenant"`

	// CORS
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"*" envSeparator:","`

	// OIDC (optional — if not set, JWT authentication is disabled)
	OIDCIssuerURL    string `env:"OIDC_ISSUER_URL"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURL  string `env:"OIDC_REDIRECT_URL" envDefault:"http://localhost:5173/auth/callback"`

	// Session
	SessionSecret string `env:"NIGHTOWL_SESSION_SECRET"`
	SessionMaxAge string `env:"NIGHTOWL_SESSION_MAX_AGE" envDefault:"24h"`

	// Slack (optional — if not set, Slack integration is disabled)
	SlackBotToken      string `env:"SLACK_BOT_TOKEN"`
	SlackSigningSecret string `env:"SLACK_SIGNING_SECRET"`
	SlackAlertChannel  string `env:"SLACK_ALERT_CHANNEL"` // e.g. "#alerts" or channel ID

	// Mattermost (optional — if not set, Mattermost integration is disabled)
	MattermostURL              string `env:"MATTERMOST_URL"`
	MattermostBotToken         string `env:"MATTERMOST_BOT_TOKEN"`
	MattermostWebhookSecret    string `env:"MATTERMOST_WEBHOOK_SECRET"`
	MattermostDefaultChannelID string `env:"MATTERMOST_DEFAULT_CHANNEL_ID"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config from env: %w", err)
	}
	return cfg, nil
}

// ListenAddr returns the address the HTTP server should listen on.
func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
