package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/ports"
)

// JobRepository is the Postgres adapter for job search.
type JobRepository struct {
	pool *pgxpool.Pool
}

// NewJobRepository creates the PostgreSQL adapter for job search.
func NewJobRepository(pool *pgxpool.Pool) ports.JobRepository {
	return &JobRepository{pool: pool}
}

// query accumulates WHERE clauses with positional args. Every user value goes
// through args — never into the SQL string.
type query struct {
	where []string
	args  []any
}

func (q *query) arg(v any) string {
	q.args = append(q.args, v)
	return fmt.Sprintf("$%d", len(q.args))
}

func (q *query) add(clause string, values ...any) {
	for _, v := range values {
		clause = strings.Replace(clause, "?", q.arg(v), 1)
	}
	q.where = append(q.where, clause)
}

// anyOf adds "column = ANY(values)" (OR within one category) when values is set.
func (q *query) anyOf(column string, values []string) {
	if len(values) > 0 {
		q.add(column+" = ANY(?)", values)
	}
}

func (r *JobRepository) Search(ctx context.Context, c *jobs.SearchCriteria, userID string) (*jobs.SearchResult, error) {
	q := &query{}
	// Public visibility: open, listed tier, cron-visible, and the 30-day hard cap.
	q.add("j.status = 'active' AND j.is_visible AND j.tier <> 'Free'")
	q.add("j.posted_at >= NOW() - make_interval(days => ?)", min(c.PostedWithin, jobs.PublicFreshnessDays))
	if userID != "" {
		q.add("NOT EXISTS (SELECT 1 FROM applications a WHERE a.job_id = j.id AND a.user_id = ?::uuid)", userID)
	}

	// Every keyword must appear in the title (requirements: search is by
	// designation/title only, never the description).
	for _, word := range strings.Fields(strings.ToLower(c.Query)) {
		q.add("j.title ILIKE ?", "%"+escapeLike(word)+"%")
	}
	if c.LocationText != "" {
		if strings.EqualFold(c.LocationText, "remote") {
			q.add("'Remote' = ANY(j.work_modes)")
		} else {
			q.add("j.location_label ILIKE ?", "%"+escapeLike(c.LocationText)+"%")
		}
	}
	if c.WorldRemote {
		q.add("j.location_type = 'world_remote'")
	}
	if len(c.LocationIDs) > 0 {
		q.add("j.location_ids && ?", c.LocationIDs)
	}
	if len(c.Skills) > 0 {
		q.add("j.skill_ids && ?::bigint[]", c.Skills)
	}
	if len(c.Designations) > 0 {
		q.add("j.designation_id = ANY(?::bigint[])", c.Designations)
	}
	if len(c.WorkModes) > 0 {
		q.add("j.work_modes && ?", c.WorkModes)
	}
	if len(c.WorkTypes) > 0 {
		q.add("j.work_types && ?", c.WorkTypes)
	}
	if c.Company != "" {
		q.add("co.slug = ?", c.Company)
	}
	if c.RecruiterID != "" {
		q.add("j.recruiter_id = ?::uuid", c.RecruiterID)
	}
	if len(c.NoticePeriods) > 0 {
		q.add("j.notice_periods && ?", c.NoticePeriods)
	}
	if len(c.Benefits) > 0 {
		q.add("j.benefits && ?", c.Benefits)
	}
	q.anyOf("j.relocation", c.Relocation)
	q.anyOf("j.category", c.Categories)
	q.anyOf("j.level", c.Levels)
	q.anyOf("j.tier", c.Tiers)
	q.anyOf("j.application", c.Applications)
	q.anyOf("co.sector", c.Industries)
	q.anyOf("co.size", c.CompanySizes)
	q.anyOf("j.id", c.JobIDs)

	// Salary works on the INR baseline. Hidden-salary jobs still match (their
	// amounts stay indexed); only the response strips the numbers.
	if c.MinSalaryINR > 0 {
		q.add("j.normalized_inr_amount >= ?", c.MinSalaryINR)
	}
	if c.MaxSalaryINR > 0 {
		q.add("COALESCE(j.salary_min_inr, j.normalized_inr_amount) <= ?", c.MaxSalaryINR)
	}
	if c.MinExperience != nil {
		q.add("j.experience_years >= ?", *c.MinExperience)
	}
	if c.MaxExperience != nil {
		q.add("j.experience_years <= ?", *c.MaxExperience)
	}
	if c.Rounds != nil {
		if c.RoundsExact {
			q.add("j.interview_rounds = ?", *c.Rounds)
		} else {
			q.add("j.interview_rounds <= ?", *c.Rounds)
		}
	}

	order := map[string]string{
		"newest": "j.posted_at DESC, j.id",
		"salary": "j.normalized_inr_amount DESC NULLS LAST, j.posted_at DESC, j.id",
		"title":  "j.title, j.id",
	}[c.Sort]

	sql := `SELECT ` + jobCardColumns + `, COUNT(*) OVER ()
		FROM jobs j LEFT JOIN companies co ON co.id = j.company_id
		WHERE ` + strings.Join(q.where, "\n\t\t\tAND ") + `
		ORDER BY ` + order + `
		LIMIT ` + q.arg(c.PageSize) + ` OFFSET ` + q.arg((c.Page-1)*c.PageSize)

	rows, err := r.pool.Query(ctx, sql, q.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &jobs.SearchResult{Jobs: []jobs.Job{}, Page: c.Page, PageSize: c.PageSize}
	for rows.Next() {
		var j jobs.Job
		if err := rows.Scan(append(jobCardTargets(&j), &result.Total)...); err != nil {
			return nil, err
		}
		result.Jobs = append(result.Jobs, j)
	}
	return result, rows.Err()
}

// jobCardColumns is the job card shape shared by search and job details
// (alias j = jobs, co = companies LEFT JOINed). Hidden salaries are nulled in
// SQL, so the amounts never leave the database.
const jobCardColumns = `j.id, j.title, j.company_id, co.slug, COALESCE(co.name, j.company_name_text),
	COALESCE(co.sector, ''), COALESCE(co.size, ''), j.designation_id, j.category, j.level, j.summary,
	j.work_modes, j.work_types, j.location_type, j.location_label, j.location_ids,
	CASE WHEN j.hide_salary THEN NULL ELSE j.salary_min::float8 END,
	CASE WHEN j.hide_salary THEN NULL ELSE j.salary_max::float8 END,
	CASE WHEN j.hide_salary THEN NULL ELSE j.currency::text END,
	CASE WHEN j.hide_salary THEN NULL ELSE j.normalized_inr_amount::float8 END,
	j.hide_salary, j.skill_ids,
	ARRAY(SELECT s.name FROM unnest(j.skill_ids) WITH ORDINALITY AS u(id, ord) JOIN skills s ON s.id = u.id ORDER BY u.ord),
	j.notice_periods, j.benefits, j.relocation, j.experience_years, j.interview_rounds, j.tier, j.application,
	j.views, j.recruiter_actions, j.posted_at`

func jobCardTargets(j *jobs.Job) []any {
	return []any{&j.ID, &j.Title, &j.CompanyID, &j.CompanySlug, &j.CompanyName, &j.CompanySector, &j.CompanySize,
		&j.DesignationID, &j.Category, &j.Level, &j.Summary, &j.WorkModes, &j.WorkTypes, &j.LocationType, &j.Location,
		&j.LocationIDs, &j.SalaryMin, &j.SalaryMax, &j.Currency, &j.SalaryMaxINR, &j.SalaryHidden, &j.SkillIDs,
		&j.Skills, &j.NoticePeriods, &j.Benefits, &j.Relocation, &j.ExperienceYears, &j.InterviewRounds, &j.Tier,
		&j.Application, &j.Views, &j.RecruiterActions, &j.PostedAt}
}

// escapeLike stops user input acting as LIKE wildcards.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
