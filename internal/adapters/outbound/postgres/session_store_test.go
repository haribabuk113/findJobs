package postgres

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/auth"
	"findJobs/internal/pkg/tokencrypt"
)

// Runs against the compose Postgres (migrated by the app); skipped when absent.
// POSTGRES_TEST_DSN overrides the default.
func testStore(t *testing.T) (*SessionStore, string) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://dbuser:dbpass@127.0.0.1:5433/jportal"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err == nil {
		err = pool.Ping(ctx)
	}
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)

	var userID string
	err = pool.QueryRow(ctx, `INSERT INTO users (iam_user_id, email) VALUES ($1, 'test@example.com') RETURNING id::text`,
		"test-"+uuid.NewString()).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID) }) // cascades sessions
	keys, err := tokencrypt.Parse("t1:AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")
	if err != nil {
		t.Fatal(err)
	}
	return &SessionStore{pool: pool, keys: keys}, userID
}

func newSession(t *testing.T, s *SessionStore, userID string, accessExpires time.Time) string {
	id, err := s.CreateSession(context.Background(), &auth.Session{
		UserID: userID, RefreshToken: "r0", AccessToken: "a0",
		AccessExpiresAt: accessExpires, ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// The property the row lock exists for: concurrent requests on one expired
// session refresh exactly once, and all see the rotated token.
func TestConcurrentRefreshRotatesOnce(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	id := newSession(t, s, userID, time.Now().Add(-time.Second))

	var refreshes atomic.Int32
	refresh := func(sess *auth.Session) (*auth.Session, error) {
		if time.Now().Before(sess.AccessExpiresAt) {
			return nil, nil // another request already refreshed
		}
		if sess.RefreshToken != "r0" {
			t.Errorf("refresh token presented twice: %q", sess.RefreshToken)
		}
		refreshes.Add(1)
		time.Sleep(100 * time.Millisecond) // simulate the IAM round trip
		next := *sess
		next.RefreshToken, next.AccessToken = "r1", "a1"
		next.AccessExpiresAt = time.Now().Add(10 * time.Minute)
		return &next, nil
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess, err := s.LockSession(ctx, id, refresh)
			if err != nil {
				t.Errorf("LockSession: %v", err)
				return
			}
			if sess.AccessToken != "a1" {
				t.Errorf("got access token %q, want a1", sess.AccessToken)
			}
		}()
	}
	wg.Wait()
	if n := refreshes.Load(); n != 1 {
		t.Fatalf("refreshed %d times, want 1", n)
	}
}

// A logout arriving mid-refresh waits for the row lock, then deletes:
// the refresh can't resurrect the session.
func TestLogoutDuringRefreshStaysLoggedOut(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	id := newSession(t, s, userID, time.Now().Add(-time.Second))

	refreshing := make(chan struct{})
	done := make(chan error)
	go func() {
		_, err := s.LockSession(ctx, id, func(sess *auth.Session) (*auth.Session, error) {
			close(refreshing)
			time.Sleep(200 * time.Millisecond)
			next := *sess
			next.RefreshToken = "r1"
			next.AccessExpiresAt = time.Now().Add(10 * time.Minute)
			return &next, nil
		})
		done <- err
	}()
	<-refreshing
	refreshToken, err := s.DeleteSession(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("refresh: %v", err)
	}
	// Logout saw the rotated token, so it revokes the live IAM session.
	if refreshToken != "r1" {
		t.Errorf("logout got refresh token %q, want r1", refreshToken)
	}
	if _, err := s.Lookup(ctx, id); !auth.IsUnauthorized(err) {
		t.Fatalf("session resurrected after logout: %v", err)
	}
}

func TestRejectsMalformedUnknownAndExpired(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	for _, id := range []string{"", "not-a-uuid", "' OR 1=1 --", uuid.NewString()} {
		if _, err := s.Lookup(ctx, id); !auth.IsUnauthorized(err) {
			t.Errorf("Lookup(%q): want ErrUnauthorized, got %v", id, err)
		}
	}
	expired, err := s.CreateSession(ctx, &auth.Session{UserID: userID, RefreshToken: "r", AccessToken: "a",
		AccessExpiresAt: time.Now(), ExpiresAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lookup(ctx, expired); !auth.IsUnauthorized(err) {
		t.Errorf("expired session: want ErrUnauthorized, got %v", err)
	}
}

// Tokens are unreadable in the table, and a ciphertext copied into another
// session's row doesn't decrypt there (AAD binds it to its row).
func TestTokensEncryptedAndBoundToRow(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	a := newSession(t, s, userID, time.Now().Add(time.Minute))
	b := newSession(t, s, userID, time.Now().Add(time.Minute))

	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT refresh_token_enc FROM sessions WHERE id = $1`, a).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "r0") || len(raw) < 28 {
		t.Fatalf("refresh token not encrypted at rest: %q", raw)
	}

	sess, err := s.LockSession(ctx, a, func(*auth.Session) (*auth.Session, error) { return nil, nil })
	if err != nil || sess.RefreshToken != "r0" || sess.AccessToken != "a0" {
		t.Fatalf("decrypt own row: %+v, %v", sess, err)
	}
	state, err := s.Lookup(ctx, a)
	if err != nil || state.IAMUserID == "" || state.LastSeenAt.IsZero() {
		t.Fatalf("lookup state: %+v, %v", state, err)
	}

	if _, err := s.pool.Exec(ctx, `UPDATE sessions SET refresh_token_enc = $1 WHERE id = $2`, raw, b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LockSession(ctx, b, func(*auth.Session) (*auth.Session, error) { return nil, nil }); !auth.IsUnauthorized(err) {
		t.Fatalf("token moved to another row: want ErrUnauthorized, got %v", err)
	}
}
