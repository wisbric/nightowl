package alertgroup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Matcher defines a single label match condition.
type Matcher struct {
	Key   string `json:"key"`
	Op    string `json:"op"` // =, !=, =~, !~
	Value string `json:"value"`
}

// CreateRuleRequest is the JSON body for POST /api/v1/alert-groups/rules.
type CreateRuleRequest struct {
	Name        string    `json:"name" validate:"required,min=2"`
	Description *string   `json:"description"`
	Position    int32     `json:"position"`
	IsEnabled   *bool     `json:"is_enabled"`
	Matchers    []Matcher `json:"matchers"`
	GroupBy     []string  `json:"group_by" validate:"required,min=1"`
}

// UpdateRuleRequest is the JSON body for PUT /api/v1/alert-groups/rules/:id.
type UpdateRuleRequest struct {
	Name        string    `json:"name" validate:"required,min=2"`
	Description *string   `json:"description"`
	Position    int32     `json:"position"`
	IsEnabled   *bool     `json:"is_enabled"`
	Matchers    []Matcher `json:"matchers"`
	GroupBy     []string  `json:"group_by" validate:"required,min=1"`
}

// RuleResponse is the API response for a grouping rule.
type RuleResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Position    int32     `json:"position"`
	IsEnabled   bool      `json:"is_enabled"`
	Matchers    []Matcher `json:"matchers"`
	GroupBy     []string  `json:"group_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GroupResponse is the API response for an alert group.
type GroupResponse struct {
	ID             uuid.UUID       `json:"id"`
	RuleID         uuid.UUID       `json:"rule_id"`
	RuleName       string          `json:"rule_name"`
	GroupKeyHash   string          `json:"group_key_hash"`
	GroupKeyLabels json.RawMessage `json:"group_key_labels"`
	Status         string          `json:"status"`
	Title          string          `json:"title"`
	AlertCount     int32           `json:"alert_count"`
	MaxSeverity    string          `json:"max_severity"`
	FirstAlertAt   time.Time       `json:"first_alert_at"`
	LastAlertAt    time.Time       `json:"last_alert_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// UngroupedAlert is a lightweight projection of an alert for backfill evaluation.
type UngroupedAlert struct {
	ID       uuid.UUID       `json:"id"`
	Severity string          `json:"severity"`
	Labels   json.RawMessage `json:"labels"`
}

// marshalMatchers marshals matchers to JSON for storage.
func marshalMatchers(matchers []Matcher) json.RawMessage {
	if matchers == nil {
		matchers = []Matcher{}
	}
	data, _ := json.Marshal(matchers)
	return data
}

// parseMatchers parses matchers from JSON.
func parseMatchers(raw json.RawMessage) []Matcher {
	var matchers []Matcher
	if err := json.Unmarshal(raw, &matchers); err != nil {
		return []Matcher{}
	}
	return matchers
}

// matchAlert checks if an alert's labels match all matchers (AND logic).
func matchAlert(matchers []Matcher, labels map[string]string) bool {
	for _, m := range matchers {
		labelVal, exists := labels[m.Key]
		switch m.Op {
		case "=":
			if labelVal != m.Value {
				return false
			}
		case "!=":
			if labelVal == m.Value {
				return false
			}
		case "=~":
			re, err := regexp.Compile(m.Value)
			if err != nil || !re.MatchString(labelVal) {
				return false
			}
		case "!~":
			re, err := regexp.Compile(m.Value)
			if err != nil || re.MatchString(labelVal) {
				return false
			}
		default:
			// Unknown operator, treat as non-match.
			_ = exists
			return false
		}
	}
	return true
}

// computeGroupKeyHash computes a SHA-256 hash of sorted key=value pairs.
func computeGroupKeyHash(groupByLabels map[string]string) string {
	pairs := make([]string, 0, len(groupByLabels))
	for k, v := range groupByLabels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	data := strings.Join(pairs, "\n")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// computeGroupTitle creates a human-readable title from group-by labels.
func computeGroupTitle(groupByLabels map[string]string) string {
	pairs := make([]string, 0, len(groupByLabels))
	for k, v := range groupByLabels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	if len(pairs) == 0 {
		return "(all)"
	}
	return strings.Join(pairs, ", ")
}

// extractGroupByLabels extracts the values for group_by keys from alert labels.
func extractGroupByLabels(groupBy []string, labels map[string]string) map[string]string {
	result := make(map[string]string, len(groupBy))
	for _, key := range groupBy {
		result[key] = labels[key]
	}
	return result
}

// severityRank returns a numeric rank for severity comparison (higher = more severe).
func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "major":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// maxSeverity returns the more severe of two severity strings.
func maxSeverityOf(a, b string) string {
	if severityRank(a) >= severityRank(b) {
		return a
	}
	return b
}
