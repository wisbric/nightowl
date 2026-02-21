package escalation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for escalation policies and events.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates an escalation Store.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

func pgtypeUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

// --- Policy operations ---

func policyToResponse(p db.EscalationPolicy) PolicyResponse {
	return PolicyResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Tiers:       parseTiers(p.Tiers),
		RepeatCount: p.RepeatCount,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func (s *Store) CreatePolicy(ctx context.Context, p db.CreateEscalationPolicyParams) (PolicyResponse, error) {
	row, err := s.q.CreateEscalationPolicy(ctx, p)
	if err != nil {
		return PolicyResponse{}, fmt.Errorf("creating escalation policy: %w", err)
	}
	return policyToResponse(row), nil
}

func (s *Store) GetPolicy(ctx context.Context, id uuid.UUID) (PolicyResponse, error) {
	row, err := s.q.GetEscalationPolicy(ctx, id)
	if err != nil {
		return PolicyResponse{}, err
	}
	return policyToResponse(row), nil
}

func (s *Store) ListPolicies(ctx context.Context) ([]PolicyResponse, error) {
	rows, err := s.q.ListEscalationPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing escalation policies: %w", err)
	}
	result := make([]PolicyResponse, 0, len(rows))
	for _, p := range rows {
		result = append(result, policyToResponse(p))
	}
	return result, nil
}

func (s *Store) UpdatePolicy(ctx context.Context, p db.UpdateEscalationPolicyParams) (PolicyResponse, error) {
	row, err := s.q.UpdateEscalationPolicy(ctx, p)
	if err != nil {
		return PolicyResponse{}, err
	}
	return policyToResponse(row), nil
}

func (s *Store) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM escalation_policies WHERE id = $1`
	tag, err := s.dbtx.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting escalation policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// --- Event operations ---

func eventToResponse(e db.EscalationEvent) EventResponse {
	return EventResponse{
		ID:           e.ID,
		AlertID:      e.AlertID,
		PolicyID:     e.PolicyID,
		Tier:         e.Tier,
		Action:       e.Action,
		TargetUserID: pgtypeUUIDToPtr(e.TargetUserID),
		NotifyMethod: e.NotifyMethod,
		NotifyResult: e.NotifyResult,
		CreatedAt:    e.CreatedAt,
	}
}

func (s *Store) ListEvents(ctx context.Context, alertID uuid.UUID) ([]EventResponse, error) {
	rows, err := s.q.ListEscalationEvents(ctx, alertID)
	if err != nil {
		return nil, fmt.Errorf("listing escalation events: %w", err)
	}
	result := make([]EventResponse, 0, len(rows))
	for _, e := range rows {
		result = append(result, eventToResponse(e))
	}
	return result, nil
}

func (s *Store) CreateEvent(ctx context.Context, p db.CreateEscalationEventParams) (EventResponse, error) {
	row, err := s.q.CreateEscalationEvent(ctx, p)
	if err != nil {
		return EventResponse{}, fmt.Errorf("creating escalation event: %w", err)
	}
	return eventToResponse(row), nil
}

// marshalTiers marshals tiers to JSON for storage.
func marshalTiers(tiers []Tier) json.RawMessage {
	data, _ := json.Marshal(tiers)
	return data
}
