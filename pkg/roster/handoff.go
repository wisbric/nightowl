package roster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// HandoffReport describes an on-call handoff event.
type HandoffReport struct {
	RosterID   string `json:"roster_id"`
	RosterName string `json:"roster_name"`
	OutgoingID string `json:"outgoing_user_id,omitempty"`
	IncomingID string `json:"incoming_user_id"`
	HandoffAt  string `json:"handoff_at"`
}

// HandoffWorker checks for handoff times and generates notifications.
type HandoffWorker struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	interval time.Duration
}

// NewHandoffWorker creates a new handoff notification worker.
func NewHandoffWorker(pool *pgxpool.Pool, logger *slog.Logger) *HandoffWorker {
	return &HandoffWorker{
		pool:     pool,
		logger:   logger,
		interval: 1 * time.Minute,
	}
}

// Run starts the handoff worker loop. It blocks until ctx is cancelled.
func (w *HandoffWorker) Run(ctx context.Context) error {
	w.logger.Info("handoff worker started", "interval", w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("handoff worker stopped")
			return nil
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.Error("handoff worker tick", "error", err)
			}
		}
	}
}

// tick checks all tenants for handoff events.
func (w *HandoffWorker) tick(ctx context.Context) error {
	q := db.New(w.pool)
	tenants, err := q.ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}

	now := time.Now()
	for _, t := range tenants {
		if err := w.processTenant(ctx, t.Slug, now); err != nil {
			w.logger.Error("processing tenant handoffs",
				"tenant", t.Slug,
				"error", err,
			)
		}
	}
	return nil
}

// processTenant checks all rosters in a tenant for handoff events near now.
func (w *HandoffWorker) processTenant(ctx context.Context, slug string, now time.Time) error {
	schema := tenant.SchemaName(slug)
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		return fmt.Errorf("setting search_path: %w", err)
	}

	tq := db.New(conn)
	rosters, err := tq.ListRosters(ctx)
	if err != nil {
		return fmt.Errorf("listing rosters: %w", err)
	}

	for _, r := range rosters {
		handoffStr := pgtypeTimeToString(r.HandoffTime)
		tz, err := time.LoadLocation(r.Timezone)
		if err != nil {
			w.logger.Warn("invalid timezone for roster",
				"roster_id", r.ID,
				"timezone", r.Timezone,
			)
			continue
		}

		localNow := now.In(tz)
		handoffTime, err := time.Parse("15:04", handoffStr)
		if err != nil {
			continue
		}

		// Check if we're within 1 minute of the handoff time.
		todayHandoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(),
			handoffTime.Hour(), handoffTime.Minute(), 0, 0, tz)

		diff := localNow.Sub(todayHandoff)
		if diff >= 0 && diff < w.interval {
			w.logger.Info("handoff triggered",
				"roster_id", r.ID,
				"roster_name", r.Name,
				"handoff_time", handoffStr,
				"timezone", r.Timezone,
			)
			// TODO: Send actual notifications via Slack/email when integrated.
			// For now, we log the handoff event.
		}
	}

	return nil
}
