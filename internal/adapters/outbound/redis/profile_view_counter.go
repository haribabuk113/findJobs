package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// Profile views are counted in one Redis hash per IST day, field
// "<user id>|<kind>|<source>", and flushed to Postgres every few minutes.
// A flush first renames the day's hash to a unique batch key, so new views
// go to a fresh hash and the batch id makes the Postgres write exactly-once.
const (
	viewCountsPrefix = "pviews:"       // pviews:2026-10-29
	viewBatchPrefix  = "pviews-batch:" // pviews-batch:2026-10-29:<uuid>
	// viewCountsTTL is a safety net only: flushes run every few minutes, but
	// if they stop, counts expire with the raw-event retention instead of piling up.
	viewCountsTTL  = 15 * 24 * time.Hour
	flushOpTimeout = 5 * time.Second
)

// ProfileViewCounter buffers profile analytics events in Redis in front of
// the Postgres store. Redis errors fall back to writing the event to
// Postgres directly, so no view is lost while Redis is down. Everything else
// is the store's. If Redis itself loses data, at most the last unflushed
// minutes of counts go with it (analytics, never money or sessions).
type ProfileViewCounter struct {
	ports.ProfileAnalyticsRepository // DailyStats, AddEventCounts: the store's
	store                            ports.ProfileAnalyticsRepository
	rdb                              *goredis.Client
	now                              func() time.Time
}

// NewProfileViewCounter wraps store with Redis event buffering.
func NewProfileViewCounter(store ports.ProfileAnalyticsRepository, rdb *goredis.Client) ports.ProfileAnalyticsRepository {
	return &ProfileViewCounter{ProfileAnalyticsRepository: store, store: store, rdb: rdb, now: time.Now}
}

func (c *ProfileViewCounter) RecordEvent(ctx context.Context, userID, kind, source string) error {
	key := viewCountsPrefix + profiles.ISTDay(c.now()).Format(time.DateOnly)
	rctx, cancel := context.WithTimeout(ctx, opTimeout)
	pipe := c.rdb.TxPipeline()
	pipe.HIncrBy(rctx, key, userID+"|"+kind+"|"+source, 1)
	pipe.Expire(rctx, key, viewCountsTTL)
	_, err := pipe.Exec(rctx)
	cancel()
	if err == nil {
		return nil
	}
	l := logger.Get()
	l.Warn().Err(err).Msg("redis unavailable for profile analytics; writing the event to Postgres")
	return c.store.RecordEvent(ctx, userID, kind, source)
}

// FlushEvents moves every buffered day into Postgres: first batches left by
// an interrupted flush, then the live day hashes.
func (c *ProfileViewCounter) FlushEvents(ctx context.Context) error {
	batches, err := c.scan(ctx, viewBatchPrefix+"*")
	if err != nil {
		return err
	}
	days, err := c.scan(ctx, viewCountsPrefix+"*")
	if err != nil {
		return err
	}
	for _, day := range days {
		batch := viewBatchPrefix + strings.TrimPrefix(day, viewCountsPrefix) + ":" + uuid.NewString()
		rctx, cancel := context.WithTimeout(ctx, flushOpTimeout)
		err := c.rdb.Rename(rctx, day, batch).Err()
		cancel()
		if err != nil {
			if isNoSuchKey(err) {
				continue // another flush took it first
			}
			return err
		}
		batches = append(batches, batch)
	}
	var errs []error
	for _, batch := range batches {
		errs = append(errs, c.flushBatch(ctx, batch))
	}
	return errors.Join(errs...)
}

// flushBatch writes one renamed hash to Postgres, then deletes it. A crash
// between the two leaves the batch in Redis; the next flush retries it and
// AddEventCounts skips it as already applied.
func (c *ProfileViewCounter) flushBatch(ctx context.Context, batch string) error {
	rest := strings.TrimPrefix(batch, viewBatchPrefix)
	day, err := time.Parse(time.DateOnly, strings.SplitN(rest, ":", 2)[0])
	if err != nil {
		return fmt.Errorf("profile views batch %q: bad day", batch)
	}
	rctx, cancel := context.WithTimeout(ctx, flushOpTimeout)
	fields, err := c.rdb.HGetAll(rctx, batch).Result()
	cancel()
	if err != nil {
		return err
	}
	counts := make([]profiles.EventCount, 0, len(fields))
	for field, value := range fields {
		parts := strings.Split(field, "|")
		n, err := strconv.Atoi(value)
		if len(parts) != 3 || uuid.Validate(parts[0]) != nil || err != nil || n <= 0 {
			continue // never written by RecordEvent: skip rather than block the batch
		}
		counts = append(counts, profiles.EventCount{UserID: parts[0], Kind: parts[1], Source: parts[2], Day: day, Count: n})
	}
	if len(counts) > 0 {
		if err := c.store.AddEventCounts(ctx, batch, counts); err != nil {
			return err
		}
	}
	rctx, cancel = context.WithTimeout(ctx, flushOpTimeout)
	defer cancel()
	return c.rdb.Del(rctx, batch).Err()
}

// Rollup flushes first, so the stats include everything counted so far. A
// failed flush (Redis down) doesn't stop the rollup: the counts stay in
// Redis, and the next rollup rebuilds those days with them.
func (c *ProfileViewCounter) Rollup(ctx context.Context, from, today time.Time) error {
	if err := c.FlushEvents(ctx); err != nil {
		l := logger.Get()
		l.Warn().Err(err).Msg("profile views flush before rollup failed; rolling up what Postgres has")
	}
	return c.store.Rollup(ctx, from, today)
}

func (c *ProfileViewCounter) scan(ctx context.Context, pattern string) ([]string, error) {
	rctx, cancel := context.WithTimeout(ctx, flushOpTimeout)
	defer cancel()
	var keys []string
	iter := c.rdb.Scan(rctx, 0, pattern, 100).Iterator()
	for iter.Next(rctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

func isNoSuchKey(err error) bool { return err != nil && strings.Contains(err.Error(), "no such key") }
