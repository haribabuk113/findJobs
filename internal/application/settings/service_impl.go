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

// WithJobPostFee makes repo report feePaise as the job-post fee, so posting,
// the checkout retry and the public settings all see the configured value.
// ponytail: an admin's job_post_fee_paise is still saved but never shown or
// used; take this wrapper out when the fee moves back to the admin settings.
func WithJobPostFee(repo ports.SettingsRepository, feePaise int64) ports.SettingsRepository {
	return jobPostFee{repo, feePaise}
}

type jobPostFee struct {
	ports.SettingsRepository
	feePaise int64
}

func (r jobPostFee) Get(ctx context.Context) (*settings.Settings, error) {
	return r.withFee(r.SettingsRepository.Get(ctx))
}

func (r jobPostFee) Update(ctx context.Context, s *settings.Settings, userID string) (*settings.Settings, error) {
	return r.withFee(r.SettingsRepository.Update(ctx, s, userID))
}

func (r jobPostFee) withFee(s *settings.Settings, err error) (*settings.Settings, error) {
	if s != nil {
		s.JobPostFeePaise = r.feePaise
	}
	return s, err
}
