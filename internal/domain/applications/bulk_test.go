package applications

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeBulkJobIDs(t *testing.T) {
	ids, err := NormalizeBulkJobIDs([]string{"a", "b", "a"})
	if err != nil || !slices.Equal(ids, []string{"a", "b"}) {
		t.Fatalf("dedupe: %v %v", ids, err)
	}
	tooMany := make([]string, MaxBulkJobs+1)
	for i := range tooMany {
		tooMany[i] = strings.Repeat("x", i+1)
	}
	for _, bad := range [][]string{nil, {""}, {strings.Repeat("x", 151)}, tooMany} {
		if _, err := NormalizeBulkJobIDs(bad); !IsValidation(err) {
			t.Errorf("%d ids: want validation error, got %v", len(bad), err)
		}
	}
}

func TestPlanBulk(t *testing.T) {
	jobs := []BulkJob{
		{ID: "e1", Open: true},
		{ID: "x1", Open: true, External: true},
		{ID: "e2", Open: true},
		{ID: "e3", Open: true},
	}
	p := PlanBulk(jobs, 2, 500)
	if !slices.Equal(p.EasyApply, []string{"e1", "e2", "e3"}) || !slices.Equal(p.External, []string{"x1"}) ||
		p.Free != 2 || p.Paid != 1 || p.FeePaise != 500 || len(p.Problems) != 0 {
		t.Fatalf("split: %+v", p)
	}
	if err := p.Err(); !IsPaymentRequired(err) {
		t.Fatalf("a paid application blocks the batch until the wallet exists: %v", err)
	}
	if p := PlanBulk(jobs, -3, 500); p.Free != 0 || p.Paid != 3 {
		t.Fatalf("over-used quota counts as zero: %+v", p)
	}
	// External jobs never use the quota.
	if p := PlanBulk(jobs[1:2], 0, 500); p.Paid != 0 || p.Err() != nil {
		t.Fatalf("external only: %+v %v", p, p.Err())
	}

	blocked := PlanBulk([]BulkJob{
		{ID: "gone"},
		{ID: "mine", Open: true, Own: true},
		{ID: "done", Open: true, Applied: true, External: true},
		{ID: "ok", Open: true},
	}, 15, 500)
	want := []BulkProblem{{"gone", ReasonNotOpen}, {"mine", ReasonOwnJob}, {"done", ReasonAlreadyApplied}}
	if !slices.Equal(blocked.Problems, want) || !slices.Equal(blocked.EasyApply, []string{"ok"}) {
		t.Fatalf("problems: %+v", blocked)
	}
	err := blocked.Err()
	if !IsConflict(err) || !strings.Contains(err.Error(), "gone (not open)") {
		t.Fatalf("problems block the batch and name the jobs: %v", err)
	}
}
