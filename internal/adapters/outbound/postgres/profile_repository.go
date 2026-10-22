package postgres

import (
	"context"
	"database/sql"
	"errors"
	apperror "findJobs/internal/utils/errors"
	"net/http"

	"findJobs/internal/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	models "findJobs/internal/domain/profiles"
)

// ProfileRepository is the outbound adapter implementing the application port.
type ProfileRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository creates the PostgreSQL adapter for profile persistence.
func NewProfileRepository(pool *pgxpool.Pool) ports.ProfileRepository {
	return &ProfileRepository{pool: pool}
}

// One round trip: scalar columns + one JSON array per child collection.
// Each json_agg is a correlated subquery, so there is no row multiplication
// between collections (no cartesian explosion from joining many 1-to-N tables).
const profileQuery = `
SELECT
    c.candidate_id,
    c.full_name,
    c.profile_slug,
    c.profile_photo_url,
    c.gender,
    c.headline_text,
    c.headline_designation_id,
    c.about_summary,
    c.about_summary_updated_at,

    COALESCE((SELECT json_agg(p.pronoun ORDER BY p.pronoun)
              FROM candidate_pronouns p
              WHERE p.candidate_id = c.candidate_id), '[]'::json)              AS pronouns,

    COALESCE((SELECT json_agg(json_build_object('platform', h.platform, 'url', h.url) ORDER BY h.id)
              FROM candidate_social_handles h
              WHERE h.candidate_id = c.candidate_id), '[]'::json)              AS social_handles,

    cs.current_amount::float8,
    cs.current_currency,
    COALESCE(cs.current_is_hidden, FALSE),
    cs.expected_amount::float8,
    cs.expected_currency,

    COALESCE((SELECT json_agg(json_build_object('designation_id', d.designation_id, 'name', d.name)
                              ORDER BY d.name)
              FROM candidate_target_roles tr
              JOIN master_designations d ON d.designation_id = tr.designation_id
              WHERE tr.candidate_id = c.candidate_id), '[]'::json)             AS target_roles,

    COALESCE((SELECT json_agg(json_build_object('location_id', l.location_id, 'name', l.name, 'level', l.level)
                              ORDER BY l.name)
              FROM candidate_target_locations tl
              JOIN master_locations l ON l.location_id = tl.location_id
              WHERE tl.candidate_id = c.candidate_id), '[]'::json)             AS target_locations,

    COALESCE(pref.open_to_remote, FALSE),

    COALESCE((SELECT json_agg(w.work_type ORDER BY w.work_type)
              FROM candidate_work_types w
              WHERE w.candidate_id = c.candidate_id), '[]'::json)              AS work_types,

    COALESCE((SELECT json_agg(json_build_object(
                  'timeline_id',         t.timeline_id,
                  'company',             CASE
                                           WHEN t.company_id IS NOT NULL
                                             THEN json_build_object('company_id', mc.company_id, 'name', mc.name)
                                           WHEN t.company_name_raw IS NOT NULL
                                             THEN json_build_object('company_id', NULL, 'name', t.company_name_raw)
                                         END,
                  'designation_title',   t.designation_title,
                  'start_date',          t.start_date,
                  'end_date',            t.end_date,
                  'is_current',          t.is_current,
                  'description',         t.description,
                  'verification_status', t.verification_status
              ) ORDER BY t.is_current DESC, t.start_date DESC, t.timeline_id DESC)
              FROM candidate_timeline t
              LEFT JOIN master_companies mc ON mc.company_id = t.company_id
              WHERE t.candidate_id = c.candidate_id), '[]'::json)              AS timeline,

    COALESCE((SELECT json_agg(json_build_object(
                  'group_id',      g.group_id,
                  'group_name',    g.group_name,
                  'display_order', g.display_order,
                  'skills',        COALESCE((SELECT json_agg(json_build_object(
                                                   'skill_id',      s.skill_id,
                                                   'name',          s.name,
                                                   'display_order', i.display_order
                                             ) ORDER BY i.display_order, i.id)
                                             FROM candidate_skill_group_items i
                                             JOIN master_skills s ON s.skill_id = i.skill_id
                                             WHERE i.group_id = g.group_id), '[]'::json)
              ) ORDER BY g.display_order, g.group_id)
              FROM candidate_skill_groups g
              WHERE g.candidate_id = c.candidate_id), '[]'::json)              AS skill_groups,

    COALESCE(cc.email, c.email),
    cc.phone_number,
    loc.location_id,
    loc.name,

    r.access_control,
    r.public_short_link,

    --Add Work Authorization query

FROM candidates c
LEFT JOIN candidate_salary      cs   ON cs.candidate_id   = c.candidate_id
LEFT JOIN candidate_preferences pref ON pref.candidate_id = c.candidate_id
LEFT JOIN candidate_contact     cc   ON cc.candidate_id   = c.candidate_id
LEFT JOIN master_locations      loc  ON loc.location_id   = cc.primary_location_id
LEFT JOIN candidate_resume      r    ON r.candidate_id    = c.candidate_id
WHERE c.profile_slug = $1
  AND c.status = 'active'
`

