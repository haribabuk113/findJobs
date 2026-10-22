package postgres

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/reference"
)

func testJob(t *testing.T, repo *RecruitingRepository, recruiter string, designation int64, skills []int64, paid bool) *jobs.Record {
	in := jobs.PostInput{Tier: jobs.TierFree, Designation: "Test role", Skills: []string{"x"}, LocationIDs: []string{"1277333"},
		SalaryMax: 100000, Currency: "INR", WorkModes: []string{"Remote"}, WorkTypes: []string{"Full-time"},
		Relocation: "On their own", Application: "Easy apply", Description: "Test."}
	if paid {
		in.Tier, in.PaymentMethod = jobs.TierPaid, jobs.PayOutOfPocket
	}
	if err := in.Normalize(); err != nil {
		t.Fatal(err)
	}
	rec := &jobs.Record{PostInput: in, ID: uuid.NewString(), ShareHash: randomHex(), RecruiterID: recruiter,
		DesignationID: designation, SkillIDs: skills, DBTier: "Free", Listed: paid}
	if paid {
		rec.DBTier = "Premium"
	}
	if err := repo.Create(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.pool.Exec(context.Background(), `DELETE FROM jobs WHERE id = $1`, rec.ID) })
	return rec
}

func randomHex() string {
	return uuid.NewString()[:8] + uuid.NewString()[:8] + uuid.NewString()[:8] + uuid.NewString()[:8]
}

func TestJobVisibility(t *testing.T) {
	pool := testPool(t)
	repo := &RecruitingRepository{pool: pool}
	taxonomy := NewTaxonomyRepository(pool)
	ctx := context.Background()
	owner, stranger := testUser(t, pool, "owner"), testUser(t, pool, "stranger")
	d, _, _ := taxonomy.Resolve(ctx, reference.Designation, names(t, "Test role"), owner)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM designations WHERE id = $1`, d[0].ID) })

	free := testJob(t, repo, owner, d[0].ID, nil, false)
	paid := testJob(t, repo, owner, d[0].ID, nil, true)

	if _, err := repo.Detail(ctx, free.ID, false, stranger); !jobs.IsNotFound(err) {
		t.Errorf("free job by id: want NotFound, got %v", err)
	}
	shared, err := repo.Detail(ctx, free.ShareHash, true, "")
	if err != nil || shared.ShareHash != "" || shared.Analytics != nil {
		t.Errorf("free job by share link: %+v, %v (share hash/analytics must not leak)", shared, err)
	}
	mine, err := repo.Detail(ctx, free.ID, false, owner)
	if err != nil || !mine.CanManage || mine.Analytics == nil || mine.ShareHash != free.ShareHash {
		t.Errorf("owner view: %+v, %v", mine, err)
	}
	if public, err := repo.Detail(ctx, paid.ID, false, ""); err != nil || public.CanManage {
		t.Errorf("paid job anonymous: %+v, %v", public, err)
	}

	if err := repo.Close(ctx, paid.ID, stranger); !jobs.IsNotFound(err) {
		t.Errorf("stranger closing: want NotFound, got %v", err)
	}
	if err := repo.Close(ctx, paid.ID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Detail(ctx, paid.ID, false, stranger); !jobs.IsNotFound(err) {
		t.Errorf("closed job: want NotFound, got %v", err)
	}
	if closed, err := repo.Detail(ctx, paid.ID, false, owner); err != nil || closed.Status != jobs.StateClosed {
		t.Errorf("owner sees closed job: %+v, %v", closed, err)
	}
}

// Merging and deleting terms rewrite the jobs that use them, keeping order.
func TestTermChangesRepointJobs(t *testing.T) {
	pool := testPool(t)
	repo := &RecruitingRepository{pool: pool}
	taxonomy := NewTaxonomyRepository(pool)
	ctx := context.Background()
	owner := testUser(t, pool, "owner")
	s, _, _ := taxonomy.Resolve(ctx, reference.Skill, names(t, "Alpha", "Beta", "Gamma"), owner)
	d, _, _ := taxonomy.Resolve(ctx, reference.Designation, names(t, "Role one", "Role two"), owner)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM skills WHERE id = ANY($1)`, []int64{s[0].ID, s[1].ID, s[2].ID})
		pool.Exec(context.Background(), `DELETE FROM designations WHERE id = ANY($1)`, []int64{d[0].ID, d[1].ID})
	})
	a, b, c := s[0].ID, s[1].ID, s[2].ID
	job := testJob(t, repo, owner, d[1].ID, []int64{b, a, c}, true)

	skillIDs := func() []int64 {
		var ids []int64
		pool.QueryRow(ctx, `SELECT skill_ids FROM jobs WHERE id = $1`, job.ID).Scan(&ids)
		return ids
	}
	if err := taxonomy.Merge(ctx, reference.Skill, []int64{b}, a); err != nil {
		t.Fatal(err)
	}
	if got := skillIDs(); !slices.Equal(got, []int64{a, c}) {
		t.Errorf("after merging b into a: %v, want [a c] (first position kept, no duplicate)", got)
	}
	if err := taxonomy.Delete(ctx, reference.Skill, c); err != nil {
		t.Fatal(err)
	}
	if got := skillIDs(); !slices.Equal(got, []int64{a}) {
		t.Errorf("after deleting c: %v, want [a]", got)
	}
	if err := taxonomy.Merge(ctx, reference.Designation, []int64{d[1].ID}, d[0].ID); err != nil {
		t.Fatal(err)
	}
	var designation int64
	pool.QueryRow(ctx, `SELECT designation_id FROM jobs WHERE id = $1`, job.ID).Scan(&designation)
	if designation != d[0].ID {
		t.Errorf("designation not repointed: %d, want %d", designation, d[0].ID)
	}
	page, _ := taxonomy.List(ctx, reference.Skill, "", 1, 100)
	for _, term := range page.Terms {
		if term.ID == a && term.Uses != 1 {
			t.Errorf("usage count for a: %d, want 1", term.Uses)
		}
	}
}
