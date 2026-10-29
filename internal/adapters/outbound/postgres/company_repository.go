package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
	"findJobs/internal/ports"
)

// CompanyRepository is the PostgreSQL adapter for companies and members.
type CompanyRepository struct {
	pool *pgxpool.Pool
}

// NewCompanyRepository creates the PostgreSQL adapter for companies.
func NewCompanyRepository(pool *pgxpool.Pool) ports.CompanyRepository {
	return &CompanyRepository{pool: pool}
}

func (r *CompanyRepository) LocationCountries(ctx context.Context, ids []string) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, country_code FROM locations WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, cc string
		if err := rows.Scan(&id, &cc); err != nil {
			return nil, err
		}
		out[id] = cc
	}
	return out, rows.Err()
}

func (r *CompanyRepository) Create(ctx context.Context, in *companies.Input, hqCountry, userID string) (*companies.Company, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	id := uuid.NewString()
	// sector mirrors the first industry: job search still filters on it.
	_, err = tx.Exec(ctx, `
		INSERT INTO companies (id, slug, name, sector, size, tagline, industries, about, website, social_links,
			founded_year, hq_location_id, hq_country_code, hq_address, branch_location_ids, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		id, in.Slug, in.Name, in.Industries[0], in.Size, in.Tagline, in.Industries, in.About, in.Website, in.Social,
		in.FoundedYear, in.HQLocationID, hqCountry, in.HQAddress, in.BranchLocationIDs, userID)
	if isUniqueViolation(err) {
		return nil, &companies.ConflictError{Message: "that company link is already taken"}
	}
	if err != nil {
		return nil, err
	}
	if err := replaceShowcase(ctx, tx, id, in); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO company_members (company_id, user_id, role, status) VALUES ($1, $2, 'admin', 'current')`, id, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetBySlug(ctx, in.Slug, userID)
}

func (r *CompanyRepository) GetBySlug(ctx context.Context, slug, viewerID string) (*companies.Company, error) {
	var c companies.Company
	var logo, banner *string
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.slug, c.name, c.tagline, c.size, c.industries, c.about, c.website, c.social_links,
			c.founded_year, c.hq_location_id, c.hq_country_code, c.hq_address, c.branch_location_ids,
			c.logo_key, c.banner_key, c.created_at,
			(SELECT m.role FROM company_members m WHERE m.company_id = c.id
				AND m.user_id = NULLIF($2, '')::uuid AND m.status = 'current')
		FROM companies c WHERE c.slug = $1 AND c.deleted_at IS NULL`, slug, viewerID).
		Scan(&c.ID, &c.Slug, &c.Name, &c.Tagline, &c.Size, &c.Industries, &c.About, &c.Website, &c.Social,
			&c.FoundedYear, &c.HQLocationID, &c.HQCountryCode, &c.HQAddress, &c.BranchLocationIDs,
			&logo, &banner, &c.CreatedAt, &c.ViewerRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, companies.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.LogoURL, c.BannerURL = companies.FileURL(logo), companies.FileURL(banner)

	rows, err := r.pool.Query(ctx, `SELECT name, about, url FROM company_products WHERE company_id = $1 ORDER BY position`, c.ID)
	if err != nil {
		return nil, err
	}
	if c.Products, err = pgx.CollectRows(rows, pgx.RowToStructByPos[companies.Product]); err != nil {
		return nil, err
	}
	rows, err = r.pool.Query(ctx, `SELECT title, details, icon FROM company_benefits WHERE company_id = $1 ORDER BY position`, c.ID)
	if err != nil {
		return nil, err
	}
	if c.Benefits, err = pgx.CollectRows(rows, pgx.RowToStructByPos[companies.Benefit]); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CompanyRepository) SlugTaken(ctx context.Context, slug string) (bool, error) {
	var taken bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM companies WHERE slug = $1)`, slug).Scan(&taken)
	return taken, err
}

