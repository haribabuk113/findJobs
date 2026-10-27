package ports

import (
	"context"
	"time"

	"findJobs/internal/domain/applications"
)

// ApplicationRepository persists applications and the daily free quota.
type ApplicationRepository interface {
	// Apply records an Easy Apply application in one transaction: the job must
	// be open and Easy Apply, the candidate needs a saved profile, the match
	// and dossier are computed from stored data, and one free application is
	// taken from day's quota (ErrQuotaExhausted when limit is reached).
	Apply(ctx context.Context, userID, jobID string, day time.Time, limit int) (*applications.Application, error)
	// ApplyExternal records "Yes, I applied" for an open External job (no quota).
	ApplyExternal(ctx context.Context, userID, jobID string) (*applications.Application, error)
	// Mine lists the candidate's applications, newest first.
	Mine(ctx context.Context, userID string) ([]applications.Application, error)
	// QuotaUsed is how many free applications the candidate used on day.
	QuotaUsed(ctx context.Context, userID string, day time.Time) (int, error)
	// Applicants lists a job's Easy Apply applicants for its recruiter or a
	// current admin of its company; anyone else gets ErrJobNotFound.
	Applicants(ctx context.Context, jobID, viewerID string, f applications.ApplicantFilter) ([]applications.Applicant, error)
	// SetStage moves an applicant, counting a change as a recruiter action.
	SetStage(ctx context.Context, applicationID, viewerID, stage string) (*applications.Applicant, error)

	// SavedJobs lists the user's saved jobs, newest first.
	SavedJobs(ctx context.Context, userID string) ([]applications.SavedJob, error)
	// SaveJob saves a job (already saved is a no-op): ErrSavedJobUnknown for
	// a missing job, ErrTooManySaved past MaxSavedJobs.
	SaveJob(ctx context.Context, userID, jobID string) error
	// UnsaveJobs removes the given saves (missing ones are ignored).
	UnsaveJobs(ctx context.Context, userID string, jobIDs []string) error
	// ClearApplied removes saves for jobs the user has applied to.
	ClearApplied(ctx context.Context, userID string) error
}
