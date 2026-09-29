package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type InterviewRepository struct {
	db *sql.DB
}

func NewInterviewRepository(db *sql.DB) *InterviewRepository {
	return &InterviewRepository{db: db}
}

func (r *InterviewRepository) Create(ctx context.Context, interview entity.Interview) (entity.Interview, error) {
	interview.ID = uuid.New().String()
	interview.CreatedAt = time.Now()
	interview.UpdatedAt = time.Now()

	query := `
		INSERT INTO interviews (id, user_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		interview.ID, interview.UserID,
		interview.Title, interview.Description, interview.Status,
		interview.ScheduledAt, interview.CreatedAt, interview.UpdatedAt,
		interview.Level, interview.Specialization,
	)
	if err != nil {
		return entity.Interview{}, fmt.Errorf("create interview: %w", err)
	}

	return interview, nil
}

func (r *InterviewRepository) GetByID(ctx context.Context, id string) (entity.Interview, error) {
	query := `
		SELECT id, user_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization
		FROM interviews WHERE id = $1
	`

	var interview entity.Interview
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&interview.ID, &interview.UserID,
		&interview.Title, &interview.Description, &interview.Status,
		&interview.ScheduledAt, &interview.CreatedAt, &interview.UpdatedAt,
		&interview.Level, &interview.Specialization,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Interview{}, fmt.Errorf("get interview: %w", ErrNotFound)
		}
		return entity.Interview{}, fmt.Errorf("get interview: %w", err)
	}

	return interview, nil
}

func (r *InterviewRepository) GetByUser(ctx context.Context, userID, status string, limit, offset int64) ([]entity.Interview, error) {
	query := `
		SELECT id, user_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization
		FROM interviews
		WHERE user_id = $1
	`
	args := []any{userID}

	if status != "" {
		query += ` AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		args = append(args, status, limit, offset)
	} else {
		query += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get interviews: %w", err)
	}
	defer rows.Close()

	return scanInterviews(rows)
}

func (r *InterviewRepository) Update(ctx context.Context, interview entity.Interview) (entity.Interview, error) {
	interview.UpdatedAt = time.Now()

	query := `
		UPDATE interviews
		SET title = $1, description = $2, status = $3, scheduled_at = $4, level = $5, specialization = $6, updated_at = $7
		WHERE id = $8
	`

	_, err := r.db.ExecContext(ctx, query,
		interview.Title, interview.Description, interview.Status,
		interview.ScheduledAt, interview.Level, interview.Specialization,
		interview.UpdatedAt, interview.ID,
	)
	if err != nil {
		return entity.Interview{}, fmt.Errorf("update interview: %w", err)
	}

	return interview, nil
}

func (r *InterviewRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM interviews WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete interview: %w", err)
	}

	return nil
}

func (r *InterviewRepository) GetScheduled(ctx context.Context, userID string, date time.Time) ([]entity.Interview, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT id, user_id, title, description, status, scheduled_at, created_at, updated_at, level, specialization
		FROM interviews
		WHERE user_id = $1 AND scheduled_at >= $2 AND scheduled_at < $3
		ORDER BY scheduled_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("get scheduled interviews: %w", err)
	}
	defer rows.Close()

	return scanInterviews(rows)
}