func (r *CompanyRepository) Search(ctx context.Context, c *companies.SearchCriteria) (*companies.SearchResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, slug, name, tagline, size, industries, hq_location_id, hq_country_code, logo_key, COUNT(*) OVER()
		FROM companies
		WHERE deleted_at IS NULL
			AND ($1 = '' OR name ILIKE '%' || $1 || '%')
			AND (cardinality($2::text[]) = 0 OR hq_country_code = ANY($2))
			AND (cardinality($3::text[]) = 0 OR EXISTS (SELECT 1 FROM unnest(industries) i WHERE lower(i) = ANY($3)))
			AND (cardinality($4::text[]) = 0 OR size = ANY($4))
		ORDER BY lower(name), id LIMIT $5 OFFSET $6`,
		escapeLike(c.Query), c.Countries, c.Industries, c.Sizes, c.PageSize, (c.Page-1)*c.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &companies.SearchResult{Companies: []companies.Summary{}, Page: c.Page, PageSize: c.PageSize}
	for rows.Next() {
		var s companies.Summary
		var logo *string
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Tagline, &s.Size, &s.Industries, &s.HQLocationID,
			&s.HQCountryCode, &logo, &result.Total); err != nil {
			return nil, err
		}
		s.LogoURL = companies.FileURL(logo)
		result.Companies = append(result.Companies, s)
	}
	return result, rows.Err()
}

func (r *CompanyRepository) Update(ctx context.Context, slug, userID string, in *companies.Input, hqCountry string) (*companies.Company, error) {
	err := r.asAdmin(ctx, slug, userID, func(tx pgx.Tx, companyID string) error {
		_, err := tx.Exec(ctx, `
			UPDATE companies SET slug = $2, name = $3, sector = $4, size = $5, tagline = $6, industries = $7,
				about = $8, website = $9, social_links = $10, founded_year = $11, hq_location_id = $12,
				hq_country_code = $13, hq_address = $14, branch_location_ids = $15, updated_at = NOW()
			WHERE id = $1`,
			companyID, in.Slug, in.Name, in.Industries[0], in.Size, in.Tagline, in.Industries, in.About, in.Website,
			in.Social, in.FoundedYear, in.HQLocationID, hqCountry, in.HQAddress, in.BranchLocationIDs)
		if isUniqueViolation(err) {
			return &companies.ConflictError{Message: "that company link is already taken"}
		}
		if err != nil {
			return err
		}
		return replaceShowcase(ctx, tx, companyID, in)
	})
	if err != nil {
		return nil, err
	}
	return r.GetBySlug(ctx, in.Slug, userID)
}

func (r *CompanyRepository) Delete(ctx context.Context, slug, userID string) error {
	return r.asAdmin(ctx, slug, userID, func(tx pgx.Tx, companyID string) error {
		// The slug stays reserved, so a deleted brand's link can't be taken over.
		// Manual: deleting "removes all active job posts, cancels open candidate
		// applications". Closing (not just hiding) the jobs stops new applications
		// through direct links. Rejected applications are already decided and stay.
		for _, q := range []string{
			`UPDATE companies SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`,
			`UPDATE applications a SET status = 'cancelled', cancelled_at = NOW() FROM jobs j
				WHERE a.job_id = j.id AND j.company_id = $1 AND a.status = 'open' AND a.stage <> 'rejected'`,
			`UPDATE jobs SET is_visible = FALSE, status = 'closed' WHERE company_id = $1`,
			`DELETE FROM company_members WHERE company_id = $1`,
		} {
			if _, err := tx.Exec(ctx, q, companyID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *CompanyRepository) SetImage(ctx context.Context, slug, userID, kind, key string) (string, error) {
	column := "logo_key" // fixed map below: column names can't be parameters
	if kind == "banner" {
		column = "banner_key"
	}
	var old *string
	err := r.asAdmin(ctx, slug, userID, func(tx pgx.Tx, companyID string) error {
		return tx.QueryRow(ctx, `
			UPDATE companies c SET `+column+` = $2, updated_at = NOW()
			FROM (SELECT `+column+` AS old FROM companies WHERE id = $1) prev
			WHERE c.id = $1 RETURNING prev.old`, companyID, key).Scan(&old)
	})
	if err != nil || old == nil {
		return "", err
	}
	return *old, nil
}

func (r *CompanyRepository) Members(ctx context.Context, slug, userID, status string) ([]companies.Member, error) {
	var members []companies.Member
	err := r.asAdmin(ctx, slug, userID, func(tx pgx.Tx, companyID string) error {
		rows, err := tx.Query(ctx, `
			SELECT m.user_id::text, u.name, u.email, m.role, m.status, m.title, m.started_on::text, m.created_at
			FROM company_members m JOIN users u ON u.id = m.user_id
			WHERE m.company_id = $1 AND m.status = $2
			ORDER BY m.created_at DESC`, companyID, status)
		if err != nil {
			return err
		}
		members, err = pgx.CollectRows(rows, pgx.RowToStructByPos[companies.Member])
		return err
	})
	if members == nil {
		members = []companies.Member{}
	}
	return members, err
}

func (r *CompanyRepository) DecideMember(ctx context.Context, slug, adminID, memberID string, approve bool) error {
	return r.asAdmin(ctx, slug, adminID, func(tx pgx.Tx, companyID string) error {
		status := companies.StatusRejected
		if approve {
			status = companies.StatusCurrent
		}
		tag, err := tx.Exec(ctx, `
			UPDATE company_members SET status = $3, updated_at = NOW()
			WHERE company_id = $1 AND user_id = $2 AND status = 'pending'`, companyID, memberID, status)
		if err == nil && tag.RowsAffected() == 0 {
			return &companies.NotFoundError{Message: "no pending request from this person"}
		}
		return err
	})
}

func (r *CompanyRepository) SetMemberRole(ctx context.Context, slug, adminID, memberID, role string) error {
	return r.asAdmin(ctx, slug, adminID, func(tx pgx.Tx, companyID string) error {
		current, err := currentRole(ctx, tx, companyID, memberID)
		if err != nil {
			return err
		}
		if current == companies.RoleAdmin && role != companies.RoleAdmin {
			if err := keepAnAdmin(ctx, tx, companyID, "make someone else an admin before changing this role"); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE company_members SET role = $3, updated_at = NOW() WHERE company_id = $1 AND user_id = $2`,
			companyID, memberID, role)
		return err
	})
}

