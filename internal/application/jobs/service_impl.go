package jobs

import (
	"context"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/ports"
)

type service struct {
	repo     ports.JobRepository
	settings ports.SettingsRepository
}

// NewJobService creates a new job service using dependency injection
func NewJobService(repo ports.JobRepository, settings ports.SettingsRepository) JobService {
	return &service{repo: repo, settings: settings}
}

var errSignInToMatch = &jobs.ValidationError{Field: "sort", Message: "sign in to sort by match"}

func (s *service) Search(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error) {
	if err := criteria.Normalize(); err != nil {
		return nil, err
	}
	if criteria.Sort == jobs.SortMatch {
		if userID == "" {
			return nil, errSignInToMatch
		}
		ok := false
		if criteria.UnlockID != "" {
			var err error
			if ok, err = s.repo.UnlockValid(ctx, userID, criteria.UnlockID, criteria.UnlockHash()); err != nil {
				return nil, err
			}
		}
		if !ok {
			st, err := s.settings.Get(ctx)
			if err != nil {
				return nil, err
			}
			return nil, &jobs.PaymentRequiredError{Reason: jobs.ReasonUnlockRequired, FeePaise: st.MatchSortFeePaise,
				Message: "sorting by match needs an unlock for these filters"}
		}
	}
	return s.repo.Search(ctx, criteria, userID)
}

func (s *service) Unlock(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.Unlock, error) {
	if err := criteria.Normalize(); err != nil {
		return nil, err
	}
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.Unlock(ctx, userID, criteria.UnlockHash(), st.MatchSortFeePaise)
}
