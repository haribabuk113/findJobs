package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPlatformStats(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewStatsRepository(pool)
	if err := repo.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := repo.Get(ctx)
	if err != nil || before.ComputedAt == nil {
		t.Fatalf("first refresh: %+v %v", before, err)
	}

	// Two new users; one creates a company, which makes them its admin.
	testUser(t, pool, "Stats user")
	admin := testUser(t, pool, "Stats admin")
	slug := "st-" + uuid.NewString()[:8]
	company, err := NewCompanyRepository(pool).Create(ctx, testCompanyInput(t, slug), "IN", admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM companies WHERE id = $1`, company.ID) })

	if err := repo.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.RegisteredUsers != before.RegisteredUsers+2 || after.HRProfessionals != before.HRProfessionals+1 ||
		after.Companies != before.Companies+1 || after.CountriesWithJobsCompanies < 1 || !after.ComputedAt.After(*before.ComputedAt) {
		t.Fatalf("deltas: before %+v after %+v", before, after)
	}

	// A deleted company stops counting, and so does its admin.
	if _, err := pool.Exec(ctx, `UPDATE companies SET deleted_at = NOW() WHERE id = $1`, company.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx); got.Companies != before.Companies || got.HRProfessionals != before.HRProfessionals {
		t.Fatalf("after delete: %+v", got)
	}
}
