package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/settings"
	"findJobs/internal/ports"
)

// SettingsRepository is the PostgreSQL adapter for the single settings row.
type SettingsRepository struct {
	pool *pgxpool.Pool
}

// NewSettingsRepository creates the PostgreSQL adapter for settings.
func NewSettingsRepository(pool *pgxpool.Pool) ports.SettingsRepository {
	return &SettingsRepository{pool: pool}
}

const settingsColumns = `daily_free_applications, apply_fee_paise, match_sort_fee_paise, job_post_fee_paise, fx_refresh_time, updated_at,
	free_applicant_visibility_days, free_application_review_days, free_grace_purge_days, free_repost_fee_paise, free_review_extend_fee_paise,
	premium_applicant_visibility_days, premium_application_review_days, premium_grace_purge_days, premium_repost_fee_paise, premium_review_extend_fee_paise,
	candidate_daily_rate_paise, candidate_subscription_duration_days`

func (r *SettingsRepository) Get(ctx context.Context) (*settings.Settings, error) {
	return scanSettings(r.pool.QueryRow(ctx, `SELECT `+settingsColumns+` FROM settings`))
}

func (r *SettingsRepository) Update(ctx context.Context, s *settings.Settings, userID string) (*settings.Settings, error) {
	const query = `
		UPDATE settings SET daily_free_applications = $1, apply_fee_paise = $2, match_sort_fee_paise = $3,
			job_post_fee_paise = $4, fx_refresh_time = $5, updated_by = $6, updated_at = NOW(),
			free_applicant_visibility_days = $7, free_application_review_days = $8, free_grace_purge_days = $9,
			free_repost_fee_paise = $10, free_review_extend_fee_paise = $11,
			premium_applicant_visibility_days = $12, premium_application_review_days = $13, premium_grace_purge_days = $14,
			premium_repost_fee_paise = $15, premium_review_extend_fee_paise = $16,
			candidate_daily_rate_paise = $17, candidate_subscription_duration_days = $18
		RETURNING ` + settingsColumns
	return scanSettings(r.pool.QueryRow(ctx, query,
		s.DailyFreeApplications, s.ApplyFeePaise, s.MatchSortFeePaise, s.JobPostFeePaise, s.FXRefreshTime, userID,
		s.FreeApplicantVisibilityDays, s.FreeApplicationReviewDays, s.FreeGracePurgeDays,
		s.FreeRepostFeePaise, s.FreeReviewExtendFeePaise,
		s.PremiumApplicantVisibilityDays, s.PremiumApplicationReviewDays, s.PremiumGracePurgeDays,
		s.PremiumRepostFeePaise, s.PremiumReviewExtendFeePaise,
		s.CandidateDailyRatePaise, s.CandidateSubscriptionDurationDays))
}

func scanSettings(row interface{ Scan(...any) error }) (*settings.Settings, error) {
	var s settings.Settings
	err := row.Scan(&s.DailyFreeApplications, &s.ApplyFeePaise, &s.MatchSortFeePaise, &s.JobPostFeePaise, &s.FXRefreshTime, &s.UpdatedAt,
		&s.FreeApplicantVisibilityDays, &s.FreeApplicationReviewDays, &s.FreeGracePurgeDays,
		&s.FreeRepostFeePaise, &s.FreeReviewExtendFeePaise,
		&s.PremiumApplicantVisibilityDays, &s.PremiumApplicationReviewDays, &s.PremiumGracePurgeDays,
		&s.PremiumRepostFeePaise, &s.PremiumReviewExtendFeePaise,
		&s.CandidateDailyRatePaise, &s.CandidateSubscriptionDurationDays)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
