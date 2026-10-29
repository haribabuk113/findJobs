package ports

import (
	"context"
	"time"

	"findJobs/internal/domain/profiles"
)

// ProfileAnalyticsRepository stores profile view events and their daily rollup.
type ProfileAnalyticsRepository interface {
	// RecordEvent counts one event for the profile owned by userID. Callers
	// check visibility and skip the owner's own views.
	RecordEvent(ctx context.Context, userID, kind, source string) error
	// AddEventCounts stores a batch of buffered counts once: a batch id seen
	// before is a no-op. Counts for profiles deleted since are dropped.
	AddEventCounts(ctx context.Context, batchID string, counts []profiles.EventCount) error
	// FlushEvents moves buffered events into durable storage (a no-op when
	// events are written directly).
	FlushEvents(ctx context.Context) error
	// DailyStats returns the rolled-up days in [from, to] for the public
	// profile behind slug, or profiles.ErrNotFound when missing or private.
	DailyStats(ctx context.Context, slug string, from, to time.Time) ([]profiles.DayStats, error)
	// Rollup rebuilds the daily stats of the IST days in [from, today) from the
	// stored events, and prunes events and stats that are no longer needed.
	Rollup(ctx context.Context, from, today time.Time) error
}
