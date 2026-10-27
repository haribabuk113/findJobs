package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/applications"
	"findJobs/internal/ports"
)

// ApplicationRepository is the Postgres adapter for applications.
type ApplicationRepository struct {
	pool *pgxpool.Pool
}

// NewApplicationRepository creates the PostgreSQL adapter for applications.
func NewApplicationRepository(pool *pgxpool.Pool) ports.ApplicationRepository {
	return &ApplicationRepository{pool: pool}
}

// openJob locks nothing; it reads what applying needs. Open means not
// closed and inside the 30-day window (free share-link jobs included).
func openJob(ctx context.Context, tx pgx.Tx, jobID string) (method string, recruiterID *string, skills []int64, err error) {
	err = tx.QueryRow(ctx, `
		SELECT application, recruiter_id::text, skill_ids FROM jobs
		WHERE id = $1 AND status = 'active' AND posted_at >= NOW() - INTERVAL '30 days'`, jobID).
		Scan(&method, &recruiterID, &skills)
	if errors.Is(err, pgx.ErrNoRows) {
		err = applications.ErrJobNotFound
	}
	return
}

func (r *ApplicationRepository) Apply(ctx context.Context, userID, jobID string, day time.Time, limit int) (*applications.Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	method, recruiterID, jobSkills, err := openJob(ctx, tx, jobID)
	if err != nil {
		return nil, err
	}
	if method != "Easy apply" {
		return nil, applications.ErrNotEasyApply
	}
	if recruiterID != nil && *recruiterID == userID {
		return nil, applications.ErrOwnJob
	}

	// The dossier: profile fields after the privacy rules. Current salary
	// travels only when the candidate doesn't keep it private.
	var d applications.Dossier
	var userSkills []int64
	var currentSalary float64
	var salaryPrivate bool
	err = tx.QueryRow(ctx, `
		SELECT p.name, p.slug, p.contact_email, p.phone, p.location_label, p.experience_years, p.notice_period,
			p.salary_currency, p.expected_salary::float8, p.current_salary::float8, p.hide_current_salary,
			ARRAY(SELECT ps.skill_id FROM profile_skills ps WHERE ps.user_id = p.user_id ORDER BY ps.position),
			ARRAY(SELECT s.name FROM profile_skills ps JOIN skills s ON s.id = ps.skill_id
				WHERE ps.user_id = p.user_id ORDER BY ps.position)
		FROM profiles p WHERE p.user_id = $1`, userID).Scan(
		&d.Name, &d.ProfileSlug, &d.Email, &d.Phone, &d.Location, &d.ExperienceYears, &d.Notice,
		&d.SalaryCurrency, &d.ExpectedSalary, &currentSalary, &salaryPrivate, &userSkills, &d.Skills)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, applications.ErrNoProfile
	}
	if err != nil {
		return nil, err
	}
	if !salaryPrivate {
		d.CurrentSalary = &currentSalary
	}
	dossier, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	pct, matched, unmatched := applications.MatchScore(jobSkills, userSkills)

	var a applications.Application
	err = tx.QueryRow(ctx, `
		INSERT INTO applications (user_id, job_id, apply_type, calculated_match_score_pct, matched_skill_ids,
			unmatched_skill_ids, dossier)
		VALUES ($1, $2, 'platform', $3, $4, $5, $6) RETURNING id, submitted_at`,
		userID, jobID, pct, matched, unmatched, dossier).Scan(&a.ID, &a.SubmittedAt)
	if isUniqueViolation(err) {
		return nil, applications.ErrAlreadyApplied
	}
	if err != nil {
		return nil, err
	}

	// Take one free application, never past the limit (a limit of 0 means none).
	var used int
	err = tx.QueryRow(ctx, `
		INSERT INTO apply_quota (user_id, day_ist, used) SELECT $1, $2, 1 WHERE $3 > 0
		ON CONFLICT (user_id, day_ist) DO UPDATE SET used = apply_quota.used + 1 WHERE apply_quota.used < $3
		RETURNING used`, userID, day, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, applications.ErrQuotaExhausted
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.one(ctx, userID, a.ID)
}

func (r *ApplicationRepository) ApplyExternal(ctx context.Context, userID, jobID string) (*applications.Application, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	method, recruiterID, _, err := openJob(ctx, tx, jobID)
	if err != nil {
		return nil, err
	}
	if method != "External" {
		return nil, applications.ErrNotExternal
	}
	if recruiterID != nil && *recruiterID == userID {
		return nil, applications.ErrOwnJob
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO applications (user_id, job_id, apply_type) VALUES ($1, $2, 'external') RETURNING id`,
		userID, jobID).Scan(&id)
	if isUniqueViolation(err) {
		return nil, applications.ErrAlreadyApplied
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.one(ctx, userID, id)
}

const myApplicationColumns = `a.id, a.job_id, j.title, COALESCE(c.name, j.company_name_text, ''), ` + jobState + `,
	a.apply_type, a.stage, a.calculated_match_score_pct, a.submitted_at
	FROM applications a JOIN jobs j ON j.id = a.job_id LEFT JOIN companies c ON c.id = j.company_id`

func (r *ApplicationRepository) Mine(ctx context.Context, userID string) ([]applications.Application, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+myApplicationColumns+` WHERE a.user_id = $1 ORDER BY a.submitted_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[applications.Application])
}

func (r *ApplicationRepository) one(ctx context.Context, userID, id string) (*applications.Application, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+myApplicationColumns+` WHERE a.user_id = $1 AND a.id = $2`, userID, id)
	if err != nil {
		return nil, err
	}
	a, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[applications.Application])
	return &a, err
}

