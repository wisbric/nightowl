package bookowl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client calls BookOwl's integration API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a BookOwl client with a 10-second timeout.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ListRunbooks fetches runbooks from BookOwl.
func (c *Client) ListRunbooks(ctx context.Context, apiURL, apiKey string, query string, limit, offset int) (*RunbookListResponse, error) {
	url := fmt.Sprintf("%s/integration/runbooks?limit=%d&offset=%d", apiURL, limit, offset)
	if query != "" {
		url += "&q=" + query
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling BookOwl: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BookOwl returned HTTP %d", resp.StatusCode)
	}

	var result RunbookListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

// GetRunbook fetches a single runbook by ID from BookOwl.
func (c *Client) GetRunbook(ctx context.Context, apiURL, apiKey, id string) (*RunbookDetail, error) {
	url := fmt.Sprintf("%s/integration/runbooks/%s", apiURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling BookOwl: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("runbook not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BookOwl returned HTTP %d", resp.StatusCode)
	}

	var result RunbookDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}

// CreatePostMortem creates a post-mortem document in BookOwl.
func (c *Client) CreatePostMortem(ctx context.Context, apiURL, apiKey string, pmReq PostMortemRequest) (*PostMortemResponse, error) {
	body, err := json.Marshal(pmReq)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	url := fmt.Sprintf("%s/integration/post-mortems", apiURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling BookOwl: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("BookOwl returned HTTP %d", resp.StatusCode)
	}

	var result PostMortemResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}
