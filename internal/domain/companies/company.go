package companies

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

// Sizes are the headcount brackets (manual "Company Size"). Job search
// filters on this exact text, so it is the canonical spelling.
var Sizes = []string{"1–10 people", "11–50 people", "51–200 people", "201–500 people",
	"501–1,000 people", "1,001–5,000 people", "5,000+ people"}

// BenefitIcons are the icons the benefit editor offers.
var BenefitIcons = []string{"heart", "laptop", "coffee", "spark", "money"}

// SocialPlatforms are the optional profile links (manual "Official Website & Social Links").
var SocialPlatforms = []string{"linkedin", "x", "github", "glassdoor"}

// Member roles and statuses (manual "Company employees").
const (
	RoleAdmin    = "admin"
	RoleHR       = "hr"
	RoleEmployee = "employee"

	StatusPending  = "pending"
	StatusCurrent  = "current"
	StatusPast     = "past"
	StatusRejected = "rejected"
)

// reservedSlugs can't be company links: they collide with routes or impersonate the platform.
var reservedSlugs = []string{"admin", "api", "app", "auth", "about", "companies", "company", "contact",
	"create", "dashboard", "edit", "faq", "help", "jobs", "login", "logout", "new", "privacy", "profile",
	"recruiter", "referise", "register", "search", "settings", "signin", "signup", "support", "terms"}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,58})[a-z0-9]$`)

// htmlTag catches markup in user text. Rendering escapes HTML anyway; this
// keeps stored text free of it (manual: server-side sanitisation).
var htmlTag = regexp.MustCompile(`<[a-zA-Z/!?]`)

// Input is the editable company document (create and full update).
type Input struct {
	Name              string            `json:"name"`
	Slug              string            `json:"slug"`
	Tagline           string            `json:"tagline"`
	Size              string            `json:"size"`
	Industries        []string          `json:"industries"`
	About             string            `json:"about"`
	Website           string            `json:"website"`
	Social            map[string]string `json:"social"`
	FoundedYear       *int              `json:"founded_year"`
	HQLocationID      string            `json:"hq_location_id"`
	HQAddress         string            `json:"hq_address"`
	BranchLocationIDs []string          `json:"branch_location_ids"`
	Products          []Product         `json:"products"`
	Benefits          []Benefit         `json:"benefits"`
}

// Product is one product/service showcase card.
type Product struct {
	Name  string `json:"name"`
	About string `json:"about"`
	URL   string `json:"url"`
}

// Benefit is one workplace perk.
type Benefit struct {
	Title   string `json:"title"`
	Details string `json:"details"`
	Icon    string `json:"icon"`
}

// Company is the public company page.
type Company struct {
	ID                string            `json:"id"`
	Slug              string            `json:"slug"`
	Name              string            `json:"name"`
	Tagline           string            `json:"tagline"`
	Size              string            `json:"size"`
	Industries        []string          `json:"industries"`
	About             string            `json:"about"`
	Website           string            `json:"website"`
	Social            map[string]string `json:"social"`
	FoundedYear       *int              `json:"founded_year"`
	HQLocationID      *string           `json:"hq_location_id"`
	HQCountryCode     *string           `json:"hq_country_code"`
	HQAddress         string            `json:"hq_address"`
	BranchLocationIDs []string          `json:"branch_location_ids"`
	LogoURL           *string           `json:"logo_url"`
	BannerURL         *string           `json:"banner_url"`
	Products          []Product         `json:"products"`
	Benefits          []Benefit         `json:"benefits"`
	CreatedAt         time.Time         `json:"created_at"`
	// ViewerRole is the signed-in viewer's role when they are a current member.
	ViewerRole *string `json:"viewer_role"`
}

// Summary is one company search result.
type Summary struct {
	ID            string   `json:"id"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Tagline       string   `json:"tagline"`
	Size          string   `json:"size"`
	Industries    []string `json:"industries"`
	HQLocationID  *string  `json:"hq_location_id"`
	HQCountryCode *string  `json:"hq_country_code"`
	LogoURL       *string  `json:"logo_url"`
}

