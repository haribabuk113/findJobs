package jobs

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Tiers as posted (manual "Post a job"): free jobs are unlisted (share link
// only), always Easy Apply, salary visible, never linked to a company page.
const (
	TierFree = "free"
	TierPaid = "paid"

	PayCompanyCredits = "company_credits"
	PayOutOfPocket    = "out_of_pocket"

	MaxJobSkills      = 15
	MaxJobLocations   = 200
	MaxInterviewSteps = 20
	MaxDescription    = 10000
)

// PostInput is the post-a-job form (create and edit).
type PostInput struct {
	Tier            string   `json:"tier"`
	CompanySlug     string   `json:"company_slug"`   // paid: an approved company of the poster
	CompanyName     string   `json:"company_name"`   // shown when there is no linked company
	PaymentMethod   string   `json:"payment_method"` // paid: company_credits | out_of_pocket
	Designation     string   `json:"designation"`    // job title; becomes a designation term
	Skills          []string `json:"skills"`         // skill names; become skill terms
	MinExperience   int      `json:"min_experience"`
	LocationIDs     []string `json:"location_ids"`
	WorldRemote     bool     `json:"worldwide_remote"`
	SalaryMax       float64  `json:"salary_max"`
	Currency        string   `json:"currency"`
	HideSalary      bool     `json:"hide_salary"`
	WorkModes       []string `json:"work_modes"`
	WorkTypes       []string `json:"work_types"`
	NoticePeriods   []string `json:"notice_periods"`
	Relocation      string   `json:"relocation"`
	InterviewStages []string `json:"interview_stages"`
	Application     string   `json:"application"` // Easy apply | External
	ExternalURL     string   `json:"external_url"`
	Description     string   `json:"description"`
}

var (
	currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
	geonameID    = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)
	htmlTag      = regexp.MustCompile(`<[a-zA-Z/!?]`)
)

// Normalize applies the tier rules and validates the form.
func (in *PostInput) Normalize() error {
	switch in.Tier {
	case TierFree:
		// Manual: free jobs are force-locked to Easy Apply with the salary shown,
		// and are never attached to a company page.
		in.CompanySlug, in.PaymentMethod = "", ""
		in.Application, in.ExternalURL, in.HideSalary = "Easy apply", "", false
	case TierPaid:
		if in.PaymentMethod != PayCompanyCredits && in.PaymentMethod != PayOutOfPocket {
			return invalidPost("payment_method", "choose company credits or out-of-pocket payment")
		}
		if in.PaymentMethod == PayCompanyCredits && in.CompanySlug == "" {
			return invalidPost("payment_method", "company credits need an approved company")
		}
	default:
		return invalidPost("tier", "tier must be free or paid")
	}
	if in.CompanySlug != "" {
		in.CompanyName = ""
		if !companySlug.MatchString(in.CompanySlug) {
			return invalidPost("company_slug", "unknown company")
		}
	}
	var err error
	if in.CompanyName, err = postText("company_name", in.CompanyName, 0, 120, false); err != nil {
		return err
	}
	if in.Designation, err = postText("designation", in.Designation, 2, 80, false); err != nil {
		return err
	}
	if len(in.Skills) < 1 || len(in.Skills) > MaxJobSkills {
		return invalidPost("skills", fmt.Sprintf("choose 1 to %d skills", MaxJobSkills))
	}
	if in.MinExperience < 0 || in.MinExperience > 99 {
		return invalidPost("min_experience", "experience must be a whole number from 0 to 99")
	}
	// Manual: locations are mandatory for every work mode, including remote.
	if len(in.LocationIDs) < 1 || len(in.LocationIDs) > MaxJobLocations {
		return invalidPost("location_ids", fmt.Sprintf("choose 1 to %d eligible locations", MaxJobLocations))
	}
	slices.Sort(in.LocationIDs)
	in.LocationIDs = slices.Compact(in.LocationIDs)
	for _, id := range in.LocationIDs {
		if !geonameID.MatchString(id) {
			return invalidPost("location_ids", "unknown location")
		}
	}
	// Manual: the exact maximum is required even when hidden; search matches on it.
	if !(in.SalaryMax > 0) || in.SalaryMax > 1e11 {
		return invalidPost("salary_max", "enter the maximum annual salary you offer")
	}
	if !currencyCode.MatchString(in.Currency) {
		return invalidPost("currency", "choose a currency")
	}
	if err := subsetOf("work_modes", in.WorkModes, WorkModes, true); err != nil {
		return err
	}
	if err := subsetOf("work_types", in.WorkTypes, WorkTypes, true); err != nil {
		return err
	}
	if err := subsetOf("notice_periods", in.NoticePeriods, NoticePeriods, false); err != nil {
		return err
	}
	if in.NoticePeriods == nil {
		in.NoticePeriods = []string{}
	}
	if !slices.Contains(RelocationOptions, in.Relocation) {
		return invalidPost("relocation", "choose a relocation option")
	}
	// Manual: rounds = the number of non-empty interview stages.
	stages := []string{}
	for _, raw := range in.InterviewStages {
		stage, err := postText("interview_stages", raw, 0, 100, false)
		if err != nil {
			return err
		}
		if stage != "" {
			stages = append(stages, stage)
		}
	}
	if len(stages) > MaxInterviewSteps {
		return invalidPost("interview_stages", fmt.Sprintf("at most %d interview stages", MaxInterviewSteps))
	}
	in.InterviewStages = stages
	switch in.Application {
	case "Easy apply":
		in.ExternalURL = ""
	case "External":
		u, err := url.Parse(strings.TrimSpace(in.ExternalURL))
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(in.ExternalURL) > 500 {
			return invalidPost("external_url", "external applications need a valid https:// link")
		}
		in.ExternalURL = u.String()
	default:
		return invalidPost("application", "application must be Easy apply or External")
	}
	if in.Description, err = postText("description", in.Description, 1, MaxDescription, true); err != nil {
		return err
	}
	return nil
}

