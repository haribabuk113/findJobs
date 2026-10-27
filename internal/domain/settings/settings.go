package settings

import (
	"regexp"
	"time"
)

// Settings are the admin-configurable platform values (manual: "Search
// jobs", "Post a job", "Salary"). Money is in paise (₹1 = 100).
type Settings struct {
	DailyFreeApplications int       `json:"daily_free_applications"`
	ApplyFeePaise         int64     `json:"apply_fee_paise"`
	MatchSortFeePaise     int64     `json:"match_sort_fee_paise"`
	JobPostFeePaise       int64     `json:"job_post_fee_paise"`
	FXRefreshTime         string    `json:"fx_refresh_time"` // HH:MM, IST
	UpdatedAt             time.Time `json:"updated_at"`
}

// PublicSettings is what anyone may read: limits and prices, nothing operational.
type PublicSettings struct {
	DailyFreeApplications int   `json:"daily_free_applications"`
	ApplyFeePaise         int64 `json:"apply_fee_paise"`
	MatchSortFeePaise     int64 `json:"match_sort_fee_paise"`
	JobPostFeePaise       int64 `json:"job_post_fee_paise"`
}

// Public returns the publicly readable subset.
func (s *Settings) Public() PublicSettings {
	return PublicSettings{s.DailyFreeApplications, s.ApplyFeePaise, s.MatchSortFeePaise, s.JobPostFeePaise}
}

// MaxFeePaise caps every fee at ₹1,00,000 so a typo can't price the site out.
const MaxFeePaise = 10_000_000

var clockTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Validate checks the values an admin submitted. The table's CHECK
// constraints repeat these limits as the last line of defence.
func (s *Settings) Validate() error {
	if s.DailyFreeApplications < 0 || s.DailyFreeApplications > 1000 {
		return &ValidationError{Field: "daily_free_applications", Message: "daily_free_applications must be between 0 and 1000"}
	}
	for field, fee := range map[string]int64{
		"apply_fee_paise":      s.ApplyFeePaise,
		"match_sort_fee_paise": s.MatchSortFeePaise,
		"job_post_fee_paise":   s.JobPostFeePaise,
	} {
		if fee < 0 || fee > MaxFeePaise {
			return &ValidationError{Field: field, Message: field + " must be between 0 and 10000000"}
		}
	}
	if !clockTime.MatchString(s.FXRefreshTime) {
		return &ValidationError{Field: "fx_refresh_time", Message: "fx_refresh_time must be HH:MM (24-hour, IST)"}
	}
	return nil
}

// ValidationError represents invalid settings
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
