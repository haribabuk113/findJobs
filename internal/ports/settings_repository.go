package ports

import (
	"context"

	"findJobs/internal/domain/settings"
)

// SettingsRepository defines the outbound port for platform settings.
type SettingsRepository interface {
	// Get returns the current settings
	Get(ctx context.Context) (*settings.Settings, error)
	// Update replaces the settings, recording which user changed them
	Update(ctx context.Context, s *settings.Settings, userID string) (*settings.Settings, error)
}