// SearchCriteria filters the company directory (manual: name search plus
// structured filters).
type SearchCriteria struct {
	Query      string   `json:"query"`
	Countries  []string `json:"countries"`
	Industries []string `json:"industries"`
	Sizes      []string `json:"sizes"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

// SearchResult is one page of companies.
type SearchResult struct {
	Companies []Summary `json:"companies"`
	Total     int       `json:"total"`
	Page      int       `json:"page"`
	PageSize  int       `json:"page_size"`
}

// Member is one person linked to a company, as admins see them.
type Member struct {
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	StartedOn   *string   `json:"started_on"`
	RequestedAt time.Time `json:"requested_at"`
}

// Dashboard holds the non-money admin widgets (manual "Company dashboard").
type Dashboard struct {
	TotalJobs       int `json:"total_jobs"`
	ActiveJobs      int `json:"active_jobs"`
	ClosedJobs      int `json:"closed_jobs"`
	Applications    int `json:"applications"`
	Admins          int `json:"admins"`
	HR              int `json:"hr"`
	Employees       int `json:"employees"`
	PendingRequests int `json:"pending_requests"`
}

// Normalize trims and validates the input in place.
func (in *Input) Normalize(now time.Time) error {
	var err error
	if in.Name, err = cleanText("name", in.Name, 2, 120, false); err != nil {
		return err
	}
	in.Slug = strings.TrimSpace(in.Slug)
	if err := ValidateSlug(in.Slug); err != nil {
		return err
	}
	if in.Tagline, err = cleanText("tagline", in.Tagline, 0, 150, false); err != nil {
		return err
	}
	if !slices.Contains(Sizes, in.Size) {
		return invalid("size", "size must be one of: "+strings.Join(Sizes, ", "))
	}
	if len(in.Industries) < 1 || len(in.Industries) > 5 {
		return invalid("industries", "add 1 to 5 industries")
	}
	seen := map[string]bool{}
	industries := make([]string, 0, len(in.Industries))
	for _, raw := range in.Industries {
		industry, err := cleanText("industries", raw, 1, 25, false)
		if err != nil {
			return err
		}
		if key := strings.ToLower(industry); !seen[key] {
			seen[key] = true
			industries = append(industries, industry)
		}
	}
	in.Industries = industries
	if in.About, err = cleanText("about", in.About, 0, 5000, true); err != nil {
		return err
	}
	if in.Website, err = cleanURL("website", in.Website, true); err != nil {
		return err
	}
	social := map[string]string{}
	for platform, link := range in.Social {
		if !slices.Contains(SocialPlatforms, platform) {
			return invalid("social", "social links can be: "+strings.Join(SocialPlatforms, ", "))
		}
		if link, err = cleanURL("social."+platform, link, false); err != nil {
			return err
		}
		if link != "" {
			social[platform] = link
		}
	}
	in.Social = social
	if in.FoundedYear != nil && (*in.FoundedYear < 1800 || *in.FoundedYear > now.Year()) {
		return invalid("founded_year", fmt.Sprintf("founded year must be between 1800 and %d", now.Year()))
	}
	if !geonameID.MatchString(in.HQLocationID) {
		return invalid("hq_location_id", "choose a headquarters location")
	}
	if in.HQAddress, err = cleanText("hq_address", in.HQAddress, 0, 250, false); err != nil {
		return err
	}
	if len(in.BranchLocationIDs) > 200 {
		return invalid("branch_location_ids", "at most 200 branch locations")
	}
	slices.Sort(in.BranchLocationIDs)
	in.BranchLocationIDs = slices.Compact(in.BranchLocationIDs)
	for _, id := range in.BranchLocationIDs {
		if !geonameID.MatchString(id) {
			return invalid("branch_location_ids", "unknown branch location")
		}
	}
	if in.BranchLocationIDs == nil {
		in.BranchLocationIDs = []string{}
	}
	if len(in.Products) > 20 {
		return invalid("products", "at most 20 products")
	}
	for i := range in.Products {
		p := &in.Products[i]
		if p.Name, err = cleanText("products.name", p.Name, 2, 80, false); err != nil {
			return err
		}
		if p.About, err = cleanText("products.about", p.About, 1, 500, true); err != nil {
			return err
		}
		if p.URL, err = cleanURL("products.url", p.URL, false); err != nil {
			return err
		}
	}
	if len(in.Benefits) > 30 {
		return invalid("benefits", "at most 30 benefits")
	}
	for i := range in.Benefits {
		b := &in.Benefits[i]
		if b.Title, err = cleanText("benefits.title", b.Title, 2, 60, false); err != nil {
			return err
		}
		if b.Details, err = cleanText("benefits.details", b.Details, 0, 300, true); err != nil {
			return err
		}
		if !slices.Contains(BenefitIcons, b.Icon) {
			return invalid("benefits.icon", "icon must be one of: "+strings.Join(BenefitIcons, ", "))
		}
	}
	if in.Products == nil {
		in.Products = []Product{}
	}
	if in.Benefits == nil {
		in.Benefits = []Benefit{}
	}
	return nil
}

var geonameID = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)

// ValidateSlug checks a company link: 3-60 lowercase letters, digits and
// inner hyphens, not reserved.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) || strings.Contains(slug, "--") {
		return invalid("slug", "use 3-60 lowercase letters, numbers and single hyphens")
	}
	if slices.Contains(reservedSlugs, slug) {
		return invalid("slug", "that link is reserved")
	}
	return nil
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
		return "", invalid(field, field+" can't contain HTML; use plain text or markdown")
	}
	return s, nil
}

// cleanURL accepts an absolute http(s) URL with a host and no credentials.
func cleanURL(field, raw string, required bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", invalid(field, field+" is required")
		}
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || len(raw) > 500 {
		return "", invalid(field, field+" must be a valid http(s) link")
	}
	return u.String(), nil
}

// ValidationError represents invalid company input
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(field, message string) error { return &ValidationError{Field: field, Message: message} }

// NotFoundError: no such (live) company or member.
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// ForbiddenError: signed in but not allowed (e.g. not a company admin).
type ForbiddenError struct{ Message string }

func (e *ForbiddenError) Error() string { return e.Message }

// ConflictError: slug taken, or a rule like "keep one admin" would break.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool { _, ok := err.(*ValidationError); return ok }

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool { _, ok := err.(*NotFoundError); return ok }

// IsForbidden checks if an error is a ForbiddenError
func IsForbidden(err error) bool { _, ok := err.(*ForbiddenError); return ok }

// IsConflict checks if an error is a ConflictError
func IsConflict(err error) bool { _, ok := err.(*ConflictError); return ok }

// ErrNotFound and ErrNotAdmin are the common cases.
var (
	ErrNotFound = &NotFoundError{Message: "company not found"}
	ErrNotAdmin = &ForbiddenError{Message: "only company admins can do this"}
)

var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// Normalize validates search input; unknown values are errors, not ignored,
// so a client bug can't silently widen a search.
func (c *SearchCriteria) Normalize() error {
	c.Query = strings.TrimSpace(c.Query)
	if utf8.RuneCountInString(c.Query) > 100 {
		return invalid("query", "search text is too long")
	}
	if len(c.Countries) > 50 || len(c.Industries) > 20 || len(c.Sizes) > len(Sizes) {
		return invalid("filters", "too many filter values")
	}
	for _, code := range c.Countries {
		if !countryCode.MatchString(code) {
			return invalid("countries", "countries must be ISO codes like IN")
		}
	}
	for i, industry := range c.Industries {
		c.Industries[i] = strings.ToLower(strings.TrimSpace(industry))
		if c.Industries[i] == "" || utf8.RuneCountInString(c.Industries[i]) > 25 {
			return invalid("industries", "industries must be 1 to 25 characters")
		}
	}
	for _, size := range c.Sizes {
		if !slices.Contains(Sizes, size) {
			return invalid("sizes", "unknown company size")
		}
	}
	if c.Page == 0 {
		c.Page = 1
	}
	if c.PageSize == 0 {
		c.PageSize = 20
	}
	if c.Page < 1 || c.Page > 1000 || c.PageSize < 1 || c.PageSize > 50 {
		return invalid("page", "page must be 1-1000 and page_size 1-50")
	}
	for _, list := range []*[]string{&c.Countries, &c.Industries, &c.Sizes} {
		if *list == nil {
			*list = []string{}
		}
	}
	return nil
}

// FileURL turns a stored image key into its public URL (served by /api/v1/files).
func FileURL(key *string) *string {
	if key == nil || *key == "" {
		return nil
	}
	u := "/api/v1/files/" + *key
	return &u
}
