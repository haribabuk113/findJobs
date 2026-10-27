package profiles

import (
	"context"
	"time"

	"findJobs/internal/domain/profiles"
	"findJobs/internal/domain/reference"
)

// TermResolver turns typed names into taxonomy ids (the reference service).
type TermResolver interface {
	ResolveTerms(ctx context.Context, kind reference.Kind, names []string, userID string) ([]reference.Term, error)
}

// MembershipRequester asks a company to verify employment (the company service).
type MembershipRequester interface {
	RequestMembership(ctx context.Context, userID, companyID, title string, startedOn *time.Time) error
}

// ProfileService is the candidate profile use case.
type ProfileService interface {
	// Mine returns the user's profile, or an unsaved starter built from their account.
	Mine(ctx context.Context, userID string) (*profiles.Profile, error)
	// Save validates and stores the whole profile document.
	Save(ctx context.Context, userID string, in *profiles.Profile) (*profiles.Profile, error)
	// Public returns what visitors see at /profiles/{slug}.
	Public(ctx context.Context, slug string) (*profiles.Profile, error)
	// SetPhoto stores a new profile photo (JPEG/PNG/WebP, checked by its bytes).
	SetPhoto(ctx context.Context, userID, contentType string, data []byte) (*profiles.Profile, error)
	// RequestVerification asks the linked company to verify experience index.
	RequestVerification(ctx context.Context, userID string, index int) (*profiles.Profile, error)
	// Skills returns the user's skill set, without the list when signature
	// already matches (the lightweight cross-device sync).
	Skills(ctx context.Context, userID, signature string) (*profiles.SkillSet, error)
	// AddSkill and RemoveSkill are the job page's one-click chips; both return the new set.
	AddSkill(ctx context.Context, userID string, skillID int64) (*profiles.SkillSet, error)
	RemoveSkill(ctx context.Context, userID string, skillID int64) (*profiles.SkillSet, error)
}
