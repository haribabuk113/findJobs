package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"findJobs/internal/domain/auth"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

const (
	// maxCacheTTL caps a cached entry; entries also expire with the access
	// token, so IAM suspension still takes effect within one access-token life.
	maxCacheTTL = 10 * time.Minute
	// tombstoneTTL must outlast maxCacheTTL so a logout can't be undone by a
	// request that read Postgres just before it (see Lookup).
	tombstoneTTL = 15 * time.Minute
	// opTimeout keeps a slow Redis from slowing requests: on timeout the
	// request falls through to Postgres.
	opTimeout = 250 * time.Millisecond
	tombstone = "revoked"
)

// invalidateScript deletes a cached entry unless it is a logout tombstone.
var invalidateScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return 0 end
return redis.call('DEL', KEYS[1])`)

// SessionCache puts a Redis read-through cache in front of the durable
// session store. It caches only SessionState — never IAM tokens — and Redis
// data alone never creates a session: misses and Redis errors go to the store.
type SessionCache struct {
	store ports.SessionStore
	rdb   *goredis.Client
}

// NewSessionCache wraps store with a Redis cache.
func NewSessionCache(store ports.SessionStore, rdb *goredis.Client) ports.SessionStore {
	return &SessionCache{store: store, rdb: rdb}
}

type cachedState struct {
	UserID          string    `json:"user_id"`
	IAMUserID       string    `json:"iam_user_id"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
}

func cacheKey(id string) string { return "sess:" + id }

func (c *SessionCache) CreateSession(ctx context.Context, sess *auth.Session) (string, error) {
	return c.store.CreateSession(ctx, sess) // the first Lookup caches it
}

// Lookup serves from Redis while the cached access token is valid. On a miss
// it reads the store and caches with SET NX, so a request that read the store
// just before a logout can't overwrite the logout's tombstone.
func (c *SessionCache) Lookup(ctx context.Context, sessionID string) (*auth.SessionState, error) {
	if uuid.Validate(sessionID) != nil {
		return nil, auth.ErrUnauthorized
	}
	key := cacheKey(sessionID)
	rctx, cancel := context.WithTimeout(ctx, opTimeout)
	value, err := c.rdb.Get(rctx, key).Result()
	cancel()
	switch {
	case err == nil && value == tombstone:
		return nil, auth.ErrUnauthorized
	case err == nil:
		var cs cachedState
		if json.Unmarshal([]byte(value), &cs) == nil && time.Now().Before(cs.AccessExpiresAt) && time.Now().Before(cs.ExpiresAt) {
			return &auth.SessionState{ID: sessionID, UserID: cs.UserID, IAMUserID: cs.IAMUserID,
				AccessExpiresAt: cs.AccessExpiresAt, ExpiresAt: cs.ExpiresAt, LastSeenAt: cs.LastSeenAt}, nil
		}
	case !errors.Is(err, goredis.Nil):
		warn(err, "session cache read failed; using Postgres")
	}
	redisDown := err != nil && !errors.Is(err, goredis.Nil)

	state, err := c.store.Lookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ttl := min(time.Until(state.AccessExpiresAt), time.Until(state.ExpiresAt), maxCacheTTL)
	if ttl > time.Second && !redisDown { // don't wait out a second timeout
		value, _ := json.Marshal(cachedState{UserID: state.UserID, IAMUserID: state.IAMUserID,
			AccessExpiresAt: state.AccessExpiresAt, ExpiresAt: state.ExpiresAt, LastSeenAt: state.LastSeenAt})
		rctx, cancel := context.WithTimeout(ctx, opTimeout)
		if err := c.rdb.SetNX(rctx, key, value, ttl).Err(); err != nil {
			warn(err, "session cache write failed")
		}
		cancel()
	}
	return state, nil
}

func (c *SessionCache) LockSession(ctx context.Context, sessionID string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error) {
	sess, err := c.store.LockSession(ctx, sessionID, fn)
	c.invalidate(ctx, sessionID) // expiries may have changed
	return sess, err
}

func (c *SessionCache) Touch(ctx context.Context, sessionID string) error {
	err := c.store.Touch(ctx, sessionID)
	c.invalidate(ctx, sessionID) // cached last_seen_at is stale
	return err
}

// DeleteSession deletes the durable session, then leaves a tombstone. If Redis
// is unreachable, so are cached entries: lookups fall through to the store,
// which no longer has the session.
func (c *SessionCache) DeleteSession(ctx context.Context, sessionID string) (string, error) {
	refreshToken, err := c.store.DeleteSession(ctx, sessionID)
	if err != nil || uuid.Validate(sessionID) != nil {
		return refreshToken, err
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opTimeout)
	defer cancel()
	if err := c.rdb.Set(rctx, cacheKey(sessionID), tombstone, tombstoneTTL).Err(); err != nil {
		warn(err, "session tombstone write failed")
	}
	return refreshToken, nil
}

func (c *SessionCache) invalidate(ctx context.Context, sessionID string) {
	if uuid.Validate(sessionID) != nil {
		return
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opTimeout)
	defer cancel()
	if err := invalidateScript.Run(rctx, c.rdb, []string{cacheKey(sessionID)}, tombstone).Err(); err != nil && !errors.Is(err, goredis.Nil) {
		warn(err, "session cache invalidate failed")
	}
}

func warn(err error, msg string) {
	l := logger.Get()
	l.Warn().Err(err).Msg(msg)
}
