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
