package jobs

import (
	"context"

	"findJobs/internal/domain/jobs"
)

// JobService defines the inbound port for job operations.
type JobService interface {
	// Search validates the criteria and returns matching public jobs.
	// userID is "" for anonymous visitors.
	Search(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error)
}
