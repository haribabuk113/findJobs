// Package applications is the apply and applicant-pipeline use case.
package applications

import (
	"context"
	"slices"
	"time"

	"findJobs/internal/domain/applications"
	"findJobs/internal/ports"
)

// ApplicationService applies to jobs and manages applicants.
type ApplicationService interface {
	// Apply is Easy Apply, using one of today's free applications.
	Apply(ctx context.Context, userID, jobID string) (*applications.Application, error)
	// ApplyExternal records "Yes, I applied" for an External job.
	ApplyExternal(ctx context.Context, userID, jobID string) (*applications.Application, error)
	Mine(ctx context.Context, userID string) ([]applications.Application, error)
	Quota(ctx context.Context, userID string) (applications.Quota, error)
	Applicants(ctx context.Context, jobID, viewerID string, f applications.ApplicantFilter) ([]applications.Applicant, error)
	SetStage(ctx context.Context, applicationID, viewerID, stage string) (*applications.Applicant, error)
}

type service struct {
	repo     ports.ApplicationRepository
	settings ports.SettingsRepository
	now      func() time.Time
}

// NewApplicationService creates the application service.
func NewApplicationService(repo ports.ApplicationRepository, settings ports.SettingsRepository) ApplicationService {
	return &service{repo: repo, settings: settings, now: time.Now}
}

func (s *service) limit(ctx context.Context) (int, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	return st.DailyFreeApplications, nil
}

func (s *service) Apply(ctx context.Context, userID, jobID string) (*applications.Application, error) {
	limit, err := s.limit(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.Apply(ctx, userID, jobID, applications.QuotaDay(s.now()), limit)
}

func (s *service) ApplyExternal(ctx context.Context, userID, jobID string) (*applications.Application, error) {
	return s.repo.ApplyExternal(ctx, userID, jobID)
}

func (s *service) Mine(ctx context.Context, userID string) ([]applications.Application, error) {
	return s.repo.Mine(ctx, userID)
}

func (s *service) Quota(ctx context.Context, userID string) (applications.Quota, error) {
	limit, err := s.limit(ctx)
	if err != nil {
		return applications.Quota{}, err
	}
	used, err := s.repo.QuotaUsed(ctx, userID, applications.QuotaDay(s.now()))
	return applications.NewQuota(limit, used), err
}

func (s *service) Applicants(ctx context.Context, jobID, viewerID string, f applications.ApplicantFilter) ([]applications.Applicant, error) {
	if err := f.Normalize(); err != nil {
		return nil, err
	}
	return s.repo.Applicants(ctx, jobID, viewerID, f)
}

func (s *service) SetStage(ctx context.Context, applicationID, viewerID, stage string) (*applications.Applicant, error) {
	if !slices.Contains(applications.Stages, stage) {
		return nil, &applications.ValidationError{Field: "stage", Message: "stage must be new, shortlisted, contacted or rejected"}
	}
	return s.repo.SetStage(ctx, applicationID, viewerID, stage)
}
