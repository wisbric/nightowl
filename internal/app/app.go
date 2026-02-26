package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"golang.org/x/oauth2"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/core/pkg/auth"
	"github.com/wisbric/nightowl/internal/config"
	"github.com/wisbric/core/pkg/httpserver"
	"github.com/wisbric/core/pkg/platform"
	"github.com/wisbric/nightowl/internal/seed"
	coretelemetry "github.com/wisbric/core/pkg/telemetry"
	nightowlmetrics "github.com/wisbric/nightowl/internal/telemetry"
	"github.com/wisbric/core/pkg/version"
	"github.com/wisbric/nightowl/pkg/alert"
	"github.com/wisbric/nightowl/pkg/apikey"
	"github.com/wisbric/nightowl/pkg/bookowl"
	"github.com/wisbric/nightowl/pkg/escalation"
	"github.com/wisbric/nightowl/pkg/incident"
	"github.com/wisbric/nightowl/pkg/integration"
	nightowlmm "github.com/wisbric/nightowl/pkg/mattermost"
	"github.com/wisbric/nightowl/pkg/messaging"
	"github.com/wisbric/nightowl/pkg/pat"
	"github.com/wisbric/nightowl/pkg/roster"
	nightowlslack "github.com/wisbric/nightowl/pkg/slack"
	"github.com/wisbric/nightowl/internal/authadapter"
	"github.com/wisbric/nightowl/pkg/tenantconfig"
	"github.com/wisbric/nightowl/pkg/user"
)

