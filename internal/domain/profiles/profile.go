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
	Resume       *Resume    `json:"resume"`        // read-only: set with PUT /profile/me/resume; null when none (or not public)
	AboutUpdated *time.Time `json:"about_updated"` // read-only
	// OwnerID is the profile's user, for server-side checks (e.g. not counting
	// the owner's own views). Never serialised: ids stay out of the API.
	OwnerID string `json:"-"`

	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Title string `json:"title"`
	// Location is the current work location's label. With CurrentLocationID
	// set the server fills it from the location tree ("Bengaluru, Karnataka,
	// India"); without one it is the free text an older client sent.
	Location string `json:"location"`

	// Current professional status (newer doc §3).
	CurrentCompany         string `json:"current_company"`
	CurrentLocationID      string `json:"current_location_id"` // GeoNames id
	CurrentWorkArrangement string `json:"current_work_arrangement"`
	CurrentWorkType        string `json:"current_work_type"`

	Website  string `json:"website"` // portfolio display name, e.g. "asha.design"
	About    string `json:"about"`
	Gender   string `json:"gender"`
	Pronouns string `json:"pronouns"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`

	ExperienceYears int `json:"experience_years"`
	// Notice is the job-filter bucket ("30 days"). When NoticeUnit is set it
	// is derived from NoticeValue + NoticeUnit (e.g. 2 + "months").
	Notice        string `json:"notice"`
	NoticeValue   int    `json:"notice_value"`
	NoticeUnit    string `json:"notice_unit"` // days, weeks, months or ""
	Availability  string `json:"availability"`
	AvailableFrom string `json:"available_from"` // YYYY-MM-DD, with "Available from date"
	// IsServingNotice mirrors Availability == "Serving notice"; older clients
	// that send only this flag still set that availability.
	IsServingNotice bool   `json:"is_serving_notice"`
	LastWorkingDate string `json:"last_working_date"` // YYYY-MM-DD, with "Serving notice"

	SalaryCurrency       string  `json:"salary_currency"`
	CurrentSalary        float64 `json:"current_salary"`
	CurrentSalaryPeriod  string  `json:"current_salary_period"` // year, month or hour
	ExpectedSalary       float64 `json:"expected_salary"`
	ExpectedSalaryPeriod string  `json:"expected_salary_period"`
	HourlyRate           float64 `json:"hourly_rate"`

	WorkTypes          []string `json:"work_types"`
	PreferredLocations []string `json:"preferred_locations"`
	TargetRoles        []string `json:"target_roles"`
	OpenToRemote       bool     `json:"open_to_remote"`
	WorkMode           string   `json:"work_mode"`
	Relocation         string   `json:"relocation"`
	Travel             string   `json:"travel"`

	Visible bool `json:"visible"`
	Open    bool `json:"open"`
	// Privacy (newer doc §12): VisibleAll, VisibleApplied ("Applied Jobs
	// Only": recruiters of jobs the candidate applied to) or, for the current
	// salary only, VisibleHidden.
	CurrentSalaryVisibility  string `json:"current_salary_visibility"`
	ExpectedSalaryVisibility string `json:"expected_salary_visibility"`
	HourlyRateVisibility     string `json:"hourly_rate_visibility"`
	ResumeVisibility         string `json:"resume_visibility"`
	ContactVisibility        string `json:"contact_visibility"`

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

