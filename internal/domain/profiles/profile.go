// Package profiles holds the candidate profile document and its rules.
package profiles

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"findJobs/internal/domain/jobs"
)

// Profile is the candidate's profile document (GET/PUT /profile/me). Field
// names match the frontend's profile store. Skills and target roles are
// taxonomy names (resolved to shared term ids on save); preferred locations
// are GeoNames ids.
type Profile struct {
	Saved        bool       `json:"saved"`         // read-only: false for a starter profile not yet stored
	Photo        *string    `json:"photo"`         // read-only: set with PUT /profile/me/photo
	AboutUpdated *time.Time `json:"about_updated"` // read-only
	// OwnerID is the profile's user, for server-side checks (e.g. not counting
	// the owner's own views). Never serialised: ids stay out of the API.
	OwnerID string `json:"-"`

	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Website  string `json:"website"` // portfolio display name, e.g. "asha.design"
	About    string `json:"about"`
	Gender   string `json:"gender"`
	Pronouns string `json:"pronouns"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`

	ExperienceYears int    `json:"experience_years"`
	Notice          string `json:"notice"`
	IsServingNotice bool   `json:"is_serving_notice"`
	LastWorkingDate string `json:"last_working_date"` // YYYY-MM-DD or ""

	SalaryCurrency string  `json:"salary_currency"`
	CurrentSalary  float64 `json:"current_salary"`
	ExpectedSalary float64 `json:"expected_salary"`
	HourlyRate     float64 `json:"hourly_rate"`

	WorkTypes          []string `json:"work_types"`
	PreferredLocations []string `json:"preferred_locations"`
	TargetRoles        []string `json:"target_roles"`
	OpenToRemote       bool     `json:"open_to_remote"`
	WorkMode           string   `json:"work_mode"`
	Relocation         string   `json:"relocation"`

	Visible              bool `json:"visible"`
	Open                 bool `json:"open"`
	CurrentSalaryPrivate bool `json:"current_salary_private"`
	HourlyPrivate        bool `json:"hourly_private"`
	ContactPrivate       bool `json:"contact_private"`
	SalaryPrivate        bool `json:"salary_private"`
	ResumePrivate        bool `json:"resume_private"`

	SkillGroups []SkillGroup `json:"skill_groups"`
	Skills      []string     `json:"skills"` // read-only on save: every skill, in display order

	Experiences []Experience `json:"experiences"`
	Details
}

// Details are small owner-edited lists stored together as JSON.
type Details struct {
	SocialLinks    []SocialLink    `json:"social_links"`
	Languages      []Language      `json:"languages"`
	Education      []Education     `json:"education"`
	Authorizations []Authorization `json:"authorizations"`
	Projects       []Project       `json:"projects"`
}

type SkillGroup struct {
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
}

type SocialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type Language struct {
	Name        string `json:"name"`
	Proficiency string `json:"proficiency"`
}

type Education struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Field       string `json:"field"`
	Start       string `json:"start"`
	End         string `json:"end"`
}

// Project is one portfolio entry ("A few things I've made"); URL and tags are optional.
type Project struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Tags        []string `json:"tags"`
}

type Authorization struct {
	Country    string `json:"country"`
	Type       string `json:"type"`
	From       string `json:"from"`
	Until      string `json:"until"`
	Indefinite bool   `json:"indefinite"`
}

