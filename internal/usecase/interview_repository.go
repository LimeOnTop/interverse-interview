package usecase

import (
	"context"
	"time"

	"github.com/LimeOnTop/interverse-interview/internal/entity"
)

type InterviewRepository interface {
	Create(ctx context.Context, interview entity.Interview) (entity.Interview, error)
	GetByID(ctx context.Context, id string) (entity.Interview, error)
	GetByUser(ctx context.Context, userID, status string, limit, offset int) ([]entity.Interview, error)
	Update(ctx context.Context, interview entity.Interview) (entity.Interview, error)
	Delete(ctx context.Context, id string) error
	GetScheduled(ctx context.Context, userID string, date time.Time) ([]entity.Interview, error)
	CountCreatedSince(ctx context.Context, userID string, since time.Time) (int, error)
	AddTechnology(ctx context.Context, interviewID, technologyID string) error
	GetTechnologies(ctx context.Context, interviewID string) ([]string, error)
	DeleteTechnologies(ctx context.Context, interviewID string) error
	DeleteSessionItems(ctx context.Context, interviewID string) error
	SaveSessionItems(ctx context.Context, items []entity.SessionItem) error
	GetSessionItems(ctx context.Context, interviewID string) ([]entity.SessionItem, error)
	UpdateSessionItemOptions(ctx context.Context, itemID string, options []entity.SessionOption) error
}

type QuestionBank interface {
	GetByTechnology(ctx context.Context, technology, difficulty, category string, limit int) ([]QuestionRef, error)
	GetByID(ctx context.Context, id string) (QuestionRef, error)
}

type QuestionOptionRef struct {
	Text      string
	IsCorrect bool
	SortOrder int
}

type QuestionRef struct {
	ID         string
	Text       string
	Category   string
	Difficulty string
	Technology string
	Tags       []string
	Options    []QuestionOptionRef
}
