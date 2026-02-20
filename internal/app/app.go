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

	"github.com/wisbric/opswatch/internal/config"
	"github.com/wisbric/opswatch/internal/httpserver"
	"github.com/wisbric/opswatch/internal/platform"
	"github.com/wisbric/opswatch/internal/telemetry"
)

// Run is the main application entry point. It reads config, connects to
// infrastructure, and starts the appropriate mode (api or worker).
func Run(ctx context.Context, cfg *config.Config) error {
	logger := telemetry.NewLogger(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting opswatch",
		"mode", cfg.Mode,
		"listen", cfg.ListenAddr(),
	)

	// Tracing
	shutdownTracer, err := telemetry.InitTracer(ctx, cfg.OTLPEndpoint, "opswatch", "0.1.0")
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

	// Metrics
	metricsReg := telemetry.NewMetricsRegistry()

	switch cfg.Mode {
	case "api":
		return runAPI(ctx, cfg, logger, db, rdb, metricsReg)
	case "worker":
		return runWorker(ctx, logger)
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func runAPI(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry) error {
	srv := httpserver.NewServer(logger, db, rdb, metricsReg)

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
