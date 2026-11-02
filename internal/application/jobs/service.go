package jobs

import (
	"context"

	"findJobs/internal/domain/jobs"
)

// JobService defines the inbound port for job operations.
type JobService interface {
	// Search validates the criteria and returns matching public jobs.
	// userID is "" for anonymous visitors.
	// sort=match needs a signed-in user and an unlock for the same filters
	// (PaymentRequiredError unlock_required otherwise).
	Search(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error)
	// Unlock buys the match-score sort for these filters from the user's wallet.
	Unlock(ctx context.Context, criteria *jobs.SearchCriteria, userID string) (*jobs.Unlock, error)
}
