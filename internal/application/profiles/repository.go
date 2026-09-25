package profiles

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProfileRepository interface {
	GetProfile(ctx context.Context, userId int64) (users []ProfileDetails, err error)
	UpdateProfile(ctx context.Context, request *ProfileRequest) error
}

type profileRepository struct {
	pool *pgxpool.Pool
}

func NewProfileRepo(db *pgxpool.Pool) ProfileRepository {
	return &profileRepository{pool: db}
}

func (r *profileRepository) GetProfile(ctx context.Context, userId int64) (users []ProfileDetails, err error) {
	return []ProfileDetails{}, nil
}

func (r *profileRepository) UpdateProfile(ctx context.Context, request *ProfileRequest) error {
	return nil
}
