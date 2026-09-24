package profiles

type ProfileService interface {
}

type profileService struct {
	repo ProfileRepository
}

func NewProfileService(repo ProfileRepository) ProfileService {
	return &profileService{repo: repo}
}
