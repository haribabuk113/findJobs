package auth

import (
	"context"

	"findJobs/internal/domain/auth"
)

// AuthService defines the inbound port for auth business operations.
// This interface defines what the application layer can do.
type AuthService interface {
	// CreateSession redeems an IAM login code and opens a findJobs session.
	// Returns the session ID the handler sets as a cookie.
	CreateSession(ctx context.Context, request *auth.SessionRequest) (string, *auth.SessionResponse, error)
	// Authenticate resolves a session ID to its state (user and expiry) for
	// per-request checks, refreshing the IAM session when needed
	Authenticate(ctx context.Context, sessionID string) (*auth.SessionState, error)
	// GetCurrentUser resolves a session ID to its user, refreshing the IAM
	// session when its access token has expired
	GetCurrentUser(ctx context.Context, sessionID string) (*auth.SessionResponse, error)
	// AccessToken returns the session's current (refreshed if needed) IAM access token
	AccessToken(ctx context.Context, sessionID string) (*auth.AccessTokenResponse, error)
	// Logout ends the findJobs session and revokes the IAM session
	Logout(ctx context.Context, sessionID string) error
}
