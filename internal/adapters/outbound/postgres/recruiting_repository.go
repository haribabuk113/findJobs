package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/jobs"
	"findJobs/internal/ports"
)

// RecruitingRepository is the PostgreSQL adapter for posting and managing jobs.
type RecruitingRepository struct {
	pool *pgxpool.Pool
}

// NewRecruitingRepository creates the PostgreSQL adapter for recruiting.
func NewRecruitingRepository(pool *pgxpool.Pool) ports.RecruitingRepository {
	return &RecruitingRepository{pool: pool}
}

func (r *RecruitingRepository) PosterCompanies(ctx context.Context, userID string) ([]jobs.PosterCompany, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.slug, c.name, m.role, c.hq_location_id, c.branch_location_ids
		FROM company_members m JOIN companies c ON c.id = m.company_id
		WHERE m.user_id = $1 AND m.status = 'current' AND c.deleted_at IS NULL ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[jobs.PosterCompany])
}

func (r *RecruitingRepository) MemberCompanyID(ctx context.Context, slug, userID string) (string, error) {
	var id string
	var member bool
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, EXISTS (SELECT 1 FROM company_members m WHERE m.company_id = c.id AND m.user_id = $2 AND m.status = 'current')
		FROM companies c WHERE c.slug = $1 AND c.deleted_at IS NULL`, slug, userID).Scan(&id, &member)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &jobs.NotFoundError{Message: "company not found"}
	}
	if err != nil {
		return "", err
	}
	if !member {
		// Manual: only approved members can attach a job to the company brand.
		return "", &jobs.ForbiddenError{Message: "you can only post under companies you're an approved member of"}
	}
	return id, nil
}

func (r *RecruitingRepository) MissingLocations(ctx context.Context, ids []string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT x FROM unnest($1::text[]) x WHERE NOT EXISTS (SELECT 1 FROM locations l WHERE l.id = x)`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *RecruitingRepository) CurrencyKnown(ctx context.Context, code string) (bool, error) {
	var known bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exchange_rates WHERE currency = $1)`, code).Scan(&known)
	return known, err
}

// INR baseline at post time; the daily rate refresh keeps it current.
const inrMax = `ROUND($8::numeric * (SELECT rate_to_inr FROM exchange_rates WHERE currency = $9), 2)`

func (r *RecruitingRepository) Create(ctx context.Context, rec *jobs.Record) error {
	if rec.SkillIDs == nil {
		rec.SkillIDs = []int64{} // NOT NULL column; nil would be sent as NULL
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jobs (id, share_hash, recruiter_id, company_id, company_name_text, title, designation_id,
			skill_ids, salary_max, currency, normalized_inr_amount, hide_salary, experience_years, location_ids,
			location_type, location_label, work_modes, work_types, notice_periods, relocation, interview_stages,
			interview_rounds, tier, application, external_url, description_md, is_visible)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $10, $8, $9, `+inrMax+`, $11, $12, $13, $14, '', $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24, $25)`,
		rec.ID, rec.ShareHash, rec.RecruiterID, rec.CompanyID, rec.CompanyName, rec.Designation, rec.DesignationID,
		rec.SalaryMax, rec.Currency, rec.SkillIDs, rec.HideSalary, rec.MinExperience, rec.LocationIDs,
		locationType(rec), rec.WorkModes, rec.WorkTypes, rec.NoticePeriods, rec.Relocation, rec.InterviewStages,
		rec.RoundCount(), rec.DBTier, rec.Application, rec.ExternalURL, rec.Description, rec.Listed)
	return err
}

func (r *RecruitingRepository) Editable(ctx context.Context, id, userID string) (string, *string, string, error) {
	var tier, name string
	var slug *string
	err := r.pool.QueryRow(ctx, `
		SELECT j.tier, c.slug, j.company_name_text FROM jobs j LEFT JOIN companies c ON c.id = j.company_id
		WHERE j.id = $1 AND j.recruiter_id = $2`, id, userID).Scan(&tier, &slug, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, "", jobs.ErrJobNotFound
	}
	return tier, slug, name, err
}

