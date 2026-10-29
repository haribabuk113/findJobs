package redis

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

// fakeAnalytics records what reaches "Postgres"; batches apply once, as in the real store.
type fakeAnalytics struct {
	ports.ProfileAnalyticsRepository
	direct  int
	applied map[string]bool
	counts  map[string]int // user|kind|source|day -> total
}

func (f *fakeAnalytics) RecordEvent(context.Context, string, string, string) error {
	f.direct++
	return nil
}
func (f *fakeAnalytics) AddEventCounts(_ context.Context, id string, counts []profiles.EventCount) error {
	if f.applied[id] {
		return nil
	}
	f.applied[id] = true
	for _, c := range counts {
		f.counts[c.UserID+"|"+c.Kind+"|"+c.Source+"|"+c.Day.Format(time.DateOnly)] += c.Count
	}
	return nil
}

func TestProfileViewCounter(t *testing.T) {
	shared := redisClient(t)
	// A separate database: flushing must not sweep the dev app's live counts (DB 0).
	opts := *shared.Options()
	opts.DB = 15
	rdb := goredis.NewClient(&opts)
	ctx := context.Background()
	clear := func() {
		for _, pattern := range []string{viewCountsPrefix + "*", viewBatchPrefix + "*"} {
			keys, _ := rdb.Keys(ctx, pattern).Result()
			if len(keys) > 0 {
				rdb.Del(ctx, keys...)
			}
		}
	}
	clear()
	t.Cleanup(func() { clear(); rdb.Close() })

	store := &fakeAnalytics{applied: map[string]bool{}, counts: map[string]int{}}
	c := NewProfileViewCounter(store, rdb).(*ProfileViewCounter)
	now := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC) // 01:30 IST on the 30th
	c.now = func() time.Time { return now }
	user := uuid.NewString()
	key := user + "|view|search|2026-10-30"

	for range 3 {
		if err := c.RecordEvent(ctx, user, profiles.EventView, profiles.SourceSearch); err != nil {
			t.Fatal(err)
		}
	}
	if store.direct != 0 || store.counts[key] != 0 {
		t.Fatalf("counted in Redis, not Postgres: direct %d, flushed %v", store.direct, store.counts)
	}
	if ttl := rdb.TTL(ctx, viewCountsPrefix+"2026-10-30").Val(); ttl <= 0 {
		t.Fatalf("day hash has no safety TTL: %v", ttl)
	}
	if err := c.FlushEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if store.counts[key] != 3 {
		t.Fatalf("after flush: %v", store.counts)
	}
	if keys := rdb.Keys(ctx, "pviews*").Val(); len(keys) != 0 {
		t.Fatalf("flushed keys left in Redis: %v", keys)
	}
	// Nothing new: a second flush adds nothing.
	if err := c.FlushEvents(ctx); err != nil || store.counts[key] != 3 {
		t.Fatalf("second flush: %v %v", store.counts, err)
	}

	// A flush that crashed after writing Postgres but before deleting the batch:
	// the replay must not count it twice.
	batch := viewBatchPrefix + "2026-10-30:" + uuid.NewString()
	rdb.HSet(ctx, batch, user+"|view|search", 2)
	if err := c.flushBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}
	rdb.HSet(ctx, batch, user+"|view|search", 2) // as if the DEL never happened
	if err := c.FlushEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if store.counts[key] != 5 {
		t.Fatalf("replayed batch: got %d, want 5 (3 + 2 once)", store.counts[key])
	}

	// Redis down: the view goes straight to Postgres, nothing is lost.
	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { dead.Close() })
	down := NewProfileViewCounter(store, dead)
	start := time.Now()
	if err := down.RecordEvent(ctx, user, profiles.EventView, profiles.SourceDirect); err != nil || store.direct != 1 {
		t.Fatalf("fallback: direct %d, %v", store.direct, err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("fallback took %v", time.Since(start))
	}
}
