package reference

import (
	"context"
	"errors"
	"testing"

	"findJobs/internal/domain/reference"
	"findJobs/internal/pkg/background"
)

type fakeFX struct {
	failures int
	calls    int
}

func (f *fakeFX) Latest(context.Context) (reference.Rates, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("provider down")
	}
	return reference.Rates{"INR": 1, "USD": 95}, nil
}

type fakeRates struct{ stored reference.Rates }

func (f *fakeRates) Replace(_ context.Context, r reference.Rates) error { f.stored = r; return nil }
func (f *fakeRates) All(context.Context) (reference.Rates, error)       { return f.stored, nil }

type fakeAssets struct{ saved []string }

func (f *fakeAssets) Save(_ context.Context, a *reference.Asset) (bool, error) {
	f.saved = append(f.saved, a.Kind)
	return true, nil
}
func (f *fakeAssets) Get(context.Context, string) (*reference.Asset, error) { return nil, nil }
func (f *fakeAssets) Manifest(context.Context) (map[string]reference.ManifestEntry, error) {
	return nil, nil
}

func newTestService(fx *fakeFX, rates *fakeRates, assets *fakeAssets) *service {
	s := NewReferenceService(background.New(), nil, nil, rates, assets, fx).(*service)
	s.retryWait = 0
	return s
}

func TestRefreshRatesRetriesThenPublishes(t *testing.T) {
	fx, rates, assets := &fakeFX{failures: 2}, &fakeRates{}, &fakeAssets{}
	if err := newTestService(fx, rates, assets).RefreshRates(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fx.calls != 3 || rates.stored["USD"] != 95 || len(assets.saved) != 1 || assets.saved[0] != reference.AssetRates {
		t.Fatalf("calls=%d stored=%v published=%v", fx.calls, rates.stored, assets.saved)
	}
}

// Manual "Salary": after 3 failures nothing changes; clients keep the last rates.json.
func TestRefreshRatesGivesUpWithoutChanges(t *testing.T) {
	fx, rates, assets := &fakeFX{failures: 3}, &fakeRates{}, &fakeAssets{}
	if err := newTestService(fx, rates, assets).RefreshRates(context.Background()); err == nil {
		t.Fatal("want error after 3 failures")
	}
	if fx.calls != 3 || rates.stored != nil || len(assets.saved) != 0 {
		t.Fatalf("calls=%d stored=%v published=%v", fx.calls, rates.stored, assets.saved)
	}
}

func TestValidationBeforeStorage(t *testing.T) {
	s := newTestService(&fakeFX{}, &fakeRates{}, &fakeAssets{}) // nil taxonomy repo: must not be reached
	ctx := context.Background()
	tooMany := make([]string, maxResolveNames+1)
	for name, err := range map[string]error{
		"too many names":    second(s.ResolveTerms(ctx, reference.Skill, tooMany, "")),
		"markup name":       second(s.ResolveTerms(ctx, reference.Skill, []string{"<b>x</b>"}, "")),
		"merge no sources":  s.MergeTerms(ctx, reference.Skill, nil, 1),
		"merge into itself": s.MergeTerms(ctx, reference.Skill, []int64{1, 2}, 2),
		"merge bad id":      s.MergeTerms(ctx, reference.Skill, []int64{-1}, 2),
		"update nothing":    second(s.UpdateTerm(ctx, reference.Skill, 1, nil, nil)),
		"update to merged":  second(s.UpdateTerm(ctx, reference.Skill, 1, nil, ptr(reference.StatusMerged))),
		"list bad status":   second(s.ListTerms(ctx, reference.Skill, "deleted", 1, 10)),
		"list page too big": second(s.ListTerms(ctx, reference.Skill, "", 1, 1000)),
	} {
		if !reference.IsValidation(err) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}
	if _, err := s.Asset(ctx, "../../etc/passwd"); !reference.IsNotFound(err) {
		t.Errorf("asset path traversal: want NotFoundError, got %v", err)
	}
	if terms, err := s.ResolveTerms(ctx, reference.Skill, nil, ""); err != nil || len(terms) != 0 {
		t.Errorf("empty resolve: %v, %v", terms, err)
	}
}

func second[T any](_ T, err error) error { return err }
func ptr[T any](v T) *T                  { return &v }
