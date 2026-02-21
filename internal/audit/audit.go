package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/opswatch/internal/auth"
	"github.com/wisbric/opswatch/internal/db"
	"github.com/wisbric/opswatch/pkg/tenant"
)

// Entry represents a single audit log entry to be written.
type Entry struct {
	TenantSchema string
	UserID       pgtype.UUID
	APIKeyID     pgtype.UUID
	Action       string
	Resource     string
	ResourceID   uuid.UUID
	Detail       json.RawMessage
	IPAddress    *netip.Addr
	UserAgent    *string
}

// Writer is an async, buffered audit log writer.
// Entries are sent to an internal channel and flushed by a background goroutine.
type Writer struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	entries chan Entry
	wg      sync.WaitGroup
}

const (
	bufferSize    = 256
	flushInterval = 2 * time.Second
	flushBatch    = 32
)

// NewWriter creates an audit Writer. Call Start to begin processing entries.
func NewWriter(pool *pgxpool.Pool, logger *slog.Logger) *Writer {
	return &Writer{
		pool:    pool,
		logger:  logger,
		entries: make(chan Entry, bufferSize),
	}
}

// Start begins the background goroutine that flushes audit entries to the database.
// It returns when the context is cancelled and all pending entries are flushed.
func (w *Writer) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.run(ctx)
	}()
}

// Close waits for all pending entries to be flushed.
func (w *Writer) Close() {
	close(w.entries)
	w.wg.Wait()
}

// Log enqueues an audit entry for async writing. It never blocks the caller;
// if the buffer is full the entry is dropped and a warning is logged.
func (w *Writer) Log(entry Entry) {
	select {
	case w.entries <- entry:
	default:
		w.logger.Warn("audit log buffer full, dropping entry",
			"action", entry.Action, "resource", entry.Resource)
	}
}

// LogFromRequest is a convenience method that extracts identity, tenant, IP,
// and user agent from the request context, then enqueues the entry.
func (w *Writer) LogFromRequest(r *http.Request, action, resource string, resourceID uuid.UUID, detail json.RawMessage) {
	entry := Entry{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
	}

	if ti := tenant.FromContext(r.Context()); ti != nil {
		entry.TenantSchema = ti.Schema
	}

	if id := auth.FromContext(r.Context()); id != nil {
		if id.UserID != nil {
			entry.UserID = pgtype.UUID{Bytes: *id.UserID, Valid: true}
		}
		if id.APIKeyID != nil {
			entry.APIKeyID = pgtype.UUID{Bytes: *id.APIKeyID, Valid: true}
		}
	}

	ip := clientIP(r)
	if ip.IsValid() {
		entry.IPAddress = &ip
	}

	ua := r.Header.Get("User-Agent")
	if ua != "" {
		entry.UserAgent = &ua
	}

	w.Log(entry)
}

// run is the background loop that drains the entries channel.
func (w *Writer) run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]Entry, 0, flushBatch)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flush(batch)
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-w.entries:
			if !ok {
				// Channel closed — flush remaining and exit.
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= flushBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			// Drain any remaining entries.
			for {
				select {
				case entry, ok := <-w.entries:
					if !ok {
						flush()
						return
					}
					batch = append(batch, entry)
				default:
					flush()
					return
				}
			}
		}
	}
}

// flush writes a batch of entries to the database, grouped by tenant schema.
func (w *Writer) flush(entries []Entry) {
	// Group by tenant schema.
	bySchema := make(map[string][]Entry)
	for _, e := range entries {
		bySchema[e.TenantSchema] = append(bySchema[e.TenantSchema], e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for schema, schemaEntries := range bySchema {
		if schema == "" {
			w.logger.Warn("audit entry without tenant schema, skipping", "count", len(schemaEntries))
			continue
		}

		conn, err := w.pool.Acquire(ctx)
		if err != nil {
			w.logger.Error("acquiring connection for audit flush", "error", err, "schema", schema)
			continue
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
			w.logger.Error("setting search_path for audit flush", "error", err, "schema", schema)
			conn.Release()
			continue
		}

		q := db.New(conn)
		for _, e := range schemaEntries {
			if _, err := q.CreateAuditLogEntry(ctx, db.CreateAuditLogEntryParams{
				UserID:     e.UserID,
				ApiKeyID:   e.APIKeyID,
				Action:     e.Action,
				Resource:   e.Resource,
				ResourceID: pgtype.UUID{Bytes: e.ResourceID, Valid: e.ResourceID != uuid.Nil},
				Detail:     e.Detail,
				IpAddress:  e.IPAddress,
				UserAgent:  e.UserAgent,
			}); err != nil {
				w.logger.Error("writing audit log entry", "error", err,
					"action", e.Action, "resource", e.Resource, "schema", schema)
			}
		}

		conn.Release()
	}
}

// clientIP extracts the client IP address from the request,
// preferring X-Forwarded-For and X-Real-IP headers over RemoteAddr.
func clientIP(r *http.Request) netip.Addr {
	// X-Forwarded-For: first entry is the original client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		if addr, err := netip.ParseAddr(strings.TrimSpace(parts[0])); err == nil {
			return addr
		}
	}

	// X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if addr, err := netip.ParseAddr(strings.TrimSpace(xri)); err == nil {
			return addr
		}
	}

	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, _ := netip.ParseAddr(host)
	return addr
}
