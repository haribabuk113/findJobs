package profiles

import (
	"context"
	"findJobs/internal/domain/profiles"
	"mime/multipart"
)

// ProfileService defines the inbound port for profile business operations.
// This interface defines what the application layer can do.
type ProfileService interface {
	GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)

	GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error)
	UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)

	GetProfilePhoto(ctx context.Context, userID int64) (profiles.ProfilePhotoResponse, error)
	CreateProfilePhoto(ctx context.Context, key string, file multipart.File, contentType string, userID int64) (string, error)
	// UpdateProfilePhoto(ctx context.Context, request *profiles.ProfileRequest) (string, error)

	GetProfileSlug(ctx context.Context, userID int64) (string, error)
	CreateProfileSlug(ctx context.Context, userID int64) (string, error)
}