/*
 COALESCE((SELECT json_agg(json_build_object(
               'country',            wa.country,
               'authorization_type', wa.authorization_type,
               'valid_from',         wa.valid_from,
               'valid_until',        wa.valid_until,
               'is_indefinite',      wa.is_indefinite
           ) ORDER BY wa.valid_until DESC NULLS FIRST, wa.id)
           FROM candidate_work_authorization wa
           WHERE wa.candidate_id = c.candidate_id), '[]'::json)             AS work_authorization
*/

// var ErrNotFound = errors.New("profile not found")

// func (r *ProfileRepository) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
// 	// TODO: replace stub with real query against the database.
// 	return []profiles.ProfileDetails{}, nil
// }

// func (r *ProfileRepository) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error {
// 	// TODO: replace stub with real persistence against the database.
// 	return nil
// }

// func (r *ProfileRepository) GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error) {
// 	var profileDetails profiles.ProfileDetails

// 	query := `SELECT id, iam_user_id, email, status FROM candidates WHERE iam_user_id = $1`
// 	var value any
// 	if userID > 0 {
// 		query += ` WHERE candidate_id = $1`
// 		value = userID
// 	} else if userSlug != "" {
// 		query += ` WHERE profile_slug = $1`
// 		value = userSlug
// 	}
// 	row := r.pool.QueryRow(ctx, query, value)
// 	CandidateProfileDtoMapper(ctx, row, &profileDetails)

// 	p, err := a.LoadProfile(r.Context(), r.PathValue("slug"))
// 	switch {
// 	case errors.Is(err, ErrNotFound):
// 		writeErr(w, http.StatusNotFound, "profile not found")
// 	case err != nil:
// 		writeErr(w, http.StatusInternalServerError, "internal error")
// 	default:
// 		writeJSON(w, http.StatusOK, p)
// 	}

// 	return profileDetails, nil
// }

// func (r *ProfileRepository) UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
// 	return "", nil
// }

// func (r *ProfileRepository) GetProfileDetails(ctx context.Context, userID int64) (profiles.Profile, error) {
// 	// TODO: replace stub with real query against the database.
// 	return profiles.Profile{}, nil
// }

// func (r *ProfileRepository) GetProfilePhoto(ctx context.Context, suserID int64) (profiles.ProfilePhotoResponse, error) {
// 	// TODO: replace stub with real persistence against the database.
// 	return profiles.ProfilePhotoResponse{}, nil
// }

// func (r *ProfileRepository) SetProfilePhotoURL(ctx context.Context, userID int64, url string) error {
// 	// TODO: replace stub with real persistence against the database.
// 	return nil
// }

// func (r *ProfileRepository) GetProfileSlug(ctx context.Context, userID int64) (string, error) {
// 	// TODO: replace stub with real persistence against the database.
// 	return "", nil
// }

// func (r *ProfileRepository) UpdateProfileSlug(ctx context.Context, slug string, userID int64) (string, error) {
// 	// TODO: replace stub with real persistence against the database.
// 	return "", nil
// }

