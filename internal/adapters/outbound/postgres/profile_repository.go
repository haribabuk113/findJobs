package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
)

// ProfileRepository is the Postgres adapter for candidate profiles.
type ProfileRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository creates the PostgreSQL adapter for profiles.
func NewProfileRepository(pool *pgxpool.Pool) ports.ProfileRepository {
	return &ProfileRepository{pool: pool}
}

const profileColumns = `p.user_id, p.slug, p.name, p.headline, p.location_label, p.about, p.about_updated_at, p.photo_key,
	p.gender, p.pronouns, p.contact_email, p.phone, p.experience_years, p.notice_period, p.serving_notice,
	COALESCE(to_char(p.last_working_date, 'YYYY-MM-DD'), ''), p.salary_currency, p.current_salary::float8,
	p.expected_salary::float8, p.hourly_rate::float8, p.work_types, p.location_ids,
	ARRAY(SELECT d.name FROM unnest(p.designation_ids) WITH ORDINALITY AS u(id, ord) JOIN designations d ON d.id = u.id ORDER BY u.ord),
	p.open_to_remote, p.relocation, p.is_public, p.open_to_work, p.hide_current_salary, p.hide_hourly_rate,
	p.hide_contact, p.hide_expected_salary, p.resume_applications_only, p.details, p.website, p.work_mode`

func (r *ProfileRepository) Get(ctx context.Context, userID string) (*profiles.Profile, error) {
	return r.load(ctx, `WHERE p.user_id = $1`, userID)
}

func (r *ProfileRepository) GetPublic(ctx context.Context, slug string) (*profiles.Profile, error) {
	return r.load(ctx, `WHERE p.slug = $1 AND p.is_public`, slug)
}

