package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"github.com/wisbric/opswatch/pkg/tenant"
)

// Server holds the HTTP server dependencies.
type Server struct {
	Router  *chi.Mux
	Logger  *slog.Logger
	DB      *pgxpool.Pool
	Redis   *redis.Client
	Metrics *prometheus.Registry
}

// NewServer creates an HTTP server with middleware and health/metrics endpoints.
func NewServer(logger *slog.Logger, db *pgxpool.Pool, rdb *redis.Client, metricsReg *prometheus.Registry) *Server {
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

	// Health endpoints
	s.Router.Get("/healthz", s.handleHealthz)
	s.Router.Get("/readyz", s.handleReadyz)

	// Prometheus metrics
	s.Router.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))

	// Tenant-scoped API routes.
	s.Router.Route("/api/v1", func(r chi.Router) {
		r.Use(tenant.Middleware(db, tenant.HeaderResolver{}, logger))

		// Placeholder — domain handlers will be mounted here in later phases.
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			t := tenant.FromContext(r.Context())
			Respond(w, http.StatusOK, map[string]string{
				"tenant": t.Slug,
				"schema": t.Schema,
			})
		})
	})

	return s
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