func (r *ProfileRepository) GetRecordBySlug(ctx context.Context, slug string) (*models.ProfileRecord, error) {
	var (
		p                                                                      models.ProfileRecord
		pronounsJ, socialJ, rolesJ, locsJ, workTypesJ, timelineJ, skillGroupsJ []byte
		locID                                                                  *int64
		locName                                                                *string
	)

	err := r.pool.QueryRow(ctx, profileQuery, slug).Scan(
		&p.CandidateID, &p.FullName, &p.ProfileSlug, &p.ProfilePhotoURL, &p.Gender,
		&p.HeadlineText, &p.HeadlineDesignationID, &p.AboutSummary, &p.AboutSummaryUpdatedAt,
		&pronounsJ, &socialJ,
		&p.Salary.CurrentAmount, &p.Salary.CurrentCurrency, &p.Salary.CurrentIsHidden,
		&p.Salary.ExpectedAmount, &p.Salary.ExpectedCurrency,
		&rolesJ, &locsJ, &p.OpenToRemote, &workTypesJ,
		&timelineJ, &skillGroupsJ,
		&p.Email, &p.Phone, &locID, &locName,
		&p.ResumeAccessControl, &p.ResumeShortLink,
		// &waJ,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &apperror.Error{Status: http.StatusNotFound, Message: "profile not found"}
		}
		return nil, translate(err)
	}

	for _, d := range []struct {
		raw []byte
		dst any
	}{
		{pronounsJ, &p.Pronouns},
		{socialJ, &p.SocialHandles},
		{rolesJ, &p.TargetRoles},
		{locsJ, &p.TargetLocations},
		{workTypesJ, &p.WorkTypes},
		{timelineJ, &p.Timeline},
		{skillGroupsJ, &p.SkillGroups},
		// {waJ, &p.WorkAuthorizations},
	} {
		if err := decodeInto(d.raw, d.dst); err != nil {
			return nil, err
		}
	}

	if locID != nil && locName != nil {
		p.PrimaryLocation = &models.LocationRef{LocationID: *locID, Name: *locName}
	}
	return &p, nil
}

func (r *ProfileRepository) Update(ctx context.Context, id int64, p models.CandidatePatch) error {
	res, err := r.pool.Exec(ctx, `
		UPDATE candidates SET
		  full_name                = COALESCE($2::varchar, full_name),
		  profile_photo_url        = COALESCE($3::text, profile_photo_url),
		  gender                   = COALESCE($4::varchar, gender),
		  headline_text            = COALESCE($5::varchar, headline_text),
		  headline_designation_id  = COALESCE($6::bigint, headline_designation_id),
		  about_summary_updated_at = CASE WHEN $7::varchar IS NOT NULL
		                                   AND $7::varchar IS DISTINCT FROM about_summary
		                                  THEN now() ELSE about_summary_updated_at END,
		  about_summary            = COALESCE($7::varchar, about_summary),
		  updated_at               = now()
		WHERE candidate_id = $1 AND status = 'active'`,
		id, p.FullName, p.ProfilePhotoURL, p.Gender, p.HeadlineText, p.HeadlineDesignationID, p.AboutSummary)
	return affected(res, err)
}

// Soft delete: flips status. FK cascades only matter if you ever hard-delete.
func (r *ProfileRepository) SoftDelete(ctx context.Context, id int64) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE candidates SET status = 'deleted', updated_at = now() WHERE candidate_id = $1 AND status = 'active'`, id)
	return affected(res, err)
}

func (r *ProfileRepository) ListPronouns(ctx context.Context, id int64) ([]string, error) {
	return queryAll(ctx, r.pool, scanString,
		`SELECT pronoun FROM candidate_pronouns WHERE candidate_id = $1 ORDER BY pronoun`, id)
}

func (r *ProfileRepository) ReplacePronouns(ctx context.Context, id int64, pronouns []string) error {
	return replaceSet(ctx, r.pool,
		`DELETE FROM candidate_pronouns WHERE candidate_id = $1`,
		`INSERT INTO candidate_pronouns (candidate_id, pronoun) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		id, pronouns)
}

func (r *ProfileRepository) ListSocial(ctx context.Context, candidateID int64) ([]models.SocialHandle, error) {
	return queryAll(ctx, r.pool, func(rs pgx.Rows) (models.SocialHandle, error) {
		var h models.SocialHandle
		err := rs.Scan(&h.ID, &h.Platform, &h.URL, &h.ClickCount)
		return h, err
	}, `SELECT id, platform, url, click_count FROM candidate_social_handles WHERE candidate_id = $1 ORDER BY id`, candidateID)
}

func (r *ProfileRepository) CreateSocial(ctx context.Context, h models.SocialHandle) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO candidate_social_handles (candidate_id, platform, url) VALUES ($1, $2, $3) RETURNING id`,
		h.CandidateID, h.Platform, h.URL).Scan(&id)
	return id, translate(err)
}

func (r *ProfileRepository) UpdateSocial(ctx context.Context, h models.SocialHandle) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE candidate_social_handles SET platform = $3, url = $4 WHERE id = $1 AND candidate_id = $2`,
		h.ID, h.CandidateID, h.Platform, h.URL)
	return affected(res, err)
}

