package redis

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"findJobs/internal/domain/auth"
)

// fakeStore stands in for Postgres and counts Lookups.
type fakeStore struct {
	mu      sync.Mutex
	state   map[string]auth.SessionState
	lookups int
}

func (f *fakeStore) CreateSession(context.Context, *auth.Session) (string, error) { return "", nil }
func (f *fakeStore) Touch(context.Context, string) error                          { return nil }
func (f *fakeStore) Lookup(_ context.Context, id string) (*auth.SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	st, ok := f.state[id]
	if !ok {
		return nil, auth.ErrUnauthorized
	}
	return &st, nil
}
func (f *fakeStore) LockSession(_ context.Context, id string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error) {
	return fn(&auth.Session{ID: id})
}
func (f *fakeStore) DeleteSession(_ context.Context, id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.state, id)
	return "rt", nil
}

func redisClient(t *testing.T) *goredis.Client {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	password := os.Getenv("REDIS_PASSWORD")
	if password == "" {
		password = "devredispass"
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: addr, Password: password})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", addr, err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func setup(t *testing.T, rdb *goredis.Client, accessFor time.Duration) (*SessionCache, *fakeStore, string) {
	id := uuid.NewString()
	now := time.Now()
	store := &fakeStore{state: map[string]auth.SessionState{id: {
		ID: id, UserID: "u1", IAMUserID: "e1",
		AccessExpiresAt: now.Add(accessFor), ExpiresAt: now.Add(time.Hour), LastSeenAt: now,
	}}}
	t.Cleanup(func() { rdb.Del(context.Background(), cacheKey(id)) })
	return &SessionCache{store: store, rdb: rdb}, store, id
}

func TestCacheHitSkipsStoreAndExpiresWithAccessToken(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	c, store, id := setup(t, rdb, 5*time.Minute)

	for range 3 {
		st, err := c.Lookup(ctx, id)
		if err != nil || st.UserID != "u1" || st.IAMUserID != "e1" {
			t.Fatalf("lookup: %+v, %v", st, err)
		}
	}
	if store.lookups != 1 {
		t.Fatalf("store read %d times, want 1 (then cache hits)", store.lookups)
	}
	if ttl := rdb.TTL(ctx, cacheKey(id)).Val(); ttl > 5*time.Minute || ttl <= 0 {
		t.Fatalf("cache TTL %v must not outlive the access token", ttl)
	}
	if v := rdb.Get(ctx, cacheKey(id)).Val(); v == "" || containsAny(v, "refresh", "access_token\"") {
		t.Fatalf("cached value must hold no tokens: %s", v)
	}
}

// The race the tombstone exists for: a request reads the store just before a
// logout, then tries to cache that stale state after it.
func TestStalePopulateCannotOverwriteLogout(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	c, store, id := setup(t, rdb, 5*time.Minute)
	stale := store.state[id]

	if _, err := c.DeleteSession(ctx, id); err != nil {
		t.Fatal(err)
	}
	store.state[id] = stale // in-flight reader saw the row before the delete
	c.Lookup(ctx, id)       // its populate must lose to the tombstone
	delete(store.state, id)

	if _, err := c.Lookup(ctx, id); !auth.IsUnauthorized(err) {
		t.Fatalf("after logout: want ErrUnauthorized, got %v", err)
	}
	if v := rdb.Get(ctx, cacheKey(id)).Val(); v != tombstone {
		t.Fatalf("tombstone overwritten: %q", v)
	}
}

func TestRefreshAndTouchInvalidateButKeepTombstone(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	c, store, id := setup(t, rdb, 5*time.Minute)

	c.Lookup(ctx, id)
	c.Touch(ctx, id)
	c.Lookup(ctx, id)
	c.LockSession(ctx, id, func(*auth.Session) (*auth.Session, error) { return nil, nil })
	c.Lookup(ctx, id)
	if store.lookups != 3 {
		t.Fatalf("store read %d times, want 3 (each invalidation forces a re-read)", store.lookups)
	}

	c.DeleteSession(ctx, id)
	c.Touch(ctx, id)
	c.LockSession(ctx, id, func(*auth.Session) (*auth.Session, error) { return nil, nil })
	if v := rdb.Get(ctx, cacheKey(id)).Val(); v != tombstone {
		t.Fatalf("invalidate removed the tombstone: %q", v)
	}
}

func TestExpiredAccessTokenIsNotCached(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	c, _, id := setup(t, rdb, -time.Second)
	if _, err := c.Lookup(ctx, id); err != nil {
		t.Fatal(err)
	}
	if n := rdb.Exists(ctx, cacheKey(id)).Val(); n != 0 {
		t.Fatal("state with an expired access token was cached")
	}
}

// Redis down: every call falls through to the store, quickly.
func TestRedisDownFallsBackToStore(t *testing.T) {
	dead := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { dead.Close() })
	c, store, id := setup(t, dead, 5*time.Minute)

	start := time.Now()
	st, err := c.Lookup(context.Background(), id)
	if err != nil || st.UserID != "u1" || store.lookups != 1 {
		t.Fatalf("fallback lookup: %+v, %v, store reads %d", st, err, store.lookups)
	}
	if _, err := c.DeleteSession(context.Background(), id); err != nil {
		t.Fatalf("logout must succeed without Redis: %v", err)
	}
	if _, err := c.Lookup(context.Background(), id); !auth.IsUnauthorized(err) {
		t.Fatalf("after logout without Redis: want ErrUnauthorized, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("fallback too slow: %v", elapsed)
	}
}

func containsAny(s string, subs ...string) bool {
	return slices.ContainsFunc(subs, func(sub string) bool { return strings.Contains(s, sub) })
}
