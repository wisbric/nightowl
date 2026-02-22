package httpserver

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/config"
	"github.com/wisbric/nightowl/internal/docs"
	"github.com/wisbric/nightowl/internal/version"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Server holds the HTTP server dependencies.
type Server struct {
	Router    *chi.Mux
	APIRouter chi.Router // authenticated, tenant-scoped /api/v1 sub-router
	Logger    *slog.Logger
	DB        *pgxpool.Pool
	Redis     *redis.Client
	Metrics   *prometheus.Registry
	startedAt time.Time
}

// NewServer creates an HTTP server with middleware and health/metrics endpoints.
// oidcAuth may be nil when OIDC is not configured (JWT auth will be unavailable).
// Domain handlers should be mounted on APIRouter after calling NewServer.
func NewServer(cfg *config.Config, logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry, oidcAuth *auth.OIDCAuthenticator) *Server {
	s := &Server{
		Router:    chi.NewRouter(),
		Logger:    logger,
		DB:        db,
		Redis:     rdb,
		Metrics:   metricsReg,
		startedAt: time.Now(),
	}

	// Global middleware
	s.Router.Use(RequestID)
	s.Router.Use(Logger(logger))
	s.Router.Use(Metrics)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key", "X-Request-ID", "X-Tenant-Slug"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health endpoints (unauthenticated)
	s.Router.Get("/healthz", s.handleHealthz)
	s.Router.Get("/readyz", s.handleReadyz)

	// Prometheus metrics (unauthenticated)
	s.Router.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))

	// API documentation (unauthenticated)
	s.Router.Get("/api/docs", docs.SwaggerUIHandler())
	s.Router.Get("/api/docs/openapi.yaml", docs.OpenAPISpecHandler())

	// Authenticated, tenant-scoped API routes.
	s.Router.Route("/api/v1", func(r chi.Router) {
		// 1. Authenticate: JWT → API key → dev header fallback.
		r.Use(auth.Middleware(oidcAuth, db, logger))

		// 2. Resolve tenant and set search_path from the authenticated identity.
		r.Use(tenant.Middleware(db, &authContextResolver{}, logger))

		// 3. Require valid authentication on all /api/v1 routes.
		r.Use(auth.RequireAuth)

		// Debug endpoint.
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			t := tenant.FromContext(r.Context())
			id := auth.FromContext(r.Context())
			Respond(w, http.StatusOK, map[string]string{
				"tenant":  t.Slug,
				"schema":  t.Schema,
				"subject": id.Subject,
				"role":    id.Role,
				"method":  id.Method,
			})
		})

		// Store reference so domain handlers can be mounted externally.
		s.APIRouter = r
	})

	return s
}

// authContextResolver reads the tenant slug from the auth Identity stored in
// the request context by the auth middleware. This connects authentication to
// tenant resolution without creating import cycles.
type authContextResolver struct{}

func (authContextResolver) Resolve(r *http.Request) (string, error) {
	id := auth.FromContext(r.Context())
	if id != nil && id.TenantSlug != "" {
		return id.TenantSlug, nil
	}
	return "", fmt.Errorf("no authenticated tenant")
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	Respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.DB.Ping(ctx); err != nil {
		s.Logger.Error("readiness check: database ping failed", "error", err)
		RespondError(w, http.StatusServiceUnavailable, "unavailable", "database not ready")
		return
	}

	if err := s.Redis.Ping(ctx).Err(); err != nil {
		s.Logger.Error("readiness check: redis ping failed", "error", err)
		RespondError(w, http.StatusServiceUnavailable, "unavailable", "redis not ready")
		return
	}

	Respond(w, http.StatusOK, map[string]string{"status": "ready"})
}

// statusResponse is the JSON shape returned by HandleStatus.
type statusResponse struct {
	Status          string  `json:"status"`
	Version         string  `json:"version"`
	CommitSHA       string  `json:"commit_sha"`
	Uptime          string  `json:"uptime"`
	UptimeSeconds   int64   `json:"uptime_seconds"`
	Database        string  `json:"database"`
	DatabaseLatency float64 `json:"database_latency_ms"`
	Redis           string  `json:"redis"`
	RedisLatency    float64 `json:"redis_latency_ms"`
	LastAlertAt     *string `json:"last_alert_at"`
}

// HandleStatus returns system health information including DB/Redis connectivity,
// uptime, and the timestamp of the most recent alert.
func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uptime := time.Since(s.startedAt)

	resp := statusResponse{
		Version:       version.Version,
		CommitSHA:     version.Commit,
		Uptime:        uptime.Truncate(time.Second).String(),
		UptimeSeconds: int64(uptime.Seconds()),
	}

	// Ping database.
	dbStart := time.Now()
	if err := s.DB.Ping(ctx); err != nil {
		s.Logger.Error("status check: database ping failed", "error", err)
		resp.Database = "error"
	} else {
		resp.Database = "ok"
	}
	resp.DatabaseLatency = math.Round(float64(time.Since(dbStart).Microseconds())/10) / 100 // ms with 2 decimal places

	// Ping Redis.
	redisStart := time.Now()
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		s.Logger.Error("status check: redis ping failed", "error", err)
		resp.Redis = "error"
	} else {
		resp.Redis = "ok"
	}
	resp.RedisLatency = math.Round(float64(time.Since(redisStart).Microseconds())/10) / 100

	// Overall status.
	if resp.Database == "ok" && resp.Redis == "ok" {
		resp.Status = "ok"
	} else {
		resp.Status = "degraded"
	}

	// Query last alert timestamp from tenant schema.
	conn := tenant.ConnFromContext(ctx)
	if conn != nil {
		var lastAlert *time.Time
		err := conn.QueryRow(ctx, "SELECT MAX(last_fired_at) FROM alerts").Scan(&lastAlert)
		if err != nil {
			s.Logger.Error("status check: querying last alert", "error", err)
		} else if lastAlert != nil {
			formatted := lastAlert.UTC().Format(time.RFC3339)
			resp.LastAlertAt = &formatted
		}
	}

	Respond(w, http.StatusOK, resp)
}
