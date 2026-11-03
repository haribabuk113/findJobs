package postgres

import (
	"context"
	"slices"
	"testing"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/reference"
)

// Title search: every word as a substring, or every word's English stem
// (newer doc §4: "Product Manager" finds "Director of Product Management").
func TestTitleSearchMatchesStems(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo, recruiting := NewJobRepository(pool), &RecruitingRepository{pool: pool}
	owner := testUser(t, pool, "Title Owner")
	role, _, err := NewTaxonomyRepository(pool).Resolve(ctx, reference.Designation, names(t, "Test role"), owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM designations WHERE id = $1`, role[0].ID) })
	titles := []string{"Director of Product Management", "Product Management Lead", "Senior Product Manager",
		"Project Manager", "Product Designer"}
	ids := make([]string, len(titles))
	byID := map[string]string{}
	for i, title := range titles {
		job := testJob(t, recruiting, owner, role[0].ID, nil, true)
		if _, err := pool.Exec(ctx, `UPDATE jobs SET title = $2 WHERE id = $1`, job.ID, title); err != nil {
			t.Fatal(err)
		}
		ids[i], byID[job.ID] = job.ID, title
	}
	for query, want := range map[string][]string{
		"Product Manager": {"Director of Product Management", "Product Management Lead", "Senior Product Manager"},
		"prod":            {"Director of Product Management", "Product Management Lead", "Senior Product Manager", "Product Designer"},
		"manager":         {"Director of Product Management", "Product Management Lead", "Senior Product Manager", "Project Manager"},
		"Designer":        {"Product Designer"},
		"of":              {"Director of Product Management"}, // a stopword: only the substring side
		"Product Nurse":   nil,
	} {
		c := jobs.SearchCriteria{Query: query, JobIDs: ids}
		if err := c.Normalize(); err != nil {
			t.Fatal(err)
		}
		res, err := repo.Search(ctx, &c, "")
		if err != nil {
			t.Fatalf("%q: %v", query, err)
		}
		var got []string
		for _, j := range res.Jobs {
			got = append(got, byID[j.ID])
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %v, want %v", query, got, want)
		}
	}
}