func (r *ProfileRepository) load(ctx context.Context, where string, arg string) (*profiles.Profile, error) {
	var p profiles.Profile
	var userID string
	var photo *string
	var details []byte
	err := r.pool.QueryRow(ctx, `SELECT `+profileColumns+` FROM profiles p `+where, arg).Scan(
		&userID, &p.Slug, &p.Name, &p.Title, &p.Location, &p.About, &p.AboutUpdated, &photo,
		&p.Gender, &p.Pronouns, &p.Email, &p.Phone, &p.ExperienceYears, &p.Notice, &p.IsServingNotice,
		&p.LastWorkingDate, &p.SalaryCurrency, &p.CurrentSalary, &p.ExpectedSalary, &p.HourlyRate,
		&p.WorkTypes, &p.PreferredLocations, &p.TargetRoles, &p.OpenToRemote, &p.Relocation,
		&p.Visible, &p.Open, &p.CurrentSalaryPrivate, &p.HourlyPrivate, &p.ContactPrivate, &p.SalaryPrivate,
		&p.ResumePrivate, &details, &p.Website, &p.WorkMode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, profiles.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Saved = true
	p.Photo = companies.FileURL(photo)
	if err := json.Unmarshal(details, &p.Details); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ps.group_name, s.name FROM profile_skills ps JOIN skills s ON s.id = ps.skill_id
		WHERE ps.user_id = $1 ORDER BY ps.position`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var group, skill string
		if err := rows.Scan(&group, &skill); err != nil {
			rows.Close()
			return nil, err
		}
		if n := len(p.SkillGroups); n == 0 || p.SkillGroups[n-1].Name != group {
			p.SkillGroups = append(p.SkillGroups, profiles.SkillGroup{Name: group})
		}
		last := &p.SkillGroups[len(p.SkillGroups)-1]
		last.Skills = append(last.Skills, skill)
		p.Skills = append(p.Skills, skill)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Verification is the company's answer to the membership request.
	rows, err = r.pool.Query(ctx, `
		SELECT e.company_name, e.title, e.work_type, e.start_month, COALESCE(e.end_month, ''), e.is_current,
			e.promoted, e.description, COALESCE(c.slug, ''),
			CASE m.status WHEN 'current' THEN 'Verified' WHEN 'past' THEN 'Verified' WHEN 'pending' THEN 'Pending'
				WHEN 'rejected' THEN 'Rejected' ELSE 'Unverified' END
		FROM profile_experiences e
		LEFT JOIN companies c ON c.id = e.company_id AND c.deleted_at IS NULL
		LEFT JOIN company_members m ON m.company_id = c.id AND m.user_id = e.user_id
		WHERE e.user_id = $1 ORDER BY e.position`, userID)
	if err != nil {
		return nil, err
	}
	p.Experiences, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (profiles.Experience, error) {
		var e profiles.Experience
		err := row.Scan(&e.Company, &e.Title, &e.Type, &e.Start, &e.End, &e.Current, &e.Promoted, &e.Description,
			&e.CompanySlug, &e.Verification)
		return e, err
	})
	if err != nil {
		return nil, err
	}
	p.EnsureLists()
	return &p, nil
}

func (r *ProfileRepository) Account(ctx context.Context, userID string) (string, string, error) {
	var name, email string
	err := r.pool.QueryRow(ctx, `SELECT name, email FROM users WHERE id = $1`, userID).Scan(&name, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", profiles.ErrNotFound
	}
	return name, email, err
}

func (r *ProfileRepository) Save(ctx context.Context, userID string, p *profiles.Profile, skills []ports.ProfileSkill, designationIDs []int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var rate float64
	err = tx.QueryRow(ctx, `SELECT rate_to_inr::float8 FROM exchange_rates WHERE currency = $1`, p.SalaryCurrency).Scan(&rate)
	if errors.Is(err, pgx.ErrNoRows) {
		return &profiles.ValidationError{Field: "salary_currency", Message: "unknown currency"}
	}
	if err != nil {
		return err
	}
	var known int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM locations WHERE id = ANY($1)`, p.PreferredLocations).Scan(&known); err != nil {
		return err
	}
	if known != len(p.PreferredLocations) {
		return &profiles.ValidationError{Field: "preferred_locations", Message: "unknown location"}
	}

	p.EnsureLists() // nil slices would be stored as NULL arrays
	if designationIDs == nil {
		designationIDs = []int64{}
	}
	details, err := json.Marshal(p.Details)
	if err != nil {
		return err
	}
	var lastDay *time.Time
	if d, err := time.Parse(time.DateOnly, p.LastWorkingDate); err == nil {
		lastDay = &d
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id, slug, name, headline, location_label, about, about_updated_at, gender, pronouns,
			contact_email, phone, experience_years, notice_period, serving_notice, last_working_date, salary_currency,
			current_salary, expected_salary, expected_salary_inr, hourly_rate, work_types, location_ids, designation_ids,
			open_to_remote, relocation, is_public, open_to_work, hide_current_salary, hide_hourly_rate, hide_contact,
			hide_expected_salary, resume_applications_only, details, website, work_mode)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $6 <> '' THEN NOW() END, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $17 * $18::numeric, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34)
		ON CONFLICT (user_id) DO UPDATE SET slug = EXCLUDED.slug, name = EXCLUDED.name, headline = EXCLUDED.headline,
			location_label = EXCLUDED.location_label, about = EXCLUDED.about,
			about_updated_at = CASE WHEN profiles.about IS DISTINCT FROM EXCLUDED.about THEN NOW() ELSE profiles.about_updated_at END,
			gender = EXCLUDED.gender, pronouns = EXCLUDED.pronouns, contact_email = EXCLUDED.contact_email,
			phone = EXCLUDED.phone, experience_years = EXCLUDED.experience_years, notice_period = EXCLUDED.notice_period,
			serving_notice = EXCLUDED.serving_notice, last_working_date = EXCLUDED.last_working_date,
			salary_currency = EXCLUDED.salary_currency, current_salary = EXCLUDED.current_salary,
			expected_salary = EXCLUDED.expected_salary, expected_salary_inr = EXCLUDED.expected_salary_inr,
			hourly_rate = EXCLUDED.hourly_rate, work_types = EXCLUDED.work_types, location_ids = EXCLUDED.location_ids,
			designation_ids = EXCLUDED.designation_ids, open_to_remote = EXCLUDED.open_to_remote,
			relocation = EXCLUDED.relocation, is_public = EXCLUDED.is_public, open_to_work = EXCLUDED.open_to_work,
			hide_current_salary = EXCLUDED.hide_current_salary, hide_hourly_rate = EXCLUDED.hide_hourly_rate,
			hide_contact = EXCLUDED.hide_contact, hide_expected_salary = EXCLUDED.hide_expected_salary,
			resume_applications_only = EXCLUDED.resume_applications_only, details = EXCLUDED.details,
			website = EXCLUDED.website, work_mode = EXCLUDED.work_mode, updated_at = NOW()`,
		userID, p.Slug, p.Name, p.Title, p.Location, p.About, p.Gender, p.Pronouns, p.Email, p.Phone,
		p.ExperienceYears, p.Notice, p.IsServingNotice, lastDay, p.SalaryCurrency, p.CurrentSalary, p.ExpectedSalary,
		rate, p.HourlyRate, p.WorkTypes, p.PreferredLocations, designationIDs, p.OpenToRemote, p.Relocation,
		p.Visible, p.Open, p.CurrentSalaryPrivate, p.HourlyPrivate, p.ContactPrivate, p.SalaryPrivate,
		p.ResumePrivate, details, p.Website, p.WorkMode)
	if isUniqueViolation(err) {
		return profiles.ErrSlugTaken
	}
	if err != nil {
		return err
	}

	ids, groups := make([]int64, len(skills)), make([]string, len(skills))
	for i, s := range skills {
		ids[i], groups[i] = s.SkillID, s.Group
	}
	if _, err := tx.Exec(ctx, `DELETE FROM profile_skills WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO profile_skills (user_id, skill_id, group_name, position)
		SELECT $1, id, g, ord FROM unnest($2::bigint[], $3::text[]) WITH ORDINALITY AS u(id, g, ord)`,
		userID, ids, groups); err != nil {
		return err
	}

	n := len(p.Experiences)
	company, title, kind, start, end, desc := make([]string, n), make([]string, n), make([]string, n), make([]string, n), make([]*string, n), make([]string, n)
	current, promoted := make([]bool, n), make([]bool, n)
	for i, e := range p.Experiences {
		company[i], title[i], kind[i], start[i], desc[i] = e.Company, e.Title, e.Type, e.Start, e.Description
		current[i], promoted[i] = e.Current, e.Promoted
		if e.End != "" {
			end[i] = &e.End
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM profile_experiences WHERE user_id = $1`, userID); err != nil {
		return err
	}
	// A company name that matches a registered company links to it, so the
	// company can verify the entry (oldest company wins a name clash).
	if _, err := tx.Exec(ctx, `
		INSERT INTO profile_experiences (user_id, position, company_id, company_name, title, work_type, start_month,
			end_month, is_current, promoted, description)
		SELECT $1, ord, (SELECT c.id FROM companies c WHERE lower(c.name) = lower(u.company) AND c.deleted_at IS NULL
				ORDER BY c.created_at, c.id LIMIT 1),
			company, title, kind, start, finish, cur, prom, descr
		FROM unnest($2::text[], $3::text[], $4::text[], $5::text[], $6::text[], $7::bool[], $8::bool[], $9::text[])
			WITH ORDINALITY AS u(company, title, kind, start, finish, cur, prom, descr, ord)`,
		userID, company, title, kind, start, end, current, promoted, desc); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ProfileRepository) SetPhoto(ctx context.Context, userID, key string) (string, error) {
	var old *string
	err := r.pool.QueryRow(ctx, `
		UPDATE profiles p SET photo_key = $2, updated_at = NOW() FROM profiles prev
		WHERE p.user_id = $1 AND prev.user_id = p.user_id RETURNING prev.photo_key`, userID, key).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", profiles.ErrNotFound
	}
	if err != nil || old == nil {
		return "", err
	}
	return *old, nil
}

