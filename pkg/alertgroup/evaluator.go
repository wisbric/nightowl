package alertgroup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/alert"
)

// Evaluator evaluates incoming alerts against grouping rules.
type Evaluator struct {
	logger *slog.Logger
}

// NewEvaluator creates an Evaluator.
func NewEvaluator(logger *slog.Logger) *Evaluator {
	return &Evaluator{logger: logger}
}

// Evaluate checks the alert against all enabled grouping rules (ordered by position).
// First matching rule wins. If a match is found, the alert is assigned to a group.
// Satisfies the alert.AlertGrouper interface.
func (e *Evaluator) Evaluate(ctx context.Context, dbtx db.DBTX, alertID uuid.UUID, severity string, labels json.RawMessage) alert.AlertGroupResult {
	store := NewStore(dbtx)

	rules, err := store.ListEnabledRules(ctx)
	if err != nil {
		e.logger.Warn("failed to load grouping rules", "error", err)
		return alert.AlertGroupResult{}
	}

	if len(rules) == 0 {
		return alert.AlertGroupResult{}
	}

	// Parse labels into map for matching.
	var labelMap map[string]string
	if err := json.Unmarshal(labels, &labelMap); err != nil {
		e.logger.Warn("failed to parse alert labels for grouping", "error", err)
		return alert.AlertGroupResult{}
	}

	for _, rule := range rules {
		if !matchAlert(rule.Matchers, labelMap) {
			continue
		}

		// First matching rule — extract group-by labels and find/create group.
		groupByLabels := extractGroupByLabels(rule.GroupBy, labelMap)
		keyHash := computeGroupKeyHash(groupByLabels)
		title := computeGroupTitle(groupByLabels)
		keyLabelsJSON, _ := json.Marshal(groupByLabels)

		groupID, err := store.FindOrCreateGroup(ctx, rule.ID, keyHash, title, keyLabelsJSON)
		if err != nil {
			e.logger.Error("failed to find or create alert group", "error", err, "rule_id", rule.ID)
			return alert.AlertGroupResult{}
		}

		if err := store.AssignAlertToGroup(ctx, alertID, groupID, severity); err != nil {
			e.logger.Error("failed to assign alert to group", "error", err, "alert_id", alertID, "group_id", groupID)
			return alert.AlertGroupResult{}
		}

		e.logger.Debug("alert grouped",
			"alert_id", alertID,
			"group_id", groupID,
			"rule_id", rule.ID,
			"rule_name", rule.Name,
			"title", title,
		)

		return alert.AlertGroupResult{
			Matched: true,
			GroupID: groupID,
			RuleID:  rule.ID,
		}
	}

	return alert.AlertGroupResult{}
}

// BackfillRule evaluates all ungrouped firing alerts against the given rule,
// assigning matching alerts to groups. Called after rule create/update so
// existing alerts are grouped retroactively.
func (e *Evaluator) BackfillRule(ctx context.Context, dbtx db.DBTX, rule RuleResponse) (int, error) {
	store := NewStore(dbtx)

	if !rule.IsEnabled {
		return 0, nil
	}

	alerts, err := store.ListUngroupedAlerts(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing ungrouped alerts: %w", err)
	}

	grouped := 0
	for _, a := range alerts {
		var labelMap map[string]string
		if err := json.Unmarshal(a.Labels, &labelMap); err != nil {
			continue
		}

		if !matchAlert(rule.Matchers, labelMap) {
			continue
		}

		groupByLabels := extractGroupByLabels(rule.GroupBy, labelMap)
		keyHash := computeGroupKeyHash(groupByLabels)
		title := computeGroupTitle(groupByLabels)
		keyLabelsJSON, _ := json.Marshal(groupByLabels)

		groupID, err := store.FindOrCreateGroup(ctx, rule.ID, keyHash, title, keyLabelsJSON)
		if err != nil {
			e.logger.Error("backfill: failed to find/create group", "error", err, "alert_id", a.ID)
			continue
		}

		if err := store.AssignAlertToGroup(ctx, a.ID, groupID, a.Severity); err != nil {
			e.logger.Error("backfill: failed to assign alert", "error", err, "alert_id", a.ID)
			continue
		}

		grouped++
	}

	if grouped > 0 {
		e.logger.Info("backfilled existing alerts into rule",
			"rule_id", rule.ID,
			"rule_name", rule.Name,
			"grouped", grouped,
		)
	}

	return grouped, nil
}
