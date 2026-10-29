// Package scheduler runs the daily background jobs (system requirements:
// low-resource window, IST). Jobs must be idempotent: each runs at startup to
// catch up after downtime, then every day at its IST time.
package scheduler

import (
	"context"
	"time"

	"findJobs/internal/pkg/background"
	"findJobs/internal/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ist = time.FixedZone("IST", 5*60*60+30*60)

// Daily runs job now and then every day at hour:minute IST, inside bg (so
// shutdown cancels and waits for it). A Postgres advisory lock named after
// the job means only one instance runs it at a time; the others skip that run.
func Daily(bg *background.Group, db *pgxpool.Pool, name string, hour, minute int, job func(ctx context.Context) error) {
	bg.Go(func(ctx context.Context) {
		for {
			runLocked(ctx, db, name, job)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(nextRun(time.Now(), hour, minute))):
			}
		}
	})
}

// Every runs job now and then every interval, with the same single-instance
// lock and shutdown handling as Daily.
func Every(bg *background.Group, db *pgxpool.Pool, name string, interval time.Duration, job func(ctx context.Context) error) {
	bg.Go(func(ctx context.Context) {
		for {
			runLocked(ctx, db, name, job)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
		}
	})
}

// nextRun is the first hour:minute IST strictly after now.
func nextRun(now time.Time, hour, minute int) time.Time {
	n := now.In(ist)
	next := time.Date(n.Year(), n.Month(), n.Day(), hour, minute, 0, 0, ist)
	if !next.After(n) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func runLocked(ctx context.Context, db *pgxpool.Pool, name string, job func(ctx context.Context) error) {
	l := logger.Get()
	conn, err := db.Acquire(ctx)
	if err != nil {
		if ctx.Err() == nil {
			l.Error().Err(err).Str("job", name).Msg("scheduled job: no database connection")
		}
		return
	}
	defer conn.Release()
	// A session lock on this connection, held while the job uses others.
	key := "findjobs:job:" + name
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, key).Scan(&locked); err != nil || !locked {
		if err != nil && ctx.Err() == nil {
			l.Error().Err(err).Str("job", name).Msg("scheduled job: lock failed")
		}
		return
	}
	// Unlock even when shutdown cancelled ctx; otherwise the pooled connection keeps the lock.
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext($1))`, key)

	start := time.Now()
	switch err := job(ctx); {
	case err != nil && ctx.Err() != nil:
		l.Info().Err(err).Str("job", name).Msg("scheduled job stopped by shutdown")
	case err != nil:
		l.Error().Err(err).Str("job", name).Msg("scheduled job failed")
	default:
		l.Info().Str("job", name).Dur("took", time.Since(start)).Msg("scheduled job done")
	}
}
