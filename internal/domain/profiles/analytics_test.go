package profiles

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return t
}

func TestBuildAnalytics(t *testing.T) {
	today := day("2026-09-15")
	a := BuildAnalytics([]DayStats{
		// Current period: 8–14 Sep.
		{Day: day("2026-09-08"), ViewsSearch: 2, ViewsDirect: 1},
		{Day: day("2026-09-10"), ViewsSearch: 5, ResumeViews: 3, ResumeDownloads: 1}, // 9: the peak
		{Day: day("2026-09-14"), ViewsDirect: 4, ResumeViews: 2},
		// Previous period: 1–7 Sep.
		{Day: day("2026-09-01"), ViewsDirect: 8, ResumeViews: 4},
		// Outside both periods, and today (not complete yet): ignored.
		{Day: day("2026-08-31"), ViewsDirect: 100},
		{Day: day("2026-09-15"), ViewsDirect: 100},
	}, today)

	if a.From != "2026-09-08" || a.To != "2026-09-14" || len(a.Daily) != 7 || a.Daily[0].Date != "2026-09-08" {
		t.Fatalf("period: %s..%s, %d days", a.From, a.To, len(a.Daily))
	}
	if a.Totals != (Metrics{ProfileViews: 12, ResumeViews: 5, ResumeDownloads: 1}) {
		t.Fatalf("totals: %+v", a.Totals)
	}
	if a.ViewSources != (ViewSources{Search: 7, Direct: 5}) {
		t.Fatalf("sources: %+v", a.ViewSources)
	}
	// 12 vs 8 views: +50%; 5 vs 4 résumé views: +25%; downloads had no baseline.
	if a.Growth.ProfileViews == nil || *a.Growth.ProfileViews != 50 || *a.Growth.ResumeViews != 25 || a.Growth.ResumeDownloads != nil {
		t.Fatalf("growth: %+v", a.Growth)
	}
	if a.PeakDay == nil || *a.PeakDay != "2026-09-10" {
		t.Fatalf("peak: %v", a.PeakDay)
	}

	empty := BuildAnalytics(nil, today)
	if empty.PeakDay != nil || empty.Growth.ProfileViews != nil || empty.Totals != (Metrics{}) || len(empty.Daily) != 7 {
		t.Fatalf("no activity: %+v", empty)
	}
	// Ties go to the earliest day; growth rounds to one decimal.
	tie := BuildAnalytics([]DayStats{
		{Day: day("2026-09-09"), ViewsDirect: 3}, {Day: day("2026-09-12"), ViewsDirect: 3},
		{Day: day("2026-09-02"), ViewsDirect: 7},
	}, today)
	if *tie.PeakDay != "2026-09-09" || *tie.Growth.ProfileViews != -14.3 {
		t.Fatalf("tie %v, growth %v", *tie.PeakDay, *tie.Growth.ProfileViews)
	}
}

func TestViewSourceAndBots(t *testing.T) {
	if ViewSource("search") != SourceSearch || ViewSource("") != SourceDirect || ViewSource("anything") != SourceDirect {
		t.Fatal("view source mapping")
	}
	for _, ua := range []string{"Googlebot/2.1", "facebookexternalhit/1.1", "Mozilla/5.0 (compatible; bingbot/2.0)", "HeadlessChrome/120"} {
		if !IsBot(ua) {
			t.Errorf("%q should be a bot", ua)
		}
	}
	// Browsers, and the SvelteKit server's own agent, still count.
	for _, ua := range []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130.0", "node", "undici", ""} {
		if IsBot(ua) {
			t.Errorf("%q counted as a bot", ua)
		}
	}
	if d := ISTDay(time.Date(2026, 9, 14, 19, 0, 0, 0, time.UTC)); d != day("2026-09-15") {
		t.Fatalf("IST day: %v", d)
	}
}
