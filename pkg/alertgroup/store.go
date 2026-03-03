package alertgroup

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// Store provides database operations for alert grouping rules and groups.
type Store struct {
	q    *db.Queries
	dbtx db.DBTX
}

// NewStore creates an alertgroup Store.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{q: db.New(dbtx), dbtx: dbtx}
}

// --- Rule operations ---

func ruleToResponse(r db.AlertGroupingRule) RuleResponse {
	return RuleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Position:    r.Position,
		IsEnabled:   r.IsEnabled,
		Matchers:    parseMatchers(r.Matchers),
		GroupBy:     r.GroupBy,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (s *Store) CreateRule(ctx context.Context, p db.CreateAlertGroupingRuleParams) (RuleResponse, error) {
	row, err := s.q.CreateAlertGroupingRule(ctx, p)
	if err != nil {
		return RuleResponse{}, fmt.Errorf("creating alert grouping rule: %w", err)
	}
	return ruleToResponse(row), nil
}

func (s *Store) GetRule(ctx context.Context, id uuid.UUID) (RuleResponse, error) {
	row, err := s.q.GetAlertGroupingRule(ctx, id)
	if err != nil {
		return RuleResponse{}, err
	}
	return ruleToResponse(row), nil
}

func (s *Store) ListRules(ctx context.Context) ([]RuleResponse, error) {
	rows, err := s.q.ListAlertGroupingRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing alert grouping rules: %w", err)
	}
	result := make([]RuleResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, ruleToResponse(r))
	}
	return result, nil
}

func (s *Store) ListEnabledRules(ctx context.Context) ([]RuleResponse, error) {
	rows, err := s.q.ListEnabledAlertGroupingRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing enabled alert grouping rules: %w", err)
	}
	result := make([]RuleResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, ruleToResponse(r))
	}
	return result, nil
}

func (s *Store) UpdateRule(ctx context.Context, p db.UpdateAlertGroupingRuleParams) (RuleResponse, error) {
	row, err := s.q.UpdateAlertGroupingRule(ctx, p)
	if err != nil {
		return RuleResponse{}, err
	}
	return ruleToResponse(row), nil
}

func (s *Store) DeleteRule(ctx context.Context, id uuid.UUID) error {
	return s.q.DeleteAlertGroupingRule(ctx, id)
}

// --- Group operations ---

// FindOrCreateGroup upserts an alert group for the given rule and key hash.
func (s *Store) FindOrCreateGroup(ctx context.Context, ruleID uuid.UUID, keyHash, title string, keyLabels []byte) (uuid.UUID, error) {
	query := `
		INSERT INTO alert_groups (rule_id, group_key_hash, group_key_labels, title)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (rule_id, group_key_hash) DO UPDATE
			SET last_alert_at = now(), updated_at = now()
		RETURNING id`

	var groupID uuid.UUID
	err := s.dbtx.QueryRow(ctx, query, ruleID, keyHash, keyLabels, title).Scan(&groupID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find or create alert group: %w", err)
	}
	return groupID, nil
}

// AssignAlertToGroup sets the alert's group_id and updates group counters.
func (s *Store) AssignAlertToGroup(ctx context.Context, alertID, groupID uuid.UUID, severity string) error {
	// Set alert_group_id on the alert.
	_, err := s.dbtx.Exec(ctx,
		`UPDATE alerts SET alert_group_id = $1, updated_at = now() WHERE id = $2`,
		groupID, alertID)
	if err != nil {
		return fmt.Errorf("assigning alert to group: %w", err)
	}

	// Update group counters.
	_, err = s.dbtx.Exec(ctx,
		`UPDATE alert_groups
		 SET alert_count = alert_count + 1,
		     max_severity = CASE
		         WHEN $2 = 'critical' THEN 'critical'
		         WHEN $2 = 'major' AND max_severity NOT IN ('critical') THEN 'major'
		         WHEN $2 = 'warning' AND max_severity NOT IN ('critical', 'major') THEN 'warning'
		         ELSE max_severity
		     END,
		     last_alert_at = now(),
		     updated_at = now()
		 WHERE id = $1`,
		groupID, severity)
	if err != nil {
		return fmt.Errorf("updating group counters: %w", err)
	}

	return nil
}

// ListGroups returns groups with JOIN on rule name.
func (s *Store) ListGroups(ctx context.Context, status string) ([]GroupResponse, error) {
	query := `
		SELECT g.id, g.rule_id, r.name AS rule_name, g.group_key_hash, g.group_key_labels,
		       g.status, g.title, g.alert_count, g.max_severity,
		       g.first_alert_at, g.last_alert_at, g.created_at, g.updated_at
		FROM alert_groups g
		JOIN alert_grouping_rules r ON r.id = g.rule_id`

	var args []any
	if status != "" {
		query += ` WHERE g.status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY g.last_alert_at DESC`

	rows, err := s.dbtx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing alert groups: %w", err)
	}
	defer rows.Close()

	var results []GroupResponse
	for rows.Next() {
		var g GroupResponse
		if err := rows.Scan(
			&g.ID, &g.RuleID, &g.RuleName, &g.GroupKeyHash, &g.GroupKeyLabels,
			&g.Status, &g.Title, &g.AlertCount, &g.MaxSeverity,
			&g.FirstAlertAt, &g.LastAlertAt, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning alert group row: %w", err)
		}
		results = append(results, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating alert group rows: %w", err)
	}
	if results == nil {
		results = []GroupResponse{}
	}
	return results, nil
}

// GetGroup returns a single group with rule name.
func (s *Store) GetGroup(ctx context.Context, id uuid.UUID) (GroupResponse, error) {
	query := `
		SELECT g.id, g.rule_id, r.name AS rule_name, g.group_key_hash, g.group_key_labels,
		       g.status, g.title, g.alert_count, g.max_severity,
		       g.first_alert_at, g.last_alert_at, g.created_at, g.updated_at
		FROM alert_groups g
		JOIN alert_grouping_rules r ON r.id = g.rule_id
		WHERE g.id = $1`

	var g GroupResponse
	err := s.dbtx.QueryRow(ctx, query, id).Scan(
		&g.ID, &g.RuleID, &g.RuleName, &g.GroupKeyHash, &g.GroupKeyLabels,
		&g.Status, &g.Title, &g.AlertCount, &g.MaxSeverity,
		&g.FirstAlertAt, &g.LastAlertAt, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return GroupResponse{}, err
	}
	return g, nil
}

// ListGroupAlerts returns alerts belonging to a group.
func (s *Store) ListGroupAlerts(ctx context.Context, groupID uuid.UUID) ([]db.Alert, error) {
	query := `SELECT * FROM alerts WHERE alert_group_id = $1 ORDER BY created_at DESC`
	rows, err := s.dbtx.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("listing group alerts: %w", err)
	}
	defer rows.Close()

	var results []db.Alert
	for rows.Next() {
		var a db.Alert
		if err := rows.Scan(
			&a.ID, &a.Fingerprint, &a.Status, &a.Severity, &a.Source, &a.Title,
			&a.Description, &a.Labels, &a.Annotations, &a.ServiceID,
			&a.MatchedIncidentID, &a.SuggestedSolution,
			&a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedBy, &a.ResolvedAt,
			&a.ResolvedByAgent, &a.AgentResolutionNotes,
			&a.OccurrenceCount, &a.FirstFiredAt, &a.LastFiredAt,
			&a.EscalationPolicyID, &a.CurrentEscalationTier,
			&a.AlertGroupID,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning group alert row: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating group alert rows: %w", err)
	}
	if results == nil {
		results = []db.Alert{}
	}
	return results, nil
}