// RoundCount is the derived interview-round count.
func (in *PostInput) RoundCount() int { return len(in.InterviewStages) }

func subsetOf(field string, got, allowed []string, required bool) error {
	if required && len(got) == 0 {
		return invalidPost(field, "choose at least one "+strings.ReplaceAll(field, "_", " "))
	}
	for _, v := range got {
		if !slices.Contains(allowed, v) {
			return invalidPost(field, fmt.Sprintf("unknown %s value %q", field, v))
		}
	}
	return nil
}

func postText(field, s string, min, max int, multiline bool) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if !utf8.ValidString(s) {
		return "", invalidPost(field, field+" isn't valid text")
	}
	if n := utf8.RuneCountInString(s); n < min || n > max {
		return "", invalidPost(field, fmt.Sprintf("%s must be %d to %d characters", strings.ReplaceAll(field, "_", " "), min, max))
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t')) {
			return "", invalidPost(field, field+" contains characters that aren't allowed")
		}
	}
	if htmlTag.MatchString(s) {
		return "", invalidPost(field, field+" can't contain HTML; use plain text or markdown")
	}
	return s, nil
}

func invalidPost(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// Job states shown to recruiters. "expired" is derived: active but past the
// 30-day public window.
const (
	StateActive  = "active"
	StateClosed  = "closed"
	StateExpired = "expired"
)

// Detail is the job details page.
type Detail struct {
	Job
	Status          string       `json:"status"` // active | closed | expired
	Description     string       `json:"description"`
	InterviewStages []string     `json:"interview_stages"`
	ExternalURL     string       `json:"external_url,omitempty"`
	Recruiter       *Person      `json:"recruiter"`
	Company         *CompanyCard `json:"company"`
	// Viewer-specific: when the signed-in viewer applied, and whether they
	// own the job (recruiter or an admin of its company).
	AppliedAt *time.Time `json:"applied_at"`
	CanManage bool       `json:"can_manage"`
	// Recruiter/admin only (manual "Job Analytics Engine").
	ShareHash string     `json:"share_hash,omitempty"`
	Analytics *Analytics `json:"analytics,omitempty"`
}

// Person is a public name card.
type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CompanyCard is the company sidebar on a linked job.
type CompanyCard struct {
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Tagline string  `json:"tagline"`
	Size    string  `json:"size"`
	LogoKey *string `json:"-"`
	LogoURL *string `json:"logo_url"`
	Website string  `json:"website"`
}

// Analytics is the recruiter/admin card on a job.
type Analytics struct {
	Views            int  `json:"views"`
	Applications     int  `json:"applications"`
	HighFit          *int `json:"high_fit"`          // Easy Apply only: match ≥ 70%
	RecruiterActions *int `json:"recruiter_actions"` // Easy Apply only
	OutboundClicks   int  `json:"outbound_clicks"`
	ExpiresInDays    int  `json:"expires_in_days"`
}

// Posted is one row of a recruiter's "manage jobs" list.
type Posted struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Tier         string    `json:"tier"`
	CompanyName  string    `json:"company_name"`
	CompanySlug  *string   `json:"company_slug"`
	Status       string    `json:"status"`
	ShareHash    string    `json:"share_hash"`
	Views        int       `json:"views"`
	Applications int       `json:"applications"`
	PostedAt     time.Time `json:"posted_at"`
}

// Draft is a saved, unvalidated post-a-job form.
type Draft struct {
	ID        string         `json:"id"`
	Form      map[string]any `json:"form"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// PosterCompany is a company the user may post under (current member).
type PosterCompany struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Role              string   `json:"role"`
	HQLocationID      *string  `json:"hq_location_id"`
	BranchLocationIDs []string `json:"branch_location_ids"`
}

// NotFoundError: no such job (or not visible to this viewer).
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// ForbiddenError: e.g. posting under a company you don't belong to.
type ForbiddenError struct{ Message string }

func (e *ForbiddenError) Error() string { return e.Message }

// PaymentRequiredError: a paid post with a fee while payments aren't connected.
type PaymentRequiredError struct{ Message string }

func (e *PaymentRequiredError) Error() string { return e.Message }

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool { _, ok := err.(*NotFoundError); return ok }

// IsForbidden checks if an error is a ForbiddenError
func IsForbidden(err error) bool { _, ok := err.(*ForbiddenError); return ok }

// IsPaymentRequired checks if an error is a PaymentRequiredError
func IsPaymentRequired(err error) bool { _, ok := err.(*PaymentRequiredError); return ok }

// ErrJobNotFound is the common not-found case.
var ErrJobNotFound = &NotFoundError{Message: "job not found"}

// Record is a validated post ready to store: the form plus resolved term ids,
// identity and tier-derived flags.
type Record struct {
	PostInput
	ID            string
	ShareHash     string
	RecruiterID   string
	CompanyID     *string
	DesignationID int64
	SkillIDs      []int64
	DBTier        string // Free | Premium (search filters on it)
	Listed        bool   // in public search: paid jobs only
}
