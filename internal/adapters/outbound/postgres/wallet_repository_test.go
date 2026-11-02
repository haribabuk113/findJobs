package postgres

import (
	"context"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/payments"
)

// cleanupWallets removes the test users' wallets, payments and (bypassing the
// append-only trigger) their ledger rows. Register it after testUser, so it
// runs before the users are deleted.
func cleanupWallets(t *testing.T, pool *pgxpool.Pool, userIDs ...string) {
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer tx.Rollback(ctx)
		for _, q := range []string{
			`ALTER TABLE wallet_ledger DISABLE TRIGGER wallet_ledger_append_only`,
			`CREATE TEMP TABLE doomed ON COMMIT DROP AS SELECT id FROM wallets w WHERE w.user_id = ANY($1::uuid[])
				OR w.company_id IN (SELECT company_id FROM company_members WHERE user_id = ANY($1::uuid[]))`,
			`DELETE FROM wallet_ledger WHERE wallet_id IN (SELECT id FROM doomed)`,
			`DELETE FROM payments WHERE wallet_id IN (SELECT id FROM doomed)`,
			`DELETE FROM wallets WHERE id IN (SELECT id FROM doomed)`,
			`ALTER TABLE wallet_ledger ENABLE TRIGGER wallet_ledger_append_only`,
		} {
			args := []any{userIDs}
			if !strings.Contains(q, "$1") {
				args = nil
			}
			if _, err := tx.Exec(ctx, q, args...); err != nil {
				t.Error(err)
				return
			}
		}
		tx.Commit(ctx)
	})
}

func TestWalletTopupCreditsOnce(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewWalletRepository(pool)
	user := testUser(t, pool, "Payer")
	cleanupWallets(t, pool, user)
	me := payments.Owner{UserID: user}

	w, err := repo.Wallet(ctx, me, 1, 20)
	if err != nil || w.BalancePaise != 0 || len(w.Entries) != 0 {
		t.Fatalf("new wallet = %+v, %v", w, err)
	}
	walletID, err := repo.OpenWallet(ctx, me)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := repo.OpenWallet(ctx, me); err != nil || again != walletID {
		t.Fatalf("OpenWallet twice = %q, %v; want %q", again, err, walletID)
	}
	order := "order_" + uuid.NewString()[:8]
	if err := repo.CreatePayment(ctx, payments.Payment{ID: uuid.NewString(), OrderID: order, WalletID: walletID,
		UserID: user, AmountPaise: 50_000}); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.CompletePayment(ctx, payments.Paid{OrderID: order, PaymentID: "pay_x", AmountPaise: 49_999, Currency: "INR"}); err != payments.ErrAmountMismatch {
		t.Fatalf("short payment: err = %v, want ErrAmountMismatch", err)
	}
	if _, err := repo.CompletePayment(ctx, payments.Paid{OrderID: "order_nope", AmountPaise: 50_000, Currency: "INR"}); err != payments.ErrUnknownOrder {
		t.Fatalf("unknown order: err = %v", err)
	}

	// The gateway retries and delivers in parallel: 8 deliveries, one credit.
	paid := payments.Paid{OrderID: order, PaymentID: "pay_1", AmountPaise: 50_000, Currency: "INR"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	var applied atomic.Int32
	for range 8 {
		wg.Go(func() {
			ok, err := repo.CompletePayment(ctx, paid)
			if ok {
				applied.Add(1)
			}
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if applied.Load() != 1 {
		t.Fatalf("applied %d times, want 1", applied.Load())
	}
	w, err = repo.Wallet(ctx, me, 1, 20)
	if err != nil || w.BalancePaise != 50_000 || w.Total != 1 || len(w.Entries) != 1 {
		t.Fatalf("after top-up = %+v, %v", w, err)
	}
	if e := w.Entries[0]; e.Kind != "topup" || e.AmountPaise != 50_000 || e.BalanceAfterPaise != 50_000 || e.ActorName != "Payer" {
		t.Fatalf("entry = %+v", e)
	}
	if all, err := repo.Wallet(ctx, me, 1, 0); err != nil || len(all.Entries) != 1 {
		t.Fatalf("pageSize 0 (all entries) = %+v, %v", all, err)
	}

	// Reconciliation: the credited top-up has its ledger row; a paid one without it is reported.
	since := time.Now().Add(-time.Hour)
	orphan := "order_" + uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO payments (id, gateway_order_id, purpose, wallet_id, user_id, amount_paise, status, paid_at)
		VALUES ($1, $2, 'topup', $3, $4, 700, 'paid', NOW())`, uuid.NewString(), orphan, walletID, user); err != nil {
		t.Fatal(err)
	}
	unledgered, err := repo.UnledgeredTopups(ctx, since)
	if err != nil || !slices.Contains(unledgered, orphan) || slices.Contains(unledgered, order) {
		t.Fatalf("UnledgeredTopups = %v, %v; want %s and not %s", unledgered, err, orphan, order)
	}

	if _, err := pool.Exec(ctx, `UPDATE wallet_ledger SET amount_paise = 1 WHERE id = $1`, w.Entries[0].ID); err == nil {
		t.Fatal("the ledger accepted an UPDATE")
	}
	if _, err := pool.Exec(ctx, `UPDATE wallets SET balance_paise = -1 WHERE id = $1`, walletID); err == nil {
		t.Fatal("a wallet went negative")
	}
}

func TestCompanyWalletNeedsAdmin(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewWalletRepository(pool)
	admin, stranger := testUser(t, pool, "Wallet Admin"), testUser(t, pool, "Wallet Stranger")
	slug := "wallet-" + uuid.NewString()[:8]
	if _, err := NewCompanyRepository(pool).Create(ctx, testCompanyInput(t, slug), "IN", admin); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE slug = $1`, slug) })
	cleanupWallets(t, pool, admin)

	if _, err := repo.OpenWallet(ctx, payments.Owner{UserID: stranger, CompanySlug: slug}); err != companies.ErrNotAdmin {
		t.Fatalf("stranger: err = %v, want ErrNotAdmin", err)
	}
	if _, err := repo.Wallet(ctx, payments.Owner{UserID: admin, CompanySlug: "no-such-" + slug}, 1, 20); err != companies.ErrNotFound {
		t.Fatalf("unknown company: err = %v", err)
	}
	companyWallet, err := repo.OpenWallet(ctx, payments.Owner{UserID: admin, CompanySlug: slug})
	if err != nil {
		t.Fatal(err)
	}
	personal, err := repo.OpenWallet(ctx, payments.Owner{UserID: admin})
	if err != nil || personal == companyWallet {
		t.Fatalf("personal wallet = %q, %v; must differ from the company's", personal, err)
	}
}
