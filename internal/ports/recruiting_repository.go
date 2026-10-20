package ports

import (
	"context"

	"findJobs/internal/domain/jobs"
)

// RecruitingRepository defines the outbound port for posting and managing jobs.
// Methods acting on a job take the acting user and only touch jobs they posted.
type RecruitingRepository interface {
	// PosterCompanies lists companies the user is a current member of
	PosterCompanies(ctx context.Context, userID string) ([]jobs.PosterCompany, error)
	// MemberCompanyID returns the id of a company the user currently belongs to
	// (NotFoundError if no such company, ForbiddenError if not a member)
	MemberCompanyID(ctx context.Context, slug, userID string) (string, error)
	// MissingLocations returns the given ids that aren't in the location tree
	MissingLocations(ctx context.Context, ids []string) ([]string, error)
	// CurrencyKnown reports whether an exchange rate exists for code
	CurrencyKnown(ctx context.Context, code string) (bool, error)

	// Create stores a new job
	Create(ctx context.Context, rec *jobs.Record) error
	// Editable returns the tier and company of a job the user posted
	Editable(ctx context.Context, id, userID string) (tier string, companySlug *string, companyName string, err error)
	// Update rewrites a job the user posted (tier and company never change)
	Update(ctx context.Context, rec *jobs.Record) error
	// Close hides a job; Repost reopens it with a fresh 30-day window
	Close(ctx context.Context, id, userID string) error
	Repost(ctx context.Context, id, userID string) error
	// Mine lists the user's jobs from the last 60 days, optionally by state
	Mine(ctx context.Context, userID, state string) ([]jobs.Posted, error)

	// Detail loads a job by id or share hash with viewer-specific fields;
	// visibility rules are applied by the caller
	Detail(ctx context.Context, key string, byShareHash bool, viewerID string) (*jobs.Detail, error)
	// RecordView and RecordOutbound bump the job's counters
	RecordView(ctx context.Context, id string) error
	RecordOutbound(ctx context.Context, id string) error
	// Recruiter returns a public recruiter card and their open-job count
	Recruiter(ctx context.Context, userID string) (*jobs.Person, int, error)

	// Drafts (owner only)
	Drafts(ctx context.Context, userID string) ([]jobs.Draft, error)
	CreateDraft(ctx context.Context, userID string, form map[string]any) (*jobs.Draft, error)
	UpdateDraft(ctx context.Context, id, userID string, form map[string]any) (*jobs.Draft, error)
	DeleteDraft(ctx context.Context, id, userID string) error
}