func (r *CompanyRepository) Offboard(ctx context.Context, slug, adminID, memberID string) error {
	return r.asAdmin(ctx, slug, adminID, func(tx pgx.Tx, companyID string) error {
		return endMembership(ctx, tx, companyID, memberID, "make someone else an admin before removing the last one")
	})
}

func (r *CompanyRepository) Leave(ctx context.Context, slug, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var companyID string
	err = tx.QueryRow(ctx, `SELECT id FROM companies WHERE slug = $1 AND deleted_at IS NULL FOR UPDATE`, slug).Scan(&companyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return companies.ErrNotFound
	}
	if err != nil {
		return err
	}
	// Manual: the sole admin must appoint another admin, or delete the company.
	if err := endMembership(ctx, tx, companyID, userID,
		"you're the only admin: make someone else an admin first, or delete the company"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *CompanyRepository) RequestMembership(ctx context.Context, companyID, userID, title string, startedOn *time.Time) error {
	// A rejected or past member re-requesting goes back to pending; a current
	// member just updates their claimed title and start date.
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO company_members (company_id, user_id, title, started_on)
		SELECT $1::varchar, $2::uuid, $3::text, $4::date FROM companies WHERE id = $1::varchar AND deleted_at IS NULL
		ON CONFLICT (company_id, user_id) DO UPDATE SET title = EXCLUDED.title, started_on = EXCLUDED.started_on,
			status = CASE WHEN company_members.status = 'current' THEN 'current' ELSE 'pending' END,
			updated_at = NOW()`, companyID, userID, title, startedOn)
	if err == nil && tag.RowsAffected() == 0 {
		return companies.ErrNotFound
	}
	return err
}

func (r *CompanyRepository) Dashboard(ctx context.Context, slug, userID string) (*companies.Dashboard, error) {
	var d companies.Dashboard
	err := r.asAdmin(ctx, slug, userID, func(tx pgx.Tx, companyID string) error {
		return tx.QueryRow(ctx, `
			SELECT
				(SELECT COUNT(*) FROM jobs WHERE company_id = $1),
				(SELECT COUNT(*) FROM jobs WHERE company_id = $1 AND is_visible AND posted_at >= NOW() - INTERVAL '30 days'),
				(SELECT COUNT(*) FROM applications a JOIN jobs j ON j.id = a.job_id WHERE j.company_id = $1),
				COUNT(*) FILTER (WHERE status = 'current' AND role = 'admin'),
				COUNT(*) FILTER (WHERE status = 'current' AND role = 'hr'),
				COUNT(*) FILTER (WHERE status = 'current'),
				COUNT(*) FILTER (WHERE status = 'pending')
			FROM company_members WHERE company_id = $1`, companyID).
			Scan(&d.TotalJobs, &d.ActiveJobs, &d.Applications, &d.Admins, &d.HR, &d.Employees, &d.PendingRequests)
	})
	d.ClosedJobs = d.TotalJobs - d.ActiveJobs
	return &d, err
}

// asAdmin runs fn in a transaction after locking the company row and checking
// userID is a current admin. Locking the company row serialises every admin
// action on one company, so rules like "keep one admin" can't race.
func (r *CompanyRepository) asAdmin(ctx context.Context, slug, userID string, fn func(tx pgx.Tx, companyID string) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var companyID string
	var isAdmin bool
	err = tx.QueryRow(ctx, `
		SELECT c.id, EXISTS (SELECT 1 FROM company_members m WHERE m.company_id = c.id AND m.user_id = $2
			AND m.role = 'admin' AND m.status = 'current')
		FROM companies c WHERE c.slug = $1 AND c.deleted_at IS NULL FOR UPDATE OF c`, slug, userID).Scan(&companyID, &isAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return companies.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !isAdmin {
		return companies.ErrNotAdmin
	}
	if err := fn(tx, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func currentRole(ctx context.Context, tx pgx.Tx, companyID, userID string) (string, error) {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM company_members WHERE company_id = $1 AND user_id = $2 AND status = 'current'`,
		companyID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &companies.NotFoundError{Message: "this person isn't a current member"}
	}
	return role, err
}

