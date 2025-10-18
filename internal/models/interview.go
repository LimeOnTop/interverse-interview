package models

import (
	"time"
)

type Candidate struct {
	ID              string    `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	Email           string    `json:"email" db:"email"`
	Phone           string    `json:"phone" db:"phone"`
	ExperienceYears int       `json:"experience_years" db:"experience_years"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type Interview struct {
	ID             string     `json:"id" db:"id"`
	CandidateID    string     `json:"candidate_id" db:"candidate_id"`
	InterviewerID  string     `json:"interviewer_id" db:"interviewer_id"`
	Title          string     `json:"title" db:"title"`
	Description    string     `json:"description" db:"description"`
	Status         string     `json:"status" db:"status"`
	ScheduledAt    time.Time  `json:"scheduled_at" db:"scheduled_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	Level          string     `json:"level" db:"level"`
	Specialization string     `json:"specialization" db:"specialization"`
	Candidate      *Candidate `json:"candidate,omitempty"`
}

type InterviewTechnology struct {
	InterviewID  string `json:"interview_id" db:"interview_id"`
	TechnologyID string `json:"technology_id" db:"technology_id"`
}

type InterviewQuestion struct {
	InterviewID string `json:"interview_id" db:"interview_id"`
	QuestionID  string `json:"question_id" db:"question_id"`
}
