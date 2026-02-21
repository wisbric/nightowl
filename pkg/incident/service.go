package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/opswatch/internal/db"
)

// Service encapsulates incident business logic.
type Service struct {
	store  *Store
	logger *slog.Logger
}

// NewService creates an incident Service backed by the given database connection.
func NewService(dbtx db.DBTX, logger *slog.Logger) *Service {
	return &Service{
		store:  NewStore(dbtx),
		logger: logger,
	}
}

// Create creates a new incident and records a "created" history entry.
func (s *Service) Create(ctx context.Context, req CreateRequest, userID pgtype.UUID) (Response, error) {
	runbookID, err := ParseUUIDPtr(req.RunbookID)
	if err != nil {
		return Response{}, fmt.Errorf("parsing runbook_id: %w", err)
	}

	row, err := s.store.Create(ctx, CreateParams{
		Title:         req.Title,
		Fingerprints:  defaultSlice(req.Fingerprints),
		Severity:      req.Severity,
		Category:      req.Category,
		Tags:          defaultSlice(req.Tags),
		Services:      defaultSlice(req.Services),
		Clusters:      defaultSlice(req.Clusters),
		Namespaces:    defaultSlice(req.Namespaces),
		Symptoms:      req.Symptoms,
		ErrorPatterns: defaultSlice(req.ErrorPatterns),
		RootCause:     req.RootCause,
		Solution:      req.Solution,
		RunbookID:     runbookID,
		CreatedBy:     userID,
	})
	if err != nil {
		return Response{}, fmt.Errorf("creating incident: %w", err)
	}

	// Record creation in history.
	diff, _ := json.Marshal(map[string]any{"title": row.Title, "severity": row.Severity})
	if histErr := s.store.CreateHistory(ctx, row.ID, userID, "created", diff); histErr != nil {
		s.logger.Warn("failed to record incident creation history", "error", histErr, "incident_id", row.ID)
	}

	return row.ToResponse(), nil
}

// Get returns an incident with its change history.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (DetailResponse, error) {
	row, err := s.store.Get(ctx, id)
	if err != nil {
		return DetailResponse{}, fmt.Errorf("getting incident: %w", err)
	}

	history, err := s.store.ListHistory(ctx, id)
	if err != nil {
		return DetailResponse{}, fmt.Errorf("listing incident history: %w", err)
	}
	if history == nil {
		history = []HistoryEntry{}
	}

	return DetailResponse{
		Response: row.ToResponse(),
		History:  history,
	}, nil
}

// List returns a paginated, filtered list of incidents.
func (s *Service) List(ctx context.Context, filters ListFilters, limit, offset int) ([]Response, int, error) {
	rows, err := s.store.ListFiltered(ctx, filters, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing incidents: %w", err)
	}

	count, err := s.store.CountFiltered(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("counting incidents: %w", err)
	}

	items := make([]Response, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToResponse())
	}
	return items, count, nil
}

// Search performs a full-text search across incidents.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	results, err := s.store.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching incidents: %w", err)
	}
	if results == nil {
		results = []SearchResult{}
	}
	return results, nil
}

// GetByFingerprint finds a single incident matching the given fingerprint.
func (s *Service) GetByFingerprint(ctx context.Context, fingerprint string) (Response, error) {
	row, err := s.store.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return Response{}, fmt.Errorf("getting incident by fingerprint: %w", err)
	}
	return row.ToResponse(), nil
}

// Update updates an incident, computes the diff, and records a history entry.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest, userID pgtype.UUID) (Response, error) {
	// Fetch current state for diff comparison.
	old, err := s.store.Get(ctx, id)
	if err != nil {
		return Response{}, fmt.Errorf("getting incident for update: %w", err)
	}

	runbookID, err := ParseUUIDPtr(req.RunbookID)
	if err != nil {
		return Response{}, fmt.Errorf("parsing runbook_id: %w", err)
	}

	updated, err := s.store.Update(ctx, UpdateParams{
		ID:            id,
		Title:         req.Title,
		Fingerprints:  defaultSlice(req.Fingerprints),
		Severity:      req.Severity,
		Category:      req.Category,
		Tags:          defaultSlice(req.Tags),
		Services:      defaultSlice(req.Services),
		Clusters:      defaultSlice(req.Clusters),
		Namespaces:    defaultSlice(req.Namespaces),
		Symptoms:      req.Symptoms,
		ErrorPatterns: defaultSlice(req.ErrorPatterns),
		RootCause:     req.RootCause,
		Solution:      req.Solution,
		RunbookID:     runbookID,
	})
	if err != nil {
		return Response{}, fmt.Errorf("updating incident: %w", err)
	}

	// Compute and record diff.
	diff := computeDiff(old, updated)
	if len(diff) > 0 {
		diffJSON, _ := json.Marshal(diff)
		if histErr := s.store.CreateHistory(ctx, id, userID, "updated", diffJSON); histErr != nil {
			s.logger.Warn("failed to record incident update history", "error", histErr, "incident_id", id)
		}
	}

	return updated.ToResponse(), nil
}

// Delete performs a soft delete (archive) of an incident.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, userID pgtype.UUID) error {
	if err := s.store.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("soft deleting incident: %w", err)
	}

	diff, _ := json.Marshal(map[string]any{"archived": true})
	if histErr := s.store.CreateHistory(ctx, id, userID, "archived", diff); histErr != nil {
		s.logger.Warn("failed to record incident archive history", "error", histErr, "incident_id", id)
	}

	return nil
}

