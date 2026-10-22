package settings

import (
	"context"

	"findJobs/internal/domain/settings"
)

// SettingsService defines the inbound port for platform settings.
type SettingsService interface {
	// Get returns the current settings
	Get(ctx context.Context) (*settings.Settings, error)
	// Update validates and saves the settings as userID
	Update(ctx context.Context, s *settings.Settings, userID string) (*settings.Settings, error)
}
