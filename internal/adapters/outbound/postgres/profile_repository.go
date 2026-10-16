package postgres

import (
	"context"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileRepository is the outbound adapter implementing the application port.
type ProfileRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository creates the PostgreSQL adapter for profile persistence.
func NewProfileRepository(pool *pgxpool.Pool) ports.ProfileRepository {
	return &ProfileRepository{pool: pool}
}

func (r *ProfileRepository) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
	// TODO: replace stub with real query against the database.
	return []profiles.ProfileDetails{}, nil
}

func (r *ProfileRepository) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error {
	// TODO: replace stub with real persistence against the database.
	return nil
}

func (r *ProfileRepository) GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error) {
	var profileDetails profiles.ProfileDetails

	query := `SELECT id, iam_user_id, email, status FROM candidates WHERE iam_user_id = $1`
	var value any
	if userID > 0 {
		query += ` WHERE candidate_id = $1`
		value = userID
	} else if userSlug != "" {
		query += ` WHERE profile_slug = $1`
		value = userSlug
	}
	row := r.pool.QueryRow(ctx, query, value)
	CandidateProfileDtoMapper(ctx, row, &profileDetails)
	return profileDetails, nil
}

func (r *ProfileRepository) UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
	return "", nil
}

func (r *ProfileRepository) GetProfileDetails(ctx context.Context, userID int64) (profiles.Profile, error) {
	// TODO: replace stub with real query against the database.
	return profiles.Profile{}, nil
}

func (r *ProfileRepository) GetProfilePhoto(ctx context.Context, suserID int64) (profiles.ProfilePhotoResponse, error) {
	// TODO: replace stub with real persistence against the database.
	return profiles.ProfilePhotoResponse{}, nil
}

func (r *ProfileRepository) SetProfilePhotoURL(ctx context.Context, userID int64, url string) error {
	// TODO: replace stub with real persistence against the database.
	return nil
}

func (r *ProfileRepository) GetProfileSlug(ctx context.Context, userID int64) (string, error) {
	// TODO: replace stub with real persistence against the database.
	return "", nil
}

func (r *ProfileRepository) UpdateProfileSlug(ctx context.Context, slug string, userID int64) (string, error) {
	// TODO: replace stub with real persistence against the database.
	return "", nil
}
