package profiles

import (
	"context"
	"fmt"
	"mime/multipart"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
	utilProfiles "findJobs/internal/utils/profiles"
)

type service struct {
	repo ports.ProfileRepository
	// storage ports.CDNProfilePhoto
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

func (s *service) GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error) {
	return s.repo.GetCandidateProfile(ctx, userID, userSlug)
}

func (s *service) UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
	return s.repo.UpdateCandidateProfile(ctx, request)
}

// GetProfilePhoto gets a user profile photo url
func (s *service) GetProfilePhoto(ctx context.Context, userID int64) (profiles.ProfilePhotoResponse, error) {
	return s.repo.GetProfilePhoto(ctx, userID)
}

// CreateProfilePhoto creates a user profile photo
func (s *service) CreateProfilePhoto(ctx context.Context, key string, file multipart.File, contentType string, userID int64) (string, error) {
	// url, err := s.storage.Upload(ctx, key, file, contentType)
	// if err != nil {
	// 	err := fmt.Errorf("Upload failed")
	// 	return "", err
	// }
	//
	url := key // temporarily
	// 5. Save the URL on the candidate record
	if err := s.repo.SetProfilePhotoURL(ctx, userID, url); err != nil {
		err := fmt.Errorf("Could not save profile photo")
		return "", err
	}

	return url, nil
}

func (s *service) GetProfileSlug(ctx context.Context, userID int64) (string, error) {
	return s.repo.GetProfileSlug(ctx, userID)
}

func (s *service) CreateProfileSlug(ctx context.Context, userID int64) (string, error) {

	profile, err := s.repo.GetProfileDetails(ctx, userID)
	if err != nil {
		err := fmt.Errorf("Could not fetch profile details")
		return "", err
	}
	baseSlug := utilProfiles.BaseSlug(profile.FirstName + " " + profile.LastName)

	slug, err := s.repo.UpdateProfileSlug(ctx, baseSlug, userID)
	if err != nil {
		err := fmt.Errorf("Could not save profile slug")
		return "", err
	}
	return slug, nil
}
