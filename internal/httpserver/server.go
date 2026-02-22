package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"

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
}

// NewServer creates an HTTP server with middleware and health/metrics endpoints.
// oidcAuth may be nil when OIDC is not configured (JWT auth will be unavailable).
// Domain handlers should be mounted on APIRouter after calling NewServer.
func NewServer(cfg *config.Config, logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry, oidcAuth *auth.OIDCAuthenticator) *Server {
	s := &Server{
		Router:  chi.NewRouter(),
		Logger:  logger,
		DB:      db,
		Redis:   rdb,
		Metrics: metricsReg,
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
