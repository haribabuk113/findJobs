package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/applications"
	"findJobs/internal/domain/reference"
)

func TestSavedJobs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewApplicationRepository(pool)
	jobsRepo := &RecruitingRepository{pool: pool}
	owner, candidate := testUser(t, pool, "Owner"), testUser(t, pool, "Candidate")
	role, _, err := NewTaxonomyRepository(pool).Resolve(ctx, reference.Designation, names(t, "Test role"), "")
	if err != nil {
		t.Fatal(err)
	}
	applied := testJob(t, jobsRepo, owner, role[0].ID, nil, false)
	kept := testJob(t, jobsRepo, owner, role[0].ID, nil, false)

	for _, id := range []string{applied.ID, kept.ID, kept.ID} { // saving twice is a no-op
		if err := repo.SaveJob(ctx, candidate, id); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	if err := repo.SaveJob(ctx, candidate, "no-such-job-"+uuid.NewString()[:6]); err != applications.ErrSavedJobUnknown {
		t.Fatalf("unknown job: %v", err)
	}
	// An external confirmation counts as applied (no profile needed).
	if _, err := pool.Exec(ctx, `UPDATE jobs SET application = 'External' WHERE id = $1`, applied.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyExternal(ctx, candidate, applied.ID); err != nil {
		t.Fatal(err)
	}
	list, err := repo.SavedJobs(ctx, candidate)
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}
	flags := map[string]bool{list[0].JobID: list[0].Applied, list[1].JobID: list[1].Applied}
	if !flags[applied.ID] || flags[kept.ID] || list[0].SavedAt.Before(list[1].SavedAt) || time.Since(list[0].SavedAt) > time.Minute {
		t.Fatalf("applied flags / newest first: %+v", list)
	}

	if err := repo.ClearApplied(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if list, _ = repo.SavedJobs(ctx, candidate); len(list) != 1 || list[0].JobID != kept.ID {
		t.Fatalf("after clear applied: %+v", list)
	}
	if err := repo.UnsaveJobs(ctx, candidate, []string{kept.ID, "missing"}); err != nil {
		t.Fatal(err)
	}
	if list, _ = repo.SavedJobs(ctx, candidate); len(list) != 0 {
		t.Fatalf("after unsave: %+v", list)
	}
}
