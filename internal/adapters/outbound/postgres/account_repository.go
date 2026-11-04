package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/account"
	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

// AccountRepository is the Postgres adapter for consents and account deletion.
type AccountRepository struct {
	pool *pgxpool.Pool
}

// NewAccountRepository creates the PostgreSQL adapter for accounts.
func NewAccountRepository(pool *pgxpool.Pool) ports.AccountRepository {
	return &AccountRepository{pool: pool}
}

func (r *AccountRepository) LatestConsents(ctx context.Context, userID string) (map[string]account.Consent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (doc_id) doc_id, doc_version, status, created_at
		FROM consents WHERE user_id = $1 ORDER BY doc_id, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[account.Consent])
	if err != nil {
		return nil, err
	}
	out := make(map[string]account.Consent, len(list))
	for _, c := range list {
		out[c.DocID] = c
	}
	return out, nil
}

func (r *AccountRepository) RecordConsent(ctx context.Context, userID, docID, version, status string) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO consents (user_id, ecosystem_id, doc_id, doc_version, status)
		SELECT id, iam_user_id, $2, $3, $4 FROM users WHERE id = $1 AND status <> 'deleted'`, userID, docID, version, status)
	if err == nil && tag.RowsAffected() == 0 {
		return auth.ErrUserNotFound
	}
	return err
}

func (r *AccountRepository) DeleteAccount(ctx context.Context, userID string) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var ecosystemID string
	err = tx.QueryRow(ctx, `SELECT iam_user_id FROM users WHERE id = $1 AND status <> 'deleted' FOR UPDATE`, userID).Scan(&ecosystemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	// Manual: the only admin must appoint another admin or delete the company
	// first. Lock the companies the user administers, as every admin change
	// does, so another admin can't leave at the same moment.
	if _, err := tx.Exec(ctx, `
		SELECT c.id FROM companies c JOIN company_members m ON m.company_id = c.id
		WHERE m.user_id = $1 AND m.status = 'current' AND m.role = 'admin' AND c.deleted_at IS NULL
		ORDER BY c.id FOR UPDATE OF c`, userID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT c.name FROM companies c JOIN company_members m ON m.company_id = c.id
		WHERE m.user_id = $1 AND m.status = 'current' AND m.role = 'admin' AND c.deleted_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM company_members o WHERE o.company_id = c.id AND o.user_id <> $1
				AND o.status = 'current' AND o.role = 'admin')
		ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	sole, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if len(sole) > 0 {
		return nil, &account.SoleAdminError{Companies: sole}
	}

	var keys []string
	if err := tx.QueryRow(ctx, `SELECT array_remove(ARRAY[photo_key, resume_key], NULL) FROM profiles WHERE user_id = $1`,
		userID).Scan(&keys); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	for _, q := range []string{
		// Product data. The profile takes its skills, experiences and analytics with it.
		`DELETE FROM profiles WHERE user_id = $1`,
		`DELETE FROM applications WHERE user_id = $1`,
		`DELETE FROM saved_jobs WHERE user_id = $1`,
		`DELETE FROM apply_quota WHERE user_id = $1`,
		`DELETE FROM search_unlocks WHERE user_id = $1`,
		`DELETE FROM job_drafts WHERE user_id = $1`,
		`DELETE FROM company_members WHERE user_id = $1`,
		// Jobs posted without a company have no one left to manage them: close
		// them. A company's jobs stay with the company.
		`UPDATE jobs SET status = 'closed', is_visible = FALSE, updated_at = NOW()
			WHERE recruiter_id = $1 AND company_id IS NULL AND status IN ('active', 'pending_payment')`,
		`UPDATE jobs SET recruiter_id = NULL, updated_at = NOW() WHERE recruiter_id = $1`,
		// The row stays (wallets, the append-only ledger and payments point at
		// it) but no longer identifies anyone. A new sign-in with the same IAM
		// identity creates a fresh account.
		`UPDATE users SET iam_user_id = 'deleted:' || id, email = '', name = 'Deleted user', status = 'deleted',
			updated_at = NOW() WHERE id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, userID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_events (user_id, ecosystem_id, kind) VALUES ($1, $2, 'deleted')`,
		userID, ecosystemID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return keys, nil
}
