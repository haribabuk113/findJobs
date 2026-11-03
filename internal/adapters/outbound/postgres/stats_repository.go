package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/stats"
	"findJobs/internal/ports"
)

// StatsRepository is the Postgres adapter for the landing page counters.
type StatsRepository struct {
	pool *pgxpool.Pool
}

// NewStatsRepository creates the PostgreSQL adapter for platform stats.
func NewStatsRepository(pool *pgxpool.Pool) ports.StatsRepository {
	return &StatsRepository{pool: pool}
}

// openListing matches the listings search shows (job_repository.Search).
const openListing = `j.status = 'active' AND j.is_visible AND j.tier <> 'Free'
	AND j.posted_at >= NOW() - make_interval(days => $1)`

func (r *StatsRepository) Refresh(ctx context.Context) error {
	// ponytail: full counts once a day; fine for the low-resource window, add
	// running counters if these scans ever get slow.
	// A user's country is their profile's current work location's country;
	// profiles without one don't count.
	_, err := r.pool.Exec(ctx, `
		UPDATE platform_stats SET
			registered_users = (SELECT COUNT(*) FROM users WHERE status = 'active'),
			hr_professionals = (SELECT COUNT(*) FROM (
				SELECT m.user_id FROM company_members m JOIN companies c ON c.id = m.company_id AND c.deleted_at IS NULL
				WHERE m.status = 'current' AND m.role IN ('admin', 'hr')
				UNION SELECT recruiter_id FROM jobs WHERE recruiter_id IS NOT NULL) hr),
			companies = (SELECT COUNT(*) FROM companies WHERE deleted_at IS NULL),
			open_jobs = (SELECT COUNT(*) FROM jobs j WHERE `+openListing+`),
			countries_with_users = (SELECT COUNT(DISTINCT l.country_code)
				FROM profiles p JOIN locations l ON l.id = p.current_location_id),
			countries_with_jobs_companies = (SELECT COUNT(*) FROM (
				SELECT hq_country_code FROM companies WHERE deleted_at IS NULL AND hq_country_code IS NOT NULL
				UNION SELECT l.country_code FROM jobs j JOIN locations l ON l.id = ANY(j.location_ids) WHERE `+openListing+`) cc),
			computed_at = NOW()`, jobs.PublicFreshnessDays)
	return err
}

func (r *StatsRepository) Get(ctx context.Context) (*stats.Platform, error) {
	var s stats.Platform
	err := r.pool.QueryRow(ctx, `
		SELECT registered_users, hr_professionals, companies, open_jobs, countries_with_users,
			countries_with_jobs_companies, computed_at
		FROM platform_stats`).Scan(&s.RegisteredUsers, &s.HRProfessionals, &s.Companies, &s.OpenJobs,
		&s.CountriesWithUsers, &s.CountriesWithJobsCompanies, &s.ComputedAt)
	return &s, err
}
