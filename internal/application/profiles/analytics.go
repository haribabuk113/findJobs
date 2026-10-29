package profiles

import (
	"context"
	"time"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// AnalyticsService is the profile's Activity & Analytics tab: recording
// views, the daily rollup, and the public numbers.
type AnalyticsService interface {
	// RecordView counts one load of the public profile owned by ownerID (best
	// effort: a failure is logged, never shown to the visitor). The owner's
	// own views and obvious bots aren't counted.
	RecordView(ctx context.Context, ownerID, viewerID, src, userAgent string)
	// Analytics is the public tab for slug; profiles.ErrNotFound when missing or private.
	Analytics(ctx context.Context, slug string) (*profiles.Analytics, error)
	// Rollup is the daily job: rebuild the stats the tab reads.
	Rollup(ctx context.Context) error
	// Flush is the frequent job: move buffered views into Postgres.
	Flush(ctx context.Context) error
}

type analyticsService struct {
	repo ports.ProfileAnalyticsRepository
	now  func() time.Time
}

func NewAnalyticsService(repo ports.ProfileAnalyticsRepository) AnalyticsService {
	return &analyticsService{repo: repo, now: time.Now}
}

func (s *analyticsService) RecordView(ctx context.Context, ownerID, viewerID, src, userAgent string) {
	if ownerID == "" || ownerID == viewerID || profiles.IsBot(userAgent) {
		return
	}
	if err := s.repo.RecordEvent(ctx, ownerID, profiles.EventView, profiles.ViewSource(src)); err != nil {
		l := logger.Get()
		l.Error().Err(err).Msg("recording a profile view failed")
	}
}

func (s *analyticsService) Analytics(ctx context.Context, slug string) (*profiles.Analytics, error) {
	today := profiles.ISTDay(s.now())
	// Both periods: the shown one and its growth baseline.
	days, err := s.repo.DailyStats(ctx, slug, today.AddDate(0, 0, -2*profiles.AnalyticsDays), today.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	a := profiles.BuildAnalytics(days, today)
	return &a, nil
}

func (s *analyticsService) Rollup(ctx context.Context) error {
	today := profiles.ISTDay(s.now())
	return s.repo.Rollup(ctx, today.AddDate(0, 0, -2*profiles.AnalyticsDays), today)
}

func (s *analyticsService) Flush(ctx context.Context) error { return s.repo.FlushEvents(ctx) }
