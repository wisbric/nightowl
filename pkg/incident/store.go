package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for incidents.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates an incident Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

// incidentColumns is the shared column list for incident queries (excludes search_vector).
const incidentColumns = `id, title, fingerprints, severity, category, tags, services, clusters,
	namespaces, symptoms, error_patterns, root_cause, solution, runbook_id,
	resolution_count, last_resolved_at, last_resolved_by, avg_resolution_mins,
	merged_into_id, created_by, created_at, updated_at`

// scanIncidentRow scans a pgx.Row into an IncidentRow.
func scanIncidentRow(row pgx.Row) (IncidentRow, error) {
	var r IncidentRow
	err := row.Scan(
		&r.ID, &r.Title, &r.Fingerprints, &r.Severity, &r.Category,
		&r.Tags, &r.Services, &r.Clusters, &r.Namespaces, &r.Symptoms,
		&r.ErrorPatterns, &r.RootCause, &r.Solution, &r.RunbookID,
		&r.ResolutionCount, &r.LastResolvedAt, &r.LastResolvedBy,
		&r.AvgResolutionMins, &r.MergedIntoID, &r.CreatedBy,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// scanIncidentRows scans multiple rows into IncidentRow slices.
func scanIncidentRows(rows pgx.Rows) ([]IncidentRow, error) {
	defer rows.Close()
	var items []IncidentRow
	for rows.Next() {
		var r IncidentRow
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Fingerprints, &r.Severity, &r.Category,
			&r.Tags, &r.Services, &r.Clusters, &r.Namespaces, &r.Symptoms,
			&r.ErrorPatterns, &r.RootCause, &r.Solution, &r.RunbookID,
			&r.ResolutionCount, &r.LastResolvedAt, &r.LastResolvedBy,
			&r.AvgResolutionMins, &r.MergedIntoID, &r.CreatedBy,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning incident row: %w", err)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating incident rows: %w", err)
	}
	return items, nil
}

// Get returns a single incident by ID.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (IncidentRow, error) {
	query := `SELECT ` + incidentColumns + ` FROM incidents WHERE id = $1`
	row := s.dbtx.QueryRow(ctx, query, id)
	return scanIncidentRow(row)
}

// Create inserts a new incident.
func (s *Store) Create(ctx context.Context, p CreateParams) (IncidentRow, error) {
	query := `INSERT INTO incidents (
		title, fingerprints, severity, category, tags,
		services, clusters, namespaces, symptoms, error_patterns,
		root_cause, solution, runbook_id, created_by
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	RETURNING ` + incidentColumns
	row := s.dbtx.QueryRow(ctx, query,
		p.Title, p.Fingerprints, p.Severity, p.Category, p.Tags,
		p.Services, p.Clusters, p.Namespaces, p.Symptoms, p.ErrorPatterns,
		p.RootCause, p.Solution, p.RunbookID, p.CreatedBy,
	)
	return scanIncidentRow(row)
}

// CreateParams holds parameters for creating an incident.
type CreateParams struct {
	Title         string
	Fingerprints  []string
	Severity      string
	Category      *string
	Tags          []string
	Services      []string
	Clusters      []string
	Namespaces    []string
	Symptoms      *string
	ErrorPatterns []string
	RootCause     *string
	Solution      *string
	RunbookID     pgtype.UUID
	CreatedBy     pgtype.UUID
}

// Update updates all editable fields of an incident.
type UpdateParams struct {
	ID            uuid.UUID
	Title         string
	Fingerprints  []string
	Severity      string
	Category      *string
	Tags          []string
	Services      []string
	Clusters      []string
	Namespaces    []string
	Symptoms      *string
	ErrorPatterns []string
	RootCause     *string
	Solution      *string
	RunbookID     pgtype.UUID
}

