package auth

import (
	"context"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

type service struct {
	repo ports.AuthRepository
}

// NewAuthService creates a new auth service using dependency injection
func NewAuthService(repo ports.AuthRepository) AuthService {
	return &service{repo: repo}
}

// CreateSession creates or resolves a session from the IAM JWT
func (s *service) CreateSession(ctx context.Context, request *auth.SessionRequest, iamUserID string) (*auth.SessionResponse, error) {
	// Validate the request
	if err := request.Validate(); err != nil {
		return nil, err
	}

	// Try to get existing user by IAM user ID
	user, err := s.repo.GetUserByIAMUserID(ctx, iamUserID)
	if err != nil {
		// If user not found, create a new user
		if auth.IsNotFound(err) {
			user = &auth.User{
				ID:        generateUserID(),
				IAMUserID: iamUserID,
				Email:     extractEmailFromIAM(iamUserID), // In reality, this would come from JWT claims
				Status:    "active",
			}
			if err := s.repo.CreateUser(ctx, user); err != nil {
				return nil, auth.NewInternalError("failed to create user: %v", err)
			}
		} else {
			return nil, auth.NewInternalError("failed to get user: %v", err)
		}
	}

	// Create session
	if err := s.repo.CreateSession(ctx, user.ID, user.IAMUserID); err != nil {
		return nil, auth.NewInternalError("failed to create session: %v", err)
	}

	// Build response
	response := &auth.SessionResponse{
		User: auth.User{
			ID:        user.ID,
			IAMUserID: user.IAMUserID,
			Email:     user.Email,
			Status:    user.Status,
		},
		Authorization: auth.Authorization{
			Roles:     []string{"candidate"}, // Default role, could be fetched from IAM
			Companies: []string{},
		},
	}

	return response, nil
}

// GetCurrentUser returns the current authenticated user
func (s *service) GetCurrentUser(ctx context.Context, userID string) (*auth.SessionResponse, error) {
	// In a real implementation, we'd fetch the user from the repository
	// For now, return a stub response
	return &auth.SessionResponse{
		User: auth.User{
			ID:        userID,
			IAMUserID: "iam_" + userID,
			Email:     "user@example.com",
			Status:    "active",
		},
		Authorization: auth.Authorization{
			Roles:     []string{"candidate"},
			Companies: []string{},
		},
	}, nil
}

// Logout logs the user out
func (s *service) Logout(ctx context.Context, userID string) error {
	return s.repo.DeleteSession(ctx, userID)
}

// generateUserID generates a simple user ID (in production, use UUID)
func generateUserID() string {
	return "usr_" + "123" // Placeholder
}

// extractEmailFromIAM extracts email from IAM user ID (placeholder)
func extractEmailFromIAM(iamUserID string) string {
	return "user@example.com" // Placeholder - in reality, parse from JWT claims
}