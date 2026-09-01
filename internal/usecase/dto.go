package usecase

import "time"

type InterviewDTO struct {
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
	Technologies   []string
}

type SessionItemDTO struct {
	ID          string
	QuestionID  string
	ItemType    string
	SortOrder   int
	Text        string
	Technology  string
	Difficulty  string
	Category    string
}

type SessionContentDTO struct {
	Interview InterviewDTO
	Questions []SessionItemDTO
	Tasks     []SessionItemDTO
}
