// Package stats holds the landing page's platform counters.
package stats

import "time"

// Platform is the landing page's six counters (requirements doc "Platform
// Analytics"), computed once a day. ComputedAt is nil until the first run.
type Platform struct {
	RegisteredUsers int64 `json:"registered_users"`
	// HRProfessionals: people who can recruit (current admins and HR of a
	// live company) or have posted a job.
	HRProfessionals int64 `json:"hr_professionals"`
	Companies       int64 `json:"companies"`
	// OpenJobs: listings search shows (active, visible, listed, at most 30 days old).
	OpenJobs int64 `json:"open_jobs"`
	// CountriesWithUsers: countries of the locations on candidate profiles.
	CountriesWithUsers int64 `json:"countries_with_users"`
	// CountriesWithJobsCompanies: company headquarters plus open jobs' locations.
	CountriesWithJobsCompanies int64      `json:"countries_with_jobs_companies"`
	ComputedAt                 *time.Time `json:"computed_at"`
}