func (r *ApplicationRepository) QuotaUsed(ctx context.Context, userID string, day time.Time) (int, error) {
	var used int
	err := r.pool.QueryRow(ctx, `SELECT COALESCE((SELECT used FROM apply_quota WHERE user_id = $1 AND day_ist = $2), 0)`,
		userID, day).Scan(&used)
	return used, err
}

// canManage: the job's recruiter or a current admin of its company (the
// same rule as the job page's analytics card).
const canManage = `(j.recruiter_id = $2::uuid OR EXISTS (SELECT 1 FROM company_members m
	WHERE m.company_id = j.company_id AND m.user_id = $2::uuid AND m.role = 'admin' AND m.status = 'current'))`

const applicantColumns = `a.id, a.stage, a.calculated_match_score_pct,
	ARRAY(SELECT s.name FROM unnest(a.matched_skill_ids) WITH ORDINALITY u(id, o) JOIN skills s ON s.id = u.id ORDER BY u.o),
	ARRAY(SELECT s.name FROM unnest(a.unmatched_skill_ids) WITH ORDINALITY u(id, o) JOIN skills s ON s.id = u.id ORDER BY u.o),
	a.submitted_at, a.stage_updated_at, a.dossier`

func scanApplicant(row pgx.CollectableRow) (applications.Applicant, error) {
	var a applications.Applicant
	var dossier []byte
	if err := row.Scan(&a.ID, &a.Stage, &a.MatchPct, &a.MatchedSkills, &a.UnmatchedSkills, &a.SubmittedAt,
		&a.StageUpdatedAt, &dossier); err != nil {
		return a, err
	}
	return a, json.Unmarshal(dossier, &a.Dossier)
}

func (r *ApplicationRepository) Applicants(ctx context.Context, jobID, viewerID string, f applications.ApplicantFilter) ([]applications.Applicant, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT `+canManage+` FROM jobs j WHERE j.id = $1`, jobID, viewerID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !allowed) {
		return nil, applications.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	order := `a.submitted_at DESC`
	if f.Sort == "match" {
		order = `a.calculated_match_score_pct DESC NULLS LAST, a.submitted_at DESC`
	}
	rows, err := r.pool.Query(ctx, `SELECT `+applicantColumns+` FROM applications a
		WHERE a.job_id = $1 AND a.apply_type = 'platform' AND ($2 = '' OR a.stage = $2)
			AND COALESCE(a.calculated_match_score_pct, 0) >= $3
		ORDER BY `+order, jobID, f.Stage, f.MinMatch)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanApplicant)
}

func (r *ApplicationRepository) SetStage(ctx context.Context, applicationID, viewerID, stage string) (*applications.Applicant, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var jobID, current string
	err = tx.QueryRow(ctx, `
		SELECT a.job_id, a.stage FROM applications a JOIN jobs j ON j.id = a.job_id
		WHERE a.id = $1 AND a.apply_type = 'platform' AND `+canManage+` FOR UPDATE OF a`, applicationID, viewerID).
		Scan(&jobID, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, applications.ErrApplicationGone
	}
	if err != nil {
		return nil, err
	}
	if current != stage {
		if _, err := tx.Exec(ctx, `UPDATE applications SET stage = $2, stage_updated_at = NOW() WHERE id = $1`,
			applicationID, stage); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET recruiter_actions = recruiter_actions + 1 WHERE id = $1`, jobID); err != nil {
			return nil, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT `+applicantColumns+` FROM applications a WHERE a.id = $1`, applicationID)
	if err != nil {
		return nil, err
	}
	a, err := pgx.CollectExactlyOneRow(rows, scanApplicant)
	if err != nil {
		return nil, err
	}
	return &a, tx.Commit(ctx)
}

func (r *ApplicationRepository) SavedJobs(ctx context.Context, userID string) ([]applications.SavedJob, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.job_id, s.saved_at,
			EXISTS (SELECT 1 FROM applications a WHERE a.user_id = s.user_id AND a.job_id = s.job_id)
		FROM saved_jobs s WHERE s.user_id = $1 ORDER BY s.saved_at DESC, s.job_id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[applications.SavedJob])
}

func (r *ApplicationRepository) SaveJob(ctx context.Context, userID, jobID string) error {
	// ponytail: two parallel saves can both pass the count check and end one past the cap; harmless.
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO saved_jobs (user_id, job_id)
		SELECT $1::uuid, $2::varchar WHERE EXISTS (SELECT 1 FROM jobs WHERE id = $2::varchar)
			AND (SELECT COUNT(*) FROM saved_jobs WHERE user_id = $1) < $3
		ON CONFLICT DO NOTHING`, userID, jobID, applications.MaxSavedJobs)
	if err != nil || tag.RowsAffected() == 1 {
		return err
	}
	// Nothing inserted: already saved (fine), no such job, or the list is full.
	var saved, exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM saved_jobs WHERE user_id = $1 AND job_id = $2),
		EXISTS (SELECT 1 FROM jobs WHERE id = $2)`, userID, jobID).Scan(&saved, &exists); err != nil {
		return err
	}
	switch {
	case saved:
		return nil
	case !exists:
		return applications.ErrSavedJobUnknown
	default:
		return applications.ErrTooManySaved
	}
}

func (r *ApplicationRepository) UnsaveJobs(ctx context.Context, userID string, jobIDs []string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM saved_jobs WHERE user_id = $1 AND job_id = ANY($2)`, userID, jobIDs)
	return err
}

func (r *ApplicationRepository) ClearApplied(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM saved_jobs s USING applications a
		WHERE s.user_id = $1 AND a.user_id = s.user_id AND a.job_id = s.job_id`, userID)
	return err
}
