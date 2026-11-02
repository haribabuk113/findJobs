package ports

import (
	"context"

	"findJobs/internal/domain/stats"
)

// StatsRepository computes and serves the landing page's platform counters.
type StatsRepository interface {
	// Refresh recomputes every counter (the daily job).
	Refresh(ctx context.Context) error
	// Get returns the last computed counters.
	Get(ctx context.Context) (*stats.Platform, error)
}
