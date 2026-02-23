package alert

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// pgtypeUUIDToPtr converts a pgtype.UUID to a *uuid.UUID, returning nil if invalid.
func pgtypeUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

// Store provides database operations for alerts.
type Store struct {
	q *db.Queries
}

// NewStore creates an alert Store backed by the given database connection.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx)}
}

// Create inserts a new alert and returns the response.
func (s *Store) Create(ctx context.Context, a NormalizedAlert) (Response, error) {
	row, err := s.q.CreateAlert(ctx, db.CreateAlertParams{
		Fingerprint:        a.Fingerprint,
		Status:             a.Status,
		Severity:           a.Severity,
		Source:             a.Source,
		Title:              a.Title,
		Description:        a.Description,
		Labels:             ensureJSON(a.Labels),
		Annotations:        ensureJSON(a.Annotations),
		ServiceID:          pgtype.UUID{},
		EscalationPolicyID: pgtype.UUID{},
	})
	if err != nil {
		return Response{}, fmt.Errorf("creating alert: %w", err)
	}

	return alertRowToResponse(row), nil
}

// alertRowToResponse converts a db.Alert row to a Response.
func alertRowToResponse(row db.Alert) Response {
	return Response{
		ID:                row.ID,
		Fingerprint:       row.Fingerprint,
		Status:            row.Status,
		Severity:          row.Severity,
		Source:            row.Source,
		Title:             row.Title,
		Description:       row.Description,
		Labels:            row.Labels,
		Annotations:       row.Annotations,
		MatchedIncidentID: pgtypeUUIDToPtr(row.MatchedIncidentID),
		SuggestedSolution: row.SuggestedSolution,
		OccurrenceCount:   row.OccurrenceCount,
		FirstFiredAt:      row.FirstFiredAt,
		LastFiredAt:       row.LastFiredAt,
		CreatedAt:         row.CreatedAt,
	}
}
