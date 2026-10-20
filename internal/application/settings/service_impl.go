package settings

import (
	"context"

	"findJobs/internal/domain/settings"
	"findJobs/internal/ports"
)

type service struct {
	repo ports.SettingsRepository
}

// NewSettingsService creates a new settings service using dependency injection
func NewSettingsService(repo ports.SettingsRepository) SettingsService {
	return &service{repo: repo}
}

func (s *service) Get(ctx context.Context) (*settings.Settings, error) {
	return s.repo.Get(ctx)
}

func (s *service) Update(ctx context.Context, values *settings.Settings, userID string) (*settings.Settings, error) {
	if err := values.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, values, userID)
}