func (r *ProfileRepository) DeleteSocial(ctx context.Context, id, candidateID int64) error {
	res, err := r.pool.Exec(ctx,
		`DELETE FROM candidate_social_handles WHERE id = $1 AND candidate_id = $2`, id, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) IncrementClickSocial(ctx context.Context, slug, platform string) error {
	res, err := r.pool.Exec(ctx, `
		UPDATE candidate_social_handles h SET click_count = h.click_count + 1
		FROM candidates c
		WHERE c.candidate_id = h.candidate_id AND c.profile_slug = $1 AND c.status = 'active' AND h.platform = $2`,
		slug, platform)
	return affected(res, err)
}

func (r *ProfileRepository) GetSalary(ctx context.Context, candidateID int64) (*models.Salary, error) {
	s := models.Salary{CandidateID: candidateID}
	err := r.pool.QueryRow(ctx, `
		SELECT current_amount::float8, current_currency, current_is_hidden,
		       expected_amount::float8, expected_currency
		FROM candidate_salary WHERE candidate_id = $1`, candidateID).
		Scan(&s.CurrentAmount, &s.CurrentCurrency, &s.CurrentIsHidden, &s.ExpectedAmount, &s.ExpectedCurrency)
	if err != nil {
		return nil, translate(err)
	}
	return &s, nil
}

func (r *ProfileRepository) UpsertSalary(ctx context.Context, s models.Salary) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO candidate_salary (candidate_id,
		    current_amount, current_currency, current_amount_inr, current_is_hidden,
		    expected_amount, expected_currency, expected_amount_inr, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
		ON CONFLICT (candidate_id) DO UPDATE SET
		    current_amount      = EXCLUDED.current_amount,
		    current_currency    = EXCLUDED.current_currency,
		    current_amount_inr  = EXCLUDED.current_amount_inr,
		    current_is_hidden   = EXCLUDED.current_is_hidden,
		    expected_amount     = EXCLUDED.expected_amount,
		    expected_currency   = EXCLUDED.expected_currency,
		    expected_amount_inr = EXCLUDED.expected_amount_inr,
		    updated_at          = now()`,
		s.CandidateID, s.CurrentAmount, s.CurrentCurrency, s.CurrentAmountINR, s.CurrentIsHidden,
		s.ExpectedAmount, s.ExpectedCurrency, s.ExpectedAmountINR)
	return translate(err)
}

func (r *ProfileRepository) DeleteSalary(ctx context.Context, candidateID int64) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM candidate_salary WHERE candidate_id = $1`, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) GetPreference(ctx context.Context, id int64) (*models.Preferences, error) {
	p := &models.Preferences{}

	err := r.pool.QueryRow(ctx,
		`SELECT open_to_remote FROM candidate_preferences WHERE candidate_id = $1`, id).Scan(&p.OpenToRemote)
	if err != nil && !errors.Is(err, sql.ErrNoRows) { // no row yet == defaults
		return nil, translate(err)
	}

	var qerr error
	if p.WorkTypes, qerr = queryAll(ctx, r.pool, scanString,
		`SELECT work_type FROM candidate_work_types WHERE candidate_id = $1 ORDER BY work_type`, id); qerr != nil {
		return nil, qerr
	}
	if p.TargetRoles, qerr = queryAll(ctx, r.pool, func(rs pgx.Rows) (models.TargetRole, error) {
		var t models.TargetRole
		err := rs.Scan(&t.DesignationID, &t.Name)
		return t, err
	}, `SELECT d.designation_id, d.name
	    FROM candidate_target_roles tr JOIN master_designations d ON d.designation_id = tr.designation_id
	    WHERE tr.candidate_id = $1 ORDER BY d.name`, id); qerr != nil {
		return nil, qerr
	}
	if p.TargetLocations, qerr = queryAll(ctx, r.pool, func(rs pgx.Rows) (models.TargetLocation, error) {
		var t models.TargetLocation
		err := rs.Scan(&t.LocationID, &t.Name, &t.Level)
		return t, err
	}, `SELECT l.location_id, l.name, l.level
	    FROM candidate_target_locations tl JOIN master_locations l ON l.location_id = tl.location_id
	    WHERE tl.candidate_id = $1 ORDER BY l.name`, id); qerr != nil {
		return nil, qerr
	}
	return p, nil
}

func (r *ProfileRepository) UpsertPreference(ctx context.Context, id int64, openToRemote bool, workTypes []string) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO candidate_preferences (candidate_id, open_to_remote) VALUES ($1, $2)
			ON CONFLICT (candidate_id) DO UPDATE SET open_to_remote = EXCLUDED.open_to_remote`,
			id, openToRemote); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM candidate_work_types WHERE candidate_id = $1`, id); err != nil {
			return err
		}
		for _, wt := range workTypes {
			if _, err := tx.Exec(ctx,
				`INSERT INTO candidate_work_types (candidate_id, work_type) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, wt); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ProfileRepository) ReplaceTargetRoles(ctx context.Context, id int64, ids []int64) error {
	return replaceSet(ctx, r.pool,
		`DELETE FROM candidate_target_roles WHERE candidate_id = $1`,
		`INSERT INTO candidate_target_roles (candidate_id, designation_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		id, ids)
}

func (r *ProfileRepository) ReplaceTargetLocations(ctx context.Context, id int64, ids []int64) error {
	return replaceSet(ctx, r.pool,
		`DELETE FROM candidate_target_locations WHERE candidate_id = $1`,
		`INSERT INTO candidate_target_locations (candidate_id, location_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		id, ids)
}

func (r *ProfileRepository) ListTimeline(ctx context.Context, candidateID int64) ([]models.TimelineEntry, error) {
	return queryAll(ctx, r.pool, func(rs pgx.Rows) (models.TimelineEntry, error) {
		var (
			e         models.TimelineEntry
			companyID *int64
			compName  *string
			rawName   *string
		)
		if err := rs.Scan(&e.TimelineID, &companyID, &compName, &rawName, &e.DesignationTitle,
			&e.StartDate, &e.EndDate, &e.IsCurrent, &e.Description, &e.VerificationStatus); err != nil {
			return e, err
		}
		switch {
		case companyID != nil && compName != nil:
			e.Company = &models.CompanyRef{CompanyID: companyID, Name: *compName}
		case rawName != nil:
			e.Company = &models.CompanyRef{Name: *rawName}
		}
		return e, nil
	}, `
		SELECT t.timeline_id, t.company_id, mc.name, t.company_name_raw, t.designation_title,
		       to_char(t.start_date, 'YYYY-MM-DD'), to_char(t.end_date, 'YYYY-MM-DD'),
		       t.is_current, t.description, t.verification_status
		FROM candidate_timeline t
		LEFT JOIN master_companies mc ON mc.company_id = t.company_id
		WHERE t.candidate_id = $1
		ORDER BY t.is_current DESC, t.start_date DESC, t.timeline_id DESC`, candidateID)
}

func (r *ProfileRepository) CreateTimeline(ctx context.Context, t models.Timeline) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO candidate_timeline (candidate_id, company_id, company_name_raw, designation_title,
		                                start_date, end_date, is_current, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING timeline_id`,
		t.CandidateID, t.CompanyID, t.CompanyNameRaw, t.DesignationTitle,
		t.StartDate, t.EndDate, t.IsCurrent, t.Description).Scan(&id)
	return id, translate(err)
}

func (r *ProfileRepository) UpdateTimeline(ctx context.Context, t models.Timeline) error {
	res, err := r.pool.Exec(ctx, `
		UPDATE candidate_timeline SET
		  company_id = $3, company_name_raw = $4, designation_title = $5,
		  start_date = $6, end_date = $7, is_current = $8, description = $9,
		  verification_status = 'UNVERIFIED', verified_at = NULL, verified_by_user_id = NULL, rejected_at = NULL,
		  updated_at = now()
		WHERE timeline_id = $1 AND candidate_id = $2`,
		t.TimelineID, t.CandidateID, t.CompanyID, t.CompanyNameRaw, t.DesignationTitle,
		t.StartDate, t.EndDate, t.IsCurrent, t.Description)
	return affected(res, err)
}

func (r *ProfileRepository) DeleteTimeline(ctx context.Context, id, candidateID int64) error {
	res, err := r.pool.Exec(ctx,
		`DELETE FROM candidate_timeline WHERE timeline_id = $1 AND candidate_id = $2`, id, candidateID)
	return affected(res, err)
}

// Only roles at a registered company can be sent for verification.
func (r *ProfileRepository) RequestTimelineVerification(ctx context.Context, id, candidateID int64) error {
	res, err := r.pool.Exec(ctx, `
		UPDATE candidate_timeline
		SET verification_status = 'PENDING_VERIFICATION', rejected_at = NULL, updated_at = now()
		WHERE timeline_id = $1 AND candidate_id = $2
		  AND company_id IS NOT NULL
		  AND verification_status IN ('UNVERIFIED', 'REJECTED')`, id, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) ReviewTimeline(ctx context.Context, id int64, approve bool, adminUserID int64) error {
	if approve {
		res, err := r.pool.Exec(ctx, `
			UPDATE candidate_timeline
			SET verification_status = 'VERIFIED', verified_at = now(), verified_by_user_id = $2,
			    rejected_at = NULL, updated_at = now()
			WHERE timeline_id = $1 AND verification_status = 'PENDING_VERIFICATION'`, id, adminUserID)
		return affected(res, err)
	}
	res, err := r.pool.Exec(ctx, `
		UPDATE candidate_timeline
		SET verification_status = 'REJECTED', rejected_at = now(), updated_at = now()
		WHERE timeline_id = $1 AND verification_status = 'PENDING_VERIFICATION'`, id)
	return affected(res, err)
}

func (r *ProfileRepository) ListSkillGroups(ctx context.Context, candidateID int64) ([]models.SkillGroup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.group_id, g.group_name, g.display_order, i.skill_id, s.name, i.display_order
		FROM candidate_skill_groups g
		LEFT JOIN candidate_skill_group_items i ON i.group_id = g.group_id
		LEFT JOIN master_skills s ON s.skill_id = i.skill_id
		WHERE g.candidate_id = $1
		ORDER BY g.display_order, g.group_id, i.display_order, i.id`, candidateID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	groups := make([]models.SkillGroup, 0)
	pos := map[int64]int{} // group_id -> index in groups
	for rows.Next() {
		var (
			gid     int64
			gname   string
			gorder  int
			skillID *int64
			sname   *string
			iorder  *int
		)
		if err := rows.Scan(&gid, &gname, &gorder, &skillID, &sname, &iorder); err != nil {
			return nil, translate(err)
		}
		idx, ok := pos[gid]
		if !ok {
			groups = append(groups, models.SkillGroup{
				GroupID: gid, GroupName: gname, DisplayOrder: gorder, Skills: []models.SkillChip{},
			})
			idx = len(groups) - 1
			pos[gid] = idx
		}
		if skillID != nil && sname != nil && iorder != nil {
			groups[idx].Skills = append(groups[idx].Skills,
				models.SkillChip{SkillID: *skillID, Name: *sname, DisplayOrder: *iorder})
		}
	}
	return groups, translate(rows.Err())
}

func (r *ProfileRepository) CreateSkillGroup(ctx context.Context, candidateID int64, name string, order int) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO candidate_skill_groups (candidate_id, group_name, display_order) VALUES ($1, $2, $3) RETURNING group_id`,
		candidateID, name, order).Scan(&id)
	return id, translate(err)
}

func (r *ProfileRepository) UpdateSkillGroup(ctx context.Context, groupID, candidateID int64, name string, order int) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE candidate_skill_groups SET group_name = $3, display_order = $4 WHERE group_id = $1 AND candidate_id = $2`,
		groupID, candidateID, name, order)
	return affected(res, err)
}

func (r *ProfileRepository) DeleteSkillGroup(ctx context.Context, groupID, candidateID int64) error {
	res, err := r.pool.Exec(ctx,
		`DELETE FROM candidate_skill_groups WHERE group_id = $1 AND candidate_id = $2`, groupID, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) OwnsGroup(ctx context.Context, groupID, candidateID int64) (bool, error) {
	var one int
	err := r.pool.QueryRow(ctx,
		`SELECT 1 FROM candidate_skill_groups WHERE group_id = $1 AND candidate_id = $2`, groupID, candidateID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, translate(err)
}

func (r *ProfileRepository) AddSkill(ctx context.Context, groupID, skillID int64, order int) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO candidate_skill_group_items (group_id, skill_id, display_order) VALUES ($1, $2, $3) RETURNING id`,
		groupID, skillID, order).Scan(&id)
	return id, translate(err)
}

