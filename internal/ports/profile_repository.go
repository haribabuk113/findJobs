package ports

import (
	"context"
	models "findJobs/internal/domain/profiles"
)

// ProfileRepository defines the outbound port for profile persistence.
// The application depends on this interface, not on any concrete implementation.
type ProfileRepository interface {
	// 	GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	// 	UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error

	// 	GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error)
	// 	UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)

	// 	GetProfileDetails(ctx context.Context, userID int64) (profiles.Profile, error)

	// 	GetProfilePhoto(ctx context.Context, userID int64) (profiles.ProfilePhotoResponse, error)
	// 	SetProfilePhotoURL(ctx context.Context, userID int64, url string) error

	// 	GetProfileSlug(ctx context.Context, userID int64) (string, error)
	// 	UpdateProfileSlug(ctx context.Context, slug string, userID int64) (string, error)
	//

	GetRecordBySlug(ctx context.Context, slug string) (*models.ProfileRecord, error)
	Update(ctx context.Context, candidateID int64, patch models.CandidatePatch) error
	SoftDelete(ctx context.Context, candidateID int64) error
	ListPronouns(ctx context.Context, candidateID int64) ([]string, error)
	ReplacePronouns(ctx context.Context, candidateID int64, pronouns []string) error

	ListSocial(ctx context.Context, candidateID int64) ([]models.SocialHandle, error)
	CreateSocial(ctx context.Context, h models.SocialHandle) (int64, error)
	UpdateSocial(ctx context.Context, h models.SocialHandle) error
	DeleteSocial(ctx context.Context, id, candidateID int64) error
	IncrementClickSocial(ctx context.Context, slug, platform string) error

	GetSalary(ctx context.Context, candidateID int64) (*models.Salary, error)
	UpsertSalary(ctx context.Context, s models.Salary) error
	DeleteSalary(ctx context.Context, candidateID int64) error

	GetPreference(ctx context.Context, id int64) (*models.Preferences, error)
	UpsertPreference(ctx context.Context, id int64, openToRemote bool, workTypes []string) error
	ReplaceTargetRoles(ctx context.Context, id int64, ids []int64) error
	ReplaceTargetLocations(ctx context.Context, id int64, ids []int64) error

	ListTimeline(ctx context.Context, candidateID int64) ([]models.TimelineEntry, error)
	CreateTimeline(ctx context.Context, t models.Timeline) (int64, error)
	UpdateTimeline(ctx context.Context, t models.Timeline) error
	DeleteTimeline(ctx context.Context, id, candidateID int64) error
	RequestTimelineVerification(ctx context.Context, id, candidateID int64) error
	ReviewTimeline(ctx context.Context, id int64, approve bool, adminUserID int64) error

	ListSkillGroups(ctx context.Context, candidateID int64) ([]models.SkillGroup, error)
	CreateSkillGroup(ctx context.Context, candidateID int64, name string, order int) (int64, error)
	UpdateSkillGroup(ctx context.Context, groupID, candidateID int64, name string, order int) error
	DeleteSkillGroup(ctx context.Context, groupID, candidateID int64) error
	OwnsGroup(ctx context.Context, groupID, candidateID int64) (bool, error)
	AddSkill(ctx context.Context, groupID, skillID int64, order int) (int64, error)
	UpdateSkill(ctx context.Context, groupID, skillID, candidateID int64, order int) error
	RemoveSkill(ctx context.Context, groupID, skillID, candidateID int64) error

	GetContact(ctx context.Context, candidateID int64) (*models.ContactS, error)
	UpsertContact(ctx context.Context, c models.ContactS) error

	GetResume(ctx context.Context, candidateID int64) (*models.ResumeS, error)
	UpsertResume(ctx context.Context, x models.ResumeS) error
	DeleteResume(ctx context.Context, candidateID int64) error
	ListResumeGrants(ctx context.Context, candidateID int64) ([]models.ResumeGrant, error)
	RevokeResumeGrant(ctx context.Context, id, candidateID int64) error
	CreateResumeGrant(ctx context.Context, g models.ResumeGrant) error
}
