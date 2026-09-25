package profiles

import (
	"context"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

type service struct {
	repo ports.ProfileRepository
}

// NewProfileService creates a new profile service using dependency injection
func NewProfileService(repo ports.ProfileRepository) ProfileService {
	return &service{repo: repo}
}

// GetProfile retrieves a user profile by ID
func (s *service) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
	return s.repo.GetProfile(ctx, userID)
}

// UpdateProfile updates a user profile
func (s *service) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}

	if err := s.repo.UpdateProfile(ctx, request); err != nil {
		return "", err
	}

	return "Profile updated successfully", nil
}