func (r *ProfileRepository) UpdateSkill(ctx context.Context, groupID, skillID, candidateID int64, order int) error {
	res, err := r.pool.Exec(ctx, `
		UPDATE candidate_skill_group_items i SET display_order = $4
		FROM candidate_skill_groups g
		WHERE g.group_id = i.group_id AND g.candidate_id = $3 AND i.group_id = $1 AND i.skill_id = $2`,
		groupID, skillID, candidateID, order)
	return affected(res, err)
}

func (r *ProfileRepository) RemoveSkill(ctx context.Context, groupID, skillID, candidateID int64) error {
	res, err := r.pool.Exec(ctx, `
		DELETE FROM candidate_skill_group_items i USING candidate_skill_groups g
		WHERE g.group_id = i.group_id AND g.candidate_id = $3 AND i.group_id = $1 AND i.skill_id = $2`,
		groupID, skillID, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) GetContact(ctx context.Context, candidateID int64) (*models.ContactS, error) {
	c := models.ContactS{CandidateID: candidateID}
	err := r.pool.QueryRow(ctx,
		`SELECT email, phone_number, primary_location_id FROM candidate_contact WHERE candidate_id = $1`, candidateID).
		Scan(&c.Email, &c.PhoneNumber, &c.PrimaryLocationID)
	if err != nil {
		return nil, translate(err)
	}
	return &c, nil
}

func (r *ProfileRepository) UpsertContact(ctx context.Context, c models.ContactS) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO candidate_contact (candidate_id, email, phone_number, primary_location_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (candidate_id) DO UPDATE SET
		    email = EXCLUDED.email,
		    phone_number = EXCLUDED.phone_number,
		    primary_location_id = EXCLUDED.primary_location_id`,
		c.CandidateID, c.Email, c.PhoneNumber, c.PrimaryLocationID)
	return translate(err)
}

func (r *ProfileRepository) GetResume(ctx context.Context, candidateID int64) (*models.ResumeS, error) {
	x := models.ResumeS{CandidateID: candidateID}
	err := r.pool.QueryRow(ctx, `
		SELECT file_url, file_size_bytes, access_control, public_short_link, uploaded_at
		FROM candidate_resume WHERE candidate_id = $1`, candidateID).
		Scan(&x.FileURL, &x.FileSizeBytes, &x.AccessControl, &x.PublicShortLink, &x.UploadedAt)
	if err != nil {
		return nil, translate(err)
	}
	return &x, nil
}

func (r *ProfileRepository) UpsertResume(ctx context.Context, x models.ResumeS) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO candidate_resume (candidate_id, file_url, file_size_bytes, access_control, public_short_link, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (candidate_id) DO UPDATE SET
		    file_url = EXCLUDED.file_url,
		    file_size_bytes = EXCLUDED.file_size_bytes,
		    access_control = EXCLUDED.access_control,
		    public_short_link = EXCLUDED.public_short_link,
		    uploaded_at = now()`,
		x.CandidateID, x.FileURL, x.FileSizeBytes, x.AccessControl, x.PublicShortLink)
	return translate(err) // short link already taken -> 409
}

