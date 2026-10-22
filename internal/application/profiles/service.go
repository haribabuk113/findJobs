package profiles

import (
	"context"
	models "findJobs/internal/domain/profiles"
)

// ProfileService defines the inbound port for profile business operations.
// This interface defines what the application layer can do.
type ProfileService interface {

	GetBySlug(ctx context.Context, slug string) (*models.ProfileResponse, error)
	Update(ctx context.Context, candidateID int64, req models.UpdateProfileRequest) error
	Delete(ctx context.Context, candidateID int64) error
	GetPronouns(ctx context.Context, candidateID int64) (models.PronounsResponse, error)
	ReplacePronouns(ctx context.Context, candidateID int64, req models.PronounsRequest) error

	ListSocial(ctx context.Context, candidateID int64) ([]models.SocialHandleResponse, error)
	CreateSocial(ctx context.Context, candidateID int64, req models.SocialHandleRequest) (int64, error)
	UpdateSocial(ctx context.Context, candidateID, id int64, req models.SocialHandleRequest) error
	DeleteSocial(ctx context.Context, candidateID, id int64) error
	TrackClickSocial(ctx context.Context, slug, platform string) error

	GetSalary(ctx context.Context, candidateID int64) (*models.SalaryResponse, error)
	PutSalary(ctx context.Context, candidateID int64, req models.SalaryRequest) error
	DeleteSalary(ctx context.Context, candidateID int64) error

	GetPreference(ctx context.Context, candidateID int64) (*models.PreferencesS, error)
	PutPreference(ctx context.Context, candidateID int64, req models.PreferencesRequest) error
	PutTargetRoles(ctx context.Context, candidateID int64, req models.TargetRolesRequest) error
	PutTargetLocations(ctx context.Context, candidateID int64, req models.TargetLocationsRequest) error

	ListTimeline(ctx context.Context, candidateID int64) ([]models.TimelineItem, error)
	CreateTimeline(ctx context.Context, candidateID int64, req models.TimelineRequest) (int64, error)
	UpdateTimeline(ctx context.Context, candidateID, id int64, req models.TimelineRequest) error
	DeleteTimeline(ctx context.Context, candidateID, id int64) error
	RequestTimelineVerification(ctx context.Context, candidateID, id int64) error
	ReviewTimeline(ctx context.Context, id int64, approve bool, adminUserID int64) error

	ListSkillGroup(ctx context.Context, candidateID int64) ([]models.SkillGroup, error)
	CreateSkillGroup(ctx context.Context, candidateID int64, req models.SkillGroupRequest) (int64, error)
	UpdateSkillGroup(ctx context.Context, candidateID, groupID int64, req models.SkillGroupRequest) error
	DeleteSkillGroup(ctx context.Context, candidateID, groupID int64) error
	AddSkill(ctx context.Context, candidateID, groupID int64, req models.AddSkillRequest) (*models.AddSkillResponse, error)
	UpdateSkill(ctx context.Context, candidateID, groupID, skillID int64, req models.UpdateSkillItemRequest) error
	RemoveSkill(ctx context.Context, candidateID, groupID, skillID int64) error

	GetContact(ctx context.Context, candidateID int64) (*models.ContactResponse, error)
	PutContact(ctx context.Context, candidateID int64, req models.ContactRequest) error

	GetResume(ctx context.Context, candidateID int64) (*models.ResumeResponse, error)
	PutResume(ctx context.Context, candidateID int64, req models.ResumeRequest) error
	DeleteResume(ctx context.Context, candidateID int64) error
	ListResumeGrants(ctx context.Context, candidateID int64) ([]models.GrantResponse, error)
	RevokeResumeGrant(ctx context.Context, candidateID, id int64) error
	CreateResumeGrant(ctx context.Context, req models.CreateGrantRequest) error
}
