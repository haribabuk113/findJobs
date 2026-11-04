package ports

import (
	"context"

	"findJobs/internal/domain/profiles"
)

// ProfileSkill is one resolved skill in display order.
type ProfileSkill struct {
	SkillID int64
	Group   string
}

// ExperienceLink is one stored experience as the verification flow needs it.
type ExperienceLink struct {
	CompanyID, Title, Start string
	Review                  string // profiles.Review*
}

// ResumeRef is a stored résumé its viewer may read.
type ResumeRef struct {
	OwnerID, Slug, Key string
}

// ProfileRepository persists candidate profiles.
type ProfileRepository interface {
	// Get returns the user's profile, or profiles.ErrNotFound.
	Get(ctx context.Context, userID string) (*profiles.Profile, error)
	// GetPublic returns the profile behind slug, or ErrNotFound when there is
	// none or it is private.
	GetPublic(ctx context.Context, slug string) (*profiles.Profile, error)
	// TakenSlugs returns the handles other users hold among base and base-*.
	TakenSlugs(ctx context.Context, base, userID string) (map[string]bool, error)
	// Account returns the name and email IAM gave the user.
	Account(ctx context.Context, userID string) (name, email string, err error)
	// Save creates or replaces the profile with resolved term ids. Unknown
	// currencies or locations are validation errors; a taken slug is ErrSlugTaken.
	Save(ctx context.Context, userID string, p *profiles.Profile, skills []ProfileSkill, designationIDs []int64) error
	// SetPhoto stores the photo key ("" removes the photo) and returns the
	// replaced one ("" if none).
	SetPhoto(ctx context.Context, userID, key string) (old string, err error)
	// SetResume stores the résumé key ("" removes it) and returns the replaced one.
	SetResume(ctx context.Context, userID, key string) (old string, err error)
	// Resume and ApplicantResume find the résumé behind a profile slug or an
	// application, when viewerID ("" signed out) may read it; ErrNotFound
	// otherwise, so a refusal looks the same as no résumé.
	Resume(ctx context.Context, slug, viewerID string) (ResumeRef, error)
	ApplicantResume(ctx context.Context, applicationID, viewerID string) (ResumeRef, error)
	// Experience returns the experience at index with its registered company
	// (CompanyID "" when the company isn't on Referise) and review state.
	Experience(ctx context.Context, userID string, index int) (ExperienceLink, error)
	// MarkPending puts the experience at index in its company's review queue.
	MarkPending(ctx context.Context, userID string, index int) error
	// Skills returns the user's skills in display order (empty without a profile).
	Skills(ctx context.Context, userID string) ([]profiles.SkillRef, error)
	// AddSkill appends a skill (its master when merged) to the last group;
	// adding one already there is a no-op. ErrNoProfile, ErrUnknownSkill and
	// ErrTooManySkills are the failures.
	AddSkill(ctx context.Context, userID string, skillID int64) error
	// RemoveSkill drops a skill; removing one that isn't there is a no-op.
	RemoveSkill(ctx context.Context, userID string, skillID int64) error
}