// Update updates all editable fields and returns the updated row.
func (s *Store) Update(ctx context.Context, p UpdateParams) (IncidentRow, error) {
	query := `UPDATE incidents
	SET title = $2, fingerprints = $3, severity = $4, category = $5, tags = $6,
	    services = $7, clusters = $8, namespaces = $9, symptoms = $10,
	    error_patterns = $11, root_cause = $12, solution = $13,
	    runbook_id = $14, updated_at = now()
	WHERE id = $1
	RETURNING ` + incidentColumns
	row := s.dbtx.QueryRow(ctx, query,
		p.ID, p.Title, p.Fingerprints, p.Severity, p.Category,
		p.Tags, p.Services, p.Clusters, p.Namespaces, p.Symptoms,
		p.ErrorPatterns, p.RootCause, p.Solution, p.RunbookID,
	)
	return scanIncidentRow(row)
}

// SoftDelete marks an incident as archived by setting merged_into_id to its own ID.
func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE incidents SET merged_into_id = id, updated_at = now()
	WHERE id = $1 AND merged_into_id IS NULL`
	tag, err := s.dbtx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft deleting incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListFiltered returns incidents matching the given filters with offset pagination.
func (s *Store) ListFiltered(ctx context.Context, filters ListFilters, limit, offset int) ([]IncidentRow, error) {
	where, args := buildFilterClauses(filters)
	argN := len(args) + 1
	query := fmt.Sprintf(
		`SELECT %s FROM incidents WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		incidentColumns, strings.Join(where, " AND "), argN, argN+1,
	)
	args = append(args, limit, offset)

	rows, err := s.dbtx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing incidents: %w", err)
	}
	return scanIncidentRows(rows)
}

// CountFiltered returns the count of incidents matching the given filters.
func (s *Store) CountFiltered(ctx context.Context, filters ListFilters) (int, error) {
	where, args := buildFilterClauses(filters)
	query := fmt.Sprintf(
		`SELECT count(*) FROM incidents WHERE %s`,
		strings.Join(where, " AND "),
	)
	var count int
	if err := s.dbtx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting incidents: %w", err)
	}
	return count, nil
}

// buildFilterClauses builds WHERE clause fragments and args for incident filters.
func buildFilterClauses(filters ListFilters) ([]string, []any) {
	where := []string{"merged_into_id IS NULL"}
	var args []any
	argN := 1

	if filters.Severity != "" {
		where = append(where, fmt.Sprintf("severity = $%d", argN))
		args = append(args, filters.Severity)
		argN++
	}
	if filters.Category != "" {
		where = append(where, fmt.Sprintf("category = $%d", argN))
		args = append(args, filters.Category)
		argN++
	}
	if filters.Service != "" {
		where = append(where, fmt.Sprintf("$%d = ANY(services)", argN))
		args = append(args, filters.Service)
		argN++
	}
	if len(filters.Tags) > 0 {
		where = append(where, fmt.Sprintf("tags && $%d::text[]", argN))
		args = append(args, filters.Tags)
	}

	return where, args
}

