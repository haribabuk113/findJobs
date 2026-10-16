package ports

import (
	"context"

	"findJobs/internal/domain/profiles"
)

// ProfileRepository defines the outbound port for profile persistence.
// The application depends on this interface, not on any concrete implementation.
type ProfileRepository interface {
	GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error

	GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error)
	UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)

	GetProfileDetails(ctx context.Context, userID int64) (profiles.Profile, error)

	GetProfilePhoto(ctx context.Context, userID int64) (profiles.ProfilePhotoResponse, error)
	SetProfilePhotoURL(ctx context.Context, userID int64, url string) error

	GetProfileSlug(ctx context.Context, userID int64) (string, error)
	UpdateProfileSlug(ctx context.Context, slug string, userID int64) (string, error)
}
