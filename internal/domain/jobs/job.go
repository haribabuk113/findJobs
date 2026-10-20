package jobs

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Job is a public job listing as returned by search. Salary fields are nil
// when the employer hid the salary — the amounts never leave the backend.
type Job struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// CompanyID/CompanySlug are nil for free and unlinked jobs; CompanyName
	// then falls back to the text the recruiter typed (or "").
	CompanyID        *string   `json:"company"`
	CompanySlug      *string   `json:"company_slug"`
	CompanyName      string    `json:"company_name"`
	CompanySector    string    `json:"company_sector"`
	CompanySize      string    `json:"company_size"`
	DesignationID    *int64    `json:"designation_id"`
	Category         string    `json:"category"`
	Level            string    `json:"level"`
	Summary          string    `json:"summary"`
	WorkModes        []string  `json:"work_modes"`
	WorkTypes        []string  `json:"work_types"`
	LocationType     string    `json:"location_type"`
	Location         string    `json:"location"`
	LocationIDs      []string  `json:"location_ids"`
	SalaryMin        *float64  `json:"salary_min"`
	SalaryMax        *float64  `json:"salary_max"`
	Currency         *string   `json:"currency"`
	SalaryMaxINR     *float64  `json:"normalized_inr_amount"`
	SalaryHidden     bool      `json:"salary_hidden"`
	SkillIDs         []int64   `json:"skill_ids"`
	Skills           []string  `json:"skills"` // names, same order as skill_ids
	NoticePeriods    []string  `json:"notice_periods"`
	Benefits         []string  `json:"benefits"`
	Relocation       string    `json:"relocation"`
	ExperienceYears  int       `json:"experience_years"`
	InterviewRounds  int       `json:"interview_rounds"`
	Tier             string    `json:"tier"`
	Application      string    `json:"application"`
	Views            int       `json:"views"`
	RecruiterActions int       `json:"recruiter_actions"`
	PostedAt         time.Time `json:"posted_at"`
}