func (r *ProfileRepository) Experience(ctx context.Context, userID string, index int) (string, string, string, error) {
	var companyID *string
	var title, start string
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, e.title, e.start_month FROM profile_experiences e
		LEFT JOIN companies c ON c.id = e.company_id AND c.deleted_at IS NULL
		WHERE e.user_id = $1 AND e.position = $2`, userID, index+1).Scan(&companyID, &title, &start)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", &profiles.NotFoundError{Message: "experience not found"}
	}
	if err != nil || companyID == nil {
		return "", title, start, err
	}
	return *companyID, title, start, nil
}

func (r *ProfileRepository) Skills(ctx context.Context, userID string) ([]profiles.SkillRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.name FROM profile_skills ps JOIN skills s ON s.id = ps.skill_id
		WHERE ps.user_id = $1 ORDER BY ps.position`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (profiles.SkillRef, error) {
		var s profiles.SkillRef
		err := row.Scan(&s.ID, &s.Name)
		return s, err
	})
}

func (r *ProfileRepository) AddSkill(ctx context.Context, userID string, skillID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock the profile so concurrent adds can't pass the limit together.
	var count int
	err = tx.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM profile_skills WHERE user_id = p.user_id)
		FROM profiles p WHERE p.user_id = $1 FOR UPDATE`, userID).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return profiles.ErrNoProfile
	}
	if err != nil {
		return err
	}
	var master int64
	err = tx.QueryRow(ctx, `SELECT COALESCE(merged_into, id) FROM skills WHERE id = $1`, skillID).Scan(&master)
	if errors.Is(err, pgx.ErrNoRows) {
		return profiles.ErrUnknownSkill
	}
	if err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM profile_skills WHERE user_id = $1 AND skill_id = $2)`,
		userID, master).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	if count >= profiles.MaxSkills {
		return profiles.ErrTooManySkills
	}
	// The end of the last group keeps every group contiguous by position.
	if _, err := tx.Exec(ctx, `
		INSERT INTO profile_skills (user_id, skill_id, group_name, position)
		SELECT $1, $2, COALESCE(last.group_name, 'Core skills'), COALESCE(last.position, 0) + 1
		FROM (SELECT 1) one LEFT JOIN LATERAL (
			SELECT group_name, position FROM profile_skills WHERE user_id = $1 ORDER BY position DESC LIMIT 1
		) last ON TRUE`, userID, master); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ProfileRepository) RemoveSkill(ctx context.Context, userID string, skillID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM profile_skills WHERE user_id = $1 AND skill_id = $2`, userID, skillID)
	return err
}
