package postgres

import (
	"context"
	"database/sql"
	models "findJobs/internal/domain/master"
	"findJobs/internal/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MasterRepository is the outbound adapter implementing the application port.
type MasterRepository struct {
	pool *pgxpool.Pool
}

// NewMasterRepository creates the PostgreSQL adapter for profile persistence.
func NewMasterRepository(pool *pgxpool.Pool) ports.MasterRepository {
	return &MasterRepository{pool: pool}
}

// func (r *MasterRepository) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
// 	// TODO: replace stub with real query against the database.
// 	return []profiles.ProfileDetails{}, nil
// }

// func (r *MasterRepository) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error {
// 	// TODO: replace stub with real persistence against the database.
// 	return nil
// }

// Public typeahead: approved entries only.
func (r *MasterRepository) SearchMaster(ctx context.Context, kind models.MasterKind, q string, limit int) ([]models.MasterEntry, error) {
	t, err := tableFor(kind)
	if err != nil {
		return nil, err
	}
	return queryAll(ctx, r.pool, scanMaster,
		`SELECT `+t.idCol+`, name, is_admin_approved, created_at FROM `+t.table+`
		 WHERE is_admin_approved AND name ILIKE $1 ORDER BY name LIMIT $2`, likePattern(q), limit)
}

func (r *MasterRepository) ListPending(ctx context.Context, kind models.MasterKind) ([]models.MasterEntry, error) {
	t, err := tableFor(kind)
	if err != nil {
		return nil, err
	}
	return queryAll(ctx, r.pool, scanMaster,
		`SELECT `+t.idCol+`, name, is_admin_approved, created_at FROM `+t.table+`
		 WHERE NOT is_admin_approved ORDER BY created_at`)
}

// Case-insensitive match first so "figma" does not create a duplicate of "Figma".
func (r *MasterRepository) FindOrCreatePending(ctx context.Context, kind models.MasterKind, name string, candidateID int64) (*models.MasterEntry, error) {
	t, err := tableFor(kind)
	if err != nil {
		return nil, err
	}
	var m models.MasterEntry
	err = r.pool.QueryRow(ctx,
		`SELECT `+t.idCol+`, name, is_admin_approved, created_at FROM `+t.table+` WHERE lower(name) = lower($1) LIMIT 1`, name).
		Scan(&m.ID, &m.Name, &m.IsAdminApproved, &m.CreatedAt)
	if err == nil {
		return &m, nil
	}
	if err != sql.ErrNoRows {
		return nil, translate(err)
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO `+t.table+` (name, is_admin_approved, created_by_candidate_id) VALUES ($1, FALSE, $2)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING `+t.idCol+`, name, is_admin_approved, created_at`, name, candidateID).
		Scan(&m.ID, &m.Name, &m.IsAdminApproved, &m.CreatedAt)
	if err != nil {
		return nil, translate(err)
	}
	return &m, nil
}

func (r *MasterRepository) Approve(ctx context.Context, kind models.MasterKind, id int64) error {
	t, err := tableFor(kind)
	if err != nil {
		return err
	}
	res, err := r.pool.Exec(ctx, `UPDATE `+t.table+` SET is_admin_approved = TRUE WHERE `+t.idCol+` = $1`, id)
	return affected(res, err)
}

func (r *MasterRepository) Rename(ctx context.Context, kind models.MasterKind, id int64, name string) error {
	t, err := tableFor(kind)
	if err != nil {
		return err
	}
	res, err := r.pool.Exec(ctx, `UPDATE `+t.table+` SET name = $2 WHERE `+t.idCol+` = $1`, id, name)
	return affected(res, err)
}

// Fails with 422 while candidates still reference the entry.
func (r *MasterRepository) Delete(ctx context.Context, kind models.MasterKind, id int64) error {
	t, err := tableFor(kind)
	if err != nil {
		return err
	}
	res, err := r.pool.Exec(ctx, `DELETE FROM `+t.table+` WHERE `+t.idCol+` = $1`, id)
	return affected(res, err)
}

// ---------------------------------------------------------------- companies

func (r *MasterRepository) SearchCompany(ctx context.Context, q string, limit int) ([]models.Company, error) {
	return queryAll(ctx, r.pool, func(rs pgx.Rows) (models.Company, error) {
		var c models.Company
		err := rs.Scan(&c.CompanyID, &c.Name, &c.IsRegistered)
		return c, err
	}, `SELECT company_id, name, is_registered FROM master_companies WHERE name ILIKE $1 ORDER BY name LIMIT $2`,
		likePattern(q), limit)
}

func (r *MasterRepository) CreateCompany(ctx context.Context, name string, isRegistered bool) (*models.Company, error) {
	c := models.Company{Name: name, IsRegistered: isRegistered}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO master_companies (name, is_registered) VALUES ($1, $2) RETURNING company_id`,
		name, isRegistered).Scan(&c.CompanyID)
	if err != nil {
		return nil, translate(err)
	}
	return &c, nil
}

func (r *MasterRepository) UpdateCompany(ctx context.Context, c models.Company) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE master_companies SET name = $2, is_registered = $3 WHERE company_id = $1`,
		c.CompanyID, c.Name, c.IsRegistered)
	return affected(res, err)
}

func (r *MasterRepository) DeleteCompany(ctx context.Context, id int64) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM master_companies WHERE company_id = $1`, id)
	return affected(res, err)
}

