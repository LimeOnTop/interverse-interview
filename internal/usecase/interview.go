package usecase

import (
	"context"
	"time"

	"github.com/LimeOnTop/interverse-interview/internal/entity"
)

type Interview interface {
	Create(ctx context.Context, interview entity.Interview, technologies []string, subscriptionPlan string) (InterviewDTO, error)
	GetByID(ctx context.Context, id string) (InterviewDTO, error)
	GetByUser(ctx context.Context, userID, status string, limit, offset int64) ([]InterviewDTO, error)
	Update(ctx context.Context, interview entity.Interview, technologies []string) (InterviewDTO, error)
	Delete(ctx context.Context, id string) error
	GetScheduled(ctx context.Context, userID string, date time.Time) ([]InterviewDTO, error)
	StartSession(ctx context.Context, interviewID, userID string) (SessionContentDTO, error)
	GetSessionContent(ctx context.Context, interviewID, userID string) (SessionContentDTO, error)
	GetTrainingStats(ctx context.Context, userID, subscriptionPlan string) (TrainingStatsDTO, error)
}

// TrainingStatsDTO is the plan quota usage and training counts of one user.
type TrainingStatsDTO struct {
	QuotaUsed     int
	QuotaLimit    int
	QuotaPeriod   string
	QuotaResetsAt *time.Time
	Total         int
	Completed     int
	InProgress    int
	Scheduled     int
}
