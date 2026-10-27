package applications

import "time"

// MaxSavedJobs caps a candidate's saved list (job search takes at most 200 ids).
const MaxSavedJobs = 200

// SavedJob is one saved job, newest first. Applied marks jobs the candidate
// has since applied to (manual: "Clear Applied Jobs").
type SavedJob struct {
	JobID   string    `json:"job_id"`
	SavedAt time.Time `json:"saved_at"`
	Applied bool      `json:"applied"`
}

var (
	ErrSavedJobUnknown = &NotFoundError{Message: "job not found"}
	ErrTooManySaved    = &ValidationError{Field: "job_id", Message: "you can save up to 200 jobs; remove some first"}
)