// Language: the doc asks for Read / Write / Speak, with no proficiency level.
type Language struct {
	Name  string `json:"name"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
	Speak bool   `json:"speak"`
	// LegacyProficiency is what older clients send instead of the three
	// flags; it is read on save (as "knows all three") and never returned.
	LegacyProficiency string `json:"proficiency,omitempty"`
}

// Education stays free-form (newer doc §7).
type Education struct {
	Institution  string   `json:"institution"`
	Degree       string   `json:"degree"`
	Field        string   `json:"field"`
	Start        string   `json:"start"`
	End          string   `json:"end"`
	Grade        string   `json:"grade"` // GPA, percentage, class: systems vary
	Honors       string   `json:"honors"`
	Description  string   `json:"description"`
	Achievements []string `json:"achievements"`
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

// Experience is one employment record of the career timeline (newer doc §6):
// a company, the employment period and the roles held there. The blue tick
// belongs to the record. Verification, CompanySlug and Location are read-only.
type Experience struct {
	Company         string `json:"company"`
	Type            string `json:"type"` // work type: jobs.WorkTypes
	WorkArrangement string `json:"work_arrangement"`
	LocationID      string `json:"location_id"` // GeoNames id, optional
	Location        string `json:"location"`    // read-only label of LocationID
	Start           string `json:"start"`       // YYYY-MM
	End             string `json:"end"`         // YYYY-MM, "" while current
	Current         bool   `json:"current"`
	Roles           []Role `json:"roles"`
	Verification    string `json:"verification"`
	CompanySlug     string `json:"company_slug"`

	// Older clients send one role flat on the record; on save these become
	// its single role. Never returned.
	LegacyTitle       string `json:"title,omitempty"`
	LegacyPromoted    bool   `json:"promoted,omitempty"`
	LegacyDescription string `json:"description,omitempty"`
}

// Role is one designation held within an employment record.
type Role struct {
	Title            string   `json:"title"`
	Start            string   `json:"start"` // YYYY-MM
	End              string   `json:"end"`   // YYYY-MM, "" for the ongoing role
	Promoted         bool     `json:"promoted"`
	Responsibilities string   `json:"responsibilities"`
	Achievements     []string `json:"achievements"`
}

// LatestTitle is the title of the role that started last: what the company
// is asked to verify.
func (e Experience) LatestTitle() string {
	title, start := "", ""
	for _, r := range e.Roles {
		if r.Start >= start {
			title, start = r.Title, r.Start
		}
	}
	return title
}

// Verification labels shown on experiences.
const (
	Unverified = "Unverified"
	Pending    = "Pending"
	Verified   = "Verified"
	Rejected   = "Rejected"
)

// Stored review states of an experience entry (profile_experiences.verification).
const (
	ReviewUnverified = "unverified"
	ReviewPending    = "pending"
	ReviewVerified   = "verified"
	ReviewRejected   = "rejected"
)

// LinkedExperience is a stored entry with its registered company ("" when
// none) and its review state.
type LinkedExperience struct {
	CompanyID string
	Entry     Experience
	Review    string
}

// CarryReviews gives each saved entry its review state (manual "Blue Tick
// Workflow"). An unchanged entry keeps its state, wherever it moved to. An
// entry edited in place (same position and company) that was verified or
// pending goes to pending, which puts it back in the company's queue; a
// rejected one stays rejected until the candidate asks again. Anything else
// starts unverified.
func CarryReviews(old, next []LinkedExperience) []string {
	out := make([]string, len(next))
	used := make([]bool, len(old))
	for i, n := range next {
		if n.CompanyID == "" {
			out[i] = ReviewUnverified
			continue
		}
		for j, o := range old {
			if !used[j] && o.CompanyID == n.CompanyID && sameEntry(o.Entry, n.Entry) {
				out[i], used[j] = o.Review, true
				break
			}
		}
	}
	for i, n := range next {
		if out[i] != "" {
			continue
		}
		out[i] = ReviewUnverified
		if i < len(old) && !used[i] && old[i].CompanyID == n.CompanyID {
			used[i] = true
			switch old[i].Review {
			case ReviewVerified, ReviewPending:
				out[i] = ReviewPending
			case ReviewRejected:
				out[i] = ReviewRejected
			}
		}
	}
	return out
}

// sameEntry: any edit to a record or one of its roles counts (manual
// "Re-Verification Rule").
func sameEntry(a, b Experience) bool {
	return a.Type == b.Type && a.WorkArrangement == b.WorkArrangement && a.LocationID == b.LocationID &&
		a.Start == b.Start && a.End == b.End && a.Current == b.Current &&
		slices.EqualFunc(a.Roles, b.Roles, func(x, y Role) bool {
			return x.Title == y.Title && x.Start == y.Start && x.End == y.End && x.Promoted == y.Promoted &&
				x.Responsibilities == y.Responsibilities && slices.Equal(x.Achievements, y.Achievements)
		})
}

// Limits on list sizes (the resolve call takes at most 50 names).
const (
	MaxSkills       = 50
	MaxSkillGroups  = 20
	MaxTargetRoles  = 20
	MaxLocations    = 50
	MaxExperiences  = 50
	MaxRoles        = 20 // per employment record
	maxListItems    = 20
	maxAchievements = 20
)

// Visibility values for the privacy settings.
const (
	VisibleAll     = "all"
	VisibleApplied = "applied"
	VisibleHidden  = "hidden"
)

// ServingNotice is the availability that needs a last working date;
// AvailableFromDate the one that needs an available-from date.
const (
	ServingNotice     = "Serving notice"
	AvailableFromDate = "Available from date"
)

var (
	SalaryPeriods  = []string{"year", "month", "hour"}
	NoticeUnits    = map[string]int{"days": 1, "weeks": 7, "months": 30}
	Availabilities = []string{"Available immediately", "Currently employed", ServingNotice, AvailableFromDate, "Not available"}
	// TravelOptions: the doc lists "No Travel, Occasional Travel, specific
	// percentage/range options".
	TravelOptions = []string{"No travel", "Occasional travel", "Up to 25%", "Up to 50%", "Up to 75%", "Up to 100%"}
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
	if err := ValidateSlug(p.Slug); err != nil {
		return err
	}
	for _, f := range []struct {
		field    string
		value    *string
		min, max int
		multi    bool
	}{
		{"name", &p.Name, 2, 100, false},
		{"title", &p.Title, 1, 120, false},
		{"location", &p.Location, 0, 200, false},
		{"current_company", &p.CurrentCompany, 0, 150, false},
		{"website", &p.Website, 0, 100, false},
		{"about", &p.About, 0, 2000, true},
		{"pronouns", &p.Pronouns, 0, 100, false},
	} {
		if *f.value, err = cleanText(f.field, *f.value, f.min, f.max, f.multi); err != nil {
			return err
		}
	}
	if p.CurrentLocationID != "" && !geonameID.MatchString(p.CurrentLocationID) {
		return invalid("current_location_id", "unknown location")
	}
	if p.CurrentWorkArrangement != "" && !slices.Contains(jobs.WorkModes, p.CurrentWorkArrangement) {
		return invalid("current_work_arrangement", "a work arrangement is Remote, Hybrid or On-site")
	}
	if p.CurrentWorkType != "" && !slices.Contains(jobs.WorkTypes, p.CurrentWorkType) {
		return invalid("current_work_type", "unknown work type")
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
	if err := p.normalizeAvailability(); err != nil {
		return err
	}
	if p.SalaryCurrency = strings.ToUpper(strings.TrimSpace(p.SalaryCurrency)); !currencyCode.MatchString(p.SalaryCurrency) {
		return invalid("salary_currency", "unknown currency")
	}
	for field, period := range map[string]*string{"current_salary_period": &p.CurrentSalaryPeriod, "expected_salary_period": &p.ExpectedSalaryPeriod} {
		if *period == "" {
			*period = "year"
		}
		if !slices.Contains(SalaryPeriods, *period) {
			return invalid(field, "a salary period is year, month or hour")
		}
	}
	if err := p.normalizePrivacy(); err != nil {
		return err
	}
	if p.Travel != "" && !slices.Contains(TravelOptions, p.Travel) {
		return invalid("travel", "unknown travel preference")
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

// normalizeAvailability checks the availability, its date and the notice
// period. A notice given as number + unit sets the job-filter bucket.
func (p *Profile) normalizeAvailability() error {
	if p.Availability == "" && p.IsServingNotice {
		p.Availability = ServingNotice
	}
	if p.Availability != "" && !slices.Contains(Availabilities, p.Availability) {
		return invalid("availability", "unknown availability")
	}
	p.IsServingNotice = p.Availability == ServingNotice
	for _, d := range []struct {
		needed       bool
		field, label string
		value        *string
	}{
		{p.IsServingNotice, "last_working_date", "add your last working date", &p.LastWorkingDate},
		{p.Availability == AvailableFromDate, "available_from", "add the date you're available from", &p.AvailableFrom},
	} {
		if !d.needed {
			*d.value = ""
		} else if _, err := time.Parse(time.DateOnly, *d.value); err != nil {
			return invalid(d.field, d.label)
		}
	}

	if p.NoticeUnit == "" {
		p.NoticeValue = 0
		if p.Notice != "" && !slices.Contains(jobs.NoticePeriods, p.Notice) {
			return invalid("notice", "unknown notice period")
		}
		return nil
	}
	days, ok := NoticeUnits[p.NoticeUnit]
	if days *= p.NoticeValue; !ok || p.NoticeValue < 0 || days > 365 {
		return invalid("notice_value", "a notice period is 0 to 365 days, 52 weeks or 12 months")
	}
	p.Notice = NoticeBucket(days)
	return nil
}

// NoticeBucket maps a notice in days to the job filter's notice periods.
func NoticeBucket(days int) string {
	for _, b := range []struct {
		max   int
		label string
	}{{0, "Immediate"}, {15, "15 days"}, {30, "30 days"}, {45, "45 days"}, {60, "60 days"}, {90, "90 days"}} {
		if days <= b.max {
			return b.label
		}
	}
	return "More than 90 days"
}

// normalizePrivacy checks each visibility. A missing one takes the private
// default, so a client that doesn't send it never makes a field public.
func (p *Profile) normalizePrivacy() error {
	for _, v := range []struct {
		field   string
		value   *string
		allowed []string
		def     string
	}{
		{"current_salary_visibility", &p.CurrentSalaryVisibility, []string{VisibleAll, VisibleApplied, VisibleHidden}, VisibleHidden},
		{"expected_salary_visibility", &p.ExpectedSalaryVisibility, []string{VisibleAll, VisibleApplied}, VisibleApplied},
		{"hourly_rate_visibility", &p.HourlyRateVisibility, []string{VisibleAll, VisibleApplied}, VisibleApplied},
		{"resume_visibility", &p.ResumeVisibility, []string{VisibleAll, VisibleApplied}, VisibleApplied},
		{"contact_visibility", &p.ContactVisibility, []string{VisibleAll, VisibleApplied}, VisibleApplied},
	} {
		if *v.value == "" {
			*v.value = v.def
		}
		if !slices.Contains(v.allowed, *v.value) {
			return invalid(v.field, fmt.Sprintf("%s must be one of %s", v.field, strings.Join(v.allowed, ", ")))
		}
	}
	return nil
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
		e.Verification, e.CompanySlug, e.Location = "", "", ""
		if len(e.Roles) == 0 && e.LegacyTitle != "" { // an older client's flat entry
			e.Roles = []Role{{Title: e.LegacyTitle, Start: e.Start, End: e.End, Promoted: e.LegacyPromoted,
				Responsibilities: e.LegacyDescription}}
		}
		e.LegacyTitle, e.LegacyPromoted, e.LegacyDescription = "", false, ""
		if e.Company, err = cleanText("company", e.Company, 1, 150, false); err != nil {
			return err
		}
		if !slices.Contains(jobs.WorkTypes, e.Type) {
			return invalid("experiences", "unknown employment type")
		}
		if e.WorkArrangement != "" && !slices.Contains(jobs.WorkModes, e.WorkArrangement) {
			return invalid("experiences", "a work arrangement is Remote, Hybrid or On-site")
		}
		if e.LocationID != "" && !geonameID.MatchString(e.LocationID) {
			return invalid("experiences", "unknown location")
		}
		if !monthPattern.MatchString(e.Start) {
			return invalid("experiences", "add a start month to each experience")
		}
		if e.Current {
			e.End = ""
		} else if !monthPattern.MatchString(e.End) || e.End < e.Start {
			return invalid("experiences", "an experience needs an end month on or after its start")
		}
		if err := e.normalizeRoles(); err != nil {
			return err
		}
	}
	return nil
}

// normalizeRoles checks each role lies within its employment period. Only a
// current record's roles may be open-ended; a past record's role without an
// end month ends with the record.
func (e *Experience) normalizeRoles() error {
	if len(e.Roles) == 0 || len(e.Roles) > MaxRoles {
		return invalid("roles", fmt.Sprintf("add 1 to %d roles to each experience", MaxRoles))
	}
	var err error
	for i := range e.Roles {
		r := &e.Roles[i]
		if r.Title, err = cleanText("role title", r.Title, 1, 150, false); err != nil {
			return err
		}
		if r.Responsibilities, err = cleanText("responsibilities", r.Responsibilities, 0, 5000, true); err != nil {
			return err
		}
		if r.Achievements, err = cleanList("achievement", r.Achievements, 500); err != nil {
			return err
		}
		if !monthPattern.MatchString(r.Start) || r.Start < e.Start {
			return invalid("roles", "give each role a start month within its experience")
		}
		if r.End == "" && !e.Current {
			r.End = e.End
		}
		if r.End != "" && (!monthPattern.MatchString(r.End) || r.End < r.Start || (!e.Current && r.End > e.End)) {
			return invalid("roles", "a role's end month must be on or after its start and within its experience")
		}
	}
	return nil
}

// cleanList cleans each item of a bullet list and drops empty ones.
func cleanList(field string, items []string, max int) ([]string, error) {
	if len(items) > maxAchievements {
		return nil, invalid(field, fmt.Sprintf("add at most %d items to a list", maxAchievements))
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item, err := cleanText(field, item, 0, max, false)
		if err != nil {
			return nil, err
		}
		if item != "" {
			out = append(out, item)
		}
	}
	return out, nil
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
		if !l.Read && !l.Write && !l.Speak && l.LegacyProficiency != "" {
			l.Read, l.Write, l.Speak = true, true, true
		}
		l.LegacyProficiency = ""
	}
	for i := range d.Education {
		e := &d.Education[i]
		for _, f := range []struct {
			field    string
			value    *string
			min, max int
		}{{"institution", &e.Institution, 0, 150}, {"degree", &e.Degree, 0, 150}, {"field of study", &e.Field, 0, 150},
			{"education start", &e.Start, 0, 10}, {"education end", &e.End, 0, 10}, {"grade", &e.Grade, 0, 50},
			{"honors", &e.Honors, 0, 200}} {
			if *f.value, err = cleanText(f.field, *f.value, f.min, f.max, false); err != nil {
				return err
			}
		}
		if e.Description, err = cleanText("education description", e.Description, 0, 2000, true); err != nil {
			return err
		}
		if e.Achievements, err = cleanList("achievement", e.Achievements, 500); err != nil {
			return err
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

// Public returns the copy public visitors see: anything not Visible to All
// is blanked (the settings stay so the page can say so). "Applied Jobs Only"
// fields reach recruiters through the application dossier instead.
func (p Profile) Public() Profile {
	if p.ContactVisibility != VisibleAll {
		p.Email, p.Phone = "", ""
	}
	if p.CurrentSalaryVisibility != VisibleAll {
		p.CurrentSalary = 0
	}
	if p.ExpectedSalaryVisibility != VisibleAll {
		p.ExpectedSalary = 0
	}
	if p.HourlyRateVisibility != VisibleAll {
		p.HourlyRate = 0
	}
	if p.ResumeVisibility != VisibleAll {
		p.Resume = nil
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
	for i := range p.Education {
		if p.Education[i].Achievements == nil {
			p.Education[i].Achievements = []string{}
		}
	}
	for i := range p.Experiences {
		e := &p.Experiences[i]
		if e.Roles == nil {
			e.Roles = []Role{}
		}
		for j := range e.Roles {
			if e.Roles[j].Achievements == nil {
				e.Roles[j].Achievements = []string{}
			}
		}
	}
	if p.Authorizations == nil {
		p.Authorizations = []Authorization{}
	}
	if p.Projects == nil {
		p.Projects = []Project{}
	}
}

// ValidateSlug checks a profile handle's format and the reserved list.
func ValidateSlug(slug string) error {
	if utf8.RuneCountInString(slug) < 3 || len(slug) > 50 || !slugPattern.MatchString(slug) {
		return invalid("slug", "use 3-50 lowercase letters, numbers and single hyphens for your profile handle")
	}
	if slices.Contains(reservedSlugs, slug) {
		return invalid("slug", "that profile handle is reserved")
	}
	return nil
}

// FreeSlug is the first valid handle among base, base-2, base-3, ... that
// isn't in taken (manual "Profile URL Slug": a numeric suffix on collision).
func FreeSlug(base string, taken map[string]bool) string {
	for n := 1; ; n++ {
		slug := base
		if n > 1 {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		if !taken[slug] && ValidateSlug(slug) == nil {
			return slug
		}
	}
}

// SlugBase turns a name into a handle base: "Asha Rao" -> "asha-rao".
// A name with no ASCII letters or digits gives "member".
func SlugBase(name string) string {
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
		return "member"
	}
	return base
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
