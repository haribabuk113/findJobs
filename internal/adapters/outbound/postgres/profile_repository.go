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
