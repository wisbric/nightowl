package roster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
)

// ScheduleTopUp tops up schedules for all active rosters across all tenants.
// It should be called periodically (e.g., weekly) by the worker.
func ScheduleTopUp(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	q := db.New(pool)
	tenants, err := q.ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}

	for _, t := range tenants {
		if err := topUpTenant(ctx, pool, t.Slug, logger); err != nil {
			logger.Error("schedule top-up failed for tenant", "tenant", t.Slug, "error", err)
		}
	}
	return nil
}

func topUpTenant(ctx context.Context, pool *pgxpool.Pool, slug string, logger *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO tenant_%s, public", slug)); err != nil {
		return fmt.Errorf("setting search_path: %w", err)
	}

	svc := NewService(conn, logger)
	rosters, err := svc.ListRosters(ctx)
	if err != nil {
		return fmt.Errorf("listing rosters: %w", err)
	}

	for _, r := range rosters {
		if !r.IsActive {
			continue
		}
		generated, err := svc.GenerateSchedule(ctx, r.ID, time.Now(), r.ScheduleWeeksAhead)
		if err != nil {
			logger.Error("schedule top-up failed for roster",
				"tenant", slug, "roster", r.Name, "roster_id", r.ID, "error", err)
			continue
		}
		if len(generated) > 0 {
			logger.Info("schedule top-up completed",
				"tenant", slug, "roster", r.Name, "weeks_generated", len(generated))
		}
	}
	return nil
}

// RunScheduleTopUpLoop runs schedule top-up periodically until ctx is cancelled.
func RunScheduleTopUpLoop(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, interval time.Duration) {
	logger.Info("schedule top-up loop started", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once at start.
	if err := ScheduleTopUp(ctx, pool, logger); err != nil {
		logger.Error("initial schedule top-up", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("schedule top-up loop stopped")
			return
		case <-ticker.C:
			if err := ScheduleTopUp(ctx, pool, logger); err != nil {
				logger.Error("schedule top-up", "error", err)
			}
		}
	}
}
