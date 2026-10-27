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

// ProfileRepository persists candidate profiles.
type ProfileRepository interface {
	// Get returns the user's profile, or profiles.ErrNotFound.
	Get(ctx context.Context, userID string) (*profiles.Profile, error)
	// GetPublic returns the profile behind slug, or ErrNotFound when there is
	// none or it is private.
	GetPublic(ctx context.Context, slug string) (*profiles.Profile, error)
	// Account returns the name and email IAM gave the user.
	Account(ctx context.Context, userID string) (name, email string, err error)
	// Save creates or replaces the profile with resolved term ids. Unknown
	// currencies or locations are validation errors; a taken slug is ErrSlugTaken.
	Save(ctx context.Context, userID string, p *profiles.Profile, skills []ProfileSkill, designationIDs []int64) error
	// SetPhoto stores the photo key and returns the replaced one ("" if none).
	SetPhoto(ctx context.Context, userID, key string) (old string, err error)
	// Experience returns the registered company linked to the experience at
	// index (companyID "" when the company isn't on Referise).
	Experience(ctx context.Context, userID string, index int) (companyID, title, start string, err error)
	// Skills returns the user's skills in display order (empty without a profile).
	Skills(ctx context.Context, userID string) ([]profiles.SkillRef, error)
	// AddSkill appends a skill (its master when merged) to the last group;
	// adding one already there is a no-op. ErrNoProfile, ErrUnknownSkill and
	// ErrTooManySkills are the failures.
	AddSkill(ctx context.Context, userID string, skillID int64) error
	// RemoveSkill drops a skill; removing one that isn't there is a no-op.
	RemoveSkill(ctx context.Context, userID string, skillID int64) error
}