// Run is the main application entry point. It reads config, connects to
// infrastructure, and starts the appropriate mode (api or worker).
func Run(ctx context.Context, cfg *config.Config) error {
	logger := coretelemetry.NewLogger(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting nightowl",
		"mode", cfg.Mode,
		"listen", cfg.ListenAddr(),
	)

	// Tracing
	shutdownTracer, err := coretelemetry.InitTracer(ctx, cfg.OTLPEndpoint, "nightowl", version.Version)
	if err != nil {
		return fmt.Errorf("initializing tracer: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(shutdownCtx); err != nil {
			logger.Error("shutting down tracer", "error", err)
		}
	}()

	// Database
	db, err := platform.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer db.Close()

	// Redis
	rdb, err := platform.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("closing redis", "error", err)
		}
	}()

	// Run global migrations.
	if err := platform.RunGlobalMigrations(cfg.DatabaseURL, cfg.MigrationsGlobalDir); err != nil {
		return fmt.Errorf("running global migrations: %w", err)
	}
	logger.Info("global migrations applied")

	// Metrics
	metricsReg := coretelemetry.NewMetricsRegistry(nightowlmetrics.All()...)

	switch cfg.Mode {
	case "api":
		return runAPI(ctx, cfg, logger, db, rdb, metricsReg)
	case "worker":
		return runWorker(ctx, logger, db, rdb, metricsReg)
	case "seed":
		return seed.Run(ctx, db, cfg.DatabaseURL, cfg.MigrationsTenantDir, logger)
	case "seed-demo":
		return seed.RunDemo(ctx, db, cfg.DatabaseURL, cfg.MigrationsTenantDir, logger)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func runAPI(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry) error {
	// Session manager.
	sessionSecret := cfg.SessionSecret
	if sessionSecret == "" {
		sessionSecret = auth.GenerateDevSecret()
		logger.Info("session: using auto-generated dev secret (set NIGHTOWL_SESSION_SECRET in production)")
	}
	sessionMaxAge, err := time.ParseDuration(cfg.SessionMaxAge)
	if err != nil {
		return fmt.Errorf("parsing session max age %q: %w", cfg.SessionMaxAge, err)
	}
	sessionMgr, err := auth.NewSessionManager(sessionSecret, sessionMaxAge)
	if err != nil {
		return fmt.Errorf("creating session manager: %w", err)
	}

	// OIDC authenticator (optional — nil if not configured).
	var oidcAuth *auth.OIDCAuthenticator
	if cfg.OIDCIssuerURL != "" && cfg.OIDCClientID != "" {
		var err error
		oidcAuth, err = auth.NewOIDCAuthenticator(ctx, cfg.OIDCIssuerURL, cfg.OIDCClientID)
		if err != nil {
			return fmt.Errorf("initializing OIDC authenticator: %w", err)
		}
		logger.Info("OIDC authentication enabled", "issuer", cfg.OIDCIssuerURL)
	} else {
		logger.Info("OIDC authentication disabled (OIDC_ISSUER_URL not set)")
	}

	// Auth storage adapter.
	authStore := authadapter.New(db)

	// PAT authenticator.
	patAuth := auth.NewPATAuthenticator(authStore)

	// Audit log writer (async, buffered).
	auditWriter := audit.NewWriter(db, logger)
	auditWriter.Start(ctx)
	defer auditWriter.Close()

	srv := httpserver.NewServer(httpserver.ServerConfig{
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	}, logger, db, rdb, metricsReg, sessionMgr, oidcAuth, patAuth, authStore)

	// --- Auth routes (public, pre-authentication) ---

	// Rate limiter: 10 failed attempts per IP per 15 minutes.
	rateLimiter := auth.NewRateLimiter(rdb, 10, 15*time.Minute)

	// Local admin login and change-password.
	localAdminHandler := auth.NewLocalAdminHandler(sessionMgr, authStore, logger, rateLimiter)
	srv.Router.Post("/auth/local", localAdminHandler.HandleLocalLogin)
	srv.Router.Post("/auth/change-password", localAdminHandler.HandleChangePassword)
	srv.Router.Get("/auth/config", localAdminHandler.HandleAuthConfig)

	// Existing email/password login (for tenant users, not local admins).
	loginHandler := auth.NewLoginHandler(sessionMgr, authStore, logger, oidcAuth != nil)
	srv.Router.Post("/auth/login", loginHandler.HandleLogin)
	srv.Router.Get("/auth/me", loginHandler.HandleMe)
	srv.Router.Post("/auth/logout", loginHandler.HandleLogout)

	// OIDC Authorization Code flow (only if OIDC is configured via env vars).
	if oidcAuth != nil && cfg.OIDCClientSecret != "" {
		oauth2Cfg := &oauth2.Config{
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
		}
		// The Endpoint is discovered from the OIDC provider, but oauth2
		// needs it explicitly. We reuse the issuer URL.
		oauth2Cfg.Endpoint = oauth2.Endpoint{
			AuthURL:  cfg.OIDCIssuerURL + "/authorize",
			TokenURL: cfg.OIDCIssuerURL + "/oauth/token",
		}

		oidcFlow := auth.NewOIDCFlowHandler(oauth2Cfg, oidcAuth, sessionMgr, authStore, rdb, logger)
		srv.Router.Get("/auth/oidc/login", oidcFlow.HandleLogin)
		srv.Router.Get("/auth/oidc/callback", oidcFlow.HandleCallback)
		logger.Info("OIDC Authorization Code flow enabled", "redirect_url", cfg.OIDCRedirectURL)
	}

	// Public status endpoint (no auth required — used by about page).
	srv.Router.Get("/status", srv.HandleStatus)

	// Authenticated status endpoint (backward compat).
	srv.APIRouter.Get("/status", srv.HandleStatus)

	// Mount domain handlers.
	incidentHandler := incident.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/incidents", incidentHandler.Routes())

	alertHandler := alert.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/alerts", alertHandler.Routes())

	dedup := alert.NewDeduplicator(rdb, logger, nightowlmetrics.AlertsDeduplicatedTotal)
	enricher := alert.NewEnricher(logger)
	webhookMetrics := &alert.WebhookMetrics{
		ReceivedTotal:      nightowlmetrics.AlertsReceivedTotal,
		ProcessingDuration: nightowlmetrics.AlertProcessingDuration,
		KBHitsTotal:        nightowlmetrics.KBHitsTotal,
		AgentResolvedTotal: nightowlmetrics.AlertsAgentResolvedTotal,
	}
	webhookHandler := alert.NewWebhookHandler(logger, auditWriter, dedup, enricher, webhookMetrics)
	srv.APIRouter.Mount("/webhooks", webhookHandler.Routes())

	rosterHandler := roster.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/rosters", rosterHandler.Routes())

	escalationHandler := escalation.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/escalation-policies", escalationHandler.Routes())

	twilioHandler := integration.NewTwilioHandler(logger)
	srv.APIRouter.Mount("/twilio", twilioHandler.Routes())

	userHandler := user.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/users", userHandler.Routes())
	srv.APIRouter.Mount("/user/preferences", userHandler.PreferencesRoutes())

	apikeyHandler := apikey.NewHandler(logger, auditWriter, db)
	srv.APIRouter.Mount("/api-keys", apikeyHandler.Routes())

	patHandler := pat.NewHandler(logger)
	srv.APIRouter.Mount("/user/tokens", patHandler.Routes())

	auditHandler := audit.NewHandler(logger)
	srv.APIRouter.Mount("/audit-log", auditHandler.Routes())

	bookowlHandler := bookowl.NewHandler(logger, db)
	srv.APIRouter.Mount("/bookowl", bookowlHandler.Routes())

	tenantConfigHandler := tenantconfig.NewHandler(logger, auditWriter, db)
	srv.APIRouter.Mount("/admin/config", tenantConfigHandler.Routes())

	// OIDC admin config endpoints (admin role required).
	oidcAdminHandler := auth.NewOIDCAdminHandler(authStore, logger, sessionSecret)
	srv.APIRouter.Route("/admin/oidc", func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleAdmin))
		r.Get("/config", oidcAdminHandler.HandleGetOIDCConfig)
		r.Put("/config", oidcAdminHandler.HandleUpdateOIDCConfig)
		r.Post("/test", oidcAdminHandler.HandleTestOIDCConnection)
	})
	srv.APIRouter.Route("/admin/local-admin", func(r chi.Router) {
		r.Use(auth.RequireRole(auth.RoleAdmin))
		r.Post("/reset", oidcAdminHandler.HandleResetLocalAdmin)
	})

	// --- Messaging provider registry ---
	msgRegistry := messaging.NewRegistry()

	// Slack provider + routes.
	slackNotifier := nightowlslack.NewNotifier(cfg.SlackBotToken, cfg.SlackAlertChannel, logger)
	slackProvider := nightowlslack.NewProvider(slackNotifier, logger)
	slackHandler := nightowlslack.NewHandler(slackNotifier, db, logger, cfg.SlackSigningSecret, "devco")
	srv.Router.Mount("/api/v1/slack", slackHandler.Routes())

	if slackNotifier.IsEnabled() {
		msgRegistry.Register(slackProvider)
		logger.Info("slack integration enabled", "channel", cfg.SlackAlertChannel)
	} else {
		logger.Info("slack integration disabled (SLACK_BOT_TOKEN not set)")
	}

	// Mattermost provider + routes.
	if cfg.MattermostURL != "" && cfg.MattermostBotToken != "" {
		mmClient := nightowlmm.NewClient(cfg.MattermostURL, cfg.MattermostBotToken, logger)
		actionURL := fmt.Sprintf("http://%s/api/v1/mattermost/actions", cfg.ListenAddr())
		mmProvider := nightowlmm.NewProvider(mmClient, cfg.MattermostDefaultChannelID, actionURL, logger)
		mmHandler := nightowlmm.NewHandler(mmProvider, db, logger, cfg.MattermostWebhookSecret, "devco")
		srv.Router.Mount("/api/v1/mattermost", mmHandler.Routes())
		msgRegistry.Register(mmProvider)
		logger.Info("mattermost integration enabled", "url", cfg.MattermostURL)
	} else {
		logger.Info("mattermost integration disabled (MATTERMOST_URL not set)")
	}

	// Store registry for use by test connection endpoint.
	_ = msgRegistry

	httpSrv := &http.Server{
		Addr:         cfg.ListenAddr(),
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api server listening", "addr", cfg.ListenAddr())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down api server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func runWorker(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry) error {
	logger.Info("worker started")

	// Schedule top-up: runs once at start, then every 6 hours.
	go roster.RunScheduleTopUpLoop(ctx, pool, logger, 6*time.Hour)

	engine := escalation.NewEngine(pool, rdb, logger, nightowlmetrics.AlertsEscalatedTotal)
	return engine.Run(ctx)
}
