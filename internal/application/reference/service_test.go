package reference

import (
	"context"
	"errors"
	"testing"
	"time"

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

type fakeRates struct {
	stored  reference.Rates
	updated time.Time
}

func (f *fakeRates) Replace(_ context.Context, r reference.Rates) error { f.stored = r; return nil }
func (f *fakeRates) All(context.Context) (reference.Rates, error)       { return f.stored, nil }
func (f *fakeRates) UpdatedAt(context.Context) (time.Time, error)       { return f.updated, nil }

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

// The FX job refreshes once per scheduled time: only when the stored rates
// predate the latest hh:mm IST, including after downtime or with no rates.
func TestRefreshRatesIfDue(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, ist)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, tc := range []struct {
		name, updated, now string
		want               bool
	}{
		{"no rates yet", "", "2026-11-03 01:00", true},
		{"before today's time, refreshed yesterday after it", "2026-11-02 05:00", "2026-11-03 04:59", false},
		{"today's time passed", "2026-11-02 05:00", "2026-11-03 05:00", true},
		{"already refreshed today", "2026-11-03 05:01", "2026-11-03 18:00", false},
		{"missed days during downtime", "2026-10-30 05:00", "2026-11-03 02:00", true},
	} {
		rates := &fakeRates{}
		if tc.updated != "" {
			rates.updated = at(tc.updated)
		}
		fx := &fakeFX{}
		if err := newTestService(fx, rates, &fakeAssets{}).RefreshRatesIfDue(context.Background(), "05:00", at(tc.now)); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := fx.calls > 0; got != tc.want {
			t.Errorf("%s: refreshed=%v, want %v", tc.name, got, tc.want)
		}
	}

	s := newTestService(&fakeFX{}, &fakeRates{}, &fakeAssets{})
	if err := s.RefreshRatesIfDue(context.Background(), "5am", time.Now()); err == nil {
		t.Error("bad time: want error")
	}
	// An admin-started refresh in progress: skip, don't run a second one.
	fx := &fakeFX{}
	s = newTestService(fx, &fakeRates{}, &fakeAssets{})
	s.refreshing.Store(true)
	if err := s.RefreshRatesIfDue(context.Background(), "05:00", time.Now()); err != nil || fx.calls != 0 {
		t.Errorf("while refreshing: calls=%d err=%v", fx.calls, err)
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
