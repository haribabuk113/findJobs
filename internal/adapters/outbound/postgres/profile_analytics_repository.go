package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

// Retention: raw events cover the two periods the analytics compare (plus a
// day of slack); stats are kept a month for a later longer view.
const (
	profileEventsKeep = "15 days"
	profileStatsKeep  = 30
)

// ProfileAnalyticsRepository is the Postgres adapter for profile analytics.
type ProfileAnalyticsRepository struct {
	pool *pgxpool.Pool
}

func NewProfileAnalyticsRepository(pool *pgxpool.Pool) ports.ProfileAnalyticsRepository {
	return &ProfileAnalyticsRepository{pool: pool}
}

// RecordEvent writes one event directly: the path when Redis is unavailable.
func (r *ProfileAnalyticsRepository) RecordEvent(ctx context.Context, userID, kind, source string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO profile_events (user_id, kind, source) VALUES ($1, $2, $3)`, userID, kind, source)
	return err
}

func (r *ProfileAnalyticsRepository) AddEventCounts(ctx context.Context, batchID string, counts []profiles.EventCount) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO profile_event_batches (id) VALUES ($1) ON CONFLICT DO NOTHING`, batchID)
	if err != nil || tag.RowsAffected() == 0 {
		return err // already applied (a retry after a crash): nothing to do
	}
	users, kinds, sources, ats, ns := make([]string, len(counts)), make([]string, len(counts)),
		make([]string, len(counts)), make([]string, len(counts)), make([]int32, len(counts))
	for i, c := range counts {
		// Noon IST on the event's day: any time inside the day rolls up the same.
		users[i], kinds[i], sources[i], ats[i], ns[i] = c.UserID, c.Kind, c.Source, isoDate(c.Day)+" 12:00:00+05:30", int32(c.Count)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO profile_events (user_id, kind, source, at, count)
		SELECT t.u, t.k, t.s, t.a, t.c
		FROM unnest($1::uuid[], $2::text[], $3::text[], $4::timestamptz[], $5::int[]) AS t(u, k, s, a, c)
		WHERE EXISTS (SELECT 1 FROM profiles p WHERE p.user_id = t.u)`, users, kinds, sources, ats, ns); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FlushEvents: events are written directly, so nothing is buffered here.
func (r *ProfileAnalyticsRepository) FlushEvents(context.Context) error { return nil }

func (r *ProfileAnalyticsRepository) DailyStats(ctx context.Context, slug string, from, to time.Time) ([]profiles.DayStats, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `SELECT user_id::text FROM profiles WHERE slug = $1 AND is_public`, slug).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, profiles.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT day, views_search, views_direct, resume_views, resume_downloads FROM profile_daily_stats
		WHERE user_id = $1 AND day BETWEEN $2::date AND $3::date`, userID, isoDate(from), isoDate(to))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[profiles.DayStats])
}

func (r *ProfileAnalyticsRepository) Rollup(ctx context.Context, from, today time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Rebuild every day in the range, so a missed or partial run heals itself.
	// Days are IST dates: $1 and $2 are midnight IST, as timestamps.
	for _, q := range []string{
		`DELETE FROM profile_daily_stats WHERE day >= $1::date AND day < $2::date`,
		`INSERT INTO profile_daily_stats (user_id, day, views_search, views_direct, resume_views, resume_downloads)
		SELECT user_id, (at AT TIME ZONE 'Asia/Kolkata')::date,
			COALESCE(SUM(count) FILTER (WHERE kind = 'view' AND source = 'search'), 0),
			COALESCE(SUM(count) FILTER (WHERE kind = 'view' AND source = 'direct'), 0),
			COALESCE(SUM(count) FILTER (WHERE kind = 'resume_view'), 0),
			COALESCE(SUM(count) FILTER (WHERE kind = 'resume_download'), 0)
		FROM profile_events
		WHERE at >= ($1::date::timestamp AT TIME ZONE 'Asia/Kolkata') AND at < ($2::date::timestamp AT TIME ZONE 'Asia/Kolkata')
		GROUP BY 1, 2`,
	} {
		if _, err := tx.Exec(ctx, q, isoDate(from), isoDate(today)); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`DELETE FROM profile_events WHERE at < NOW() - $1::interval`,
		// A batch id only has to outlive any retry of that batch.
		`DELETE FROM profile_event_batches WHERE applied_at < NOW() - $1::interval`,
	} {
		if _, err := tx.Exec(ctx, q, profileEventsKeep); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM profile_daily_stats WHERE day < $1::date - $2::int`, isoDate(today), profileStatsKeep); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// isoDate passes an IST day as text, so no session time zone can shift it.
func isoDate(day time.Time) string { return day.Format(time.DateOnly) }
