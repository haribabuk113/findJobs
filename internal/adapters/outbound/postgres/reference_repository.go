package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

// LocationRepository is the PostgreSQL adapter for the location tree.
type LocationRepository struct {
	pool *pgxpool.Pool
}

// NewLocationRepository creates the PostgreSQL adapter for locations.
func NewLocationRepository(pool *pgxpool.Pool) ports.LocationRepository {
	return &LocationRepository{pool: pool}
}

func (r *LocationRepository) ReplaceAll(ctx context.Context, locations []reference.Location) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM locations`); err != nil {
		return err
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"locations"},
		[]string{"id", "parent_id", "level", "name", "country_code", "lat", "lng"},
		pgx.CopyFromSlice(len(locations), func(i int) ([]any, error) {
			l := locations[i]
			return []any{l.ID, l.ParentID, l.Level, l.Name, l.CountryCode, l.Lat, l.Lng}, nil
		}))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *LocationRepository) All(ctx context.Context) ([]reference.Location, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, parent_id, level, name, country_code, lat, lng FROM locations
		ORDER BY CASE level WHEN 'country' THEN 0 WHEN 'region' THEN 1 ELSE 2 END, name, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (reference.Location, error) {
		var l reference.Location
		err := row.Scan(&l.ID, &l.ParentID, &l.Level, &l.Name, &l.CountryCode, &l.Lat, &l.Lng)
		return l, err
	})
}

// RatesRepository is the PostgreSQL adapter for exchange rates.
type RatesRepository struct {
	pool *pgxpool.Pool
}

// NewRatesRepository creates the PostgreSQL adapter for exchange rates.
func NewRatesRepository(pool *pgxpool.Pool) ports.RatesRepository {
	return &RatesRepository{pool: pool}
}

func (r *RatesRepository) Replace(ctx context.Context, rates reference.Rates) error {
	codes := make([]string, 0, len(rates))
	values := make([]float64, 0, len(rates))
	for code, rate := range rates {
		codes, values = append(codes, code), append(values, rate)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM exchange_rates`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO exchange_rates (currency, rate_to_inr)
		SELECT c, v FROM unnest($1::text[], $2::float8[]) AS u(c, v)`, codes, values); err != nil {
		return err
	}
	// Manual "Salary": the daily job re-normalises every active listing to INR.
	// Listings in a currency the provider dropped keep their previous amount.
	if _, err := tx.Exec(ctx, `
		UPDATE jobs j SET
			normalized_inr_amount = ROUND(j.salary_max * r.rate_to_inr, 2),
			salary_min_inr = ROUND(j.salary_min * r.rate_to_inr, 2)
		FROM exchange_rates r
		WHERE r.currency = j.currency AND j.is_visible`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *RatesRepository) All(ctx context.Context) (reference.Rates, error) {
	rows, err := r.pool.Query(ctx, `SELECT currency, rate_to_inr::float8 FROM exchange_rates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rates := reference.Rates{}
	for rows.Next() {
		var code string
		var rate float64
		if err := rows.Scan(&code, &rate); err != nil {
			return nil, err
		}
		rates[code] = rate
	}
	return rates, rows.Err()
}

// AssetRepository is the PostgreSQL adapter for published static JSON files.
type AssetRepository struct {
	pool *pgxpool.Pool
}

// NewAssetRepository creates the PostgreSQL adapter for published assets.
func NewAssetRepository(pool *pgxpool.Pool) ports.AssetRepository {
	return &AssetRepository{pool: pool}
}

// keepVersions is how many files per kind stay downloadable, so clients that
// fetched the previous manifest can still download what it pointed to.
const keepVersions = 3

func (r *AssetRepository) Save(ctx context.Context, a *reference.Asset) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	// Serialise publishers of the same kind (several instances, debounced saves).
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('published_assets:' || $1))`, a.Kind); err != nil {
		return false, err
	}
	var latest string
	err = tx.QueryRow(ctx, `SELECT sha256 FROM published_assets WHERE kind = $1 ORDER BY created_at DESC LIMIT 1`, a.Kind).Scan(&latest)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if latest == a.SHA256 {
		return false, nil // unchanged: keep the current file so clients skip a download
	}
	if _, err := tx.Exec(ctx, `INSERT INTO published_assets (file, kind, content, sha256, created_at) VALUES ($1, $2, $3, $4, $5)`,
		a.File, a.Kind, a.Content, a.SHA256, a.CreatedAt); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM published_assets WHERE kind = $1 AND file NOT IN (
			SELECT file FROM published_assets WHERE kind = $1 ORDER BY created_at DESC LIMIT $2)`, a.Kind, keepVersions); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (r *AssetRepository) Get(ctx context.Context, file string) (*reference.Asset, error) {
	var a reference.Asset
	err := r.pool.QueryRow(ctx, `SELECT file, kind, content, sha256, created_at FROM published_assets WHERE file = $1`, file).
		Scan(&a.File, &a.Kind, &a.Content, &a.SHA256, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &reference.NotFoundError{Message: "asset not found"}
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AssetRepository) Manifest(ctx context.Context) (map[string]reference.ManifestEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT ON (kind) kind, file, sha256, created_at FROM published_assets ORDER BY kind, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	manifest := map[string]reference.ManifestEntry{}
	for rows.Next() {
		var kind string
		var e reference.ManifestEntry
		if err := rows.Scan(&kind, &e.File, &e.SHA256, &e.UpdatedAt); err != nil {
			return nil, err
		}
		manifest[kind] = e
	}
	return manifest, rows.Err()
}
