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

	// Tier configuration (manual: God Mode tier settings).
	FreeApplicantVisibilityDays   int   `json:"free_applicant_visibility_days"`
	FreeApplicationReviewDays     int   `json:"free_application_review_days"`
	FreeGracePurgeDays            int   `json:"free_grace_purge_days"`
	FreeRepostFeePaise            int64 `json:"free_repost_fee_paise"`
	FreeReviewExtendFeePaise      int64 `json:"free_review_extend_fee_paise"`
	PremiumApplicantVisibilityDays int `json:"premium_applicant_visibility_days"`
	PremiumApplicationReviewDays   int `json:"premium_application_review_days"`
	PremiumGracePurgeDays           int `json:"premium_grace_purge_days"`
	PremiumRepostFeePaise            int64 `json:"premium_repost_fee_paise"`
	PremiumReviewExtendFeePaise       int64 `json:"premium_review_extend_fee_paise"`
	CandidateDailyRatePaise       int64 `json:"candidate_daily_rate_paise"`
	CandidateSubscriptionDurationDays []int `json:"candidate_subscription_duration_days"`
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
		"apply_fee_paise":        s.ApplyFeePaise,
		"match_sort_fee_paise":   s.MatchSortFeePaise,
		"job_post_fee_paise":     s.JobPostFeePaise,
		"free_repost_fee_paise":  s.FreeRepostFeePaise,
		"free_review_extend_fee_paise": s.FreeReviewExtendFeePaise,
		"premium_repost_fee_paise":     s.PremiumRepostFeePaise,
		"premium_review_extend_fee_paise": s.PremiumReviewExtendFeePaise,
		"candidate_daily_rate_paise":      s.CandidateDailyRatePaise,
	} {
		if fee < 0 || fee > MaxFeePaise {
			return &ValidationError{Field: field, Message: field + " must be between 0 and 10000000"}
		}
	}
	if !clockTime.MatchString(s.FXRefreshTime) {
		return &ValidationError{Field: "fx_refresh_time", Message: "fx_refresh_time must be HH:MM (24-hour, IST)"}
	}
	if s.FreeApplicantVisibilityDays < 0 || s.FreeApplicantVisibilityDays > 365 {
		return &ValidationError{Field: "free_applicant_visibility_days", Message: "free_applicant_visibility_days must be between 0 and 365"}
	}
	if s.FreeApplicationReviewDays < 0 || s.FreeApplicationReviewDays > 365 {
		return &ValidationError{Field: "free_application_review_days", Message: "free_application_review_days must be between 0 and 365"}
	}
	if s.FreeGracePurgeDays < 0 || s.FreeGracePurgeDays > 365 {
		return &ValidationError{Field: "free_grace_purge_days", Message: "free_grace_purge_days must be between 0 and 365"}
	}
	if s.PremiumApplicantVisibilityDays < 0 || s.PremiumApplicantVisibilityDays > 365 {
		return &ValidationError{Field: "premium_applicant_visibility_days", Message: "premium_applicant_visibility_days must be between 0 and 365"}
	}
	if s.PremiumApplicationReviewDays < 0 || s.PremiumApplicationReviewDays > 365 {
		return &ValidationError{Field: "premium_application_review_days", Message: "premium_application_review_days must be between 0 and 365"}
	}
	if s.PremiumGracePurgeDays < 0 || s.PremiumGracePurgeDays > 365 {
		return &ValidationError{Field: "premium_grace_purge_days", Message: "premium_grace_purge_days must be between 0 and 365"}
	}
	for _, d := range s.CandidateSubscriptionDurationDays {
		if d < 1 || d > 365 {
			return &ValidationError{Field: "candidate_subscription_duration_days", Message: "candidate_subscription_duration_days must each be between 1 and 365"}
		}
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
