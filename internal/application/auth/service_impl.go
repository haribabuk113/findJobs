package auth

import (
	"context"
	"time"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

type service struct {
	repo ports.AuthRepository
	iam  ports.IAMClient
}

// NewAuthService creates a new auth service using dependency injection
func NewAuthService(repo ports.AuthRepository, iam ports.IAMClient) AuthService {
	return &service{repo: repo, iam: iam}
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

	sessionID, err := s.repo.CreateSession(ctx, &auth.Session{
		UserID:          user.ID,
		RefreshToken:    tokens.RefreshToken,
		AccessToken:     tokens.AccessToken,
		AccessExpiresAt: tokens.ExpiresAt,
		ExpiresAt:       tokens.RefreshExpiresAt,
	})
	if err != nil {
		return "", nil, auth.NewInternalError("failed to create session: %v", err)
	}
	return sessionID, toResponse(user, tokens.RefreshExpiresAt), nil
}

// GetCurrentUser returns the session's user.
func (s *service) GetCurrentUser(ctx context.Context, sessionID string) (*auth.SessionResponse, error) {
	sess, err := s.session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return toResponse(user, sess.ExpiresAt), nil
}

func (s *service) AccessToken(ctx context.Context, sessionID string) (*auth.AccessTokenResponse, error) {
	sess, err := s.session(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return &auth.AccessTokenResponse{AccessToken: sess.AccessToken, ExpiresAt: sess.AccessExpiresAt}, nil
}

// session loads a live session. Once the IAM access token has expired it
// rotates the refresh token — that is where IAM re-checks the account, so a
// suspended user loses access within one access-token TTL.
func (s *service) session(ctx context.Context, sessionID string) (*auth.Session, error) {
	sess, err := s.repo.LockSession(ctx, sessionID, func(sess *auth.Session) (*auth.Session, error) {
		if time.Now().Before(sess.AccessExpiresAt) {
			return nil, nil
		}
		tokens, err := s.iam.Refresh(ctx, sess.RefreshToken)
		if err != nil {
			return nil, err
		}
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
	if auth.IsUnauthorized(err) {
		// Session gone or IAM refused it (expired, reused, suspended): drop ours too.
		s.repo.DeleteSession(ctx, sessionID)
		return nil, err
	}
	return sess, err
}

// Logout deletes the session and revokes it at IAM. Idempotent.
func (s *service) Logout(ctx context.Context, sessionID string) error {
	refreshToken, err := s.repo.DeleteSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if refreshToken == "" {
		return nil
	}
	return s.iam.Logout(ctx, refreshToken)
}

func toResponse(user *auth.User, expiresAt time.Time) *auth.SessionResponse {
	return &auth.SessionResponse{
		User: *user,
		Authorization: auth.Authorization{
			Roles:     []string{"candidate"}, // ponytail: fixed role until findJobs has its own roles table (IAM doesn't own authorization)
			Companies: []string{},
		},
		ExpiresAt: expiresAt,
	}
}
