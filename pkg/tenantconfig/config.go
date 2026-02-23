package tenantconfig

// TenantConfig is the JSONB config stored in public.tenants.config.
type TenantConfig struct {
	MessagingProvider          string `json:"messaging_provider"` // "slack", "mattermost", "none"
	SlackWorkspaceURL          string `json:"slack_workspace_url"`
	SlackChannel               string `json:"slack_channel"`
	MattermostURL              string `json:"mattermost_url"`
	MattermostDefaultChannelID string `json:"mattermost_default_channel_id"`
	TwilioSID                  string `json:"twilio_sid"`
	TwilioPhoneNumber          string `json:"twilio_phone_number"`
	DefaultTimezone            string `json:"default_timezone"`
}

// UpdateRequest is the payload for PUT /admin/config.
type UpdateRequest struct {
	MessagingProvider          string `json:"messaging_provider"`
	SlackWorkspaceURL          string `json:"slack_workspace_url"`
	SlackChannel               string `json:"slack_channel"`
	MattermostURL              string `json:"mattermost_url"`
	MattermostDefaultChannelID string `json:"mattermost_default_channel_id"`
	TwilioSID                  string `json:"twilio_sid"`
	TwilioPhoneNumber          string `json:"twilio_phone_number"`
	DefaultTimezone            string `json:"default_timezone" validate:"required"`
}

// ConfigResponse is the JSON response for GET /admin/config.
type ConfigResponse struct {
	MessagingProvider          string `json:"messaging_provider"`
	SlackWorkspaceURL          string `json:"slack_workspace_url"`
	SlackChannel               string `json:"slack_channel"`
	MattermostURL              string `json:"mattermost_url"`
	MattermostDefaultChannelID string `json:"mattermost_default_channel_id"`
	TwilioSID                  string `json:"twilio_sid"`
	TwilioPhoneNumber          string `json:"twilio_phone_number"`
	DefaultTimezone            string `json:"default_timezone"`
	UpdatedAt                  string `json:"updated_at"`
}
