package alert

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/wisbric/opswatch/internal/db"
)

const (
	// dedupTTL is the Redis TTL for dedup keys (5 minutes).
	dedupTTL = 5 * time.Minute

	// redisKeyPrefix is the prefix for all dedup keys in Redis.
	redisKeyPrefix = "alert:dedup:"
)

// DedupResult describes the outcome of a deduplication check.
type DedupResult struct {
	IsDuplicate bool
	AlertID     uuid.UUID
}

// Deduplicator checks whether an incoming alert fingerprint matches an existing
// open alert. It uses Redis as a fast cache with DB fallback.
type Deduplicator struct {
	rdb     *redis.Client
	logger  *slog.Logger
	counter prometheus.Counter
}

// NewDeduplicator creates a Deduplicator. The counter is incremented each time
// a duplicate is detected.
func NewDeduplicator(rdb *redis.Client, logger *slog.Logger, counter prometheus.Counter) *Deduplicator {
	return &Deduplicator{rdb: rdb, logger: logger, counter: counter}
}

// redisKey builds the dedup cache key for a tenant + fingerprint.
func redisKey(tenantSchema, fingerprint string) string {
	return redisKeyPrefix + tenantSchema + ":" + fingerprint
}

// Check looks up the fingerprint in Redis, falling back to the database.
// If a matching open alert is found, it returns its ID.
func (d *Deduplicator) Check(ctx context.Context, tenantSchema, fingerprint string, dbtx db.DBTX) (DedupResult, error) {
	// 1. Redis hot path.
	key := redisKey(tenantSchema, fingerprint)
	val, err := d.rdb.Get(ctx, key).Result()
	if err == nil {
		// Cache hit — parse the alert UUID.
		id, parseErr := uuid.Parse(val)
		if parseErr == nil {
			return DedupResult{IsDuplicate: true, AlertID: id}, nil
		}
		d.logger.Warn("invalid UUID in dedup cache", "key", key, "value", val)
	} else if err != redis.Nil {
		// Redis error — log and fall through to DB.
		d.logger.Warn("redis dedup lookup failed, falling back to DB", "error", err)
	}

	// 2. DB fallback.
	q := db.New(dbtx)
	alert, err := q.GetAlertByFingerprint(ctx, fingerprint)
	if err != nil {
		if err == pgx.ErrNoRows {
			return DedupResult{IsDuplicate: false}, nil
		}
		return DedupResult{}, fmt.Errorf("dedup DB lookup: %w", err)
	}

	// Found in DB — warm the Redis cache.
	d.cacheSet(ctx, tenantSchema, fingerprint, alert.ID)

	return DedupResult{IsDuplicate: true, AlertID: alert.ID}, nil
}

// RecordNew stores the new alert's fingerprint in Redis for future dedup checks.
func (d *Deduplicator) RecordNew(ctx context.Context, tenantSchema, fingerprint string, alertID uuid.UUID) {
	d.cacheSet(ctx, tenantSchema, fingerprint, alertID)
}

// IncrementAndReturn bumps the occurrence count of an existing alert and returns
// its updated state as a Response.
func (d *Deduplicator) IncrementAndReturn(ctx context.Context, dbtx db.DBTX, alertID uuid.UUID) (Response, error) {
	q := db.New(dbtx)
	if err := q.IncrementAlertOccurrence(ctx, alertID); err != nil {
		return Response{}, fmt.Errorf("incrementing alert occurrence: %w", err)
	}

	row, err := q.GetAlert(ctx, alertID)
	if err != nil {
		return Response{}, fmt.Errorf("fetching updated alert: %w", err)
	}

	d.counter.Inc()

	return Response{
		ID:              row.ID,
		Fingerprint:     row.Fingerprint,
		Status:          row.Status,
		Severity:        row.Severity,
		Source:          row.Source,
		Title:           row.Title,
		Description:     row.Description,
		Labels:          row.Labels,
		Annotations:     row.Annotations,
		OccurrenceCount: row.OccurrenceCount,
		FirstFiredAt:    row.FirstFiredAt,
		LastFiredAt:     row.LastFiredAt,
		CreatedAt:       row.CreatedAt,
	}, nil
}

// cacheSet stores a fingerprint → alertID mapping in Redis with TTL.
func (d *Deduplicator) cacheSet(ctx context.Context, tenantSchema, fingerprint string, alertID uuid.UUID) {
	key := redisKey(tenantSchema, fingerprint)
	if err := d.rdb.Set(ctx, key, alertID.String(), dedupTTL).Err(); err != nil {
		d.logger.Warn("failed to set dedup cache", "error", err, "key", key)
	}
}
