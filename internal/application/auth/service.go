package auth

import (
	"context"

	"findJobs/internal/domain/auth"
)

// AuthService defines the inbound port for auth business operations.
// This interface defines what the application layer can do.
type AuthService interface {
	// CreateSession creates or resolves a session from the IAM JWT
	CreateSession(ctx context.Context, request *auth.SessionRequest, iamUserID string) (*auth.SessionResponse, error)
	// GetCurrentUser returns the current authenticated user
	GetCurrentUser(ctx context.Context, userID string) (*auth.SessionResponse, error)
	// Logout logs the user out
	Logout(ctx context.Context, userID string) error
}