// SearchResult is one page of matching jobs plus the total match count.
type SearchResult struct {
	Jobs     []Job `json:"jobs"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Allowed values, mirrored from the filter specification (manual "Filters").
var (
	WorkModes          = []string{"Remote", "Hybrid", "On-site"}
	WorkTypes          = []string{"Full-time", "Part-time", "Contract", "Internship", "Freelance", "Consultant", "Gig", "Advisory", "Board member", "Volunteering", "Others"}
	NoticePeriods      = []string{"Immediate", "15 days", "30 days", "45 days", "60 days", "90 days", "More than 90 days"}
	RelocationOptions  = []string{"On their own", "Supported locally", "Supported internationally", "Not supported"}
	Tiers              = []string{"Free", "Premium"}
	ApplicationMethods = []string{"Easy apply", "External"}
	PostedWithinDays   = []int{1, 3, 7, 9, 15, 30}
	Sorts              = []string{"newest", "salary", "title"}
)

const (
	MaxPageSize     = 50
	DefaultPageSize = 20
	// PublicFreshnessDays is the 30-day public visibility cap.
	PublicFreshnessDays = 30
)

// SearchCriteria is the single consolidated payload the Search button sends.
// Categories are ANDed together; values inside one multi-select are ORed.
type SearchCriteria struct {
	// Query matches job titles only (requirements: job search is by
	// designation/title, never description text).
	Query         string   `json:"query"`
	LocationText  string   `json:"location_text"` // free-text location ("Bengaluru", "remote")
	Designations  []int64  `json:"designations"`  // designation term ids
	Skills        []int64  `json:"skills"`        // skill term ids
	Company       string   `json:"company"`       // company slug: its jobs tab
	RecruiterID   string   `json:"recruiter_id"`  // a recruiter's open jobs
	LocationIDs   []string `json:"location_ids"`  // leaf city ids, flattened from the tristate tree by the client
	WorldRemote   bool     `json:"worldwide_remote"`
	MinSalaryINR  float64  `json:"min_salary_inr"` // client converts the user's currency with rates.json
	MaxSalaryINR  float64  `json:"max_salary_inr"`
	NoticePeriods []string `json:"notice_periods"`
	Relocation    []string `json:"relocation"`
	WorkModes     []string `json:"work_modes"`
	WorkTypes     []string `json:"work_types"`
	Categories    []string `json:"categories"`
	Levels        []string `json:"levels"`
	Tiers         []string `json:"tiers"`
	Applications  []string `json:"application_methods"`
	Industries    []string `json:"industries"`
	CompanySizes  []string `json:"company_sizes"`
	Benefits      []string `json:"benefits"`
	JobIDs        []string `json:"job_ids"` // restrict to these ids, e.g. the user's saved jobs
	MinExperience *int     `json:"min_experience"`
	MaxExperience *int     `json:"max_experience"`
	Rounds        *int     `json:"interview_rounds"`
	RoundsExact   bool     `json:"interview_rounds_exact"` // false = "at most N rounds"
	PostedWithin  int      `json:"posted_within_days"`
	Sort          string   `json:"sort"`
	Page          int      `json:"page"`
	PageSize      int      `json:"page_size"`
}

// Normalize applies defaults and validates. Unknown enum values are rejected
// rather than silently ignored, so a client bug can't widen a search.
func (c *SearchCriteria) Normalize() error {
	c.Query = strings.TrimSpace(c.Query)
	c.LocationText = strings.TrimSpace(c.LocationText)
	if len(c.Query) > 200 || len(c.LocationText) > 100 {
		return &ValidationError{Field: "query", Message: "search text is too long"}
	}
	for field, list := range map[string][]string{
		"location_ids": c.LocationIDs, "categories": c.Categories, "levels": c.Levels, "industries": c.Industries,
		"company_sizes": c.CompanySizes, "benefits": c.Benefits, "job_ids": c.JobIDs,
	} {
		if len(list) > 200 {
			return &ValidationError{Field: field, Message: field + " has too many values"}
		}
	}
	for field, check := range map[string]struct{ got, allowed []string }{
		"notice_periods":      {c.NoticePeriods, NoticePeriods},
		"relocation":          {c.Relocation, RelocationOptions},
		"work_modes":          {c.WorkModes, WorkModes},
		"work_types":          {c.WorkTypes, WorkTypes},
		"tiers":               {c.Tiers, Tiers},
		"application_methods": {c.Applications, ApplicationMethods},
	} {
		for _, v := range check.got {
			if !slices.Contains(check.allowed, v) {
				return &ValidationError{Field: field, Message: fmt.Sprintf("unknown %s value %q", field, v)}
			}
		}
	}
	if len(c.Designations) > 200 || len(c.Skills) > 200 {
		return &ValidationError{Field: "skills", Message: "too many skills or designations"}
	}
	if c.Company != "" && !companySlug.MatchString(c.Company) {
		return &ValidationError{Field: "company", Message: "unknown company"}
	}
	if c.RecruiterID != "" && !uuidPattern.MatchString(c.RecruiterID) {
		return &ValidationError{Field: "recruiter_id", Message: "unknown recruiter"}
	}
	if c.MinSalaryINR < 0 || c.MaxSalaryINR < 0 || (c.MaxSalaryINR > 0 && c.MinSalaryINR > c.MaxSalaryINR) {
		return &ValidationError{Field: "salary", Message: "maximum salary must be at least the minimum salary"}
	}
	for field, v := range map[string]*int{"min_experience": c.MinExperience, "max_experience": c.MaxExperience, "interview_rounds": c.Rounds} {
		if v != nil && (*v < 0 || *v > 99) {
			return &ValidationError{Field: field, Message: field + " must be between 0 and 99"}
		}
	}
	if c.MinExperience != nil && c.MaxExperience != nil && *c.MinExperience > *c.MaxExperience {
		return &ValidationError{Field: "experience", Message: "maximum experience must be at least the minimum experience"}
	}
	if c.PostedWithin == 0 {
		c.PostedWithin = PublicFreshnessDays
	}
	if !slices.Contains(PostedWithinDays, c.PostedWithin) {
		return &ValidationError{Field: "posted_within_days", Message: "posted_within_days must be one of 1, 3, 7, 9, 15, 30"}
	}
	if c.Sort == "" {
		c.Sort = "newest"
	}
	if !slices.Contains(Sorts, c.Sort) {
		return &ValidationError{Field: "sort", Message: "sort must be newest, salary or title"}
	}
	if c.Page < 1 {
		c.Page = 1
	}
	if c.PageSize < 1 {
		c.PageSize = DefaultPageSize
	}
	if c.PageSize > MaxPageSize {
		c.PageSize = MaxPageSize
	}
	if c.Page > 1000 {
		return &ValidationError{Field: "page", Message: "page is too large"}
	}
	return nil
}

var (
	companySlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,58}[a-z0-9]$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// ValidationError represents an invalid search request
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Message
}

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}
