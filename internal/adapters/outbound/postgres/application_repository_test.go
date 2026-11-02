package postgres

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/applications"
	"findJobs/internal/domain/payments"
	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

func TestApplicationLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewApplicationRepository(pool)
	jobsRepo := &RecruitingRepository{pool: pool}
	taxonomy := NewTaxonomyRepository(pool)
	owner, candidate, stranger := testUser(t, pool, "Owner"), testUser(t, pool, "Candidate"), testUser(t, pool, "Stranger")
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	n := names(t, "Apply skill A "+uuid.NewString()[:6], "Apply skill B "+uuid.NewString()[:6], "Test role")
	skills, _, err := taxonomy.Resolve(ctx, reference.Skill, n[:2], "")
	if err != nil {
		t.Fatal(err)
	}
	role, _, err := taxonomy.Resolve(ctx, reference.Designation, n[2:], "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM skills WHERE id = ANY($1)`, []int64{skills[0].ID, skills[1].ID})
	})
	easy := testJob(t, jobsRepo, owner, role[0].ID, []int64{skills[0].ID, skills[1].ID}, false)
	other := testJob(t, jobsRepo, owner, role[0].ID, []int64{skills[0].ID}, false)
	external := testJob(t, jobsRepo, owner, role[0].ID, nil, true)
	if _, err := pool.Exec(ctx, `UPDATE jobs SET application = 'External' WHERE id = $1`, external.ID); err != nil {
		t.Fatal(err)
	}

	// The candidate has one of the two required skills and keeps current salary visible.
	p := testProfile("ap-" + uuid.NewString()[:8])
	p.CurrentSalary = 90
	if err := NewProfileRepository(pool).Save(ctx, candidate, p, []ports.ProfileSkill{{SkillID: skills[0].ID, Group: "Core"}}, nil); err != nil {
		t.Fatal(err)
	}
	day := applications.QuotaDay(time.Now())

	if _, err := repo.Apply(ctx, stranger, easy.ID, day, 5, 500, false); err != applications.ErrNoProfile {
		t.Fatalf("no profile: %v", err)
	}
	if _, err := repo.Apply(ctx, owner, easy.ID, day, 5, 500, false); err != applications.ErrOwnJob {
		t.Fatalf("own job: %v", err)
	}
	if _, err := repo.Apply(ctx, candidate, external.ID, day, 5, 500, false); err != applications.ErrNotEasyApply {
		t.Fatalf("external via easy apply: %v", err)
	}
	a, err := repo.Apply(ctx, candidate, easy.ID, day, 1, 500, false)
	if err != nil || *a.MatchPct != 50 || a.Stage != applications.StageNew || a.JobStatus != "active" {
		t.Fatalf("apply: %+v %v", a, err)
	}
	if _, err := repo.Apply(ctx, candidate, easy.ID, day, 5, 500, false); err != applications.ErrAlreadyApplied {
		t.Fatalf("duplicate: %v", err)
	}
	// Limit 1 is used up: the second job is refused and nothing is stored.
	if _, err := repo.Apply(ctx, candidate, other.ID, day, 1, 500, false); !needs(err, applications.ReasonConfirmPaid, 0) {
		t.Fatalf("quota: %v", err)
	}
	if used, wallet, _ := repo.QuotaUsed(ctx, candidate, day); used != 1 || wallet != 0 {
		t.Fatalf("quota used: %d", used)
	}
	if _, err := repo.ApplyExternal(ctx, candidate, easy.ID); err != applications.ErrNotExternal {
		t.Fatalf("external confirm on easy apply job: %v", err)
	}
	if _, err := repo.ApplyExternal(ctx, candidate, external.ID); err != nil {
		t.Fatalf("external confirm: %v", err)
	}
	if mine, err := repo.Mine(ctx, candidate); err != nil || len(mine) != 2 {
		t.Fatalf("mine: %+v %v", mine, err)
	}

	// Recruiter side: only the job's team sees applicants; external ones aren't listed.
	if _, err := repo.Applicants(ctx, easy.ID, stranger, applications.ApplicantFilter{Sort: "recent"}); err != applications.ErrJobNotFound {
		t.Fatalf("stranger applicants: %v", err)
	}
	list, err := repo.Applicants(ctx, easy.ID, owner, applications.ApplicantFilter{Sort: "match"})
	if err != nil || len(list) != 1 {
		t.Fatalf("applicants: %+v %v", list, err)
	}
	got := list[0]
	if got.MatchPct != 50 || !slices.Equal(got.MatchedSkills, []string{skills[0].Name}) || !slices.Equal(got.UnmatchedSkills, []string{skills[1].Name}) ||
		got.Dossier.Name != p.Name || got.Dossier.CurrentSalary == nil || *got.Dossier.CurrentSalary != 90 {
		t.Fatalf("applicant: %+v", got)
	}
	if high, _ := repo.Applicants(ctx, easy.ID, owner, applications.ApplicantFilter{MinMatch: 70, Sort: "recent"}); len(high) != 0 {
		t.Fatalf("min match filter: %+v", high)
	}

	// Stage changes count as recruiter actions; repeating a stage doesn't.
	if _, err := repo.SetStage(ctx, got.ID, stranger, applications.StageRejected); err != applications.ErrApplicationGone {
		t.Fatalf("stranger stage: %v", err)
	}
	for range 2 {
		if s, err := repo.SetStage(ctx, got.ID, owner, applications.StageShortlisted); err != nil || s.Stage != applications.StageShortlisted || s.StageUpdatedAt == nil {
			t.Fatalf("shortlist: %+v %v", s, err)
		}
	}
	var actions int
	pool.QueryRow(ctx, `SELECT recruiter_actions FROM jobs WHERE id = $1`, easy.ID).Scan(&actions)
	if actions != 1 {
		t.Fatalf("recruiter actions: %d", actions)
	}
	if mine, _ := repo.Mine(ctx, candidate); mine[len(mine)-1].Stage != applications.StageShortlisted {
		t.Fatalf("candidate sees the stage: %+v", mine)
	}

	// Past the free allowance, confirming pays the fee from the wallet. Two
	// paid applications race for a balance that covers one: one is charged,
	// the other is refused and stores nothing.
	cleanupWallets(t, pool, candidate)
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (user_id, balance_paise) VALUES ($1, 700)`, candidate); err != nil {
		t.Fatal(err)
	}
	third := testJob(t, jobsRepo, owner, role[0].ID, []int64{skills[0].ID}, false)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, jobID := range []string{other.ID, third.ID} {
		wg.Go(func() { _, errs[i] = repo.Apply(ctx, candidate, jobID, day, 1, 500, true) })
	}
	wg.Wait()
	if !(errs[0] == nil && needs(errs[1], applications.ReasonInsufficientBalance, 200) ||
		errs[1] == nil && needs(errs[0], applications.ReasonInsufficientBalance, 200)) {
		t.Fatalf("racing paid applications: %v", errs)
	}
	w, err := NewWalletRepository(pool).Wallet(ctx, payments.Owner{UserID: candidate}, 1, 20)
	if err != nil || w.BalancePaise != 200 || len(w.Entries) != 1 || w.Entries[0].Kind != "apply" || w.Entries[0].AmountPaise != -500 {
		t.Fatalf("wallet after a paid application: %+v %v", w, err)
	}
	var paid, stored int
	pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE paid), COUNT(*) FROM applications WHERE user_id = $1`, candidate).Scan(&paid, &stored)
	if paid != 1 || stored != 3 {
		t.Fatalf("paid %d of %d applications; want 1 of 3", paid, stored)
	}
}

