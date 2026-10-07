package service

import (
	"testing"
	"time"
)

func TestTrainingQuotaWindow(t *testing.T) {
	// 23:30 UTC is already the next day in Moscow (UTC+3).
	now := time.Date(2026, 10, 7, 23, 30, 0, 0, time.UTC)

	pro := trainingQuotaWindow("paid", now)
	if pro.limit != paidTrainingsPerDay || pro.period != "day" || pro.resetsAt == nil {
		t.Fatalf("pro window = %+v", pro)
	}
	if want := time.Date(2026, 10, 7, 21, 0, 0, 0, time.UTC); !pro.since.Equal(want) {
		t.Fatalf("pro since = %v, want %v", pro.since.UTC(), want)
	}
	if want := time.Date(2026, 10, 8, 21, 0, 0, 0, time.UTC); !pro.resetsAt.Equal(want) {
		t.Fatalf("pro resets at = %v, want %v", pro.resetsAt.UTC(), want)
	}

	basic := trainingQuotaWindow("", now)
	if basic.limit != freeTrainingsTotal || basic.period != "total" || !basic.since.IsZero() || basic.resetsAt != nil {
		t.Fatalf("basic window = %+v", basic)
	}
}