// ---------------------------------------------------------------- locations

func (r *MasterRepository) ListLocation(ctx context.Context, f models.LocationFilter, limit int) ([]models.Location, error) {
	return queryAll(ctx, r.pool, func(rs pgx.Rows) (models.Location, error) {
		var l models.Location
		err := rs.Scan(&l.LocationID, &l.ParentLocationID, &l.Level, &l.Name)
		return l, err
	}, `SELECT location_id, parent_location_id, level, name FROM master_locations
	    WHERE ($1::bigint IS NULL OR parent_location_id = $1::bigint)
	      AND ($2::text = '' OR level = $2::text)
	      AND name ILIKE $3
	    ORDER BY name LIMIT $4`,
		f.ParentID, f.Level, likePattern(f.Q), limit)
}

func (r *MasterRepository) CreateLocation(ctx context.Context, l models.Location) (*models.Location, error) {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO master_locations (parent_location_id, level, name) VALUES ($1, $2, $3) RETURNING location_id`,
		l.ParentLocationID, l.Level, l.Name).Scan(&l.LocationID)
	if err != nil {
		return nil, translate(err)
	}
	return &l, nil
}

func (r *MasterRepository) UpdateLocation(ctx context.Context, l models.Location) error {
	res, err := r.pool.Exec(ctx,
		`UPDATE master_locations SET parent_location_id = $2, level = $3, name = $4 WHERE location_id = $1`,
		l.LocationID, l.ParentLocationID, l.Level, l.Name)
	return affected(res, err)
}

func (r *MasterRepository) DeleteLocation(ctx context.Context, id int64) error {
	res, err := r.pool.Exec(ctx, `DELETE FROM master_locations WHERE location_id = $1`, id)
	return affected(res, err)
}

// ---------------------------------------------------------------- fx rates

func scanFx(rs pgx.Rows) (models.FxRate, error) {
	var f models.FxRate
	err := rs.Scan(&f.RateDate, &f.CurrencyCode, &f.INRConversionRate)
	return f, err
}

// INR per one unit of the currency, most recent snapshot.
func (r *MasterRepository) LatestRate(ctx context.Context, currency string) (float64, error) {
	var rate float64
	err := r.pool.QueryRow(ctx, `
		SELECT inr_conversion_rate::float8 FROM fx_rates_snapshot
		WHERE currency_code = $1 ORDER BY rate_date DESC LIMIT 1`, currency).Scan(&rate)
	return rate, translate(err)
}

func (r *MasterRepository) FxLatest(ctx context.Context) ([]models.FxRate, error) {
	return queryAll(ctx, r.pool, scanFx, `
		SELECT DISTINCT ON (currency_code) to_char(rate_date, 'YYYY-MM-DD'), currency_code, inr_conversion_rate::float8
		FROM fx_rates_snapshot ORDER BY currency_code, rate_date DESC`)
}

func (r *MasterRepository) FxByDate(ctx context.Context, date string) ([]models.FxRate, error) {
	return queryAll(ctx, r.pool, scanFx, `
		SELECT to_char(rate_date, 'YYYY-MM-DD'), currency_code, inr_conversion_rate::float8
		FROM fx_rates_snapshot WHERE rate_date = $1 ORDER BY currency_code`, date)
}

func (r *MasterRepository) UpsertFx(ctx context.Context, date string, rates map[string]float64) error {
	return inTx(ctx, r.pool, func(tx pgx.Tx) error {
		for code, rate := range rates {
			if _, err := tx.Exec(ctx, `
				INSERT INTO fx_rates_snapshot (rate_date, currency_code, inr_conversion_rate) VALUES ($1, $2, $3)
				ON CONFLICT (rate_date, currency_code) DO UPDATE SET inr_conversion_rate = EXCLUDED.inr_conversion_rate`,
				date, code, rate); err != nil {
				return err
			}
		}
		return nil
	})
}
