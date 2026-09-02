package entity

import "time"

const (
	StatusScheduled  = "scheduled"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"

	ItemTypeQuestion = "question"
	ItemTypeTask     = "task"
)

type Interview struct {
	ID             string
	UserID         string
	Title          string
	Description    string
	Status         string
	ScheduledAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Level          string
	Specialization string
}

type SessionItem struct {
	ID          string
	InterviewID string
	QuestionID  string
	ItemType    string
	SortOrder   int
	Text        string
	Technology  string
	Difficulty  string
	Category    string
	Options     []SessionOption
	CreatedAt   time.Time
}
