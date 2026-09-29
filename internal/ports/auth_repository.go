package ports

import (
	"context"

	"findJobs/internal/domain/auth"
)

// AuthRepository defines the outbound port for auth persistence.
// The application depends on this interface, not on any concrete implementation.
type AuthRepository interface {
	// GetUserByIAMUserID retrieves a user by their IAM user ID
	GetUserByIAMUserID(ctx context.Context, iamUserID string) (*auth.User, error)
	// CreateUser creates a new user from IAM data
	CreateUser(ctx context.Context, user *auth.User) error
	// CreateSession creates a new session for a user
	CreateSession(ctx context.Context, userID string, iamUserID string) error
	// GetSession retrieves a session by user ID
	GetSession(ctx context.Context, userID string) (string, error)
	// DeleteSession deletes a session
	DeleteSession(ctx context.Context, userID string) error
}