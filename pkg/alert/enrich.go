package alert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/wisbric/nightowl/internal/db"
)

// EnrichResult describes the outcome of knowledge base enrichment for an alert.
type EnrichResult struct {
	IsEnriched        bool
	MatchedIncidentID uuid.UUID
	SuggestedSolution string
	RunbookURL        string
	MatchMethod       string // "fingerprint" or "text_search"
}

// Enricher looks up the knowledge base for matching incidents and attaches
// the matched incident ID and suggested solution to new alerts.
type Enricher struct {
	logger *slog.Logger
}

// NewEnricher creates an Enricher.
func NewEnricher(logger *slog.Logger) *Enricher {
	return &Enricher{logger: logger}
}

// Enrich attempts to find a matching incident for the alert by fingerprint,
// falling back to full-text search on the alert title and description.
// If a match is found, the alert row is updated with the enrichment data.
func (e *Enricher) Enrich(ctx context.Context, dbtx db.DBTX, alertID uuid.UUID, fingerprint, title string, description *string) EnrichResult {
	// 1. Try fingerprint match.
	result, err := e.matchByFingerprint(ctx, dbtx, fingerprint)
	if err != nil {
		e.logger.Warn("enrichment fingerprint lookup failed", "error", err, "alert_id", alertID)
	}
	if result.IsEnriched {
		if err := e.applyEnrichment(ctx, dbtx, alertID, result); err != nil {
			e.logger.Warn("failed to apply enrichment", "error", err, "alert_id", alertID)
		}
		return result
	}

	// 2. Fallback: text search on title (+ description if available).
	searchQuery := title
	if description != nil && *description != "" {
		searchQuery = title + " " + *description
	}
	result, err = e.matchByTextSearch(ctx, dbtx, searchQuery)
	if err != nil {
		e.logger.Warn("enrichment text search failed", "error", err, "alert_id", alertID)
	}
	if result.IsEnriched {
		if err := e.applyEnrichment(ctx, dbtx, alertID, result); err != nil {
			e.logger.Warn("failed to apply enrichment", "error", err, "alert_id", alertID)
		}
		return result
	}

	return EnrichResult{}
}

// matchByFingerprint looks up a non-merged incident whose fingerprints array
// contains the given fingerprint.
func (e *Enricher) matchByFingerprint(ctx context.Context, dbtx db.DBTX, fingerprint string) (EnrichResult, error) {
	query := `SELECT i.id, i.solution, r.id, r.content
		FROM incidents i
		LEFT JOIN runbooks r ON r.id = i.runbook_id
		WHERE $1 = ANY(i.fingerprints)
		  AND i.merged_into_id IS NULL
		LIMIT 1`

	var incidentID uuid.UUID
	var solution *string
	var runbookID *uuid.UUID
	var runbookContent *string
	err := dbtx.QueryRow(ctx, query, fingerprint).Scan(&incidentID, &solution, &runbookID, &runbookContent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrichResult{}, nil
		}
		return EnrichResult{}, fmt.Errorf("fingerprint lookup: %w", err)
	}

	return e.buildEnrichResult(incidentID, "fingerprint", solution, runbookID, runbookContent), nil
}

// matchByTextSearch uses PostgreSQL full-text search to find the best matching
// incident for the given query text. Only returns a match if the rank is above
// a minimum threshold to avoid low-quality suggestions.
func (e *Enricher) matchByTextSearch(ctx context.Context, dbtx db.DBTX, searchQuery string) (EnrichResult, error) {
	const minRank = 0.1

	query := `SELECT i.id, i.solution, r.id, r.content,
		ts_rank(i.search_vector, plainto_tsquery('english', $1)) AS rank
		FROM incidents i
		LEFT JOIN runbooks r ON r.id = i.runbook_id
		WHERE i.search_vector @@ plainto_tsquery('english', $1)
		  AND i.merged_into_id IS NULL
		ORDER BY rank DESC
		LIMIT 1`

	var incidentID uuid.UUID
	var solution *string
	var runbookID *uuid.UUID
	var runbookContent *string
	var rank float32
	err := dbtx.QueryRow(ctx, query, searchQuery).Scan(&incidentID, &solution, &runbookID, &runbookContent, &rank)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EnrichResult{}, nil
		}
		return EnrichResult{}, fmt.Errorf("text search: %w", err)
	}

	if rank < minRank {
		return EnrichResult{}, nil
	}

	return e.buildEnrichResult(incidentID, "text_search", solution, runbookID, runbookContent), nil
}

// buildEnrichResult constructs an EnrichResult from matched incident data.
// If the incident has a linked runbook, the runbook content is appended to the
// suggested solution and a RunbookURL is generated.
func (e *Enricher) buildEnrichResult(incidentID uuid.UUID, method string, solution *string, runbookID *uuid.UUID, runbookContent *string) EnrichResult {
	result := EnrichResult{
		IsEnriched:        true,
		MatchedIncidentID: incidentID,
		MatchMethod:       method,
	}

	// Build suggested solution: incident solution + runbook content.
	if solution != nil {
		result.SuggestedSolution = *solution
	}
	if runbookContent != nil && *runbookContent != "" {
		if result.SuggestedSolution != "" {
			result.SuggestedSolution += "\n\n---\n\n" + *runbookContent
		} else {
			result.SuggestedSolution = *runbookContent
		}
	}
	if runbookID != nil {
		result.RunbookURL = fmt.Sprintf("/runbooks/%s", runbookID.String())
	}

	return result
}

// applyEnrichment updates the alert row with the matched incident and solution.
func (e *Enricher) applyEnrichment(ctx context.Context, dbtx db.DBTX, alertID uuid.UUID, result EnrichResult) error {
	query := `UPDATE alerts
		SET matched_incident_id = $2, suggested_solution = $3, updated_at = now()
		WHERE id = $1`

	var solution *string
	if result.SuggestedSolution != "" {
		solution = &result.SuggestedSolution
	}

	_, err := dbtx.Exec(ctx, query, alertID, result.MatchedIncidentID, solution)
	if err != nil {
		return fmt.Errorf("updating alert enrichment: %w", err)
	}
	return nil
}
