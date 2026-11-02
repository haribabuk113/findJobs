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
	// Unlock buys the match sort for the user's filters (criteriaHash),
	// debiting feePaise from their wallet in the same transaction
	// (PaymentRequiredError when short). Filters already unlocked return the
	// existing unlock and charge nothing.
	Unlock(ctx context.Context, userID, criteriaHash string, feePaise int64) (*jobs.Unlock, error)
	// UnlockValid reports whether unlockID is the user's unlock for criteriaHash.
	UnlockValid(ctx context.Context, userID, unlockID, criteriaHash string) (bool, error)
}
