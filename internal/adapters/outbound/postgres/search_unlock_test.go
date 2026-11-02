package postgres

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

func TestMatchSortUnlock(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewJobRepository(pool)
	jobsRepo := &RecruitingRepository{pool: pool}
	owner, viewer := testUser(t, pool, "Unlock Owner"), testUser(t, pool, "Unlock Viewer")
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	taxonomy := NewTaxonomyRepository(pool)
	skills, _, err := taxonomy.Resolve(ctx, reference.Skill, names(t, "Unlock A", "Unlock B"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM skills WHERE id = ANY($1)`, []int64{skills[0].ID, skills[1].ID})
	})
	role, _, err := taxonomy.Resolve(ctx, reference.Designation, names(t, "Unlock role"), "")
	if err != nil {
		t.Fatal(err)
	}
	a, b := skills[0].ID, skills[1].ID
	// The viewer has skill A: half match, full match, no match (posted newest first).
	half := testJob(t, jobsRepo, owner, role[0].ID, []int64{a, b}, true)
	full := testJob(t, jobsRepo, owner, role[0].ID, []int64{a}, true)
	none := testJob(t, jobsRepo, owner, role[0].ID, []int64{b}, true)
	if err := NewProfileRepository(pool).Save(ctx, viewer, testProfile("unlock-"+uuid.NewString()[:8]),
		[]ports.ProfileSkill{{SkillID: a, Group: "Core"}}, nil); err != nil {
		t.Fatal(err)
	}
	cleanupWallets(t, pool, viewer)
	if _, err := pool.Exec(ctx, `INSERT INTO wallets (user_id, balance_paise) VALUES ($1, 3000)`, viewer); err != nil {
		t.Fatal(err)
	}

	c := jobs.SearchCriteria{JobIDs: []string{half.ID, full.ID, none.ID}, Sort: jobs.SortMatch}
	if err := c.Normalize(); err != nil {
		t.Fatal(err)
	}
	u, err := repo.Unlock(ctx, viewer, c.UnlockHash(), 2000)
	if err != nil || u.ChargedPaise != 2000 {
		t.Fatalf("unlock: %+v %v", u, err)
	}
	again, err := repo.Unlock(ctx, viewer, c.UnlockHash(), 2000)
	if err != nil || again.ID != u.ID || again.ChargedPaise != 0 {
		t.Fatalf("unlocking the same filters again must be free: %+v %v", again, err)
	}
	if bal, _ := walletBalance(ctx, pool, viewer); bal != 1000 {
		t.Fatalf("balance %d, want 1000", bal)
	}
	other := c
	other.Query = "nothing like this"
	if _, err := repo.Unlock(ctx, viewer, other.UnlockHash(), 2000); !isJobsPayment(err, jobs.ReasonInsufficientBalance, 1000) {
		t.Fatalf("short balance: %v", err)
	}
	if ok, _ := repo.UnlockValid(ctx, viewer, u.ID, other.UnlockHash()); ok {
		t.Fatal("an unlock must not cover other filters")
	}
	if ok, _ := repo.UnlockValid(ctx, owner, u.ID, c.UnlockHash()); ok {
		t.Fatal("an unlock must not cover another user")
	}

	res, err := repo.Search(ctx, &c, viewer)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, j := range res.Jobs {
		got = append(got, j.ID)
	}
	if want := []string{full.ID, half.ID, none.ID}; !slices.Equal(got, want) {
		t.Fatalf("match order %v, want %v", got, want)
	}
	// A page past the end still reports the total (the match sort's own
	// argument must not reach the fallback count).
	past := c
	past.Page = 9
	if res, err := repo.Search(ctx, &past, viewer); err != nil || len(res.Jobs) != 0 || res.Total != 3 {
		t.Fatalf("past the end: %+v %v", res, err)
	}
}

func isJobsPayment(err error, reason string, balance int64) bool {
	pe, ok := err.(*jobs.PaymentRequiredError)
	return ok && pe.Reason == reason && pe.FeePaise == 2000 && pe.BalancePaise == balance
}
