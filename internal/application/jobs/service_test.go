package jobs

import (
	"context"
	"testing"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/ports"
)

// fakeSearch accepts one unlock, for the criteria it was given.
type fakeSearch struct {
	ports.JobRepository
	unlockID, hash string
}

func (f *fakeSearch) Search(context.Context, *jobs.SearchCriteria, string) (*jobs.SearchResult, error) {
	return &jobs.SearchResult{}, nil
}
func (f *fakeSearch) UnlockValid(_ context.Context, _, id, hash string) (bool, error) {
	return id == f.unlockID && hash == f.hash, nil
}

func TestMatchSortNeedsUnlock(t *testing.T) {
	ctx := context.Background()
	unlocked := jobs.SearchCriteria{Query: "go"}
	unlocked.Normalize()
	repo := &fakeSearch{unlockID: "11111111-1111-1111-1111-111111111111", hash: unlocked.UnlockHash()}
	s := NewJobService(repo, fakeSettings{})

	match := func(query, unlockID string) *jobs.SearchCriteria {
		return &jobs.SearchCriteria{Query: query, Sort: jobs.SortMatch, UnlockID: unlockID, Page: 3}
	}
	if _, err := s.Search(ctx, match("go", repo.unlockID), ""); err != errSignInToMatch {
		t.Fatalf("anonymous: %v", err)
	}
	for _, c := range []*jobs.SearchCriteria{match("go", ""), match("golang", repo.unlockID)} {
		if _, err := s.Search(ctx, c, "u1"); err == nil || err.(*jobs.PaymentRequiredError).Reason != jobs.ReasonUnlockRequired {
			t.Fatalf("%+v: %v", c, err)
		}
	}
	if _, err := s.Search(ctx, match("go", repo.unlockID), "u1"); err != nil {
		t.Fatalf("unlocked, another page: %v", err)
	}
}
