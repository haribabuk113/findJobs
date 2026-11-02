package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

// fakeSessions is one stored session. saveErr makes LockSession fail after fn
// ran, the way a failed UPDATE would after IAM already rotated.
type fakeSessions struct {
	ports.SessionStore
	state    auth.SessionState
	stored   auth.Session
	touchErr error
	saveErr  error
	touched  int
	deleted  []string
}

func (f *fakeSessions) Lookup(_ context.Context, id string) (*auth.SessionState, error) {
	if id != f.state.ID {
		return nil, auth.ErrUnauthorized
	}
	st := f.state
	return &st, nil
}
func (f *fakeSessions) Touch(context.Context, string) error {
	f.touched++
	return f.touchErr
}
func (f *fakeSessions) LockSession(_ context.Context, id string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error) {
	cur := f.stored
	next, err := fn(&cur)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return &cur, nil
	}
	if f.saveErr != nil {
		return nil, f.saveErr
	}
	f.stored = *next
	return next, nil
}
func (f *fakeSessions) DeleteSession(_ context.Context, id string) (string, error) {
	f.deleted = append(f.deleted, id)
	return f.stored.RefreshToken, nil
}

type fakeIAM struct {
	ports.IAMClient
	refreshErr error
	refreshed  int
	loggedOut  []string
}

func (f *fakeIAM) Refresh(context.Context, string) (*auth.IAMTokens, error) {
	f.refreshed++
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	now := time.Now()
	return &auth.IAMTokens{AccessToken: "a2", RefreshToken: "r2", ExpiresAt: now.Add(15 * time.Minute), RefreshExpiresAt: now.Add(30 * 24 * time.Hour)}, nil
}
func (f *fakeIAM) Verify(context.Context, string) (*auth.IAMClaims, error) {
	return &auth.IAMClaims{EcosystemID: "e1", Status: "active"}, nil
}
func (f *fakeIAM) Logout(_ context.Context, token string) error {
	f.loggedOut = append(f.loggedOut, token)
	return nil
}

const idle = 7 * 24 * time.Hour

// setup returns a session whose access token expires accessIn from now and
// that was last seen seenAgo ago.
func setup(accessIn, seenAgo time.Duration) (*fakeSessions, *fakeIAM, AuthService) {
	now := time.Now()
	sessions := &fakeSessions{
		state: auth.SessionState{ID: "s1", UserID: "u1", IAMUserID: "e1", AccessExpiresAt: now.Add(accessIn),
			ExpiresAt: now.Add(time.Hour), LastSeenAt: now.Add(-seenAgo)},
		stored: auth.Session{ID: "s1", UserID: "u1", RefreshToken: "r1", AccessToken: "a1", AccessExpiresAt: now.Add(accessIn),
			ExpiresAt: now.Add(time.Hour)},
	}
	iam := &fakeIAM{}
	return sessions, iam, NewAuthService(nil, sessions, iam, idle)
}

func TestAuthenticateTouchesAtMostEvery15Minutes(t *testing.T) {
	sessions, _, s := setup(time.Hour, time.Minute)
	if _, err := s.Authenticate(context.Background(), "s1"); err != nil || sessions.touched != 0 {
		t.Fatalf("recently seen: touched %d, %v", sessions.touched, err)
	}
	sessions, _, s = setup(time.Hour, 20*time.Minute)
	st, err := s.Authenticate(context.Background(), "s1")
	if err != nil || sessions.touched != 1 || time.Since(st.LastSeenAt) > time.Second {
		t.Fatalf("seen 20 min ago: touched %d, %+v %v", sessions.touched, st, err)
	}
}

// A failed last_seen_at write must not fail the request.
func TestAuthenticateSurvivesTouchFailure(t *testing.T) {
	sessions, _, s := setup(time.Hour, 20*time.Minute)
	sessions.touchErr = errors.New("db down")
	st, err := s.Authenticate(context.Background(), "s1")
	if err != nil || st == nil || st.UserID != "u1" || time.Since(st.LastSeenAt) < 19*time.Minute {
		t.Fatalf("touch failure: %+v %v", st, err)
	}
}

func TestAuthenticateEndsIdleSessionAtIAM(t *testing.T) {
	sessions, iam, s := setup(time.Hour, idle+time.Minute)
	if _, err := s.Authenticate(context.Background(), "s1"); !auth.IsUnauthorized(err) {
		t.Fatalf("idle: want ErrUnauthorized, got %v", err)
	}
	if len(sessions.deleted) != 1 || len(iam.loggedOut) != 1 || iam.loggedOut[0] != "r1" {
		t.Fatalf("idle session must be deleted and revoked: deleted %v, revoked %v", sessions.deleted, iam.loggedOut)
	}
	if _, err := s.Authenticate(context.Background(), "nope"); !auth.IsUnauthorized(err) {
		t.Fatalf("unknown session: %v", err)
	}
}

func TestAuthenticateRefreshesExpiredAccessToken(t *testing.T) {
	sessions, iam, s := setup(-time.Minute, time.Minute)
	st, err := s.Authenticate(context.Background(), "s1")
	if err != nil || iam.refreshed != 1 || sessions.stored.RefreshToken != "r2" || !st.AccessExpiresAt.After(time.Now()) {
		t.Fatalf("refresh: %+v %v (refreshed %d, stored %+v)", st, err, iam.refreshed, sessions.stored)
	}
}

// IAM refusing the refresh (expired, reused, suspended) ends our session too.
func TestRefreshRefusedDeletesSession(t *testing.T) {
	sessions, iam, s := setup(-time.Minute, time.Minute)
	iam.refreshErr = auth.ErrUnauthorized
	if _, err := s.Authenticate(context.Background(), "s1"); !auth.IsUnauthorized(err) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
	if len(sessions.deleted) != 1 || len(iam.loggedOut) != 0 {
		t.Fatalf("deleted %v, revoked %v", sessions.deleted, iam.loggedOut)
	}
}

// IAM rotated but saving failed: the stored refresh token is now a used one,
// and presenting it later would make IAM revoke the session as theft. The
// service ends the session cleanly with the NEW token instead.
func TestRotatedButSaveFailedLogsOutWithNewToken(t *testing.T) {
	sessions, iam, s := setup(-time.Minute, time.Minute)
	sessions.saveErr = errors.New("update failed")
	if _, err := s.Authenticate(context.Background(), "s1"); err == nil {
		t.Fatal("want the save error")
	}
	if len(sessions.deleted) != 1 || len(iam.loggedOut) != 1 || iam.loggedOut[0] != "r2" {
		t.Fatalf("deleted %v, revoked %v (must revoke with the rotated token r2)", sessions.deleted, iam.loggedOut)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	sessions, iam, s := setup(time.Hour, time.Minute)
	if err := s.Logout(context.Background(), "s1"); err != nil || len(iam.loggedOut) != 1 {
		t.Fatalf("logout: %v %v", err, iam.loggedOut)
	}
	sessions.stored.RefreshToken = "" // gone: nothing to revoke
	if err := s.Logout(context.Background(), "s1"); err != nil || len(iam.loggedOut) != 1 {
		t.Fatalf("second logout: %v %v", err, iam.loggedOut)
	}
}
