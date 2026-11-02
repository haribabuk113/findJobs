package postgres

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (company_id, balance_paise) VALUES ($1, 2500)`, companyID); err != nil {
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

	// Employees start at ₹0 (manual: they post out of pocket); with credits
	// assigned they post from the company wallet like HR.
	if rec, err := post(employee, credits); !payReason(err, jobs.ReasonAllowanceExceeded, 0) || jobExists(rec) {
		t.Fatalf("employee without credits: %v", err)
	}
	if err := companyRepo.SetAllowance(ctx, slug, admin, employee, companies.Allowance{CapType: "total", LimitPaise: 500}); err != nil {
		t.Fatal(err)
	}
	if err := companyRepo.SetAllowance(ctx, slug, admin, admin, companies.Allowance{CapType: "total", LimitPaise: 500}); !companies.IsNotFound(err) {
		t.Fatalf("a limit on an admin: %v", err)
	}
	if _, err := post(employee, credits); err != nil || balance() != 2000 {
		t.Fatalf("employee with credits: %v, balance %d", err, balance())
	}
	if _, err := post(employee, credits); !payReason(err, jobs.ReasonAllowanceExceeded, 0) {
		t.Fatalf("employee's total limit used up: %v", err)
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
	if _, err := post(hr, credits); err != nil || balance() != 1500 {
		t.Fatalf("HR within the limit: %v, balance %d", err, balance())
	}
	if _, err := post(hr, credits); !payReason(err, jobs.ReasonAllowanceExceeded, 100) || balance() != 1500 {
		t.Fatalf("HR over the limit: %v, balance %d", err, balance())
	}
	poster, _ := repo.PosterCompanies(ctx, hr)
	if len(poster) != 1 || poster[0].CreditsAllowancePaise == nil || *poster[0].CreditsAllowancePaise != 100 {
		t.Fatalf("HR's remaining allowance: %+v", poster)
	}
	// 6 parallel posts with room for one more (₹6 left of ₹11): the member-row
	// lock lets exactly one through, and only that one is charged.
	if err := companyRepo.SetAllowance(ctx, slug, admin, hr, companies.Allowance{CapType: "monthly", LimitPaise: 1100}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var posted, refused atomic.Int32
	for range 6 {
		wg.Go(func() {
			switch _, err := post(hr, credits); {
			case err == nil:
				posted.Add(1)
			case payReason(err, jobs.ReasonAllowanceExceeded, 100):
				refused.Add(1)
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if posted.Load() != 1 || refused.Load() != 5 || balance() != 1000 {
		t.Fatalf("parallel HR posts: %d posted, %d refused, balance %d", posted.Load(), refused.Load(), balance())
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
	if debits != 5 {
		t.Fatalf("job_post ledger rows: %d, want 5", debits)
	}
	// Dashboard money: 5 × ₹5 posts this month; the HR member's ₹11 cap plus the employee's ₹5.
	if d, err := companyRepo.Dashboard(ctx, slug, admin); err != nil || d.SpentThisMonthPaise != 2500 || d.TotalCapsPaise != 1600 {
		t.Fatalf("dashboard money = %+v, %v", d, err)
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
	if d, _ := companyRepo.Dashboard(ctx, slug, admin); d.TotalCapsPaise != 500 {
		t.Fatalf("total caps after offboarding: %d", d.TotalCapsPaise)
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
		if _, err := wallets.CompletePayment(ctx, payments.Paid{OrderID: order, PaymentID: "pay_j", AmountPaise: 500, Currency: "INR"}); err != nil {
			t.Fatal(err)
		}
	}
	if d, err := repo.Detail(ctx, rec.ID, false, employee); err != nil || d.Status != jobs.StateActive {
		t.Fatalf("published after payment: %+v %v", d, err)
	}
	if err := repo.PublishPending(ctx, rec.ID, admin); err != jobs.ErrAlreadyPaid {
		t.Fatalf("publish twice: %v", err)
	}
	// A second order for the same job, paid too: reconciliation reports it, not the first.
	second := "order_" + uuid.NewString()[:8]
	if err := wallets.CreatePayment(ctx, payments.Payment{ID: uuid.NewString(), OrderID: second, JobID: rec.ID, UserID: admin, AmountPaise: 500}); err != nil {
		t.Fatal(err)
	}
	if _, err := wallets.CompletePayment(ctx, payments.Paid{OrderID: second, PaymentID: "pay_j2", AmountPaise: 500, Currency: "INR"}); err != nil {
		t.Fatal(err)
	}
	dups, err := wallets.DuplicateJobPayments(ctx, time.Now().Add(-time.Hour))
	if err != nil || !slices.Contains(dups, second) || slices.Contains(dups, order) {
		t.Fatalf("DuplicateJobPayments = %v, %v; want %s only", dups, err, second)
	}

	// A member who left can't pay to publish their pending company job.
	left, err := post(employee, &jobs.Charge{Method: jobs.PayOutOfPocket, FeePaise: 500})
	if err != nil {
		t.Fatal(err)
	}
	if pending, err := repo.Pending(ctx, left.ID, employee); err != nil || !pending {
		t.Fatalf("pending while a member: %v %v", pending, err)
	}
	if err := companyRepo.Offboard(ctx, slug, admin, employee); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pending(ctx, left.ID, employee); !jobs.IsForbidden(err) {
		t.Fatalf("pending after leaving: %v", err)
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
