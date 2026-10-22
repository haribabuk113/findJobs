package auth

import (
	"context"
	"time"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

type service struct {
	repo        ports.AuthRepository
	sessions    ports.SessionStore
	iam         ports.IAMClient
	idleTimeout time.Duration
}

// NewAuthService creates a new auth service using dependency injection.
// Sessions unused for idleTimeout end; IAM's 30-day absolute limit still applies.
func NewAuthService(repo ports.AuthRepository, sessions ports.SessionStore, iam ports.IAMClient, idleTimeout time.Duration) AuthService {
	return &service{repo: repo, sessions: sessions, iam: iam, idleTimeout: idleTimeout}
}

// CreateSession redeems the IAM code, verifies the issued token, upserts the
// user keyed by ecosystem ID (sub) and stores a session.
func (s *service) CreateSession(ctx context.Context, request *auth.SessionRequest) (string, *auth.SessionResponse, error) {
	if err := request.Validate(); err != nil {
		return "", nil, err
	}

	tokens, err := s.iam.ExchangeCode(ctx, request.Code)
	if err != nil {
		return "", nil, err
	}
	claims, err := s.iam.Verify(ctx, tokens.AccessToken)
	if err != nil {
		return "", nil, err
	}
	if claims.Status != "active" {
		return "", nil, auth.ErrUnauthorized
	}

	user, err := s.repo.UpsertUser(ctx, &auth.User{IAMUserID: claims.EcosystemID, Name: claims.Name, Email: claims.Email, Status: claims.Status})
	if err != nil {
		return "", nil, auth.NewInternalError("failed to save user: %v", err)
	}

	sessionID, err := s.sessions.CreateSession(ctx, &auth.Session{
		UserID:          user.ID,
		RefreshToken:    tokens.RefreshToken,
		AccessToken:     tokens.AccessToken,
		AccessExpiresAt: tokens.ExpiresAt,
		ExpiresAt:       tokens.RefreshExpiresAt,
	})
	if err != nil {
		return "", nil, auth.NewInternalError("failed to create session: %v", err)
	}
	return sessionID, toResponse(user, tokens.RefreshExpiresAt, s.companies(ctx, user.ID)), nil
}

// GetCurrentUser returns the session's user.
// touchEvery throttles last_seen_at writes: idle tracking costs at most one
// UPDATE per session per 15 minutes, not one per request.
const touchEvery = 15 * time.Minute

// Authenticate resolves a session to its state without touching the users
// table: the per-request path for middlewares. Enforces the idle timeout and
// refreshes the IAM tokens once the access token has expired.
func (s *service) Authenticate(ctx context.Context, sessionID string) (*auth.SessionState, error) {
	state, err := s.sessions.Lookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if now.Sub(state.LastSeenAt) > s.idleTimeout {
		s.Logout(ctx, sessionID) // best effort: also revokes at IAM
		return nil, auth.ErrUnauthorized
	}
	if now.Before(state.AccessExpiresAt) {
		if now.Sub(state.LastSeenAt) > touchEvery {
			if err := s.sessions.Touch(ctx, sessionID); err != nil {
				return nil, err
			}
			state.LastSeenAt = now
		}
		return state, nil
	}
	sess, err := s.refresh(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	state.AccessExpiresAt, state.ExpiresAt, state.LastSeenAt = sess.AccessExpiresAt, sess.ExpiresAt, now
	return state, nil
}

func (s *service) GetCurrentUser(ctx context.Context, sessionID string) (*auth.SessionResponse, error) {
	state, err := s.Authenticate(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetUserByID(ctx, state.UserID)
	if err != nil {
		return nil, err
	}
	return toResponse(user, state.ExpiresAt, s.companies(ctx, user.ID)), nil
}

// AccessToken is rare (admin proxies only), so it reads the encrypted tokens
// from Postgres every time; they are never cached.
func (s *service) AccessToken(ctx context.Context, sessionID string) (*auth.AccessTokenResponse, error) {
	if _, err := s.Authenticate(ctx, sessionID); err != nil {
		return nil, err
	}
	sess, err := s.sessions.LockSession(ctx, sessionID, func(*auth.Session) (*auth.Session, error) { return nil, nil })
	if err != nil {
		return nil, err
	}
	return &auth.AccessTokenResponse{AccessToken: sess.AccessToken, ExpiresAt: sess.AccessExpiresAt}, nil
}

// refresh rotates the refresh token under the session lock — that is where
// IAM re-checks the account, so a suspended user loses access within one
// access-token TTL.
func (s *service) refresh(ctx context.Context, sessionID string) (*auth.Session, error) {
	var rotated *auth.IAMTokens
	sess, err := s.sessions.LockSession(ctx, sessionID, func(sess *auth.Session) (*auth.Session, error) {
		if time.Now().Before(sess.AccessExpiresAt) {
			return nil, nil // another request refreshed while we waited
		}
		tokens, err := s.iam.Refresh(ctx, sess.RefreshToken)
		if err != nil {
			return nil, err
		}
		rotated = tokens
		if _, err := s.iam.Verify(ctx, tokens.AccessToken); err != nil {
			return nil, err
		}
		return &auth.Session{
			ID:              sess.ID,
			UserID:          sess.UserID,
			RefreshToken:    tokens.RefreshToken,
			AccessToken:     tokens.AccessToken,
			AccessExpiresAt: tokens.ExpiresAt,
			ExpiresAt:       tokens.RefreshExpiresAt,
		}, nil
	})
	switch {
	case auth.IsUnauthorized(err):
		// Session gone or IAM refused it (expired, reused, suspended): drop ours too.
		s.sessions.DeleteSession(ctx, sessionID)
	case err != nil && rotated != nil:
		// IAM rotated but saving failed: the stored refresh token is now used,
		// and presenting it later would make IAM revoke as theft. End the
		// session cleanly with the new token instead; the user signs in again.
		s.sessions.DeleteSession(context.WithoutCancel(ctx), sessionID)
		s.iam.Logout(context.WithoutCancel(ctx), rotated.RefreshToken)
	}
	return sess, err
}

// Logout deletes the session and revokes it at IAM. Idempotent.
func (s *service) Logout(ctx context.Context, sessionID string) error {
	refreshToken, err := s.sessions.DeleteSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if refreshToken == "" {
		return nil
	}
	return s.iam.Logout(ctx, refreshToken)
}

// companies is best effort: a failure here shouldn't sign anyone out.
func (s *service) companies(ctx context.Context, userID string) []auth.CompanyMembership {
	list, err := s.repo.UserCompanies(ctx, userID)
	if err != nil || list == nil {
		return []auth.CompanyMembership{}
	}
	return list
}

func toResponse(user *auth.User, expiresAt time.Time, companies []auth.CompanyMembership) *auth.SessionResponse {
	return &auth.SessionResponse{
		User: *user,
		Authorization: auth.Authorization{
			Roles:     []string{"candidate"}, // ponytail: fixed role until findJobs has its own roles table (IAM doesn't own authorization)
			Companies: companies,
		},
		ExpiresAt: expiresAt,
	}
}