// Merge merges the source incident into the target incident.
// Combined fields: fingerprints, services, tags, clusters, namespaces, error_patterns.
// The longer solution wins. Source is marked as merged into target.
// All alerts referencing source are reassigned to target.
func (s *Service) Merge(ctx context.Context, targetID, sourceID uuid.UUID, userID pgtype.UUID) (Response, error) {
	if targetID == sourceID {
		return Response{}, fmt.Errorf("cannot merge an incident into itself")
	}

	target, err := s.store.Get(ctx, targetID)
	if err != nil {
		return Response{}, fmt.Errorf("getting target incident: %w", err)
	}
	if target.MergedIntoID.Valid {
		return Response{}, fmt.Errorf("target incident is already merged")
	}

	source, err := s.store.Get(ctx, sourceID)
	if err != nil {
		return Response{}, fmt.Errorf("getting source incident: %w", err)
	}
	if source.MergedIntoID.Valid {
		return Response{}, fmt.Errorf("source incident is already merged")
	}

	// Build merged fields.
	merged := UpdateParams{
		ID:            targetID,
		Title:         target.Title,
		Fingerprints:  unionSlice(target.Fingerprints, source.Fingerprints),
		Severity:      bestSeverity(target.Severity, source.Severity),
		Category:      target.Category,
		Tags:          unionSlice(target.Tags, source.Tags),
		Services:      unionSlice(target.Services, source.Services),
		Clusters:      unionSlice(target.Clusters, source.Clusters),
		Namespaces:    unionSlice(target.Namespaces, source.Namespaces),
		Symptoms:      bestText(target.Symptoms, source.Symptoms),
		ErrorPatterns: unionSlice(target.ErrorPatterns, source.ErrorPatterns),
		RootCause:     bestText(target.RootCause, source.RootCause),
		Solution:      bestText(target.Solution, source.Solution),
		RunbookID:     bestUUID(target.RunbookID, source.RunbookID),
	}

	// Update target with merged data.
	updated, err := s.store.Update(ctx, merged)
	if err != nil {
		return Response{}, fmt.Errorf("updating target with merged data: %w", err)
	}

	// Mark source as merged into target.
	if err := s.store.SetMergedInto(ctx, sourceID, targetID); err != nil {
		return Response{}, fmt.Errorf("marking source as merged: %w", err)
	}

	// Reassign alerts from source to target.
	reassigned, err := s.store.ReassignAlerts(ctx, sourceID, targetID)
	if err != nil {
		s.logger.Warn("failed to reassign alerts during merge", "error", err,
			"source_id", sourceID, "target_id", targetID)
	}

	// Record merge history on target.
	targetDiff, _ := json.Marshal(map[string]any{
		"merged_source_id":   sourceID,
		"reassigned_alerts":  reassigned,
		"added_fingerprints": source.Fingerprints,
	})
	if histErr := s.store.CreateHistory(ctx, targetID, userID, "merged", targetDiff); histErr != nil {
		s.logger.Warn("failed to record merge history on target", "error", histErr, "incident_id", targetID)
	}

	// Record merge history on source.
	sourceDiff, _ := json.Marshal(map[string]any{
		"merged_into_id": targetID,
	})
	if histErr := s.store.CreateHistory(ctx, sourceID, userID, "merged", sourceDiff); histErr != nil {
		s.logger.Warn("failed to record merge history on source", "error", histErr, "incident_id", sourceID)
	}

	return updated.ToResponse(), nil
}

// unionSlice returns the deduplicated union of two string slices, preserving order.
func unionSlice(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// bestText returns the longer of two optional text values, preferring the first if equal.
func bestText(a, b *string) *string {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if len(*b) > len(*a) {
		return b
	}
	return a
}

// bestSeverity returns the more severe of two severity values.
func bestSeverity(a, b string) string {
	order := map[string]int{"info": 0, "warning": 1, "major": 2, "critical": 3}
	if order[b] > order[a] {
		return b
	}
	return a
}

// bestUUID returns the first valid UUID, preferring a over b.
func bestUUID(a, b pgtype.UUID) pgtype.UUID {
	if a.Valid {
		return a
	}
	return b
}

// computeDiff compares old and new incident rows and returns a map of changed fields.
// Each entry has the shape { "old": ..., "new": ... }.
func computeDiff(old, new IncidentRow) map[string]any {
	diff := make(map[string]any)

	addIfChanged := func(field string, oldVal, newVal any) {
		if !reflect.DeepEqual(oldVal, newVal) {
			diff[field] = map[string]any{"old": oldVal, "new": newVal}
		}
	}

	addIfChanged("title", old.Title, new.Title)
	addIfChanged("fingerprints", old.Fingerprints, new.Fingerprints)
	addIfChanged("severity", old.Severity, new.Severity)
	addIfChanged("category", old.Category, new.Category)
	addIfChanged("tags", old.Tags, new.Tags)
	addIfChanged("services", old.Services, new.Services)
	addIfChanged("clusters", old.Clusters, new.Clusters)
	addIfChanged("namespaces", old.Namespaces, new.Namespaces)
	addIfChanged("symptoms", old.Symptoms, new.Symptoms)
	addIfChanged("error_patterns", old.ErrorPatterns, new.ErrorPatterns)
	addIfChanged("root_cause", old.RootCause, new.RootCause)
	addIfChanged("solution", old.Solution, new.Solution)
	addIfChanged("runbook_id", old.RunbookID, new.RunbookID)

	return diff
}

// defaultSlice returns s if non-nil, otherwise an empty string slice.
func defaultSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
