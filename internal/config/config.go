package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration, loaded from environment variables.
type Config struct {
	// Mode selects the runtime mode: "api" or "worker".
	Mode string `env:"OPSWATCH_MODE" envDefault:"api"`

	// Server
	Host string `env:"OPSWATCH_HOST" envDefault:"0.0.0.0"`
	Port int    `env:"OPSWATCH_PORT" envDefault:"8080"`

	// Database
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://opswatch:opswatch@localhost:5432/opswatch?sslmode=disable"`

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
	OIDCIssuerURL string `env:"OIDC_ISSUER_URL"`
	OIDCClientID  string `env:"OIDC_CLIENT_ID"`
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
