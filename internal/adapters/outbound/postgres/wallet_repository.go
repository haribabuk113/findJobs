package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/payments"
	"findJobs/internal/ports"
)

// WalletRepository is the Postgres adapter for wallets and payments.
type WalletRepository struct {
	pool *pgxpool.Pool
}

// NewWalletRepository creates the PostgreSQL adapter for wallets.
func NewWalletRepository(pool *pgxpool.Pool) ports.WalletRepository {
	return &WalletRepository{pool: pool}
}

// walletKey is the wallets column and value that identify owner's wallet,
// after checking that a company owner is a current admin of a live company.
func (r *WalletRepository) walletKey(ctx context.Context, owner payments.Owner) (column, value string, err error) {
	if owner.CompanySlug == "" {
		return "user_id", owner.UserID, nil
	}
	var companyID string
	var isAdmin bool
	err = r.pool.QueryRow(ctx, `
		SELECT c.id, EXISTS (SELECT 1 FROM company_members m WHERE m.company_id = c.id AND m.user_id = $2
			AND m.role = 'admin' AND m.status = 'current')
		FROM companies c WHERE c.slug = $1 AND c.deleted_at IS NULL`, owner.CompanySlug, owner.UserID).Scan(&companyID, &isAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", companies.ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	if !isAdmin {
		return "", "", companies.ErrNotAdmin
	}
	return "company_id", companyID, nil
}

func (r *WalletRepository) Wallet(ctx context.Context, owner payments.Owner, page, pageSize int) (*payments.Wallet, error) {
	column, value, err := r.walletKey(ctx, owner)
	if err != nil {
		return nil, err
	}
	w := &payments.Wallet{Entries: []payments.Entry{}, Page: page, PageSize: pageSize}
	var walletID string
	// column is one of two fixed names from walletKey, never user input.
	err = r.pool.QueryRow(ctx, `SELECT id, balance_paise FROM wallets WHERE `+column+` = $1`, value).Scan(&walletID, &w.BalancePaise)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, nil
	}
	if err != nil {
		return nil, err
	}
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_ledger WHERE wallet_id = $1`, walletID).Scan(&w.Total); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.kind, l.amount_paise, l.balance_after_paise, COALESCE(u.name, ''), l.ref_type, l.ref_id, l.created_at
		FROM wallet_ledger l LEFT JOIN users u ON u.id = l.actor_user_id
		WHERE l.wallet_id = $1 ORDER BY l.id DESC LIMIT $2 OFFSET $3`, walletID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	w.Entries, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (payments.Entry, error) {
		var e payments.Entry
		err := row.Scan(&e.ID, &e.Kind, &e.AmountPaise, &e.BalanceAfterPaise, &e.ActorName, &e.RefType, &e.RefID, &e.CreatedAt)
		return e, err
	})
	return w, err
}

func (r *WalletRepository) OpenWallet(ctx context.Context, owner payments.Owner) (string, error) {
	column, value, err := r.walletKey(ctx, owner)
	if err != nil {
		return "", err
	}
	var id string
	// The no-op update makes RETURNING give the existing row's id too.
	err = r.pool.QueryRow(ctx, `INSERT INTO wallets (`+column+`) VALUES ($1)
		ON CONFLICT (`+column+`) DO UPDATE SET `+column+` = EXCLUDED.`+column+` RETURNING id`, value).Scan(&id)
	return id, err
}

