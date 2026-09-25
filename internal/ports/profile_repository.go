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
}