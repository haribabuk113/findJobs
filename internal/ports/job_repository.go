package ports

import (
	"context"

	"findJobs/internal/domain/jobs"
)

// JobRepository defines the outbound port for job search. Postgres implements
// it today; a search engine (Meilisearch) can implement it later unchanged.
type JobRepository interface {
	// Search returns one page of publicly visible jobs matching the criteria.
	// userID, when non-empty, excludes jobs that user already applied to.
	Search(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error)
}
