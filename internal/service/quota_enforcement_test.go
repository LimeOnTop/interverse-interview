package service

import (
	"context"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"testing"
	"time"
)

type quotaRepository struct {
	usecase.InterviewRepository
	used int64
}

func (r quotaRepository) CountCreatedSince(context.Context, string, time.Time) (int64, error) {
	return r.used, nil
}
func TestFreeQuotaEnforcement(t *testing.T) {
	t.Setenv("FREE_TRAININGS_TOTAL", "3")
	for _, used := range []int64{0, 1, 2, 3, 4} {
		s := NewInterviewService(quotaRepository{used: used}, nil)
		err := s.ensureTrainingQuota(context.Background(), "0", "free")
		if (err != nil) != (used >= 3) {
			t.Fatalf("used=%d error=%v", used, err)
		}
	}
}
