package profiles

type ProfileController struct {
	profileService ProfileService
}

func NewProfileController(profileService ProfileService) *ProfileController {
	return &ProfileController{profileService: profileService}
}