func (r *WalletRepository) CreatePayment(ctx context.Context, p payments.Payment) error {
	purpose := "topup"
	if p.JobID != "" {
		purpose = "job_post"
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (id, gateway_order_id, purpose, wallet_id, job_id, user_id, amount_paise)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, NULLIF($5, ''), $6, $7)`,
		p.ID, p.OrderID, purpose, p.WalletID, p.JobID, p.UserID, p.AmountPaise)
	return err
}

func (r *WalletRepository) CompletePayment(ctx context.Context, paid payments.Paid) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// The row lock serialises concurrent deliveries of the same webhook: the
	// second one waits, then sees status 'paid' and does nothing.
	var id, userID, status string
	var walletID, jobID *string
	var amount int64
	err = tx.QueryRow(ctx, `
		SELECT id, wallet_id, job_id, user_id, amount_paise, status FROM payments WHERE gateway_order_id = $1 FOR UPDATE`,
		paid.OrderID).Scan(&id, &walletID, &jobID, &userID, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return payments.ErrUnknownOrder
	}
	if err != nil {
		return err
	}
	if status == "paid" {
		return nil
	}
	if paid.AmountPaise != amount || paid.Currency != "INR" {
		return payments.ErrAmountMismatch
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status = 'paid', gateway_payment_id = $2, paid_at = NOW() WHERE id = $1`,
		id, paid.PaymentID); err != nil {
		return err
	}
	if walletID == nil {
		// A job post. Only a pending job is published; a job that left
		// pending meanwhile (its company was deleted) stays as it is.
		if jobID != nil {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET `+publishPending+` WHERE id = $1 AND status = 'pending_payment'`, *jobID); err != nil {
				return err
			}
		}
		return tx.Commit(ctx)
	}
	var balance int64
	if err := tx.QueryRow(ctx, `UPDATE wallets SET balance_paise = balance_paise + $2, updated_at = NOW() WHERE id = $1
		RETURNING balance_paise`, *walletID, amount).Scan(&balance); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledger (wallet_id, kind, amount_paise, balance_after_paise, actor_user_id, ref_type, ref_id, idempotency_key)
		VALUES ($1, 'topup', $2, $3, $4, 'payment', $5, 'payment:' || $5)`, *walletID, amount, balance, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// walletBalance is the user's personal balance (0 without a wallet), read
// through the pool or inside a transaction.
func walletBalance(ctx context.Context, db queryRower, userID string) (int64, error) {
	return walletBalanceOf(ctx, db, "user_id", userID)
}

// walletBalanceOf is the balance of the wallet whose column (user_id or
// company_id, never user input) is ownerID; 0 without a wallet.
func walletBalanceOf(ctx context.Context, db queryRower, column, ownerID string) (int64, error) {
	var balance int64
	err := db.QueryRow(ctx, `SELECT COALESCE((SELECT balance_paise FROM wallets WHERE `+column+` = $1), 0)`, ownerID).Scan(&balance)
	return balance, err
}

// debitWallet takes amount from the wallet whose column (user_id or
// company_id) is ownerID, with its ledger row, inside the caller's
// transaction. The conditional UPDATE never overdraws: ok is false (nothing
// changed, balance is the current one) when the balance is short. The row
// lock serialises concurrent debits.
func debitWallet(ctx context.Context, tx pgx.Tx, column, ownerID, actorID string, amount int64,
	kind, refType, refID string) (ok bool, balance int64, err error) {
	var walletID string
	err = tx.QueryRow(ctx, `UPDATE wallets SET balance_paise = balance_paise - $2, updated_at = NOW()
		WHERE `+column+` = $1 AND balance_paise >= $2 RETURNING id, balance_paise`, ownerID, amount).Scan(&walletID, &balance)
	if errors.Is(err, pgx.ErrNoRows) {
		balance, err = walletBalanceOf(ctx, tx, column, ownerID)
		return false, balance, err
	}
	if err != nil {
		return false, 0, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO wallet_ledger (wallet_id, kind, amount_paise, balance_after_paise, actor_user_id, ref_type, ref_id, idempotency_key)
		VALUES ($1, $2, -$3::bigint, $4, $5, $6, $7, $2 || ':' || $7)`, walletID, kind, amount, balance, actorID, refType, refID)
	return err == nil, balance, err
}

// capMonth is the IST month a monthly allowance counts; spentNow is a
// member's (alias m) spending in the current cap period.
const (
	capMonth = `date_trunc('month', NOW() AT TIME ZONE 'Asia/Kolkata')::date`
	spentNow = `(CASE WHEN m.cap_type = 'monthly' AND m.spent_month IS DISTINCT FROM ` + capMonth + ` THEN 0 ELSE m.spent_paise END)`
)

// chargeCompanyPost pays a job post from the company wallet inside the
// caller's transaction: admins without a cap, HR within their allowance,
// employees not at all. The member row lock serialises one person's posts.
func chargeCompanyPost(ctx context.Context, tx pgx.Tx, companyID, userID, jobID string, fee int64) error {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM company_members WHERE company_id = $1 AND user_id = $2 AND status = 'current'
		FOR UPDATE`, companyID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return &jobs.ForbiddenError{Message: "you can only post under companies you're an approved member of"}
	}
	if err != nil {
		return err
	}
	switch role {
	case companies.RoleEmployee:
		return &jobs.ForbiddenError{Message: "only company admins and HR can spend company credits"}
	case companies.RoleHR:
		var remaining int64
		err := tx.QueryRow(ctx, `
			UPDATE company_members m SET spent_paise = `+spentNow+` + $3, spent_month = `+capMonth+`, updated_at = NOW()
			WHERE company_id = $1 AND user_id = $2 AND `+spentNow+` + $3 <= m.limit_paise
			RETURNING m.limit_paise - m.spent_paise`, companyID, userID, fee).Scan(&remaining)
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(ctx, `SELECT GREATEST(0, m.limit_paise - `+spentNow+`) FROM company_members m
				WHERE company_id = $1 AND user_id = $2`, companyID, userID).Scan(&remaining); err != nil {
				return err
			}
			return &jobs.PaymentRequiredError{Reason: jobs.ReasonAllowanceExceeded, FeePaise: fee, BalancePaise: remaining,
				Message: "this post is over your company spending limit; ask a company admin to raise it"}
		}
		if err != nil {
			return err
		}
	}
	ok, balance, err := debitWallet(ctx, tx, "company_id", companyID, userID, fee, "job_post", "job", jobID)
	if err != nil {
		return err
	}
	if !ok {
		return &jobs.PaymentRequiredError{Reason: jobs.ReasonInsufficientBalance, FeePaise: fee, BalancePaise: balance,
			Message: "the company wallet balance is too low for this post; ask a company admin to top it up"}
	}
	return nil
}
