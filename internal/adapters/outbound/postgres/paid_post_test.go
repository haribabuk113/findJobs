package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/payments"
	"findJobs/internal/domain/reference"
)

func TestPaidJobPosts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := &RecruitingRepository{pool: pool}
	companyRepo := NewCompanyRepository(pool)
	admin, hr, employee := testUser(t, pool, "Post Admin"), testUser(t, pool, "Post HR"), testUser(t, pool, "Post Employee")
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	slug := "paidpost-" + uuid.NewString()[:8]
	if _, err := companyRepo.Create(ctx, testCompanyInput(t, slug), "IN", admin); err != nil {
		t.Fatal(err)
	}
	var companyID string
	pool.QueryRow(ctx, `SELECT id FROM companies WHERE slug = $1`, slug).Scan(&companyID)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM jobs WHERE company_id = $1`, companyID)
		pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, companyID)
	})
	for user, role := range map[string]string{hr: "hr", employee: "employee"} {
		if _, err := pool.Exec(ctx, `INSERT INTO company_members (company_id, user_id, role, status) VALUES ($1, $2, $3, 'current')`,
			companyID, user, role); err != nil {
			t.Fatal(err)
		}
	}
	cleanupWallets(t, pool, admin)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM payments WHERE user_id = $1`, admin) })
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (company_id, balance_paise) VALUES ($1, 1500)`, companyID); err != nil {
		t.Fatal(err)
	}

	role, _, err := NewTaxonomyRepository(pool).Resolve(ctx, reference.Designation, names(t, "Paid role"), "")
	if err != nil {
		t.Fatal(err)
	}
	credits := &jobs.Charge{Method: jobs.PayCompanyCredits, FeePaise: 500}
	post := func(user string, charge *jobs.Charge) (*jobs.Record, error) {
		rec := testJob(t, repo, user, role[0].ID, nil, true)
		pool.Exec(ctx, `DELETE FROM jobs WHERE id = $1`, rec.ID) // reuse the normalized record, store it here
		rec.ID, rec.ShareHash, rec.CompanyID = uuid.NewString(), randomHex(), &companyID
		return rec, repo.Create(ctx, rec, charge)
	}
	balance := func() int64 {
		b, _ := walletBalanceOf(ctx, pool, "company_id", companyID)
		return b
	}
	jobExists := func(rec *jobs.Record) bool {
		var n int
		pool.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE id = $1`, rec.ID).Scan(&n)
		return n == 1
	}

	// Employees can't spend company credits; HR starts with a ₹0 limit.
	if rec, err := post(employee, credits); !jobs.IsForbidden(err) || jobExists(rec) {
		t.Fatalf("employee: %v", err)
	}
	if _, err := post(hr, credits); !payReason(err, jobs.ReasonAllowanceExceeded, 0) {
		t.Fatalf("HR without a limit: %v", err)
	}
	// ₹6 a month: one ₹5 post fits, the second doesn't (₹1 left).
	if err := companyRepo.SetAllowance(ctx, slug, admin, hr, companies.Allowance{CapType: "monthly", LimitPaise: 600}); err != nil {
		t.Fatal(err)
	}
	if err := companyRepo.SetAllowance(ctx, slug, hr, hr, companies.Allowance{CapType: "monthly", LimitPaise: 99999}); err != companies.ErrNotAdmin {
		t.Fatalf("HR raising their own limit: %v", err)
	}
	if _, err := post(hr, credits); err != nil || balance() != 1000 {
		t.Fatalf("HR within the limit: %v, balance %d", err, balance())
	}
	if _, err := post(hr, credits); !payReason(err, jobs.ReasonAllowanceExceeded, 100) || balance() != 1000 {
		t.Fatalf("HR over the limit: %v, balance %d", err, balance())
	}
	poster, _ := repo.PosterCompanies(ctx, hr)
	if len(poster) != 1 || poster[0].CreditsAllowancePaise == nil || *poster[0].CreditsAllowancePaise != 100 {
		t.Fatalf("HR's remaining allowance: %+v", poster)
	}
	// Admins have no cap, only the balance.
	for range 2 {
		if _, err := post(admin, credits); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := post(admin, credits); !payReason(err, jobs.ReasonInsufficientBalance, 0) {
		t.Fatalf("empty wallet: %v", err)
	}
	var debits int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM wallet_ledger l JOIN wallets w ON w.id = l.wallet_id
		WHERE w.company_id = $1 AND l.kind = 'job_post' AND l.amount_paise = -500`, companyID).Scan(&debits)
	if debits != 3 {
		t.Fatalf("job_post ledger rows: %d, want 3", debits)
	}

	// Offboarding releases the allowance.
	if err := companyRepo.Offboard(ctx, slug, admin, hr); err != nil {
		t.Fatal(err)
	}
	var limit int64
	pool.QueryRow(ctx, `SELECT limit_paise FROM company_members WHERE company_id = $1 AND user_id = $2`, companyID, hr).Scan(&limit)
	if limit != 0 {
		t.Fatalf("offboarded limit: %d", limit)
	}

	// Out of pocket: pending (hidden, can't be closed or reposted) until the webhook publishes it.
	rec, err := post(admin, &jobs.Charge{Method: jobs.PayOutOfPocket, FeePaise: 500})
	if err != nil {
		t.Fatal(err)
	}
	if d, err := repo.Detail(ctx, rec.ID, false, admin); err != nil || d.Status != jobs.StatePending {
		t.Fatalf("owner's view of a pending job: %+v %v", d, err)
	}
	if d, _ := repo.Detail(ctx, rec.ID, false, employee); d != nil && !d.CanManage {
		t.Fatalf("a pending job is public: %+v", d)
	}
	if err := repo.Close(ctx, rec.ID, admin); err != jobs.ErrAwaitingPayment {
		t.Fatalf("close pending: %v", err)
	}
	if err := repo.Repost(ctx, rec.ID, admin); err != jobs.ErrAwaitingPayment {
		t.Fatalf("repost pending: %v", err)
	}
	wallets := NewWalletRepository(pool)
	order := "order_" + uuid.NewString()[:8]
	if err := wallets.CreatePayment(ctx, payments.Payment{ID: uuid.NewString(), OrderID: order, JobID: rec.ID, UserID: admin, AmountPaise: 500}); err != nil {
		t.Fatal(err)
	}
	for range 2 { // a replayed webhook is a no-op
		if err := wallets.CompletePayment(ctx, payments.Paid{OrderID: order, PaymentID: "pay_j", AmountPaise: 500, Currency: "INR"}); err != nil {
			t.Fatal(err)
		}
	}
	if d, err := repo.Detail(ctx, rec.ID, false, employee); err != nil || d.Status != jobs.StateActive {
		t.Fatalf("published after payment: %+v %v", d, err)
	}
	if err := repo.PublishPending(ctx, rec.ID, admin); err != jobs.ErrAlreadyPaid {
		t.Fatalf("publish twice: %v", err)
	}

	// The Idempotency-Key finds the first post; a second insert with it is refused.
	keyed, _ := post(admin, nil)
	pool.Exec(ctx, `DELETE FROM jobs WHERE id = $1`, keyed.ID)
	keyed.IdempotencyKey = "key-" + uuid.NewString()
	if err := repo.Create(ctx, keyed, nil); err != nil {
		t.Fatal(err)
	}
	if id, err := repo.PostByKey(ctx, admin, keyed.IdempotencyKey); err != nil || id != keyed.ID {
		t.Fatalf("PostByKey = %q, %v", id, err)
	}
	keyed.ID, keyed.ShareHash = uuid.NewString(), randomHex()
	if err := repo.Create(ctx, keyed, nil); err != jobs.ErrPostInProgress {
		t.Fatalf("second post with the same key: %v", err)
	}
}

func payReason(err error, reason string, available int64) bool {
	pe, ok := err.(*jobs.PaymentRequiredError)
	return ok && pe.Reason == reason && pe.FeePaise == 500 && pe.BalancePaise == available
}
