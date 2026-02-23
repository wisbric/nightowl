package slack

// AlertInfo holds the data needed to build a Slack alert notification.
type AlertInfo struct {
	AlertID           string
	Title             string
	Severity          string
	Description       string
	Cluster           string
	Namespace         string
	Service           string
	OnCallUser        string
	SuggestedSolution string
	RunbookURL        string
}

// SearchResult represents a KB search result for Slack display.
type SearchResult struct {
	ID       string
	Title    string
	Severity string
	Solution string
}

// OnCallEntry represents a single on-call roster entry for display.
type OnCallEntry struct {
	RosterName  string
	UserDisplay string
	Timezone    string
	IsOverride  bool
}
