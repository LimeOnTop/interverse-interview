package service

import (
	"testing"
	"time"
)

func TestTrainingQuotaWindow(t *testing.T) {
	cases := []struct {
		name     string
		now      time.Time
		since    time.Time
		resetsAt time.Time
	}{
		// 08:30 UTC is 11:30 MSK: still yesterday's window, renews at 12:00 MSK today.
		{"before noon", time.Date(2026, 10, 8, 8, 30, 0, 0, time.UTC),
			time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)},
		// 09:00 UTC is exactly 12:00 MSK: a fresh window starts.
		{"at noon", time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC),
			time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)},
		// 23:30 UTC is 02:30 MSK next day: window started 12:00 MSK on the 8th.
		{"after midnight", time.Date(2026, 10, 8, 23, 30, 0, 0, time.UTC),
			time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC), time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		pro := trainingQuotaWindow("paid", c.now)
		if pro.limit != 15 || pro.period != "day" || pro.resetsAt == nil {
			t.Fatalf("%s: pro window = %+v", c.name, pro)
		}
		if !pro.since.Equal(c.since) || !pro.resetsAt.Equal(c.resetsAt) {
			t.Fatalf("%s: since %v resets %v, want %v / %v", c.name, pro.since.UTC(), pro.resetsAt.UTC(), c.since, c.resetsAt)
		}
	}

	basic := trainingQuotaWindow("", time.Now())
	if basic.limit != freeTrainingsTotal || basic.period != "total" || !basic.since.IsZero() || basic.resetsAt != nil {
		t.Fatalf("basic window = %+v", basic)
	}
}
