package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appprofiles "findJobs/internal/application/profiles"
	"findJobs/internal/domain/profiles"
)

func TestProfileAnalytics(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewProfileAnalyticsRepository(pool)
	if tag, err := pool.Exec(ctx, `INSERT INTO locations (id, level, name, country_code) VALUES ('1277333', 'city', 'Bengaluru', 'IN')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	} else if tag.RowsAffected() == 1 {
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM locations WHERE id = '1277333'`) })
	}
	owner, visitor := testUser(t, pool, "Analytics owner"), testUser(t, pool, "Analytics visitor")
	slug := "an-" + uuid.NewString()[:8]
	if err := NewProfileRepository(pool).Save(ctx, owner, testProfile(slug), nil, nil); err != nil {
		t.Fatal(err)
	}

	svc := appprofiles.NewAnalyticsService(repo)
	// Views: anonymous and another user count; the owner's own view and bots don't.
	for _, viewer := range []string{"", visitor, owner} {
		svc.RecordView(ctx, owner, viewer, "", "Mozilla/5.0")
	}
	svc.RecordView(ctx, owner, "", "", "Googlebot/2.1")
	var stored int
	pool.QueryRow(ctx, `SELECT COALESCE(SUM(count), 0) FROM profile_events WHERE user_id = $1`, owner).Scan(&stored)
	if stored != 2 {
		t.Fatalf("stored events: %d, want 2 (owner's view and the bot skipped)", stored)
	}

	// A buffered batch (as Redis flushes it) is applied once, however often
	// it's retried; counts for a deleted profile are dropped, not an error.
	gone := uuid.NewString()
	batch := []profiles.EventCount{
		{UserID: owner, Kind: profiles.EventView, Source: profiles.SourceDirect, Day: profiles.ISTDay(time.Now()), Count: 5},
		{UserID: gone, Kind: profiles.EventView, Source: profiles.SourceDirect, Day: profiles.ISTDay(time.Now()), Count: 9},
	}
	batchID := "test-batch-" + uuid.NewString()
	for range 2 {
		if err := repo.AddEventCounts(ctx, batchID, batch); err != nil {
			t.Fatal(err)
		}
	}
	pool.QueryRow(ctx, `SELECT COALESCE(SUM(count), 0) FROM profile_events WHERE user_id = $1`, owner).Scan(&stored)
	if stored != 7 {
		t.Fatalf("after the batch: %d, want 7 (2 + 5, applied once)", stored)
	}

	// Backdated events: 23:00 IST two days ago belongs to that IST day, even
	// though it's still the previous day in UTC... and 8 days ago is the
	// previous period.
	today := profiles.ISTDay(time.Now())
	ist := time.FixedZone("IST", 5*60*60+30*60)
	at := func(daysAgo, hour int) time.Time {
		d := today.AddDate(0, 0, -daysAgo)
		return time.Date(d.Year(), d.Month(), d.Day(), hour, 0, 0, 0, ist)
	}
	for _, e := range []struct {
		kind, source string
		at           time.Time
	}{
		{profiles.EventView, profiles.SourceSearch, at(2, 23)},
		{profiles.EventView, profiles.SourceSearch, at(2, 1)},
		{profiles.EventResumeView, profiles.SourceDirect, at(2, 12)},
		{profiles.EventView, profiles.SourceDirect, at(8, 12)},
		{profiles.EventView, profiles.SourceDirect, at(20, 12)}, // past retention: pruned
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO profile_events (user_id, kind, source, at) VALUES ($1, $2, $3, $4)`,
			owner, e.kind, e.source, e.at); err != nil {
			t.Fatal(err)
		}
	}

	for range 2 { // idempotent: a second run gives the same numbers
		if err := svc.Rollup(ctx); err != nil {
			t.Fatal(err)
		}
	}
	a, err := svc.Analytics(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	twoDaysAgo := today.AddDate(0, 0, -2).Format(time.DateOnly)
	// Today's two views aren't in yet: the tab shows complete days only.
	if a.Totals != (profiles.Metrics{ProfileViews: 2, ResumeViews: 1}) || a.ViewSources.Search != 2 ||
		a.PeakDay == nil || *a.PeakDay != twoDaysAgo {
		t.Fatalf("analytics: %+v peak %v", a, a.PeakDay)
	}
	if a.Growth.ProfileViews == nil || *a.Growth.ProfileViews != 100 { // 2 vs 1
		t.Fatalf("growth: %v", a.Growth.ProfileViews)
	}
	var old int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM profile_events WHERE user_id = $1 AND at < NOW() - INTERVAL '15 days'`, owner).Scan(&old)
	if old != 0 {
		t.Fatalf("old events not pruned: %d", old)
	}

	// A private profile's analytics are a 404. (Its views aren't recorded
	// because GET /profiles/{slug} itself 404s first.)
	if _, err := pool.Exec(ctx, `UPDATE profiles SET is_public = FALSE WHERE user_id = $1`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Analytics(ctx, slug); err != profiles.ErrNotFound {
		t.Fatalf("private analytics: %v", err)
	}
}
