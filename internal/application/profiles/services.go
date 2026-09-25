package profiles

import "context"

type ProfileService interface {
	GetProfile(ctx context.Context, userId int64) ([]ProfileDetails, error)
	UpdateProfile(ctx context.Context, request *ProfileRequest) (string, error)
}

type profileService struct {
	repo ProfileRepository
}

func NewProfileService(repo ProfileRepository) ProfileService {
	return &profileService{repo: repo}
}

func (s *profileService) GetProfile(ctx context.Context, userId int64) ([]ProfileDetails, error) {
	return []ProfileDetails{}, nil
}

func (s *profileService) UpdateProfile(ctx context.Context, request *ProfileRequest) (string, error) {
	return "", nil
}
