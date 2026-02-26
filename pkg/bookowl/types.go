package bookowl

import "time"

// RunbookListItem represents a runbook in list responses from BookOwl.
type RunbookListItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Tags      []string  `json:"tags"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RunbookListResponse is the response from GET /integration/runbooks.
type RunbookListResponse struct {
	Items  []RunbookListItem `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// RunbookDetail is the response from GET /integration/runbooks/{id}.
type RunbookDetail struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	ContentText string    `json:"content_text"`
	ContentHTML string    `json:"content_html"`
	URL         string    `json:"url"`
	Tags        []string  `json:"tags"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PostMortemRequest is the request body for POST /integration/post-mortems.
type PostMortemRequest struct {
	Title     string             `json:"title"`
	SpaceSlug string             `json:"space_slug"`
	Incident  PostMortemIncident `json:"incident"`
}

// PostMortemIncident contains the incident data for post-mortem creation.
type PostMortemIncident struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	RootCause  string `json:"root_cause"`
	Solution   string `json:"solution"`
	CreatedAt  string `json:"created_at"`
	ResolvedAt string `json:"resolved_at"`
	ResolvedBy string `json:"resolved_by"`
}

// PostMortemResponse is the response from POST /integration/post-mortems.
type PostMortemResponse struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// StatusResponse is the response from GET /bookowl/status.
type StatusResponse struct {
	Integrated bool   `json:"integrated"`
	URL        string `json:"url,omitempty"`
}
