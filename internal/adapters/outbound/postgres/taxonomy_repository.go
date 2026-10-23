package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

// TaxonomyRepository is the PostgreSQL adapter for skills and designations.
type TaxonomyRepository struct {
	pool *pgxpool.Pool
}

// NewTaxonomyRepository creates the PostgreSQL adapter for the taxonomies.
func NewTaxonomyRepository(pool *pgxpool.Pool) ports.TaxonomyRepository {
	return &TaxonomyRepository{pool: pool}
}

// table maps a validated kind to its table. Table names can't be query
// parameters, so they only ever come from this fixed map.
func table(kind reference.Kind) string {
	if kind == reference.Skill {
		return "skills"
	}
	return "designations"
}

const termColumns = `id, name, status, merged_into, created_at`

func scanTerm(row pgx.Row) (*reference.Term, error) {
	var t reference.Term
	if err := row.Scan(&t.ID, &t.Name, &t.Status, &t.MergedInto, &t.CreatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// usage counts the jobs referencing a term (t = the term's table alias).
func usage(kind reference.Kind) string {
	if kind == reference.Skill {
		return `(SELECT COUNT(*) FROM jobs WHERE t.id = ANY(jobs.skill_ids))`
	}
	return `(SELECT COUNT(*) FROM jobs WHERE jobs.designation_id = t.id)`
}

func (r *TaxonomyRepository) List(ctx context.Context, kind reference.Kind, status string, page, pageSize int) (*reference.TermPage, error) {
	// ponytail: usage is counted per listed row (≤100); add a counter column if this gets slow.
	query := `SELECT t.id, t.name, t.status, t.merged_into, t.created_at, ` + usage(kind) + `, COUNT(*) OVER()
		FROM ` + table(kind) + ` t
		WHERE ($1 = '' AND t.status <> 'merged') OR t.status = $1
		ORDER BY t.created_at DESC, t.id DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &reference.TermPage{Terms: []reference.Term{}, Page: page, PageSize: pageSize}
	for rows.Next() {
		var t reference.Term
		if err := rows.Scan(&t.ID, &t.Name, &t.Status, &t.MergedInto, &t.CreatedAt, &t.Uses, &result.Total); err != nil {
			return nil, err
		}
		result.Terms = append(result.Terms, t)
	}
	return result, rows.Err()
}

func (r *TaxonomyRepository) Resolve(ctx context.Context, kind reference.Kind, names []reference.CleanedName, userID string) ([]reference.Term, bool, error) {
	display := make([]string, len(names))
	normalized := make([]string, len(names))
	for i, n := range names {
		display[i], normalized[i] = n.Display, n.Normalized
	}
	t := table(kind)
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO `+t+` (name, name_norm, created_by)
		SELECT DISTINCT ON (nn) n, nn, NULLIF($3, '')::uuid FROM unnest($1::text[], $2::text[]) AS u(n, nn)
		ON CONFLICT (name_norm) DO NOTHING`, display, normalized, userID)
	if err != nil {
		return nil, false, err
	}

	// Aliases resolve to their master (merges keep aliases one level deep).
	rows, err := r.pool.Query(ctx, `
		SELECT a.name_norm, m.id, m.name, m.status, m.merged_into, m.created_at
		FROM `+t+` a JOIN `+t+` m ON m.id = COALESCE(a.merged_into, a.id)
		WHERE a.name_norm = ANY($1)`, normalized)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	byNorm := make(map[string]reference.Term, len(names))
	for rows.Next() {
		var key string
		var term reference.Term
		if err := rows.Scan(&key, &term.ID, &term.Name, &term.Status, &term.MergedInto, &term.CreatedAt); err != nil {
			return nil, false, err
		}
		byNorm[key] = term
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	terms := make([]reference.Term, len(names))
	for i, n := range names {
		term, ok := byNorm[n.Normalized]
		if !ok {
			return nil, false, fmt.Errorf("term %q vanished during resolve", n.Normalized) // deleted concurrently
		}
		terms[i] = term
	}
	return terms, tag.RowsAffected() > 0, nil
}

func (r *TaxonomyRepository) Merge(ctx context.Context, kind reference.Kind, sourceIDs []int64, masterID int64) error {
	t := table(kind)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock every involved row so a concurrent merge or delete can't interleave.
	var locked int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM (SELECT id FROM `+t+` WHERE id = ANY($1) OR id = $2 FOR UPDATE) l`,
		sourceIDs, masterID).Scan(&locked)
	if err != nil {
		return err
	}
	if locked != len(sourceIDs)+1 {
		return &reference.NotFoundError{Message: "one or more terms not found"}
	}
	var masterStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM `+t+` WHERE id = $1`, masterID).Scan(&masterStatus); err != nil {
		return err
	}
	if masterStatus == reference.StatusMerged {
		return &reference.ValidationError{Field: "master_id", Message: "the master term is itself merged; merge into its master instead"}
	}

	// Repoint existing aliases of the sources first, keeping aliases one level deep.
	if _, err := tx.Exec(ctx, `UPDATE `+t+` SET merged_into = $2, updated_at = NOW() WHERE merged_into = ANY($1)`, sourceIDs, masterID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE `+t+` SET status = 'merged', merged_into = $2, updated_at = NOW() WHERE id = ANY($1)`, sourceIDs, masterID); err != nil {
		return err
	}
	// The admin chose the master, so it is now verified.
	if _, err := tx.Exec(ctx, `UPDATE `+t+` SET status = 'verified', updated_at = NOW() WHERE id = $1`, masterID); err != nil {
		return err
	}
	// Repoint jobs to the master in the same transaction. Skills keep their
	// order, without duplicates. (Profiles: the profile module adds its own
	// repoint here once it stores these ids.)
	if err := repointJobs(ctx, tx, kind, sourceIDs, &masterID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *TaxonomyRepository) Update(ctx context.Context, kind reference.Kind, id int64, name *reference.CleanedName, status *string) (*reference.Term, error) {
	var display, normalized *string
	if name != nil {
		display, normalized = &name.Display, &name.Normalized
	}
	query := `UPDATE ` + table(kind) + ` SET
			name = COALESCE($2, name), name_norm = COALESCE($3, name_norm),
			status = COALESCE($4, status), updated_at = NOW()
		WHERE id = $1 AND status <> 'merged'
		RETURNING ` + termColumns
	term, err := scanTerm(r.pool.QueryRow(ctx, query, id, display, normalized, status))
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, &reference.NotFoundError{Message: "term not found"}
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return nil, &reference.ConflictError{Message: "a term with this name already exists; merge them instead"}
	}
	return term, err
}

func (r *TaxonomyRepository) Delete(ctx context.Context, kind reference.Kind, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// ON DELETE CASCADE removes the term's aliases too; collect their ids first
	// so jobs drop every one of them (designation FKs are SET NULL by the DB).
	rows, err := tx.Query(ctx, `DELETE FROM `+table(kind)+` WHERE id = $1 OR merged_into = $1 RETURNING id`, id)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return &reference.NotFoundError{Message: "term not found"}
	}
	if err := repointJobs(ctx, tx, kind, ids, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// repointJobs replaces term ids in jobs and profiles with master (merge), or
// removes them when master is nil (delete).
func repointJobs(ctx context.Context, tx pgx.Tx, kind reference.Kind, ids []int64, master *int64) error {
	if kind == reference.Designation {
		if _, err := tx.Exec(ctx, `UPDATE jobs SET designation_id = $2, updated_at = NOW() WHERE designation_id = ANY($1)`, ids, master); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE profiles SET updated_at = NOW(), designation_ids = `+repointArray("designation_ids")+`
			WHERE designation_ids && $1`, ids, master)
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE jobs SET updated_at = NOW(), skill_ids = `+repointArray("skill_ids")+`
		WHERE skill_ids && $1`, ids, master); err != nil {
		return err
	}
	if master == nil {
		return nil // profile_skills rows go with the deleted terms (ON DELETE CASCADE)
	}
	// A profile keeps one row per skill: drop an alias when the profile already
	// has the master or an earlier alias, then rename the rest.
	if _, err := tx.Exec(ctx, `
		DELETE FROM profile_skills p USING profile_skills q
		WHERE p.skill_id = ANY($1) AND q.user_id = p.user_id
			AND (q.skill_id = $2 OR (q.skill_id = ANY($1) AND q.position < p.position))`, ids, *master); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE profile_skills SET skill_id = $2 WHERE skill_id = ANY($1)`, ids, *master)
	return err
}

// repointArray rewrites an ordered id array: ids in $1 become $2 (or are
// dropped when $2 is null), duplicates collapse to their first position.
func repointArray(column string) string {
	return `ARRAY(
		SELECT v FROM (
			SELECT CASE WHEN u.id = ANY($1) THEN $2::bigint ELSE u.id END AS v, MIN(u.ord) AS first
			FROM unnest(` + column + `) WITH ORDINALITY AS u(id, ord) GROUP BY 1
		) kept WHERE v IS NOT NULL ORDER BY first)`
}

func (r *TaxonomyRepository) Export(ctx context.Context, kind reference.Kind) ([]reference.Term, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+termColumns+` FROM `+table(kind)+` WHERE status <> 'merged' ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (reference.Term, error) {
		t, err := scanTerm(row)
		if err != nil {
			return reference.Term{}, err
		}
		return *t, nil
	})
}
