package scheduler

import (
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, ist)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, tc := range []struct{ now, want string }{
		{"2026-10-29 00:10", "2026-10-29 00:20"}, // later today
		{"2026-10-29 00:20", "2026-10-30 00:20"}, // exactly now: tomorrow, never twice
		{"2026-10-29 18:00", "2026-10-30 00:20"},
		{"2026-12-31 23:59", "2027-01-01 00:20"}, // year end
	} {
		if got := nextRun(at(tc.now), 0, 20); !got.Equal(at(tc.want)) {
			t.Errorf("after %s: got %s, want %s", tc.now, got.In(ist), tc.want)
		}
	}
	// The host's zone doesn't matter: 18:50 UTC is 00:20 IST the next day.
	utc := time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC)
	if got := nextRun(utc, 0, 20); !got.Equal(time.Date(2026, 10, 28, 18, 50, 0, 0, time.UTC)) {
		t.Errorf("from UTC: %s", got.UTC())
	}
}
