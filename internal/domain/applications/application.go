// Package applications is applying to jobs and the recruiter's applicant
// pipeline (manual: "Apply for jobs", "View applicants").
package applications

import (
	"slices"
	"time"
)

// Stages. Moving an applicant out of "new" is a recruiter action (manual:
// "Shortlisted, Contacted, Rejected").
const (
	StageNew         = "new"
	StageShortlisted = "shortlisted"
	StageContacted   = "contacted"
	StageRejected    = "rejected"
)

var Stages = []string{StageNew, StageShortlisted, StageContacted, StageRejected}

// Statuses. Cancelled applications (their company was deleted) leave the
// recruiter's applicant list; the candidate still sees them, marked.
const (
	StatusOpen      = "open"
	StatusCancelled = "cancelled"
)

const (
	TypePlatform = "platform" // Easy Apply
	TypeExternal = "external" // the candidate confirmed they applied on the employer's site
)

// HighFitPct is the match at which an applicant counts as high-fit.
const HighFitPct = 70

// Dossier is what the recruiter receives: a snapshot of the profile at apply
// time with the privacy rules applied. Manual exclusions (work preferences,
// target locations, social links, employer and designation, education) are
// never copied in.
type Dossier struct {
	Name            string   `json:"name"`
	ProfileSlug     string   `json:"profile_slug"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	Location        string   `json:"location"`
	ExperienceYears int      `json:"experience_years"`
	Notice          string   `json:"notice"`
	SalaryCurrency  string   `json:"salary_currency"`
	ExpectedSalary  float64  `json:"expected_salary"`
	CurrentSalary   *float64 `json:"current_salary"` // nil when the candidate keeps it private
	Skills          []string `json:"skills"`
}

// Application is one of the candidate's own applications.
type Application struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	JobTitle    string    `json:"job_title"`
	CompanyName string    `json:"company_name"`
	JobStatus   string    `json:"job_status"` // active | closed | expired
	ApplyType   string    `json:"apply_type"`
	Stage       string    `json:"stage"`
	Status      string    `json:"status"` // open | cancelled
	MatchPct    *int      `json:"match_pct"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// Applicant is an Easy Apply application as its recruiter sees it.
type Applicant struct {
	ID              string     `json:"id"`
	Stage           string     `json:"stage"`
	MatchPct        int        `json:"match_pct"`
	MatchedSkills   []string   `json:"matched_skills"`
	UnmatchedSkills []string   `json:"unmatched_skills"`
	SubmittedAt     time.Time  `json:"submitted_at"`
	StageUpdatedAt  *time.Time `json:"stage_updated_at"`
	Dossier         Dossier    `json:"dossier"`
}

// ApplicantFilter narrows a job's applicant list.
type ApplicantFilter struct {
	Stage    string // "" for all
	MinMatch int
	Sort     string // "recent" (default) or "match"
}

// Quota is today's free Easy Apply allowance.
type Quota struct {
	Limit     int `json:"limit"`
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

func NewQuota(limit, used int) Quota {
	return Quota{Limit: limit, Used: used, Remaining: max(0, limit-used)}
}

// Normalize validates a filter and fills defaults.
func (f *ApplicantFilter) Normalize() error {
	if f.Stage != "" && !slices.Contains(Stages, f.Stage) {
		return &ValidationError{Field: "stage", Message: "unknown stage"}
	}
	if f.MinMatch < 0 || f.MinMatch > 100 {
		return &ValidationError{Field: "min_match", Message: "min_match must be 0 to 100"}
	}
	switch f.Sort {
	case "":
		f.Sort = "recent"
	case "recent", "match":
	default:
		return &ValidationError{Field: "sort", Message: "sort must be recent or match"}
	}
	return nil
}

// MatchScore is the manual's formula: the share of the job's required skills
// the candidate has, rounded to a whole percent, plus which ones matched.
// The server computes it from stored ids and ignores any client value (D1).
func MatchScore(jobSkills, userSkills []int64) (pct int, matched, unmatched []int64) {
	matched, unmatched = []int64{}, []int64{}
	if len(jobSkills) == 0 {
		return 0, matched, unmatched
	}
	for _, id := range jobSkills {
		if slices.Contains(userSkills, id) {
			matched = append(matched, id)
		} else {
			unmatched = append(unmatched, id)
		}
	}
	return (200*len(matched) + len(jobSkills)) / (2 * len(jobSkills)), matched, unmatched
}

var ist = time.FixedZone("IST", 5*60*60+30*60)

// QuotaDay is the IST calendar day the free allowance resets on.
func QuotaDay(t time.Time) time.Time {
	y, m, d := t.In(ist).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// PaymentRequiredError: today's free applications are used up.
type PaymentRequiredError struct{ Message string }

func (e *PaymentRequiredError) Error() string { return e.Message }

var (
	ErrJobNotFound     = &NotFoundError{Message: "this job isn't open for applications"}
	ErrApplicationGone = &NotFoundError{Message: "application not found"}
	ErrNoProfile       = &ConflictError{Message: "save your profile before applying"}
	ErrAlreadyApplied  = &ConflictError{Message: "you've already applied to this job"}
	ErrOwnJob          = &ValidationError{Field: "job_id", Message: "you can't apply to your own job"}
	ErrNotEasyApply    = &ValidationError{Field: "job_id", Message: "this job takes applications on the employer's site"}
	ErrNotExternal     = &ValidationError{Field: "job_id", Message: "this job takes applications here; use Easy Apply"}
	ErrQuotaExhausted  = &PaymentRequiredError{Message: "you've used today's free applications; paid applications arrive with the wallet"}
)

func IsValidation(err error) bool      { _, ok := err.(*ValidationError); return ok }
func IsNotFound(err error) bool        { _, ok := err.(*NotFoundError); return ok }
func IsConflict(err error) bool        { _, ok := err.(*ConflictError); return ok }
func IsPaymentRequired(err error) bool { _, ok := err.(*PaymentRequiredError); return ok }
