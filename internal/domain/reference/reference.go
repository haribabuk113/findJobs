package reference

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Kind is a global taxonomy: skills or designations (manual "Skills and Designations").
type Kind string

const (
	Skill       Kind = "skill"
	Designation Kind = "designation"
)

// ParseKind validates a kind from a URL or query string.
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case Skill, Designation:
		return Kind(s), nil
	}
	return "", &ValidationError{Field: "kind", Message: "kind must be skill or designation"}
}

// Asset kinds of the published static JSON files.
const (
	AssetSkills       = "skills"
	AssetDesignations = "designations"
	AssetLocations    = "locations"
	AssetRates        = "rates"
)

// AssetKind maps a taxonomy kind to its published file kind.
func (k Kind) AssetKind() string {
	if k == Skill {
		return AssetSkills
	}
	return AssetDesignations
}

// Term statuses.
const (
	StatusVerified = "verified"
	StatusPending  = "pending" // custom entry awaiting site admin review
	StatusMerged   = "merged"  // alias of MergedInto
)

// Term is one skill or designation.
type Term struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	MergedInto *int64    `json:"merged_into"`
	CreatedAt  time.Time `json:"created_at"`
	// Uses is how many jobs reference the term (admin list only).
	Uses int `json:"uses"`
}

// TermPage is one page of terms for the admin review queue.
type TermPage struct {
	Terms    []Term `json:"terms"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// MaxNameLength is the longest skill or designation name, in characters.
const MaxNameLength = 80

// CleanedName is validated term text: Display is stored and shown,
// Normalized decides uniqueness ("  Svelte  JS " and "svelte js" are one term).
type CleanedName struct {
	Display    string
	Normalized string
}

// CleanName validates user-typed term text. Markup characters are rejected
// outright, so a term can never carry HTML or script into a page.
func CleanName(raw string) (CleanedName, error) {
	display := strings.Join(strings.Fields(norm.NFKC.String(raw)), " ")
	n := utf8.RuneCountInString(display)
	if n == 0 || n > MaxNameLength {
		return CleanedName{}, &ValidationError{Field: "name", Message: fmt.Sprintf("name must be 1 to %d characters", MaxNameLength)}
	}
	for _, r := range display {
		if unicode.IsControl(r) || strings.ContainsRune("<>{}`\\", r) {
			return CleanedName{}, &ValidationError{Field: "name", Message: "name contains characters that aren't allowed"}
		}
	}
	return CleanedName{Display: display, Normalized: strings.ToLower(display)}, nil
}

// Location is one node of the country -> region -> city tree.
type Location struct {
	ID          string   `json:"id"`
	ParentID    *string  `json:"parent_id"`
	Level       string   `json:"level"`
	Name        string   `json:"name"`
	CountryCode string   `json:"country_code"`
	Lat         *float64 `json:"lat,omitempty"`
	Lng         *float64 `json:"lng,omitempty"`
}

// Rates maps currency code to INR per one unit of that currency.
type Rates map[string]float64

// Validate rejects malformed provider data before it can touch salaries.
func (r Rates) Validate() error {
	if len(r) == 0 || r["INR"] != 1 {
		return fmt.Errorf("rates must include INR = 1")
	}
	for code, rate := range r {
		if len(code) != 3 || strings.ToUpper(code) != code || !(rate > 0) || rate > 1e9 {
			return fmt.Errorf("invalid rate %s = %v", code, rate)
		}
	}
	return nil
}

// Asset is a published static JSON file.
type Asset struct {
	File      string
	Kind      string
	Content   []byte
	SHA256    string
	CreatedAt time.Time
}

// ManifestEntry tells the client which file is current for one asset kind.
type ManifestEntry struct {
	File      string    `json:"file"`
	SHA256    string    `json:"sha256"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidationError represents invalid reference-data input
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string { return e.Message }

// NotFoundError represents a missing term or asset
type NotFoundError struct{ Message string }

// Error implements the error interface
func (e *NotFoundError) Error() string { return e.Message }

// ConflictError represents a name that already exists
type ConflictError struct{ Message string }

// Error implements the error interface
func (e *ConflictError) Error() string { return e.Message }

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool { _, ok := err.(*ValidationError); return ok }

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool { _, ok := err.(*NotFoundError); return ok }

// IsConflict checks if an error is a ConflictError
func IsConflict(err error) bool { _, ok := err.(*ConflictError); return ok }
