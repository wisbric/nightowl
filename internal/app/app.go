package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/wisbric/nightowl/internal/audit"
	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/config"
	"github.com/wisbric/nightowl/internal/httpserver"
	"github.com/wisbric/nightowl/internal/platform"
	"github.com/wisbric/nightowl/internal/seed"
	"github.com/wisbric/nightowl/internal/telemetry"
	"github.com/wisbric/nightowl/pkg/alert"
	"github.com/wisbric/nightowl/pkg/incident"
	"github.com/wisbric/nightowl/pkg/runbook"
)

// Run is the main application entry point. It reads config, connects to
// infrastructure, and starts the appropriate mode (api or worker).
func Run(ctx context.Context, cfg *config.Config) error {
	logger := telemetry.NewLogger(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting nightowl",
		"mode", cfg.Mode,
		"listen", cfg.ListenAddr(),
	)

	// Tracing
	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "nightowl", "0.1.0")
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
	metricsReg := telemetry.NewMetricsRegistry()

	switch cfg.Mode {
	case "api":
		return runAPI(ctx, cfg, logger, db, rdb, metricsReg)
	case "worker":
		return runWorker(ctx, logger)
	case "seed":
		return seed.Run(ctx, db, cfg.DatabaseURL, cfg.MigrationsTenantDir, logger)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func runAPI(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry) error {
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

	// Audit log writer (async, buffered).
	auditWriter := audit.NewWriter(db, logger)
	auditWriter.Start(ctx)
	defer auditWriter.Close()

	srv := httpserver.NewServer(cfg, logger, db, rdb, metricsReg, oidcAuth)

	// Mount domain handlers.
	incidentHandler := incident.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/incidents", incidentHandler.Routes())

	runbookHandler := runbook.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/runbooks", runbookHandler.Routes())

	alertHandler := alert.NewHandler(logger, auditWriter)
	srv.APIRouter.Mount("/alerts", alertHandler.Routes())

	dedup := alert.NewDeduplicator(rdb, logger, telemetry.AlertsDeduplicatedTotal)
	enricher := alert.NewEnricher(logger)
	webhookMetrics := &alert.WebhookMetrics{
		ReceivedTotal:      telemetry.AlertsReceivedTotal,
		ProcessingDuration: telemetry.AlertProcessingDuration,
		KBHitsTotal:        telemetry.KBHitsTotal,
		AgentResolvedTotal: telemetry.AlertsAgentResolvedTotal,
	}
	webhookHandler := alert.NewWebhookHandler(logger, auditWriter, dedup, enricher, webhookMetrics)
	srv.APIRouter.Mount("/webhooks", webhookHandler.Routes())

	auditHandler := audit.NewHandler(logger)
	srv.APIRouter.Mount("/audit-log", auditHandler.Routes())

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

func runWorker(ctx context.Context, logger *slog.Logger) error {
	logger.Info("worker started")
	// Worker loop — placeholder for escalation engine, handoff notifications, etc.
	<-ctx.Done()
	logger.Info("worker stopped")
	return nil
}