func (r *InterviewRepository) CountCreatedSince(ctx context.Context, userID string, since time.Time) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM interviews
		WHERE user_id = $1 AND created_at >= $2
	`
	var count int64
	if err := r.db.QueryRowContext(ctx, query, userID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count interviews created since: %w", err)
	}
	return count, nil
}

func (r *InterviewRepository) AddTechnology(ctx context.Context, interviewID, technologyID string) error {
	query := `INSERT INTO interview_technologies (interview_id, technology_id) VALUES ($1, $2)`

	_, err := r.db.ExecContext(ctx, query, interviewID, technologyID)
	if err != nil {
		return fmt.Errorf("add interview technology: %w", err)
	}

	return nil
}

func (r *InterviewRepository) GetTechnologies(ctx context.Context, interviewID string) ([]string, error) {
	query := `SELECT technology_id FROM interview_technologies WHERE interview_id = $1`

	rows, err := r.db.QueryContext(ctx, query, interviewID)
	if err != nil {
		return nil, fmt.Errorf("get interview technologies: %w", err)
	}
	defer rows.Close()

	var technologies []string
	for rows.Next() {
		var technologyID string
		if err := rows.Scan(&technologyID); err != nil {
			return nil, fmt.Errorf("scan technology: %w", err)
		}
		technologies = append(technologies, technologyID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate technologies: %w", err)
	}

	return technologies, nil
}

func (r *InterviewRepository) DeleteTechnologies(ctx context.Context, interviewID string) error {
	query := `DELETE FROM interview_technologies WHERE interview_id = $1`

	_, err := r.db.ExecContext(ctx, query, interviewID)
	if err != nil {
		return fmt.Errorf("delete interview technologies: %w", err)
	}

	return nil
}

func (r *InterviewRepository) DeleteSessionItems(ctx context.Context, interviewID string) error {
	query := `DELETE FROM interview_session_items WHERE interview_id = $1`

	_, err := r.db.ExecContext(ctx, query, interviewID)
	if err != nil {
		return fmt.Errorf("delete session items: %w", err)
	}

	return nil
}

func (r *InterviewRepository) SaveSessionItems(ctx context.Context, items []entity.SessionItem) error {
	query := `
		INSERT INTO interview_session_items (id, interview_id, question_id, item_type, sort_order, text, technology, difficulty, category, options, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, item := range items {
		if item.ID == "" {
			item.ID = uuid.New().String()
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}

		options := item.Options
		if options == nil {
			options = []entity.SessionOption{}
		}

		optionsJSON, err := json.Marshal(options)
		if err != nil {
			return fmt.Errorf("marshal session item options: %w", err)
		}

		_, err = r.db.ExecContext(ctx, query,
			item.ID, item.InterviewID, nullString(item.QuestionID), item.ItemType, item.SortOrder,
			item.Text, item.Technology, item.Difficulty, item.Category, optionsJSON, item.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("save session item: %w", err)
		}
	}

	return nil
}

func (r *InterviewRepository) GetSessionItems(ctx context.Context, interviewID string) ([]entity.SessionItem, error) {
	query := `
		SELECT id, interview_id, question_id, item_type, sort_order, text, technology, difficulty, category, options, created_at
		FROM interview_session_items
		WHERE interview_id = $1
		ORDER BY item_type ASC, sort_order ASC
	`

	rows, err := r.db.QueryContext(ctx, query, interviewID)
	if err != nil {
		return nil, fmt.Errorf("get session items: %w", err)
	}
	defer rows.Close()

	var items []entity.SessionItem
	for rows.Next() {
		var item entity.SessionItem
		var questionID sql.NullString
		var optionsJSON []byte

		if err := rows.Scan(
			&item.ID, &item.InterviewID, &questionID, &item.ItemType, &item.SortOrder,
			&item.Text, &item.Technology, &item.Difficulty, &item.Category, &optionsJSON, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session item: %w", err)
		}

		if questionID.Valid {
			item.QuestionID = questionID.String
		}

		if len(optionsJSON) > 0 {
			parsed, err := unmarshalSessionOptions(optionsJSON)
			if err != nil {
				return nil, fmt.Errorf("unmarshal session item options: %w", err)
			}
			item.Options = parsed
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session items: %w", err)
	}

	return items, nil
}

func (r *InterviewRepository) UpdateSessionItemOptions(ctx context.Context, itemID string, options []entity.SessionOption) error {
	if options == nil {
		options = []entity.SessionOption{}
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("marshal session item options: %w", err)
	}

	query := `UPDATE interview_session_items SET options = $1 WHERE id = $2`
	if _, err := r.db.ExecContext(ctx, query, optionsJSON, itemID); err != nil {
		return fmt.Errorf("update session item options: %w", err)
	}

	return nil
}

func scanInterviews(rows *sql.Rows) ([]entity.Interview, error) {
	var interviews []entity.Interview

	for rows.Next() {
		var interview entity.Interview
		if err := rows.Scan(
			&interview.ID, &interview.UserID,
			&interview.Title, &interview.Description, &interview.Status,
			&interview.ScheduledAt, &interview.CreatedAt, &interview.UpdatedAt,
			&interview.Level, &interview.Specialization,
		); err != nil {
			return nil, fmt.Errorf("scan interview: %w", err)
		}

		interviews = append(interviews, interview)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate interviews: %w", err)
	}

	return interviews, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func unmarshalSessionOptions(data []byte) ([]entity.SessionOption, error) {
	var structured []entity.SessionOption
	if err := json.Unmarshal(data, &structured); err == nil {
		if len(structured) == 0 {
			return structured, nil
		}
		if structured[0].Text != "" || structured[0].IsCorrect || structured[0].SortOrder != 0 {
			return structured, nil
		}
	}

	var legacy []string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}

	result := make([]entity.SessionOption, 0, len(legacy))
	for index, text := range legacy {
		result = append(result, entity.SessionOption{
			Text:      text,
			SortOrder: index,
		})
	}

	return result, nil
}
