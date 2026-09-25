package profiles

import (
	"context"

	"findJobs/internal/domain/profiles"
)

// ProfileService defines the inbound port for profile business operations.
// This interface defines what the application layer can do.
type ProfileService interface {
	GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)
}