package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/inter-verse/services/interview-service/internal/models"
)

type InterviewRepository struct {
	db *sql.DB
}

func NewInterviewRepository(db *sql.DB) *InterviewRepository {
	return &InterviewRepository{db: db}
}

func (r *InterviewRepository) CreateInterview(interview *models.Interview) error {
	interview.ID = uuid.New().String()
	interview.CreatedAt = time.Now()
	interview.UpdatedAt = time.Now()

	query := `
		INSERT INTO interviews (id, candidate_id, interviewer_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.Exec(query,
		interview.ID, interview.CandidateID, interview.InterviewerID,
		interview.Title, interview.Description, interview.Status,
		interview.ScheduledAt, interview.CreatedAt, interview.UpdatedAt,
		interview.Level, interview.Specialization)
	if err != nil {
		return fmt.Errorf("failed to create interview: %w", err)
	}

	return nil
}

func (r *InterviewRepository) GetInterviewByID(id string) (*models.Interview, error) {
	query := `
		SELECT id, candidate_id, interviewer_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization 
		FROM interviews WHERE id = $1
	`

	interview := &models.Interview{}
	err := r.db.QueryRow(query, id).Scan(
		&interview.ID, &interview.CandidateID, &interview.InterviewerID,
		&interview.Title, &interview.Description, &interview.Status,
		&interview.ScheduledAt, &interview.CreatedAt, &interview.UpdatedAt,
		&interview.Level, &interview.Specialization,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("interview not found")
		}
		return nil, fmt.Errorf("failed to get interview: %w", err)
	}

	return interview, nil
}

func (r *InterviewRepository) GetInterviewsByInterviewer(interviewerID string, limit, offset int) ([]*models.Interview, error) {
	query := `
		SELECT i.id, i.candidate_id, i.interviewer_id, i.title, i.description, i.status, i.scheduled_at, i.created_at, i.updated_at, i.level, i.specialization,
		       c.id, c.name, c.email, c.phone, c.experience_years, c.created_at, c.updated_at
		FROM interviews i
		LEFT JOIN candidates c ON i.candidate_id = c.id
		WHERE i.interviewer_id = $1 
		ORDER BY i.created_at DESC 
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, interviewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get interviews: %w", err)
	}
	defer rows.Close()

	var interviews []*models.Interview
	for rows.Next() {
		interview := &models.Interview{}
		candidate := &models.Candidate{}
		err := rows.Scan(
			&interview.ID, &interview.CandidateID, &interview.InterviewerID,
			&interview.Title, &interview.Description, &interview.Status,
			&interview.ScheduledAt, &interview.CreatedAt, &interview.UpdatedAt,
			&interview.Level, &interview.Specialization,
			&candidate.ID, &candidate.Name, &candidate.Email, &candidate.Phone,
			&candidate.ExperienceYears, &candidate.CreatedAt, &candidate.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan interview: %w", err)
		}

		// Only set candidate if we have valid data
		if candidate.ID != "" {
			interview.Candidate = candidate
			fmt.Printf("DEBUG: Found candidate %s for interview %s\n", candidate.Name, interview.ID)
		} else {
			fmt.Printf("DEBUG: No candidate found for interview %s\n", interview.ID)
		}

		interviews = append(interviews, interview)
	}

	return interviews, nil
}

func (r *InterviewRepository) UpdateInterview(interview *models.Interview) error {
	interview.UpdatedAt = time.Now()

	query := `
		UPDATE interviews 
		SET title = $1, description = $2, status = $3, scheduled_at = $4, level = $5, specialization = $6, updated_at = $7
		WHERE id = $8
	`

	_, err := r.db.Exec(query,
		interview.Title, interview.Description, interview.Status,
		interview.ScheduledAt, interview.Level, interview.Specialization,
		interview.UpdatedAt, interview.ID)
	if err != nil {
		return fmt.Errorf("failed to update interview: %w", err)
	}

	return nil
}

func (r *InterviewRepository) DeleteInterview(id string) error {
	query := `DELETE FROM interviews WHERE id = $1`

	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete interview: %w", err)
	}

	return nil
}

func (r *InterviewRepository) GetScheduledInterviews(interviewerID string, date time.Time) ([]*models.Interview, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT id, candidate_id, interviewer_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization
		FROM interviews 
		WHERE interviewer_id = $1 AND scheduled_at >= $2 AND scheduled_at < $3
		ORDER BY scheduled_at ASC
	`

	rows, err := r.db.Query(query, interviewerID, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled interviews: %w", err)
	}
	defer rows.Close()

	var interviews []*models.Interview
	for rows.Next() {
		interview := &models.Interview{}
		err := rows.Scan(
			&interview.ID, &interview.CandidateID, &interview.InterviewerID,
			&interview.Title, &interview.Description, &interview.Status,
			&interview.ScheduledAt, &interview.CreatedAt, &interview.UpdatedAt,
			&interview.Level, &interview.Specialization,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan interview: %w", err)
		}
		interviews = append(interviews, interview)
	}

	return interviews, nil
}

func (r *InterviewRepository) AddInterviewTechnology(interviewID, technologyID string) error {
	query := `INSERT INTO interview_technologies (interview_id, technology_id) VALUES ($1, $2)`

	_, err := r.db.Exec(query, interviewID, technologyID)
	if err != nil {
		return fmt.Errorf("failed to add interview technology: %w", err)
	}

	return nil
}

func (r *InterviewRepository) GetInterviewTechnologies(interviewID string) ([]string, error) {
	query := `SELECT technology_id FROM interview_technologies WHERE interview_id = $1`

	rows, err := r.db.Query(query, interviewID)
	if err != nil {
		return nil, fmt.Errorf("failed to get interview technologies: %w", err)
	}
	defer rows.Close()

	var technologies []string
	for rows.Next() {
		var technologyID string
		err := rows.Scan(&technologyID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan technology: %w", err)
		}
		technologies = append(technologies, technologyID)
	}

	return technologies, nil
}
