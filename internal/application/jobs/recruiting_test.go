package jobs

import (
	"context"
	"testing"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/payments"
	"findJobs/internal/domain/reference"
	"findJobs/internal/domain/settings"
	"findJobs/internal/ports"
)

// fakeRepo stores posts in memory; methods the test doesn't reach panic via
// the nil embedded interface.
type fakeRepo struct {
	ports.RecruitingRepository
	created []*jobs.Record
	charges []*jobs.Charge
}

func (f *fakeRepo) MemberCompanyID(context.Context, string, string) (string, error) { return "co-1", nil }
func (f *fakeRepo) MissingLocations(context.Context, []string) ([]string, error)    { return nil, nil }
func (f *fakeRepo) CurrencyKnown(context.Context, string) (bool, error)             { return true, nil }
func (f *fakeRepo) Create(_ context.Context, rec *jobs.Record, c *jobs.Charge) error {
	f.created, f.charges = append(f.created, rec), append(f.charges, c)
	return nil
}
func (f *fakeRepo) PostByKey(_ context.Context, _, key string) (string, error) {
	for _, r := range f.created {
		if r.IdempotencyKey == key {
			return r.ID, nil
		}
	}
	return "", nil
}
func (f *fakeRepo) Detail(_ context.Context, id string, _ bool, _ string) (*jobs.Detail, error) {
	return &jobs.Detail{Job: jobs.Job{ID: id}}, nil
}

type fakeTerms struct{}

func (fakeTerms) ResolveTerms(_ context.Context, _ reference.Kind, names []string, _ string) ([]reference.Term, error) {
	out := make([]reference.Term, len(names))
	for i := range names {
		out[i] = reference.Term{ID: int64(i + 1)}
	}
	return out, nil
}

type fakeSettings struct{ fee int64 }

func (f fakeSettings) Get(context.Context) (*settings.Settings, error) {
	return &settings.Settings{JobPostFeePaise: f.fee}, nil
}
func (fakeSettings) Update(context.Context, *settings.Settings, string) (*settings.Settings, error) {
	return nil, nil
}

type fakePayer struct{ configured bool }

func (p fakePayer) Configured() bool { return p.configured }
func (fakePayer) JobPostCheckout(_ context.Context, _, _ string, amount int64) (*payments.Checkout, error) {
	return &payments.Checkout{OrderID: "order_1", AmountPaise: amount}, nil
}

func paidInput(method string) *jobs.PostInput {
	return &jobs.PostInput{Tier: jobs.TierPaid, PaymentMethod: method, CompanySlug: "acme-labs", Designation: "Engineer",
		Skills: []string{"Go"}, LocationIDs: []string{"1277333"}, SalaryMax: 100000, Currency: "INR",
		WorkModes: []string{"Remote"}, WorkTypes: []string{"Full-time"}, Relocation: "On their own",
		Application: "Easy apply", Description: "Test."}
}

func TestPaidPost(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	s := NewRecruitingService(repo, fakeTerms{}, fakeSettings{fee: 500}, fakePayer{configured: true})

	if _, err := s.Post(ctx, "u1", "", paidInput(jobs.PayCompanyCredits)); !jobs.IsValidation(err) {
		t.Fatalf("a charged post without an Idempotency-Key: %v", err)
	}
	first, err := s.Post(ctx, "u1", "key-00000001", paidInput(jobs.PayCompanyCredits))
	if err != nil || len(repo.charges) != 1 || *repo.charges[0] != (jobs.Charge{Method: jobs.PayCompanyCredits, FeePaise: 500}) {
		t.Fatalf("credits post: %v, charges %+v", err, repo.charges)
	}
	again, err := s.Post(ctx, "u1", "key-00000001", paidInput(jobs.PayCompanyCredits))
	if err != nil || again.ID != first.ID || len(repo.created) != 1 {
		t.Fatalf("a retry must return the first job without posting again: %v, %d posts", err, len(repo.created))
	}
	d, err := s.Post(ctx, "u1", "key-00000002", paidInput(jobs.PayOutOfPocket))
	if err != nil || d.Checkout == nil || d.Checkout.AmountPaise != 500 {
		t.Fatalf("out of pocket returns a checkout: %+v %v", d, err)
	}

	// No gateway: refused before anything is stored. A fee of 0 posts free.
	noGateway := NewRecruitingService(&fakeRepo{}, fakeTerms{}, fakeSettings{fee: 500}, fakePayer{})
	if _, err := noGateway.Post(ctx, "u1", "key-00000003", paidInput(jobs.PayOutOfPocket)); err != payments.ErrNotConfigured {
		t.Fatalf("out of pocket without a gateway: %v", err)
	}
	free := &fakeRepo{}
	if _, err := NewRecruitingService(free, fakeTerms{}, fakeSettings{}, fakePayer{}).Post(ctx, "u1", "", paidInput(jobs.PayOutOfPocket)); err != nil || free.charges[0] != nil {
		t.Fatalf("fee 0: %v, charge %+v", err, free.charges)
	}
}