func (r *RecruitingRepository) Update(ctx context.Context, rec *jobs.Record) error {
	if rec.SkillIDs == nil {
		rec.SkillIDs = []int64{} // NOT NULL column; nil would be sent as NULL
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs SET company_name_text = $3, title = $4, designation_id = $5, skill_ids = $6,
			salary_max = $7, currency = $8, normalized_inr_amount = ROUND($7::numeric *
				(SELECT rate_to_inr FROM exchange_rates WHERE currency = $8), 2),
			hide_salary = $9, experience_years = $10, location_ids = $11, location_type = $12, work_modes = $13,
			work_types = $14, notice_periods = $15, relocation = $16, interview_stages = $17, interview_rounds = $18,
			application = $19, external_url = $20, description_md = $21, updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2`,
		rec.ID, rec.RecruiterID, rec.CompanyName, rec.Designation, rec.DesignationID, rec.SkillIDs,
		rec.SalaryMax, rec.Currency, rec.HideSalary, rec.MinExperience, rec.LocationIDs, locationType(rec),
		rec.WorkModes, rec.WorkTypes, rec.NoticePeriods, rec.Relocation, rec.InterviewStages, rec.RoundCount(),
		rec.Application, rec.ExternalURL, rec.Description)
	if err == nil && tag.RowsAffected() == 0 {
		return jobs.ErrJobNotFound
	}
	return err
}

func (r *RecruitingRepository) Close(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE jobs SET status = 'closed', is_visible = FALSE, updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return jobs.ErrJobNotFound
	}
	return err
}

func (r *RecruitingRepository) Repost(ctx context.Context, id, userID string) error {
	// Manual: reposting restarts the 30-day window. A company job can only be
	// reposted while the poster is still a current member.
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs j SET status = 'active', posted_at = NOW(), is_visible = (j.tier = 'Premium'), updated_at = NOW()
		WHERE j.id = $1 AND j.recruiter_id = $2 AND (j.company_id IS NULL OR EXISTS (
			SELECT 1 FROM company_members m JOIN companies c ON c.id = m.company_id
			WHERE m.company_id = j.company_id AND m.user_id = $2 AND m.status = 'current' AND c.deleted_at IS NULL))`,
		id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		if _, _, _, err := r.Editable(ctx, id, userID); err != nil {
			return err
		}
		return &jobs.ForbiddenError{Message: "you're no longer a member of this job's company, so it can't be reposted"}
	}
	return nil
}

// state derives active / closed / expired (past the 30-day public window).
const jobState = `CASE WHEN j.status = 'closed' THEN 'closed'
	WHEN j.posted_at < NOW() - INTERVAL '30 days' THEN 'expired' ELSE 'active' END`

func (r *RecruitingRepository) Mine(ctx context.Context, userID, state string) ([]jobs.Posted, error) {
	// Manual: recruiter dashboards keep jobs visible for 60 days.
	rows, err := r.pool.Query(ctx, `
		SELECT * FROM (
			SELECT j.id, j.title, j.tier, COALESCE(c.name, j.company_name_text), c.slug, `+jobState+` AS state,
				j.share_hash, j.views, (SELECT COUNT(*) FROM applications a WHERE a.job_id = j.id), j.posted_at
			FROM jobs j LEFT JOIN companies c ON c.id = j.company_id
			WHERE j.recruiter_id = $1 AND j.posted_at >= NOW() - INTERVAL '60 days') mine
		WHERE $2 = '' OR state = $2 ORDER BY posted_at DESC, id`, userID, state)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[jobs.Posted])
}

