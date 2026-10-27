package applications

import (
	"slices"
	"testing"
	"time"
)

func TestMatchScore(t *testing.T) {
	pct, matched, unmatched := MatchScore([]int64{1, 2, 3}, []int64{3, 1, 9})
	if pct != 67 || !slices.Equal(matched, []int64{1, 3}) || !slices.Equal(unmatched, []int64{2}) {
		t.Fatalf("2 of 3: %d %v %v", pct, matched, unmatched)
	}
	if pct, _, _ := MatchScore([]int64{1, 2}, []int64{1}); pct != 50 {
		t.Fatalf("half: %d", pct)
	}
	if pct, _, _ := MatchScore([]int64{1, 2, 3, 4, 5, 6, 7, 8}, []int64{1}); pct != 13 { // 12.5 rounds up
		t.Fatalf("rounding: %d", pct)
	}
	if pct, m, u := MatchScore(nil, []int64{1}); pct != 0 || m == nil || u == nil {
		t.Fatalf("no required skills: %d %v %v", pct, m, u)
	}
}

func TestQuotaDayIsIST(t *testing.T) {
	// 19:00 UTC is 00:30 IST the next day.
	if d := QuotaDay(time.Date(2026, 10, 27, 19, 0, 0, 0, time.UTC)); d.Day() != 28 {
		t.Fatalf("got %v", d)
	}
	if d := QuotaDay(time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC)); d.Day() != 27 {
		t.Fatalf("got %v", d)
	}
}

func TestFilterNormalize(t *testing.T) {
	f := ApplicantFilter{}
	if err := f.Normalize(); err != nil || f.Sort != "recent" {
		t.Fatalf("defaults: %+v %v", f, err)
	}
	for _, bad := range []ApplicantFilter{{Stage: "hired"}, {MinMatch: 101}, {Sort: "name"}} {
		if err := bad.Normalize(); !IsValidation(err) {
			t.Errorf("%+v: want validation error, got %v", bad, err)
		}
	}
	if q := NewQuota(15, 20); q.Remaining != 0 {
		t.Fatalf("remaining never negative: %+v", q)
	}
}
