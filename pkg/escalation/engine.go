package escalation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// Engine is a background worker that polls for unacknowledged alerts and
// escalates them through their configured escalation policy tiers.
type Engine struct {
	pool     *pgxpool.Pool
	rdb      *redis.Client
	logger   *slog.Logger
	interval time.Duration
	metric   *prometheus.CounterVec // alerts_escalated_total{tier}
}

// NewEngine creates a new escalation engine.
func NewEngine(pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger, metric *prometheus.CounterVec) *Engine {
	return &Engine{
		pool:     pool,
		rdb:      rdb,
		logger:   logger,
		interval: 30 * time.Second,
		metric:   metric,
	}
}

// Run starts the escalation engine loop. It blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) error {
	e.logger.Info("escalation engine started", "interval", e.interval)

	// Subscribe to acknowledgment events via Redis pub/sub.
	pubsub := e.rdb.Subscribe(ctx, "nightowl:alert:ack")
	defer pubsub.Close()

	ackCh := pubsub.Channel()
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("escalation engine stopped")
			return nil
		case msg := <-ackCh:
			e.logger.Debug("received ack event via pub/sub", "payload", msg.Payload)
			// Ack events stop escalation for the specific alert.
			// The next poll cycle will skip acknowledged alerts naturally
			// since we only query status='firing'.
		case <-ticker.C:
			if err := e.tick(ctx); err != nil {
				e.logger.Error("escalation engine tick", "error", err)
			}
		}
	}
}

// tick performs a single escalation check across all tenants.
func (e *Engine) tick(ctx context.Context) error {
	q := db.New(e.pool)
	tenants, err := q.ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}

	for _, t := range tenants {
		if err := e.processTenant(ctx, t.Slug); err != nil {
			e.logger.Error("processing tenant escalations",
				"tenant", t.Slug,
				"error", err,
			)
		}
	}
	return nil
}

// processTenant checks and escalates alerts for a single tenant.
func (e *Engine) processTenant(ctx context.Context, slug string) error {
	schema := tenant.SchemaName(slug)
	conn, err := e.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		return fmt.Errorf("setting search_path: %w", err)
	}

	tq := db.New(conn)

	alerts, err := tq.ListPendingEscalationAlerts(ctx)
	if err != nil {
		return fmt.Errorf("listing pending escalation alerts: %w", err)
	}

	for _, a := range alerts {
		if err := e.processAlert(ctx, tq, a); err != nil {
			e.logger.Error("processing alert escalation",
				"alert_id", a.ID,
				"error", err,
			)
		}
	}
	return nil
}

// processAlert evaluates whether an alert needs escalation and performs it.
func (e *Engine) processAlert(ctx context.Context, q *db.Queries, a db.Alert) error {
	if !a.EscalationPolicyID.Valid {
		return nil
	}

	policyID := uuid.UUID(a.EscalationPolicyID.Bytes)
	policy, err := q.GetEscalationPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("getting escalation policy %s: %w", policyID, err)
	}

	tiers := parseTiers(policy.Tiers)
	if len(tiers) == 0 {
		return nil
	}

	currentTier := int32(0)
	if a.CurrentEscalationTier != nil {
		currentTier = *a.CurrentEscalationTier
	}

	// Find the next tier to escalate to.
	var nextTierIdx int
	found := false
	for i, t := range tiers {
		if int32(t.Tier) > currentTier {
			nextTierIdx = i
			found = true
			break
		}
	}

	if !found {
		// Already at the highest tier. Check if repeat is configured.
		if policy.RepeatCount != nil && *policy.RepeatCount > 0 {
			// Calculate how many full cycles we've done.
			maxTier := int32(tiers[len(tiers)-1].Tier)
			if currentTier >= maxTier {
				// Reset to tier 1 for repeat (simplified: just re-enter from tier 1).
				nextTierIdx = 0
				found = true
			}
		}
		if !found {
			return nil // Fully escalated, no repeat.
		}
	}

	nextTier := tiers[nextTierIdx]

	// Check if enough time has elapsed since the alert was created (or last escalation).
	// We calculate cumulative timeout: sum of all tier timeouts up to and including the next tier.
	cumulativeTimeout := 0
	for i := 0; i <= nextTierIdx; i++ {
		cumulativeTimeout += tiers[i].TimeoutMinutes
	}

	elapsed := time.Since(a.CreatedAt)
	if elapsed < time.Duration(cumulativeTimeout)*time.Minute {
		return nil // Not yet time to escalate.
	}

	e.logger.Info("escalating alert",
		"alert_id", a.ID,
		"from_tier", currentTier,
		"to_tier", nextTier.Tier,
		"elapsed_minutes", int(elapsed.Minutes()),
	)

	// Update alert's current escalation tier.
	newTier := int32(nextTier.Tier)
	if err := q.UpdateAlertEscalationTier(ctx, db.UpdateAlertEscalationTierParams{
		ID:                    a.ID,
		CurrentEscalationTier: &newTier,
	}); err != nil {
		return fmt.Errorf("updating escalation tier: %w", err)
	}

	// Persist escalation event.
	notifyVia := ""
	if len(nextTier.NotifyVia) > 0 {
		notifyVia = nextTier.NotifyVia[0]
	}
	if _, err := q.CreateEscalationEvent(ctx, db.CreateEscalationEventParams{
		AlertID:      a.ID,
		PolicyID:     policyID,
		Tier:         int32(nextTier.Tier),
		Action:       "escalate",
		NotifyMethod: &notifyVia,
	}); err != nil {
		return fmt.Errorf("creating escalation event: %w", err)
	}

	// Publish escalation event to Redis for notification consumers.
	payload, _ := json.Marshal(map[string]any{
		"alert_id":  a.ID.String(),
		"policy_id": policyID.String(),
		"tier":      nextTier.Tier,
		"title":     a.Title,
		"severity":  a.Severity,
	})
	e.rdb.Publish(ctx, "nightowl:alert:escalated", string(payload))

	// Record metric.
	if e.metric != nil {
		e.metric.WithLabelValues(strconv.Itoa(nextTier.Tier)).Inc()
	}

	return nil
}

// PublishAck publishes an alert acknowledgment event to Redis pub/sub,
// which the escalation engine listens to.
func PublishAck(ctx context.Context, rdb *redis.Client, alertID uuid.UUID) {
	rdb.Publish(ctx, "nightowl:alert:ack", alertID.String())
}