func (r *RecruitingRepository) Detail(ctx context.Context, key string, byShareHash bool, viewerID string) (*jobs.Detail, error) {
	keyColumn := "j.id"
	if byShareHash {
		keyColumn = "j.share_hash"
	}
	var d jobs.Detail
	var recruiterID, recruiterName, companySlug *string
	var card jobs.CompanyCard
	var cardName, cardTagline, cardSize, cardWebsite *string
	var listed bool
	var analytics jobs.Analytics
	var rawMin, rawMax *float64
	var rawCurrency string
	targets := append(jobCardTargets(&d.Job),
		&d.Status, &d.Description, &d.InterviewStages, &d.ExternalURL, &recruiterID, &recruiterName,
		&companySlug, &cardName, &cardTagline, &cardSize, &card.LogoKey, &cardWebsite,
		&d.AppliedAt, &d.CanManage, &d.ShareHash, &listed,
		&analytics.Applications, &analytics.HighFit, &analytics.OutboundClicks, &rawMin, &rawMax, &rawCurrency)
	err := r.pool.QueryRow(ctx, `
		SELECT `+jobCardColumns+`, `+jobState+`, j.description_md, j.interview_stages,
			CASE WHEN j.application = 'External' THEN j.external_url ELSE '' END,
			u.id::text, u.name, co.slug, co.name, co.tagline, co.size, co.logo_key, co.website,
			(SELECT a.submitted_at FROM applications a WHERE a.job_id = j.id AND a.user_id = NULLIF($2, '')::uuid),
			-- COALESCE: an anonymous viewer makes the comparison NULL, not false.
			COALESCE(j.recruiter_id = NULLIF($2, '')::uuid OR EXISTS (SELECT 1 FROM company_members m
				WHERE m.company_id = j.company_id AND m.user_id = NULLIF($2, '')::uuid
				AND m.role = 'admin' AND m.status = 'current'), FALSE),
			j.share_hash, (j.tier = 'Premium' AND j.is_visible),
			(SELECT COUNT(*) FROM applications a WHERE a.job_id = j.id),
			(SELECT COUNT(*) FROM applications a WHERE a.job_id = j.id AND a.calculated_match_score_pct >= 70),
			j.outbound_clicks, j.salary_min::float8, j.salary_max::float8, j.currency::text
		FROM jobs j LEFT JOIN companies co ON co.id = j.company_id AND co.deleted_at IS NULL
		LEFT JOIN users u ON u.id = j.recruiter_id
		WHERE `+keyColumn+` = $1`, key, viewerID).Scan(targets...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, jobs.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	// Who may see it: its recruiter and company admins always; everyone else
	// only while it's open and inside the 30-day window, and a free
	// (unlisted) job only through its share link.
	if !d.CanManage && (d.Status != jobs.StateActive || (!byShareHash && !listed)) {
		return nil, jobs.ErrJobNotFound
	}
	if recruiterID != nil {
		d.Recruiter = &jobs.Person{ID: *recruiterID, Name: deref(recruiterName)}
	}
	// Manual: unlinked and free jobs show no company sidebar.
	if companySlug != nil && d.Tier == "Premium" {
		card.Slug, card.Name, card.Tagline, card.Size, card.Website =
			*companySlug, deref(cardName), deref(cardTagline), deref(cardSize), deref(cardWebsite)
		card.LogoURL = companies.FileURL(card.LogoKey)
		d.Company = &card
	}
	if d.CanManage {
		// The job's own team sees its real salary even when hidden from candidates.
		d.SalaryMin, d.SalaryMax, d.Currency = rawMin, rawMax, &rawCurrency
		analytics.Views = d.Views
		if d.Application == "Easy apply" {
			analytics.RecruiterActions = &d.RecruiterActions
		} else {
			analytics.HighFit = nil
		}
		analytics.ExpiresInDays = max(0, 30-int(time.Since(d.PostedAt).Hours()/24))
		d.Analytics = &analytics
	} else {
		d.ShareHash = ""
	}
	return &d, nil
}

func (r *RecruitingRepository) RecordView(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE jobs SET views = views + 1 WHERE id = $1`, id)
	return err
}

func (r *RecruitingRepository) RecordOutbound(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE jobs SET outbound_clicks = outbound_clicks + 1
		WHERE id = $1 AND application = 'External' AND status = 'active'`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return jobs.ErrJobNotFound
	}
	return err
}

func (r *RecruitingRepository) Recruiter(ctx context.Context, userID string) (*jobs.Person, int, error) {
	var p jobs.Person
	var open int
	err := r.pool.QueryRow(ctx, `
		SELECT u.id::text, u.name, (SELECT COUNT(*) FROM jobs j WHERE j.recruiter_id = u.id AND j.status = 'active'
			AND j.is_visible AND j.tier <> 'Free' AND j.posted_at >= NOW() - INTERVAL '30 days')
		FROM users u WHERE u.id = $1`, userID).Scan(&p.ID, &p.Name, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, &jobs.NotFoundError{Message: "recruiter not found"}
	}
	return &p, open, err
}

func (r *RecruitingRepository) Drafts(ctx context.Context, userID string) ([]jobs.Draft, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, form, updated_at FROM job_drafts WHERE user_id = $1 ORDER BY updated_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[jobs.Draft])
}

func (r *RecruitingRepository) CreateDraft(ctx context.Context, userID string, form map[string]any) (*jobs.Draft, error) {
	var d jobs.Draft
	err := r.pool.QueryRow(ctx, `INSERT INTO job_drafts (user_id, form) VALUES ($1, $2) RETURNING id::text, form, updated_at`,
		userID, form).Scan(&d.ID, &d.Form, &d.UpdatedAt)
	return &d, err
}

func (r *RecruitingRepository) UpdateDraft(ctx context.Context, id, userID string, form map[string]any) (*jobs.Draft, error) {
	if uuid.Validate(id) != nil {
		return nil, &jobs.NotFoundError{Message: "draft not found"}
	}
	var d jobs.Draft
	err := r.pool.QueryRow(ctx, `UPDATE job_drafts SET form = $3, updated_at = NOW() WHERE id = $1 AND user_id = $2
		RETURNING id::text, form, updated_at`, id, userID, form).Scan(&d.ID, &d.Form, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &jobs.NotFoundError{Message: "draft not found"}
	}
	return &d, err
}

func (r *RecruitingRepository) DeleteDraft(ctx context.Context, id, userID string) error {
	if uuid.Validate(id) != nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM job_drafts WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func locationType(rec *jobs.Record) string {
	if rec.WorldRemote {
		return "world_remote"
	}
	return "specific"
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