// keepAnAdmin fails when the company has only one current admin left.
func keepAnAdmin(ctx context.Context, tx pgx.Tx, companyID, message string) error {
	var admins int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM company_members WHERE company_id = $1 AND role = 'admin' AND status = 'current'`,
		companyID).Scan(&admins); err != nil {
		return err
	}
	if admins <= 1 {
		return &companies.ConflictError{Message: message}
	}
	return nil
}

func endMembership(ctx context.Context, tx pgx.Tx, companyID, userID, lastAdminMessage string) error {
	role, err := currentRole(ctx, tx, companyID, userID)
	if err != nil {
		return err
	}
	if role == companies.RoleAdmin {
		if err := keepAnAdmin(ctx, tx, companyID, lastAdminMessage); err != nil {
			return err
		}
	}
	// Role drops to employee: a past member keeps no rights. (Credit release
	// arrives with the payments section.)
	_, err = tx.Exec(ctx, `UPDATE company_members SET status = 'past', role = 'employee', updated_at = NOW()
		WHERE company_id = $1 AND user_id = $2`, companyID, userID)
	return err
}

func replaceShowcase(ctx context.Context, tx pgx.Tx, companyID string, in *companies.Input) error {
	for _, q := range []string{`DELETE FROM company_products WHERE company_id = $1`, `DELETE FROM company_benefits WHERE company_id = $1`} {
		if _, err := tx.Exec(ctx, q, companyID); err != nil {
			return err
		}
	}
	for i, p := range in.Products {
		if _, err := tx.Exec(ctx, `INSERT INTO company_products (company_id, position, name, about, url) VALUES ($1, $2, $3, $4, $5)`,
			companyID, i, p.Name, p.About, p.URL); err != nil {
			return err
		}
	}
	for i, b := range in.Benefits {
		if _, err := tx.Exec(ctx, `INSERT INTO company_benefits (company_id, position, title, details, icon) VALUES ($1, $2, $3, $4, $5)`,
			companyID, i, b.Title, b.Details, b.Icon); err != nil {
			return err
		}
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
