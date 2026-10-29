package profiles

import (
	"math"
	"regexp"
	"time"
)

// Analytics events (requirements doc "Profile Analytics"). Résumé events are
// recorded once résumés are streamed through the backend.
const (
	EventView           = "view"
	EventResumeView     = "resume_view"
	EventResumeDownload = "resume_download"

	SourceSearch = "search" // reached from the platform's search or discovery pages
	SourceDirect = "direct" // the public URL, or anything unlabelled
)

// AnalyticsDays is the period shown; the previous period of the same length
// is the growth baseline.
const AnalyticsDays = 7

// ViewSource maps the ?src= the frontend sends to a source; anything but
// "search" counts as direct.
func ViewSource(src string) string {
	if src == SourceSearch {
		return SourceSearch
	}
	return SourceDirect
}

// botAgent matches self-declared crawlers and link previewers ("standard
// technical safeguards against obvious automated traffic"). Generic HTTP
// clients aren't listed: until the frontend forwards the browser's
// User-Agent, every view arrives with the SvelteKit server's own agent, and
// treating that as a bot would count nothing.
var botAgent = regexp.MustCompile(`(?i)bot|crawl|spider|slurp|facebookexternalhit|embedly|preview|headless|lighthouse`)

// IsBot reports whether a User-Agent is an obvious bot.
func IsBot(userAgent string) bool { return botAgent.MatchString(userAgent) }

// EventCount is a batch of identical events: one profile, kind, source and
// IST day (how Redis buffers them before they reach Postgres).
type EventCount struct {
	UserID string
	Kind   string
	Source string
	Day    time.Time // an ISTDay
	Count  int
}

// DayStats is one rolled-up IST day.
type DayStats struct {
	Day             time.Time // midnight UTC of the IST date
	ViewsSearch     int
	ViewsDirect     int
	ResumeViews     int
	ResumeDownloads int
}

type Metrics struct {
	ProfileViews    int `json:"profile_views"`
	ResumeViews     int `json:"resume_views"`
	ResumeDownloads int `json:"resume_downloads"`
}

// Growth is the % change against the previous period, one decimal. nil means
// the previous period had no activity: show "New" or "No previous activity",
// never an infinite percentage.
type Growth struct {
	ProfileViews    *float64 `json:"profile_views"`
	ResumeViews     *float64 `json:"resume_views"`
	ResumeDownloads *float64 `json:"resume_downloads"`
}

type ViewSources struct {
	Search int `json:"search"` // "Recruiter Searches"
	Direct int `json:"direct"` // "Direct Impressions"
}

type AnalyticsDay struct {
	Date string `json:"date"` // IST, yyyy-mm-dd
	Metrics
}

// Analytics is the public Activity & Analytics tab: aggregates only, never
// who viewed.
type Analytics struct {
	From        string         `json:"from"` // first and last day of the period, IST
	To          string         `json:"to"`
	Totals      Metrics        `json:"totals"`
	Growth      Growth         `json:"growth"`
	ViewSources ViewSources    `json:"view_sources"`
	Daily       []AnalyticsDay `json:"daily"`    // one entry per day, oldest first
	PeakDay     *string        `json:"peak_day"` // nil when the period had no activity
}

var ist = time.FixedZone("IST", 5*60*60+30*60)

// ISTDay is the IST calendar date of t, as midnight UTC (how DATE columns scan).
func ISTDay(t time.Time) time.Time {
	y, m, d := t.In(ist).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// BuildAnalytics turns rolled-up days into the tab's numbers. The period is
// the AnalyticsDays complete days before today (stats update once a day);
// today is an ISTDay.
func BuildAnalytics(days []DayStats, today time.Time) Analytics {
	byDay := make(map[time.Time]DayStats, len(days))
	for _, d := range days {
		byDay[d.Day] = d
	}
	first := today.AddDate(0, 0, -AnalyticsDays)
	a := Analytics{From: first.Format(time.DateOnly), To: today.AddDate(0, 0, -1).Format(time.DateOnly),
		Daily: make([]AnalyticsDay, 0, AnalyticsDays)}

	var prev Metrics
	for i := 1; i <= AnalyticsDays; i++ {
		d := byDay[first.AddDate(0, 0, -i)]
		prev.ProfileViews += d.ViewsSearch + d.ViewsDirect
		prev.ResumeViews += d.ResumeViews
		prev.ResumeDownloads += d.ResumeDownloads
	}

	peak := 0
	for i := 0; i < AnalyticsDays; i++ {
		date := first.AddDate(0, 0, i)
		d := byDay[date]
		m := Metrics{ProfileViews: d.ViewsSearch + d.ViewsDirect, ResumeViews: d.ResumeViews, ResumeDownloads: d.ResumeDownloads}
		a.Daily = append(a.Daily, AnalyticsDay{Date: date.Format(time.DateOnly), Metrics: m})
		a.Totals.ProfileViews += m.ProfileViews
		a.Totals.ResumeViews += m.ResumeViews
		a.Totals.ResumeDownloads += m.ResumeDownloads
		a.ViewSources.Search += d.ViewsSearch
		a.ViewSources.Direct += d.ViewsDirect
		// Peak: the most total activity; ties go to the earliest day.
		if total := m.ProfileViews + m.ResumeViews + m.ResumeDownloads; total > peak {
			peak = total
			a.PeakDay = &a.Daily[i].Date
		}
	}
	a.Growth = Growth{
		ProfileViews:    growth(a.Totals.ProfileViews, prev.ProfileViews),
		ResumeViews:     growth(a.Totals.ResumeViews, prev.ResumeViews),
		ResumeDownloads: growth(a.Totals.ResumeDownloads, prev.ResumeDownloads),
	}
	return a
}

func growth(current, previous int) *float64 {
	if previous == 0 {
		return nil
	}
	pct := math.Round(float64(current-previous)/float64(previous)*1000) / 10
	return &pct
}
