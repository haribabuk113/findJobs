package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/applications"
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

	if _, err := repo.Apply(ctx, stranger, easy.ID, day, 5); err != applications.ErrNoProfile {
		t.Fatalf("no profile: %v", err)
	}
	if _, err := repo.Apply(ctx, owner, easy.ID, day, 5); err != applications.ErrOwnJob {
		t.Fatalf("own job: %v", err)
	}
	if _, err := repo.Apply(ctx, candidate, external.ID, day, 5); err != applications.ErrNotEasyApply {
		t.Fatalf("external via easy apply: %v", err)
	}
	a, err := repo.Apply(ctx, candidate, easy.ID, day, 1)
	if err != nil || *a.MatchPct != 50 || a.Stage != applications.StageNew || a.JobStatus != "active" {
		t.Fatalf("apply: %+v %v", a, err)
	}
	if _, err := repo.Apply(ctx, candidate, easy.ID, day, 5); err != applications.ErrAlreadyApplied {
		t.Fatalf("duplicate: %v", err)
	}
	// Limit 1 is used up: the second job is refused and nothing is stored.
	if _, err := repo.Apply(ctx, candidate, other.ID, day, 1); err != applications.ErrQuotaExhausted {
		t.Fatalf("quota: %v", err)
	}
	if used, _ := repo.QuotaUsed(ctx, candidate, day); used != 1 {
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
}
