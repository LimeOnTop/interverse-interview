package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PreparationStep struct {
	ID             string `json:"id"`
	Technology     string `json:"technology"`
	Specialization string `json:"specialization"`
}
type PreparationAttempt struct {
	StepIndex   int       `json:"step_index"`
	InterviewID string    `json:"interview_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
type Preparation struct {
	Revision   int                  `json:"revision"`
	Archived   bool                 `json:"archived"`
	ID         string               `json:"id"`
	VacancyKey string               `json:"vacancy_key"`
	Title      string               `json:"title"`
	Company    string               `json:"company"`
	URL        string               `json:"url"`
	Level      string               `json:"level"`
	Steps      []PreparationStep    `json:"steps"`
	CreatedAt  time.Time            `json:"created_at"`
	Attempts   []PreparationAttempt `json:"attempts"`
}
type PreparationService struct {
	db        *sql.DB
	interview *InterviewService
}

func NewPreparationService(db *sql.DB, interview *InterviewService) *PreparationService {
	return &PreparationService{db: db, interview: interview}
}
func validatePreparation(p *Preparation) error {
	p.Title = strings.TrimSpace(p.Title)
	p.VacancyKey = strings.TrimSpace(p.VacancyKey)
	if p.Title == "" || len(p.Title) > 300 || p.VacancyKey == "" || len(p.VacancyKey) > 500 || len(p.Company) > 300 || len(p.URL) > 2000 || len(p.Steps) == 0 || len(p.Steps) > 30 {
		return status.Error(codes.InvalidArgument, "Укажите вакансию и от 1 до 30 технологий")
	}
	if p.URL != "" {
		u, e := url.Parse(p.URL)
		if e != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return status.Error(codes.InvalidArgument, "Некорректная ссылка вакансии")
		}
	}
	switch p.Level {
	case "intern", "junior", "middle", "senior", "lead":
	default:
		return status.Error(codes.InvalidArgument, "Выберите уровень подготовки")
	}
	seen := map[string]bool{}
	for i := range p.Steps {
		step := &p.Steps[i]
		step.Technology = strings.TrimSpace(step.Technology)
		key := strings.ToLower(step.Technology)
		if key == "" || len(key) > 100 || seen[key] {
			return status.Error(codes.InvalidArgument, "Технологии должны быть непустыми и не повторяться")
		}
		seen[key] = true
		switch step.Specialization {
		case "backend", "frontend", "devops", "qa", "data_science":
		default:
			return status.Error(codes.InvalidArgument, "Выберите направление для каждой технологии")
		}
	}
	return nil
}
func (s *PreparationService) Execute(ctx context.Context, r *pb.VacancyPreparationRequest) (*pb.VacancyPreparationResponse, error) {
	if id, e := strconv.ParseInt(r.UserId, 10, 64); e != nil || id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Некорректный пользователь")
	}
	var result any
	var err error
	switch r.Action {
	case "list":
		result, err = s.list(ctx, r.UserId)
	case "update", "archive":
		result, err = s.manage(ctx, r)
	case "create":
		var p Preparation
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректные данные подготовки")
		}
		if err = validatePreparation(&p); err != nil {
			return nil, err
		}
		result, err = s.create(ctx, r.UserId, r.SubscriptionPlan, p)
	case "prepare_step":
		result, err = s.prepare(ctx, r)
	default:
		return nil, status.Error(codes.InvalidArgument, "Неизвестное действие")
	}
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		log.Printf("preparation %s failed: %v", r.Action, err)
		return nil, status.Error(codes.Internal, "Не удалось сохранить подготовку. Попробуйте ещё раз")
	}
	raw, e := json.Marshal(result)
	if e != nil {
		return nil, e
	}
	return &pb.VacancyPreparationResponse{PayloadJson: string(raw)}, nil
}
func (s *PreparationService) list(ctx context.Context, user string) ([]Preparation, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT payload FROM vacancy_preparations WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, user)
	if e != nil {
		return nil, e
	}
	var ps = []Preparation{}
	for rows.Next() {
		var raw []byte
		var p Preparation
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return nil, e
		}
		if e = json.Unmarshal(raw, &p); e != nil {
			rows.Close()
			return nil, e
		}
		ensurePreparationStepIDs(&p)
		p.Attempts = []PreparationAttempt{}
		ps = append(ps, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range ps {
		rs, err := s.db.QueryContext(ctx, `SELECT a.step_index,COALESCE(a.interview_id::text,''),COALESCE(i.status,'deleted'),a.created_at FROM vacancy_preparation_attempts a LEFT JOIN interviews i ON i.id=a.interview_id WHERE a.preparation_id=$1 ORDER BY a.created_at DESC`, ps[i].ID)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var a PreparationAttempt
			if err = rs.Scan(&a.StepIndex, &a.InterviewID, &a.Status, &a.CreatedAt); err != nil {
				rs.Close()
				return nil, err
			}
			ps[i].Attempts = append(ps[i].Attempts, a)
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return nil, err
		}
	}
	return ps, nil
}
func (s *PreparationService) prepare(ctx context.Context, r *pb.VacancyPreparationRequest) (any, error) {
	if _, e := uuid.Parse(r.PreparationId); e != nil {
		return nil, status.Error(codes.NotFound, "Подготовка не найдена")
	}
	if _, e := uuid.Parse(r.RequestId); e != nil {
		return nil, status.Error(codes.InvalidArgument, "Некорректный идентификатор попытки")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	// Serialize path attempts and quota decisions for this account.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "training-quota:"+r.UserId); e != nil {
		return nil, e
	}
	var raw []byte
	e = tx.QueryRowContext(ctx, `SELECT payload FROM vacancy_preparations WHERE id=$1 AND user_id=$2 FOR UPDATE`, r.PreparationId, r.UserId).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "Подготовка не найдена")
	}
	if e != nil {
		return nil, e
	}
	var p Preparation
	if e = json.Unmarshal(raw, &p); e != nil {
		return nil, e
	}
	if p.Archived {
		return nil, status.Error(codes.FailedPrecondition, "Подготовка в архиве. Сначала восстановите её")
	}
	idx := int(r.StepIndex)
	if idx < 0 || idx >= len(p.Steps) {
		return nil, status.Error(codes.InvalidArgument, "Этап не найден")
	}
	var existing string
	var oldStep int
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(interview_id::text,''),step_index FROM vacancy_preparation_attempts WHERE preparation_id=$1 AND request_id=$2`, p.ID, r.RequestId).Scan(&existing, &oldStep)
	if e == nil {
		if oldStep != idx || existing == "" {
			return nil, status.Error(codes.FailedPrecondition, "Эта попытка больше недоступна")
		}
		return map[string]string{"interview_id": existing}, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	e = tx.QueryRowContext(ctx, `SELECT i.id::text FROM vacancy_preparation_attempts a JOIN interviews i ON i.id=a.interview_id WHERE a.preparation_id=$1 AND a.step_index=$2 AND i.status IN ('scheduled','in_progress') ORDER BY a.created_at DESC LIMIT 1`, p.ID, idx).Scan(&existing)
	if e == nil {
		return map[string]string{"interview_id": existing}, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if s.interview != nil {
		_, _, err := s.interview.collectSessionQuestions(ctx, []string{p.Steps[idx].Technology}, p.Level, p.Steps[idx].Specialization, "theory")
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, "По этой технологии пока недостаточно вопросов для тренировки. Попытка не потрачена. Выберите другой этап")
		}
	}
	id := uuid.NewString()
	step := p.Steps[idx]
	title := fmt.Sprintf("%s · %s", step.Technology, p.Title)
	if len([]rune(title)) > 250 {
		title = string([]rune(title)[:250])
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO interviews(id,user_id,title,description,status,scheduled_at,created_at,updated_at,level,specialization,task_mode) VALUES($1,$2,$3,$4,'scheduled',NOW(),NOW(),NOW(),$5,$6,'theory')`, id, r.UserId, title, "Подготовка по вакансии: "+p.Title, p.Level, step.Specialization)
	if e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO interview_technologies(interview_id,technology_id) VALUES($1,$2)`, id, step.Technology); e != nil {
		return nil, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id) VALUES($1,$2,$3,$4)`, p.ID, idx, r.RequestId, id); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]string{"interview_id": id}, nil
}

// A vacancy preparation is one quota unit, regardless of its stages or retakes.
func (s *PreparationService) create(ctx context.Context, user, plan string, p Preparation) (Preparation, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "training-quota:"+user); e != nil {
		return p, e
	}
	var raw []byte
	e = tx.QueryRowContext(ctx, `SELECT payload FROM vacancy_preparations WHERE user_id=$1 AND vacancy_key=$2`, user, p.VacancyKey).Scan(&raw)
	if e == nil {
		e = json.Unmarshal(raw, &p)
		return p, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return p, e
	}
	if s.interview != nil {
		for _, step := range p.Steps {
			_, _, err := s.interview.collectSessionQuestions(ctx, []string{step.Technology}, p.Level, step.Specialization, "theory")
			if err != nil {
				return p, status.Errorf(codes.FailedPrecondition, "Для %s пока недостаточно вопросов. Уберите технологию или измените уровень. Попытка не потрачена", step.Technology)
			}
		}
	}
	window := trainingQuotaWindow(plan, time.Now())
	var used int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM interview_quota_log q WHERE q.user_id=$1 AND q.created_at >= $2 AND NOT EXISTS(SELECT 1 FROM vacancy_preparation_attempts a WHERE a.interview_id=q.interview_id)`, user, window.since.UTC()).Scan(&used); e != nil {
		return p, e
	}
	if used >= int64(window.limit) {
		return p, status.Error(codes.ResourceExhausted, "training_limit_exceeded: Лимит тренировок исчерпан")
	}
	p.ID = uuid.NewString()
	p.Revision = 1
	p.Archived = false
	ensurePreparationStepIDs(&p)
	p.CreatedAt = time.Now()
	p.Attempts = []PreparationAttempt{}
	raw, _ = json.Marshal(p)
	if _, e = tx.ExecContext(ctx, `INSERT INTO vacancy_preparations(id,user_id,vacancy_key,payload) VALUES($1,$2,$3,$4)`, p.ID, user, p.VacancyKey, string(raw)); e != nil {
		return p, e
	}
	// interview_id is deliberately the preparation UUID: this log has no interview FK.
	if _, e = tx.ExecContext(ctx, `INSERT INTO interview_quota_log(user_id,interview_id) VALUES($1,$2)`, user, p.ID); e != nil {
		return p, e
	}
	if e = tx.Commit(); e != nil {
		return p, e
	}
	return p, nil
}