// Experience is one work-history entry. Verification and CompanySlug are
// read-only: they come from the company's membership records.
type Experience struct {
	Company      string `json:"company"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Start        string `json:"start"` // YYYY-MM
	End          string `json:"end"`   // YYYY-MM, "" while current
	Current      bool   `json:"current"`
	Promoted     bool   `json:"promoted"`
	Description  string `json:"description"`
	Verification string `json:"verification"`
	CompanySlug  string `json:"company_slug"`
}

// Verification labels shown on experiences.
const (
	Unverified = "Unverified"
	Pending    = "Pending"
	Verified   = "Verified"
	Rejected   = "Rejected"
)

// Limits on list sizes (the resolve call takes at most 50 names).
const (
	MaxSkills      = 50
	MaxSkillGroups = 20
	MaxTargetRoles = 20
	MaxLocations   = 50
	MaxExperiences = 50
	maxListItems   = 20
)

var (
	Genders   = []string{"Male", "Female", "Non-binary", "Prefer not to say"}
	WorkModes = []string{"Remote or hybrid", "Remote", "Hybrid", "On-site", "Open to all arrangements"}
	// reservedSlugs collide with routes or impersonate the platform.
	reservedSlugs = []string{"admin", "api", "app", "auth", "about", "companies", "company", "contact", "dashboard",
		"edit", "faq", "help", "jobs", "login", "logout", "me", "new", "privacy", "profile", "profiles",
		"recruiter", "referise", "register", "search", "settings", "signin", "signup", "support", "terms"}
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	geonameID    = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)
	monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	phonePattern = regexp.MustCompile(`^[0-9+()\-. ]{0,30}$`)
	currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
	htmlTag      = regexp.MustCompile(`<[a-zA-Z/!?]`)
)

// Normalize trims and validates the document in place. Read-only fields are
// cleared so a client can't set them.
func (p *Profile) Normalize() error {
	p.Saved, p.Photo, p.AboutUpdated, p.Skills = false, nil, nil, nil
	var err error
	p.Slug = strings.TrimSpace(p.Slug)
	if utf8.RuneCountInString(p.Slug) < 3 || len(p.Slug) > 50 || !slugPattern.MatchString(p.Slug) {
		return invalid("slug", "use 3-50 lowercase letters, numbers and single hyphens for your profile handle")
	}
	if slices.Contains(reservedSlugs, p.Slug) {
		return invalid("slug", "that profile handle is reserved")
	}
	for _, f := range []struct {
		field    string
		value    *string
		min, max int
		multi    bool
	}{
		{"name", &p.Name, 2, 100, false},
		{"title", &p.Title, 1, 120, false},
		{"location", &p.Location, 0, 100, false},
		{"website", &p.Website, 0, 100, false},
		{"about", &p.About, 0, 2000, true},
		{"pronouns", &p.Pronouns, 0, 100, false},
	} {
		if *f.value, err = cleanText(f.field, *f.value, f.min, f.max, f.multi); err != nil {
			return err
		}
	}
	if p.Gender != "" && !slices.Contains(Genders, p.Gender) {
		return invalid("gender", "unknown gender option")
	}
	if p.Email = strings.TrimSpace(p.Email); p.Email != "" {
		if a, err := mail.ParseAddress(p.Email); err != nil || a.Address != p.Email || len(p.Email) > 255 {
			return invalid("email", "enter a valid email address")
		}
	}
	if p.Phone = strings.TrimSpace(p.Phone); !phonePattern.MatchString(p.Phone) {
		return invalid("phone", "a phone number can only contain digits, spaces and + ( ) - .")
	}
	if p.ExperienceYears < 0 || p.ExperienceYears > 99 {
		return invalid("experience_years", "experience must be 0-99 years")
	}
	if p.Notice != "" && !slices.Contains(jobs.NoticePeriods, p.Notice) {
		return invalid("notice", "unknown notice period")
	}
	if p.IsServingNotice {
		if _, err := time.Parse(time.DateOnly, p.LastWorkingDate); err != nil {
			return invalid("last_working_date", "add your last working date")
		}
	} else {
		p.LastWorkingDate = ""
	}
	if p.SalaryCurrency = strings.ToUpper(strings.TrimSpace(p.SalaryCurrency)); !currencyCode.MatchString(p.SalaryCurrency) {
		return invalid("salary_currency", "unknown currency")
	}
	for field, v := range map[string]float64{"current_salary": p.CurrentSalary, "expected_salary": p.ExpectedSalary} {
		if v < 0 || v > 1e12 {
			return invalid(field, "salary must be between 0 and 1,000,000,000,000")
		}
	}
	if p.HourlyRate < 0 || p.HourlyRate > 1e6 {
		return invalid("hourly_rate", "hourly rate must be between 0 and 1,000,000")
	}
	if p.WorkTypes, err = enumList("work_types", p.WorkTypes, jobs.WorkTypes); err != nil {
		return err
	}
	if p.WorkMode != "" && !slices.Contains(WorkModes, p.WorkMode) {
		return invalid("work_mode", "unknown work arrangement")
	}
	if p.Relocation != "" && !slices.Contains(jobs.RelocationOptions, p.Relocation) {
		return invalid("relocation", "unknown relocation preference")
	}
	p.PreferredLocations = dedupe(p.PreferredLocations)
	if len(p.PreferredLocations) > MaxLocations {
		return invalid("preferred_locations", fmt.Sprintf("choose at most %d locations", MaxLocations))
	}
	for _, id := range p.PreferredLocations {
		if !geonameID.MatchString(id) {
			return invalid("preferred_locations", "unknown location")
		}
	}
	if p.TargetRoles = dedupe(p.TargetRoles); len(p.TargetRoles) > MaxTargetRoles {
		return invalid("target_roles", fmt.Sprintf("choose at most %d target roles", MaxTargetRoles))
	}
	if err := p.normalizeSkills(); err != nil {
		return err
	}
	if err := p.normalizeExperiences(); err != nil {
		return err
	}
	return p.Details.normalize()
}

// normalizeSkills keeps each skill once (in its first group) and drops
// empty groups. Names are cleaned again when resolved to terms.
func (p *Profile) normalizeSkills() error {
	if len(p.SkillGroups) > MaxSkillGroups {
		return invalid("skill_groups", fmt.Sprintf("use at most %d skill groups", MaxSkillGroups))
	}
	seen := map[string]bool{}
	groups := make([]SkillGroup, 0, len(p.SkillGroups))
	total := 0
	for _, g := range p.SkillGroups {
		name, err := cleanText("skill group name", g.Name, 1, 60, false)
		if err != nil {
			return err
		}
		var skills []string
		for _, s := range g.Skills {
			s = strings.Join(strings.Fields(s), " ")
			key := strings.ToLower(s)
			if s == "" || seen[key] {
				continue
			}
			seen[key] = true
			skills = append(skills, s)
		}
		if total += len(skills); total > MaxSkills {
			return invalid("skills", fmt.Sprintf("add at most %d skills", MaxSkills))
		}
		if len(skills) > 0 {
			groups = append(groups, SkillGroup{Name: name, Skills: skills})
		}
	}
	p.SkillGroups = groups
	return nil
}

func (p *Profile) normalizeExperiences() error {
	if len(p.Experiences) > MaxExperiences {
		return invalid("experiences", fmt.Sprintf("add at most %d experiences", MaxExperiences))
	}
	var err error
	for i := range p.Experiences {
		e := &p.Experiences[i]
		e.Verification, e.CompanySlug = "", ""
		if e.Company, err = cleanText("company", e.Company, 1, 150, false); err != nil {
			return err
		}
		if e.Title, err = cleanText("job title", e.Title, 1, 150, false); err != nil {
			return err
		}
		if e.Description, err = cleanText("description", e.Description, 0, 5000, true); err != nil {
			return err
		}
		if !slices.Contains(jobs.WorkTypes, e.Type) {
			return invalid("experiences", "unknown employment type")
		}
		if !monthPattern.MatchString(e.Start) {
			return invalid("experiences", "add a start month to each experience")
		}
		if e.Current {
			e.End = ""
		} else if !monthPattern.MatchString(e.End) || e.End < e.Start {
			return invalid("experiences", "an experience needs an end month on or after its start")
		}
	}
	return nil
}

func (d *Details) normalize() error {
	if len(d.SocialLinks) > 10 || len(d.Languages) > maxListItems || len(d.Education) > maxListItems || len(d.Authorizations) > maxListItems || len(d.Projects) > maxListItems {
		return invalid("details", fmt.Sprintf("add at most 10 links and %d entries per list", maxListItems))
	}
	var err error
	for i := range d.SocialLinks {
		l := &d.SocialLinks[i]
		if l.Platform, err = cleanText("platform", l.Platform, 1, 30, false); err != nil {
			return err
		}
		if l.URL, err = cleanURL(l.URL); err != nil {
			return err
		}
	}
	for i := range d.Languages {
		l := &d.Languages[i]
		if l.Name, err = cleanText("language", l.Name, 1, 60, false); err != nil {
			return err
		}
		if l.Proficiency, err = cleanText("proficiency", l.Proficiency, 0, 30, false); err != nil {
			return err
		}
	}
	for i := range d.Education {
		e := &d.Education[i]
		for _, f := range []struct {
			field    string
			value    *string
			min, max int
		}{{"institution", &e.Institution, 0, 150}, {"degree", &e.Degree, 0, 150}, {"field of study", &e.Field, 0, 150},
			{"education start", &e.Start, 0, 10}, {"education end", &e.End, 0, 10}} {
			if *f.value, err = cleanText(f.field, *f.value, f.min, f.max, false); err != nil {
				return err
			}
		}
		if e.Institution == "" && e.Degree == "" {
			return invalid("education", "give each education entry an institution or a degree")
		}
	}
	for i := range d.Authorizations {
		a := &d.Authorizations[i]
		if a.Country, err = cleanText("country", a.Country, 1, 100, false); err != nil {
			return err
		}
		if a.Type, err = cleanText("authorization type", a.Type, 1, 100, false); err != nil {
			return err
		}
		if a.Indefinite {
			a.Until = ""
		}
		for _, date := range []string{a.From, a.Until} {
			if _, err := time.Parse(time.DateOnly, date); date != "" && err != nil {
				return invalid("authorizations", "authorization dates must be YYYY-MM-DD")
			}
		}
		if a.From != "" && a.Until != "" && a.Until < a.From {
			return invalid("authorizations", "authorization expiry cannot be before its start date")
		}
	}
	for i := range d.Projects {
		pr := &d.Projects[i]
		if pr.Title, err = cleanText("project title", pr.Title, 1, 100, false); err != nil {
			return err
		}
		if pr.Description, err = cleanText("project description", pr.Description, 0, 1000, true); err != nil {
			return err
		}
		if pr.URL != "" {
			if pr.URL, err = cleanURL(pr.URL); err != nil {
				return invalid("projects", "project links must be valid http(s) links")
			}
		}
		if len(pr.Tags) > 6 {
			return invalid("projects", "add at most 6 tags per project")
		}
		tags := make([]string, 0, len(pr.Tags))
		for _, t := range pr.Tags {
			if t, err = cleanText("project tag", t, 0, 30, false); err != nil {
				return err
			}
			if t != "" {
				tags = append(tags, t)
			}
		}
		pr.Tags = tags
	}
	return nil
}

// Public returns the copy public visitors see: private amounts and contact
// details are blanked (the privacy flags stay so the page can say so).
func (p Profile) Public() Profile {
	if p.ContactPrivate {
		p.Email, p.Phone = "", ""
	}
	if p.CurrentSalaryPrivate {
		p.CurrentSalary = 0
	}
	if p.SalaryPrivate {
		p.ExpectedSalary = 0
	}
	if p.HourlyPrivate {
		p.HourlyRate = 0
	}
	p.LastWorkingDate = ""
	return p
}

// EnsureLists replaces nil slices with empty ones, so JSON has [] not null.
func (p *Profile) EnsureLists() {
	for _, s := range []*[]string{&p.WorkTypes, &p.PreferredLocations, &p.TargetRoles, &p.Skills} {
		if *s == nil {
			*s = []string{}
		}
	}
	if p.SkillGroups == nil {
		p.SkillGroups = []SkillGroup{}
	}
	if p.Experiences == nil {
		p.Experiences = []Experience{}
	}
	if p.SocialLinks == nil {
		p.SocialLinks = []SocialLink{}
	}
	if p.Languages == nil {
		p.Languages = []Language{}
	}
	if p.Education == nil {
		p.Education = []Education{}
	}
	if p.Authorizations == nil {
		p.Authorizations = []Authorization{}
	}
	if p.Projects == nil {
		p.Projects = []Project{}
	}
}

// StarterSlug suggests a handle from a name: "Asha Rao" -> "asha-rao-<suffix>".
func StarterSlug(name, suffix string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	base := strings.Trim(b.String(), "-")
	if len(base) > 40 {
		base = strings.Trim(base[:40], "-")
	}
	if base == "" {
		return "member-" + suffix
	}
	return base + "-" + suffix
}

func enumList(field string, values, allowed []string) ([]string, error) {
	values = dedupe(values)
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return nil, invalid(field, fmt.Sprintf("unknown %s value %q", field, v))
		}
	}
	return values, nil
}

func dedupe(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// cleanText trims s and enforces length in characters. Control characters
// and HTML-like tags are rejected; multiline allows line breaks.
func cleanText(field, s string, min, max int, multiline bool) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if !utf8.ValidString(s) {
		return "", invalid(field, field+" isn't valid text")
	}
	if n := utf8.RuneCountInString(s); n < min || n > max {
		if min > 0 {
			return "", invalid(field, fmt.Sprintf("%s must be %d to %d characters", field, min, max))
		}
		return "", invalid(field, fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t')) {
			return "", invalid(field, field+" contains characters that aren't allowed")
		}
	}
	if htmlTag.MatchString(s) {
		return "", invalid(field, field+" can't contain HTML; use plain text")
	}
	return s, nil
}

// cleanURL accepts an absolute http(s) URL with a host and no credentials.
func cleanURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || len(raw) > 500 {
		return "", invalid("social_links", "social links must be valid http(s) links")
	}
	return u.String(), nil
}

// ValidationError represents an invalid profile.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(field, message string) error { return &ValidationError{Field: field, Message: message} }

// NotFoundError: no such profile (or it is private).
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// ConflictError: the slug is taken.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

var (
	ErrNotFound  = &NotFoundError{Message: "profile not found"}
	ErrSlugTaken = &ConflictError{Message: "that profile handle is taken"}
)

func IsValidation(err error) bool { _, ok := err.(*ValidationError); return ok }
func IsNotFound(err error) bool   { _, ok := err.(*NotFoundError); return ok }
func IsConflict(err error) bool   { _, ok := err.(*ConflictError); return ok }