func (r *ProfileRepository) DeleteResume(ctx context.Context, candidateID int64) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM candidate_resume WHERE candidate_id = $1`, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) ListResumeGrants(ctx context.Context, candidateID int64) ([]models.ResumeGrant, error) {
	return queryAll(ctx, r.pool, func(rs pgx.Rows) (models.ResumeGrant, error) {
		var g models.ResumeGrant
		err := rs.Scan(&g.ID, &g.RecruiterID, &g.JobPostingID, &g.GrantedAt)
		return g, err
	}, `SELECT id, recruiter_id, job_posting_id, granted_at
	    FROM resume_access_grants WHERE candidate_id = $1 ORDER BY granted_at DESC`, candidateID)
}

func (r *ProfileRepository) RevokeResumeGrant(ctx context.Context, id, candidateID int64) error {
	res, err := r.pool.Exec(ctx,
		`DELETE FROM resume_access_grants WHERE id = $1 AND candidate_id = $2`, id, candidateID)
	return affected(res, err)
}

func (r *ProfileRepository) CreateResumeGrant(ctx context.Context, g models.ResumeGrant) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO resume_access_grants (candidate_id, recruiter_id, job_posting_id)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, g.CandidateID, g.RecruiterID, g.JobPostingID)
	return translate(err)
}
