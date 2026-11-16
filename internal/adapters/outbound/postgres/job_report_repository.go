package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// JobReportRepository is the PostgreSQL adapter for job reports / moderation.
type JobReportRepository struct {
	pool *pgxpool.Pool
}

// NewJobReportRepository creates the PostgreSQL adapter for job reports.
func NewJobReportRepository(pool *pgxpool.Pool) ports.JobReportRepository {
	return &JobReportRepository{pool: pool}
}

func (r *JobReportRepository) Create(ctx context.Context, rep *admin.JobReport) error {
	const query = `
		INSERT INTO job_reports (job_id, reporter_id, reason, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, rep.JobID, rep.ReporterID, rep.Reason, admin.ReportNew).
		Scan(&rep.ID, &rep.CreatedAt)
}

func (r *JobReportRepository) List(ctx context.Context, status string, page, pageSize int) (*admin.JobReportPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	from := `FROM job_reports r WHERE r.status = $1`
	args := []any{status}

	query := `SELECT r.id, r.job_id, r.reporter_id, r.reason, r.status, r.moderation_note, r.created_at, r.resolved_at, COUNT(*) OVER() ` + from + `
		ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &admin.JobReportPage{Reports: make([]admin.JobReport, 0, pageSize), Page: page, PageSize: pageSize}
	for rows.Next() {
		var rpt admin.JobReport
		if err := rows.Scan(&rpt.ID, &rpt.JobID, &rpt.ReporterID, &rpt.Reason, &rpt.Status, &rpt.ModerationNote, &rpt.CreatedAt, &rpt.ResolvedAt, &result.Total); err != nil {
			return nil, err
		}
		result.Reports = append(result.Reports, rpt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result.Reports) == 0 && page > 1 {
		// Fallback total count
		r.pool.QueryRow(ctx, `SELECT COUNT(*) `+from, status).Scan(&result.Total)
	}
	return result, nil
}

func (r *JobReportRepository) Resolve(ctx context.Context, id, moderatorID string, reject bool, note string) (*admin.JobReport, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE job_reports SET status = 'resolved', moderation_note = $1, resolved_at = NOW()
		WHERE id = $2`, note, id)
	if err != nil {
		return nil, err
	}

	// When rejecting, hide the reported job from public listing.
	if reject {
		if _, err := r.pool.Exec(ctx, `UPDATE jobs SET is_visible = false WHERE id = $1`, id); err != nil {
			return nil, err
		}
	}

	var rpt admin.JobReport
	err = r.pool.QueryRow(ctx, `
		SELECT id, job_id, reporter_id, reason, status, moderation_note, created_at, resolved_at
		FROM job_reports WHERE id = $1`, id).Scan(&rpt.ID, &rpt.JobID, &rpt.ReporterID, &rpt.Reason, &rpt.Status, &rpt.ModerationNote, &rpt.CreatedAt, &rpt.ResolvedAt)
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}
