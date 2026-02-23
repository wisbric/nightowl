// Package mattermost implements the messaging.Provider interface for Mattermost.
package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Client wraps the Mattermost REST API v4.
type Client struct {
	baseURL    string // e.g. "https://mattermost.example.com"
	botToken   string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a Mattermost API client.
func NewClient(baseURL, botToken string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		botToken:   botToken,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// IsEnabled returns true if the client has a valid URL and token.
func (c *Client) IsEnabled() bool {
	return c.baseURL != "" && c.botToken != ""
}

// --- Posts ---

// Post represents a Mattermost post.
type Post struct {
	ID        string         `json:"id,omitempty"`
	ChannelID string         `json:"channel_id"`
	Message   string         `json:"message"`
	Props     map[string]any `json:"props,omitempty"`
}

// CreatePost sends a post to a channel.
func (c *Client) CreatePost(ctx context.Context, post Post) (*Post, error) {
	var result Post
	if err := c.do(ctx, http.MethodPost, "/api/v4/posts", post, &result); err != nil {
		return nil, fmt.Errorf("creating post: %w", err)
	}
	return &result, nil
}

// UpdatePost updates an existing post.
func (c *Client) UpdatePost(ctx context.Context, postID string, post Post) (*Post, error) {
	post.ID = postID
	var result Post
	if err := c.do(ctx, http.MethodPut, "/api/v4/posts/"+postID, post, &result); err != nil {
		return nil, fmt.Errorf("updating post: %w", err)
	}
	return &result, nil
}

// --- Direct Messages ---

// DMChannel represents a direct message channel.
type DMChannel struct {
	ID string `json:"id"`
}

// CreateDMChannel opens a DM channel between two users (typically bot + user).
func (c *Client) CreateDMChannel(ctx context.Context, userIDs [2]string) (*DMChannel, error) {
	var result DMChannel
	if err := c.do(ctx, http.MethodPost, "/api/v4/channels/direct", userIDs[:], &result); err != nil {
		return nil, fmt.Errorf("creating DM channel: %w", err)
	}
	return &result, nil
}

// --- Users ---

// MMUser represents a Mattermost user (partial).
type MMUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// GetUserByEmail looks up a user by email address.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*MMUser, error) {
	var user MMUser
	if err := c.do(ctx, http.MethodGet, "/api/v4/users/email/"+email, nil, &user); err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return &user, nil
}

// GetMe returns the authenticated bot user.
func (c *Client) GetMe(ctx context.Context) (*MMUser, error) {
	var user MMUser
	if err := c.do(ctx, http.MethodGet, "/api/v4/users/me", nil, &user); err != nil {
		return nil, fmt.Errorf("getting bot user: %w", err)
	}
	return &user, nil
}

// --- Dialogs ---

// DialogRequest is the payload for opening an interactive dialog.
type DialogRequest struct {
	TriggerID string `json:"trigger_id"`
	URL       string `json:"url"`
	Dialog    Dialog `json:"dialog"`
}

// Dialog defines a Mattermost interactive dialog.
type Dialog struct {
	Title       string          `json:"title"`
	SubmitLabel string          `json:"submit_label"`
	Elements    []DialogElement `json:"elements"`
}

// DialogElement is a single form element in a dialog.
type DialogElement struct {
	DisplayName string         `json:"display_name"`
	Name        string         `json:"name"`
	Type        string         `json:"type"` // "text", "textarea", "select"
	Placeholder string         `json:"placeholder,omitempty"`
	Default     string         `json:"default,omitempty"`
	Optional    bool           `json:"optional"`
	Options     []DialogOption `json:"options,omitempty"`
}

// DialogOption is an option for a select element.
type DialogOption struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// OpenDialog opens an interactive dialog.
func (c *Client) OpenDialog(ctx context.Context, req DialogRequest) error {
	if err := c.do(ctx, http.MethodPost, "/api/v4/actions/dialogs/open", req, nil); err != nil {
		return fmt.Errorf("opening dialog: %w", err)
	}
	return nil
}

// --- Ping ---

// Ping checks if the Mattermost server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/v4/system/ping", nil, nil)
}

// --- HTTP helper ---

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mattermost API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