// needs reports whether err is a 402 for reason with the given balance.
func needs(err error, reason string, balance int64) bool {
	pe, ok := err.(*applications.PaymentRequiredError)
	return ok && pe.Reason == reason && pe.FeePaise == 500 && pe.BalancePaise == balance
}

func TestApplyBulkPaid(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewApplicationRepository(pool)
	jobsRepo := &RecruitingRepository{pool: pool}
	owner, candidate := testUser(t, pool, "Bulk Owner"), testUser(t, pool, "Bulk Candidate")
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	role, _, err := NewTaxonomyRepository(pool).Resolve(ctx, reference.Designation, names(t, "Bulk role"), "")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for range 4 {
		ids = append(ids, testJob(t, jobsRepo, owner, role[0].ID, nil, false).ID)
	}
	if err := NewProfileRepository(pool).Save(ctx, candidate, testProfile("bulk-"+uuid.NewString()[:8]), nil, nil); err != nil {
		t.Fatal(err)
	}
	cleanupWallets(t, pool, candidate)
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (user_id, balance_paise) VALUES ($1, 1000)`, candidate); err != nil {
		t.Fatal(err)
	}
	day := applications.QuotaDay(time.Now())

	// 1 free + 2 paid at ₹5, with ₹10 in the wallet.
	plan, err := repo.ApplyBulk(ctx, candidate, ids[:3], day, 1, 500, true, 0)
	if err != nil || plan.Free != 1 || plan.Paid != 2 || plan.FeePaise != 1000 || plan.WalletPaise != 1000 {
		t.Fatalf("dry run: %+v %v", plan, err)
	}
	if _, err := repo.ApplyBulk(ctx, candidate, ids[:3], day, 1, 500, false, 1); !isPaymentReason(err, applications.ReasonConfirmPaid) {
		t.Fatalf("wrong confirm count: %v", err)
	}
	res, err := repo.ApplyBulk(ctx, candidate, ids[:3], day, 1, 500, false, 2)
	if err != nil || len(res.Applications) != 3 {
		t.Fatalf("confirmed: %+v %v", res, err)
	}
	var balance int64
	var paid, entries int
	pool.QueryRow(ctx, `SELECT balance_paise, (SELECT COUNT(*) FROM wallet_ledger l WHERE l.wallet_id = w.id AND l.amount_paise = -500),
		(SELECT COUNT(*) FROM applications WHERE user_id = $1 AND paid) FROM wallets w WHERE user_id = $1`, candidate).Scan(&balance, &entries, &paid)
	if balance != 0 || entries != 2 || paid != 2 {
		t.Fatalf("balance %d, ledger rows %d, paid applications %d; want 0, 2, 2", balance, entries, paid)
	}
	// The wallet is empty now: nothing is applied.
	if _, err := repo.ApplyBulk(ctx, candidate, ids[3:], day, 1, 500, false, 1); !isPaymentReason(err, applications.ReasonInsufficientBalance) {
		t.Fatalf("empty wallet: %v", err)
	}
}

func isPaymentReason(err error, reason string) bool {
	pe, ok := err.(*applications.PaymentRequiredError)
	return ok && pe.Reason == reason
}
