package jobs

import (
	"context"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/ports"
)

type service struct {
	repo ports.JobRepository
}

// NewJobService creates a new job service using dependency injection
func NewJobService(repo ports.JobRepository) JobService {
	return &service{repo: repo}
}

func (s *service) Search(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error) {
	if err := criteria.Normalize(); err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, criteria, userID)
}
