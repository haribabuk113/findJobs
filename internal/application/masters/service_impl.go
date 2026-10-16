package masters

import "findJobs/internal/ports"

type service struct {
	repo ports.MasterRepository
	// storage ports.CDNProfilePhoto
}

// NewProfileService creates a new profile service using dependency injection
func NewMasterService(repo ports.MasterRepository) MasterService {
	return &service{repo: repo}
}
