package incident

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateRequest is the JSON body for POST /api/v1/incidents.
type CreateRequest struct {
	Title         string   `json:"title" validate:"required,min=3"`
	Fingerprints  []string `json:"fingerprints"`
	Severity      string   `json:"severity" validate:"required,oneof=info warning critical major"`
	Category      *string  `json:"category"`
	Tags          []string `json:"tags"`
	Services      []string `json:"services"`
	Clusters      []string `json:"clusters"`
	Namespaces    []string `json:"namespaces"`
	Symptoms      *string  `json:"symptoms"`
	ErrorPatterns []string `json:"error_patterns"`
	RootCause     *string  `json:"root_cause"`
	Solution      *string  `json:"solution"`
	RunbookID     *string  `json:"runbook_id" validate:"omitempty,uuid"`
}

// UpdateRequest is the JSON body for PUT /api/v1/incidents/:id.
type UpdateRequest struct {
	Title         string   `json:"title" validate:"required,min=3"`
	Fingerprints  []string `json:"fingerprints"`
	Severity      string   `json:"severity" validate:"required,oneof=info warning critical major"`
	Category      *string  `json:"category"`
	Tags          []string `json:"tags"`
	Services      []string `json:"services"`
	Clusters      []string `json:"clusters"`
	Namespaces    []string `json:"namespaces"`
	Symptoms      *string  `json:"symptoms"`
	ErrorPatterns []string `json:"error_patterns"`
	RootCause     *string  `json:"root_cause"`
	Solution      *string  `json:"solution"`
	RunbookID     *string  `json:"runbook_id" validate:"omitempty,uuid"`
}

// MergeRequest is the JSON body for POST /api/v1/incidents/:id/merge.
// The URL :id is the target (surviving) incident; source_id is the incident being merged in.
type MergeRequest struct {
	SourceID string `json:"source_id" validate:"required,uuid"`
}

// ListFilters holds the optional filter parameters for listing incidents.
type ListFilters struct {
	Severity string
	Category string
	Service  string
	Tags     []string
}

// Response is the JSON response for a single incident.
type Response struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Fingerprints      []string   `json:"fingerprints"`
	Severity          string     `json:"severity"`
	Category          *string    `json:"category"`
	Tags              []string   `json:"tags"`
	Services          []string   `json:"services"`
	Clusters          []string   `json:"clusters"`
	Namespaces        []string   `json:"namespaces"`
	Symptoms          *string    `json:"symptoms"`
	ErrorPatterns     []string   `json:"error_patterns"`
	RootCause         *string    `json:"root_cause"`
	Solution          *string    `json:"solution"`
	RunbookID         *uuid.UUID `json:"runbook_id,omitempty"`
	RunbookTitle      *string    `json:"runbook_title,omitempty"`
	RunbookContent    *string    `json:"runbook_content,omitempty"`
	PostMortemURL     *string    `json:"post_mortem_url,omitempty"`
	ResolutionCount   int32      `json:"resolution_count"`
	LastResolvedAt    *time.Time `json:"last_resolved_at,omitempty"`
	LastResolvedBy    *uuid.UUID `json:"last_resolved_by,omitempty"`
	AvgResolutionMins *float64   `json:"avg_resolution_mins,omitempty"`
	MergedIntoID      *uuid.UUID `json:"merged_into_id,omitempty"`
	CreatedBy         *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DetailResponse includes the incident and its change history.
type DetailResponse struct {
	Response
	History []HistoryEntry `json:"history"`
}

// HistoryEntry is one change record in the incident history.
type HistoryEntry struct {
	ID         uuid.UUID       `json:"id"`
	IncidentID uuid.UUID       `json:"incident_id"`
	ChangedBy  *uuid.UUID      `json:"changed_by,omitempty"`
	ChangeType string          `json:"change_type"`
	Diff       json.RawMessage `json:"diff"`
	CreatedAt  time.Time       `json:"created_at"`
}

// SearchResult is a single result from full-text search with ranking and highlights.
type SearchResult struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Severity          string     `json:"severity"`
	Category          *string    `json:"category,omitempty"`
	Services          []string   `json:"services"`
	Tags              []string   `json:"tags"`
	Symptoms          *string    `json:"symptoms,omitempty"`
	RootCause         *string    `json:"root_cause,omitempty"`
	Solution          *string    `json:"solution,omitempty"`
	RunbookID         *uuid.UUID `json:"runbook_id,omitempty"`
	Rank              float32    `json:"rank"`
	TitleHighlight    string     `json:"title_highlight"`
	SymptomsHighlight string     `json:"symptoms_highlight"`
	SolutionHighlight string     `json:"solution_highlight"`
	ResolutionCount   int32      `json:"resolution_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

// IncidentRow represents a row returned from the incidents table (excluding search_vector).
type IncidentRow struct {
	ID                uuid.UUID
	Title             string
	Fingerprints      []string
	Severity          string
	Category          *string
	Tags              []string
	Services          []string
	Clusters          []string
	Namespaces        []string
	Symptoms          *string
	ErrorPatterns     []string
	RootCause         *string
	Solution          *string
	RunbookID         pgtype.UUID
	PostMortemURL     pgtype.Text
	ResolutionCount   int32
	LastResolvedAt    pgtype.Timestamptz
	LastResolvedBy    pgtype.UUID
	AvgResolutionMins *float64
	MergedIntoID      pgtype.UUID
	CreatedBy         pgtype.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ToResponse converts an IncidentRow to a Response DTO.
func (r *IncidentRow) ToResponse() Response {
	resp := Response{
		ID:              r.ID,
		Title:           r.Title,
		Fingerprints:    ensureSlice(r.Fingerprints),
		Severity:        r.Severity,
		Category:        r.Category,
		Tags:            ensureSlice(r.Tags),
		Services:        ensureSlice(r.Services),
		Clusters:        ensureSlice(r.Clusters),
		Namespaces:      ensureSlice(r.Namespaces),
		Symptoms:        r.Symptoms,
		ErrorPatterns:   ensureSlice(r.ErrorPatterns),
		RootCause:       r.RootCause,
		Solution:        r.Solution,
		ResolutionCount: r.ResolutionCount,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}

	if r.RunbookID.Valid {
		id := r.RunbookID.Bytes
		uid := uuid.UUID(id)
		resp.RunbookID = &uid
	}
	if r.PostMortemURL.Valid {
		s := r.PostMortemURL.String
		resp.PostMortemURL = &s
	}
	if r.LastResolvedAt.Valid {
		t := r.LastResolvedAt.Time
		resp.LastResolvedAt = &t
	}
	if r.LastResolvedBy.Valid {
		id := r.LastResolvedBy.Bytes
		uid := uuid.UUID(id)
		resp.LastResolvedBy = &uid
	}
	resp.AvgResolutionMins = r.AvgResolutionMins
	if r.MergedIntoID.Valid {
		id := r.MergedIntoID.Bytes
		uid := uuid.UUID(id)
		resp.MergedIntoID = &uid
	}
	if r.CreatedBy.Valid {
		id := r.CreatedBy.Bytes
		uid := uuid.UUID(id)
		resp.CreatedBy = &uid
	}

	return resp
}

// ensureSlice returns s if non-nil, otherwise an empty slice.
// This ensures JSON output is [] rather than null.
func ensureSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ParseUUIDPtr parses an optional UUID string pointer into a pgtype.UUID.
func ParseUUIDPtr(s *string) (pgtype.UUID, error) {
	if s == nil || *s == "" {
		return pgtype.UUID{}, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
