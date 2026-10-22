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

const settingsColumns = `daily_free_applications, apply_fee_paise, match_sort_fee_paise, job_post_fee_paise, fx_refresh_time, updated_at`

func (r *SettingsRepository) Get(ctx context.Context) (*settings.Settings, error) {
	return scanSettings(r.pool.QueryRow(ctx, `SELECT `+settingsColumns+` FROM settings`))
}

func (r *SettingsRepository) Update(ctx context.Context, s *settings.Settings, userID string) (*settings.Settings, error) {
	const query = `
		UPDATE settings SET daily_free_applications = $1, apply_fee_paise = $2, match_sort_fee_paise = $3,
			job_post_fee_paise = $4, fx_refresh_time = $5, updated_by = $6, updated_at = NOW()
		RETURNING ` + settingsColumns
	return scanSettings(r.pool.QueryRow(ctx, query,
		s.DailyFreeApplications, s.ApplyFeePaise, s.MatchSortFeePaise, s.JobPostFeePaise, s.FXRefreshTime, userID))
}

func scanSettings(row interface{ Scan(...any) error }) (*settings.Settings, error) {
	var s settings.Settings
	err := row.Scan(&s.DailyFreeApplications, &s.ApplyFeePaise, &s.MatchSortFeePaise, &s.JobPostFeePaise, &s.FXRefreshTime, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
