package ports

import (
	"context"

	"findJobs/internal/domain/auth"
)

// AuthRepository defines the outbound port for user persistence.
// The application depends on this interface, not on any concrete implementation.
type AuthRepository interface {
	// UpsertUser creates the user for an IAM ecosystem ID, or refreshes its
	// email/status, and returns the stored row.
	UpsertUser(ctx context.Context, user *auth.User) (*auth.User, error)
	// GetUserByID retrieves a user by findJobs user ID
	GetUserByID(ctx context.Context, userID string) (*auth.User, error)
	// UserCompanies lists live companies the user is a current member of
	UserCompanies(ctx context.Context, userID string) ([]auth.CompanyMembership, error)
}

// SessionStore defines the outbound port for login sessions.
type SessionStore interface {
	// CreateSession stores a new session and returns its ID
	CreateSession(ctx context.Context, session *auth.Session) (string, error)
	// Lookup returns an unexpired session's state (no tokens), or ErrUnauthorized.
	Lookup(ctx context.Context, sessionID string) (*auth.SessionState, error)
	// LockSession loads an unexpired session under a per-session lock and
	// calls fn. If fn returns a non-nil session, it is saved. Returns
	// ErrUnauthorized if the session doesn't exist or has expired.
	LockSession(ctx context.Context, sessionID string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error)
	// Touch records activity now, for the idle timeout
	Touch(ctx context.Context, sessionID string) error
	// DeleteSession deletes a session and returns its IAM refresh token
	// ("" if it didn't exist)
	DeleteSession(ctx context.Context, sessionID string) (string, error)
}

// IAMClient defines the outbound port to the central IAM service.
type IAMClient interface {
	// ExchangeCode redeems the one-time login code at IAM /token
	ExchangeCode(ctx context.Context, code string) (*auth.IAMTokens, error)
	// Refresh rotates the refresh token at IAM /refresh
	Refresh(ctx context.Context, refreshToken string) (*auth.IAMTokens, error)
	// Logout revokes the IAM session at IAM /logout
	Logout(ctx context.Context, refreshToken string) error
	// Verify checks an access token against IAM's JWKS
	Verify(ctx context.Context, accessToken string) (*auth.IAMClaims, error)
}