// Search performs a full-text search with ranking and ts_headline highlighting.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	sql := `SELECT i.id, i.title, i.severity, i.category, i.services, i.tags,
		i.symptoms, i.root_cause, i.solution, i.runbook_id,
		ts_rank(i.search_vector, q) AS rank,
		ts_headline('english', COALESCE(i.title, ''), q,
			'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=10') AS title_highlight,
		ts_headline('english', COALESCE(i.symptoms, ''), q,
			'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=10') AS symptoms_highlight,
		ts_headline('english', COALESCE(i.solution, ''), q,
			'StartSel=<mark>, StopSel=</mark>, MaxWords=80, MinWords=15') AS solution_highlight,
		i.resolution_count, i.created_at
	FROM incidents i, plainto_tsquery('english', $1) q
	WHERE i.search_vector @@ q
	  AND i.merged_into_id IS NULL
	ORDER BY rank DESC
	LIMIT $2`

	rows, err := s.dbtx.Query(ctx, sql, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching incidents: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var runbookID pgtype.UUID
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Severity, &r.Category, &r.Services, &r.Tags,
			&r.Symptoms, &r.RootCause, &r.Solution, &runbookID,
			&r.Rank, &r.TitleHighlight, &r.SymptomsHighlight, &r.SolutionHighlight,
			&r.ResolutionCount, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning search row: %w", err)
		}
		if runbookID.Valid {
			uid := uuid.UUID(runbookID.Bytes)
			r.RunbookID = &uid
		}
		r.Services = ensureSlice(r.Services)
		r.Tags = ensureSlice(r.Tags)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search rows: %w", err)
	}
	return results, nil
}

// GetByFingerprint returns the first non-merged incident matching the given fingerprint.
func (s *Store) GetByFingerprint(ctx context.Context, fingerprint string) (IncidentRow, error) {
	query := `SELECT ` + incidentColumns + ` FROM incidents
	WHERE $1 = ANY(fingerprints)
	  AND merged_into_id IS NULL
	LIMIT 1`
	row := s.dbtx.QueryRow(ctx, query, fingerprint)
	return scanIncidentRow(row)
}

// RunbookSummary holds the title and content of a linked runbook.
type RunbookSummary struct {
	Title   string
	Content string
}

// GetRunbookSummary returns the title and content for a runbook by ID.
func (s *Store) GetRunbookSummary(ctx context.Context, id uuid.UUID) (*RunbookSummary, error) {
	var rb RunbookSummary
	err := s.dbtx.QueryRow(ctx, `SELECT title, content FROM runbooks WHERE id = $1`, id).Scan(&rb.Title, &rb.Content)
	if err != nil {
		return nil, err
	}
	return &rb, nil
}

// SetMergedInto marks the source incident as merged into the target.
func (s *Store) SetMergedInto(ctx context.Context, sourceID, targetID uuid.UUID) error {
	query := `UPDATE incidents SET merged_into_id = $2, updated_at = now()
	WHERE id = $1 AND merged_into_id IS NULL`
	tag, err := s.dbtx.Exec(ctx, query, sourceID, targetID)
	if err != nil {
		return fmt.Errorf("setting merged_into_id: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ReassignAlerts updates all alerts that reference the source incident to point to the target.
func (s *Store) ReassignAlerts(ctx context.Context, sourceID, targetID uuid.UUID) (int64, error) {
	query := `UPDATE alerts SET matched_incident_id = $2, updated_at = now()
	WHERE matched_incident_id = $1`
	tag, err := s.dbtx.Exec(ctx, query, sourceID, targetID)
	if err != nil {
		return 0, fmt.Errorf("reassigning alerts: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CreateHistory inserts a history entry for an incident.
func (s *Store) CreateHistory(ctx context.Context, incidentID uuid.UUID, changedBy pgtype.UUID, changeType string, diff json.RawMessage) error {
	query := `INSERT INTO incident_history (incident_id, changed_by, change_type, diff)
	VALUES ($1, $2, $3, $4)`
	_, err := s.dbtx.Exec(ctx, query, incidentID, changedBy, changeType, diff)
	if err != nil {
		return fmt.Errorf("creating incident history: %w", err)
	}
	return nil
}

// ListHistory returns all history entries for an incident, newest first.
func (s *Store) ListHistory(ctx context.Context, incidentID uuid.UUID) ([]HistoryEntry, error) {
	query := `SELECT id, incident_id, changed_by, change_type, diff, created_at
	FROM incident_history WHERE incident_id = $1 ORDER BY created_at DESC`
	rows, err := s.dbtx.Query(ctx, query, incidentID)
	if err != nil {
		return nil, fmt.Errorf("listing incident history: %w", err)
	}
	defer rows.Close()

	var items []HistoryEntry
	for rows.Next() {
		var h HistoryEntry
		var changedBy pgtype.UUID
		if err := rows.Scan(&h.ID, &h.IncidentID, &changedBy, &h.ChangeType, &h.Diff, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning history row: %w", err)
		}
		if changedBy.Valid {
			uid := uuid.UUID(changedBy.Bytes)
			h.ChangedBy = &uid
		}
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating history rows: %w", err)
	}
	return items, nil
}
