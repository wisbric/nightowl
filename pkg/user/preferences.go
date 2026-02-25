package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/wisbric/nightowl/internal/db"
)

// Preferences is the JSON shape stored in users.preferences.
type Preferences struct {
	Timezone      string                    `json:"timezone,omitempty"`
	Theme         string                    `json:"theme,omitempty"`
	Notifications *NotificationPreferences  `json:"notifications,omitempty"`
	Dashboard     *DashboardPreferences     `json:"dashboard,omitempty"`
}

// NotificationPreferences controls per-severity notification toggles.
type NotificationPreferences struct {
	Critical bool `json:"critical"`
	Major    bool `json:"major"`
	Warning  bool `json:"warning"`
	Info     bool `json:"info"`
}

// DashboardPreferences controls dashboard display settings.
type DashboardPreferences struct {
	DefaultTimeRange string `json:"default_time_range,omitempty"`
}

// PreferencesStore provides database operations for user preferences.
type PreferencesStore struct {
	dbtx db.DBTX
}

// NewPreferencesStore creates a preferences store.
func NewPreferencesStore(dbtx db.DBTX) *PreferencesStore {
	return &PreferencesStore{dbtx: dbtx}
}

// GetPreferences returns the preferences JSON for a user.
func (s *PreferencesStore) GetPreferences(ctx context.Context, userID uuid.UUID) (json.RawMessage, error) {
	var prefs json.RawMessage
	err := s.dbtx.QueryRow(ctx,
		`SELECT preferences FROM users WHERE id = $1`,
		userID,
	).Scan(&prefs)
	if err != nil {
		return nil, fmt.Errorf("getting preferences: %w", err)
	}
	if len(prefs) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return prefs, nil
}

// UpdatePreferences replaces the preferences JSON for a user.
func (s *PreferencesStore) UpdatePreferences(ctx context.Context, userID uuid.UUID, prefs json.RawMessage) error {
	_, err := s.dbtx.Exec(ctx,
		`UPDATE users SET preferences = $2, updated_at = now() WHERE id = $1`,
		userID, prefs,
	)
	if err != nil {
		return fmt.Errorf("updating preferences: %w", err)
	}
	return nil
}
