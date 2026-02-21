package escalation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Policy types ---

// Tier defines a single escalation tier in a policy.
type Tier struct {
	Tier           int      `json:"tier"`
	TimeoutMinutes int      `json:"timeout_minutes"`
	NotifyVia      []string `json:"notify_via"` // slack_dm, phone, sms, slack_channel
	Targets        []string `json:"targets"`    // oncall_primary, oncall_backup, team_lead, user:<id>
}

// CreatePolicyRequest is the JSON body for POST /api/v1/escalation-policies.
type CreatePolicyRequest struct {
	Name        string  `json:"name" validate:"required,min=2"`
	Description *string `json:"description"`
	Tiers       []Tier  `json:"tiers" validate:"required,min=1"`
	RepeatCount *int32  `json:"repeat_count"`
}

// UpdatePolicyRequest is the JSON body for PUT /api/v1/escalation-policies/:id.
type UpdatePolicyRequest struct {
	Name        string  `json:"name" validate:"required,min=2"`
	Description *string `json:"description"`
	Tiers       []Tier  `json:"tiers" validate:"required,min=1"`
	RepeatCount *int32  `json:"repeat_count"`
}

// PolicyResponse is the JSON response for a single escalation policy.
type PolicyResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Tiers       []Tier    `json:"tiers"`
	RepeatCount *int32    `json:"repeat_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// --- Event types ---

// EventResponse is the JSON response for a single escalation event.
type EventResponse struct {
	ID           uuid.UUID  `json:"id"`
	AlertID      uuid.UUID  `json:"alert_id"`
	PolicyID     uuid.UUID  `json:"policy_id"`
	Tier         int32      `json:"tier"`
	Action       string     `json:"action"`
	TargetUserID *uuid.UUID `json:"target_user_id,omitempty"`
	NotifyMethod *string    `json:"notify_method,omitempty"`
	NotifyResult *string    `json:"notify_result,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// --- Dry-run types ---

// DryRunRequest is the JSON body for POST /api/v1/escalation-policies/:id/dry-run.
type DryRunRequest struct {
	AlertTitle   string `json:"alert_title"`
	AlertSeverity string `json:"alert_severity"`
}

// DryRunStep describes a simulated escalation step.
type DryRunStep struct {
	Tier           int      `json:"tier"`
	TimeoutMinutes int      `json:"timeout_minutes"`
	CumulativeMinutes int   `json:"cumulative_minutes"`
	NotifyVia      []string `json:"notify_via"`
	Targets        []string `json:"targets"`
	Action         string   `json:"action"`
}

// DryRunResponse is the response for a dry-run simulation.
type DryRunResponse struct {
	PolicyID   uuid.UUID    `json:"policy_id"`
	PolicyName string       `json:"policy_name"`
	Steps      []DryRunStep `json:"steps"`
	TotalTime  int          `json:"total_time_minutes"`
}

// parseTiers parses the JSONB tiers column into a slice of Tier.
func parseTiers(raw json.RawMessage) []Tier {
	var tiers []Tier
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return []Tier{}
	}
	return tiers
}
