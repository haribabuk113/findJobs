package ports

import (
	"context"

	"findJobs/internal/domain/stats"
)

// StatsRepository computes and serves the landing page's platform counters.
type StatsRepository interface {
	// Refresh recomputes every counter (the daily job).
	Refresh(ctx context.Context) error
	// Get returns the last computed counters.
	Get(ctx context.Context) (*stats.Platform, error)
	// PlatformStats returns the extended stats for the God Mode dashboard
	// including report and user metrics.
	PlatformStats(ctx context.Context) (*AdminStats, error)
}

// AdminStats is the platform-wide aggregate for the God Mode dashboard.
type AdminStats struct {
	RegisteredUsers            int `json:"registered_users"`
	HRProfessionals            int `json:"hr_professionals"`
	Companies                  int `json:"companies"`
	OpenJobs                   int `json:"open_jobs"`
	CountriesWithUsers         int `json:"countries_with_users"`
	CountriesWithJobsCompanies int `json:"countries_with_jobs_companies"`
	NewReports24h              int `json:"new_reports_24h"`
	PendingReports             int `json:"pending_reports"`
	NewUsers7d                 int `json:"new_users_7d"`
}
