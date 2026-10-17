package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type ReviewTrainingService struct {
	db        *sql.DB
	interview *InterviewService
}

func NewReviewTrainingService(db *sql.DB, s *InterviewService) *ReviewTrainingService {
	return &ReviewTrainingService{db, s}
}
func (s *ReviewTrainingService) Create(ctx context.Context, r *pb.ReviewTrainingRequest) (usecase.SessionContentDTO, error) {
	fail := func(code codes.Code, text string) (usecase.SessionContentDTO, error) {
		return usecase.SessionContentDTO{}, status.Error(code, text)
	}
	if r.SubscriptionPlan != "paid" {
		return fail(codes.PermissionDenied, "Повторение по ошибкам доступно в Pro")
	}
	for _, id := range []string{r.SourceReportId, r.SourceInterviewId, r.RequestId} {
		if parsed, e := uuid.Parse(id); e != nil || parsed == uuid.Nil {
			return fail(codes.InvalidArgument, "Некорректные параметры повторения")
		}
	}
	if len(r.StepIds) == 0 || len(r.StepIds) > 40 {
		return fail(codes.InvalidArgument, "В отчёте нет материала для повторения")
	}
	source, e := s.interview.repository.GetByID(ctx, r.SourceInterviewId)
	if e != nil || source.UserID != r.UserId {
		return fail(codes.NotFound, "Исходная тренировка не найдена")
	}
	sourceItems, e := s.interview.repository.GetSessionItems(ctx, source.ID)
	if e != nil {
		return usecase.SessionContentDTO{}, e
	}
	selected := map[string]bool{}
	for _, id := range r.StepIds {
		selected[id] = true
	}
	items := []entity.SessionItem{}
	technologies := []string{}
	for _, item := range sourceItems {
		if selected[item.ID] {
			if item.ItemType == "question" && len(item.Options) < 2 {
				return fail(codes.FailedPrecondition, "У старого вопроса нет вариантов ответа. Создай новую тренировку по этой технологии")
			}
			items = append(items, item)
			technologies = append(technologies, item.Technology)
			delete(selected, item.ID)
		}
	}
	if len(selected) > 0 || len(items) == 0 {
		return fail(codes.FailedPrecondition, "Материал исходной тренировки недоступен для повторения")
	}
	technologies = uniqueTechnologies(technologies)
	if len(technologies) == 0 {
		return fail(codes.FailedPrecondition, "В исходной тренировке не указаны технологии")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return usecase.SessionContentDTO{}, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "review:"+r.UserId); e != nil {
		return usecase.SessionContentDTO{}, e
	}
	var existing sql.NullString
	var reportID string
	e = tx.QueryRowContext(ctx, `SELECT interview_id::text,source_report_id::text FROM review_training_requests WHERE user_id=$1 AND request_id=$2`, r.UserId, r.RequestId).Scan(&existing, &reportID)
	if e == nil {
		if !existing.Valid || reportID != r.SourceReportId {
			return fail(codes.FailedPrecondition, "Эта попытка повторения больше недоступна")
		}
		tx.Rollback()
		return s.interview.GetSessionContent(ctx, existing.String, r.UserId)
	}
	if e != sql.ErrNoRows {
		return usecase.SessionContentDTO{}, e
	}
	e = tx.QueryRowContext(ctx, `SELECT i.id::text FROM review_training_requests r JOIN interviews i ON i.id=r.interview_id WHERE r.user_id=$1 AND r.source_report_id=$2 AND i.status='in_progress' ORDER BY r.created_at DESC LIMIT 1`, r.UserId, r.SourceReportId).Scan(&existing)
	if e == nil {
		if _, e = tx.ExecContext(ctx, `INSERT INTO review_training_requests(user_id,request_id,source_report_id,interview_id) VALUES($1,$2,$3,$4)`, r.UserId, r.RequestId, r.SourceReportId, existing.String); e != nil {
			return usecase.SessionContentDTO{}, e
		}
		if e = tx.Commit(); e != nil {
			return usecase.SessionContentDTO{}, e
		}
		return s.interview.GetSessionContent(ctx, existing.String, r.UserId)
	}
	if e != sql.ErrNoRows {
		return usecase.SessionContentDTO{}, e
	}
	window := trainingQuotaWindow(r.SubscriptionPlan, time.Now())
	var used int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM interview_quota_log WHERE user_id=$1 AND created_at>=$2`, r.UserId, window.since.UTC()).Scan(&used); e != nil {
		return usecase.SessionContentDTO{}, e
	}
	if used >= window.limit {
		return fail(codes.ResourceExhausted, "Лимит тренировок исчерпан")
	}
	id := uuid.NewString()
	now := time.Now()
	taskCount := 0
	for _, item := range items {
		if item.ItemType == "task" {
			taskCount++
		}
	}
	mode := "theory"
	if taskCount > 0 {
		mode = "few"
	}
	if taskCount > 3 {
		mode = "many"
	}
	title := "Повторение по ошибкам"
	description := "Вопросы и задачи с ошибками из отчёта " + r.SourceReportId
	if _, e = tx.ExecContext(ctx, `INSERT INTO interviews(id,user_id,title,description,status,scheduled_at,created_at,updated_at,level,specialization,task_mode) VALUES($1,$2,$3,$4,'in_progress',$5,$5,$5,$6,$7,$8)`, id, r.UserId, title, description, now, source.Level, source.Specialization, mode); e != nil {
		return usecase.SessionContentDTO{}, e
	}
	for _, tech := range technologies {
		if _, e = tx.ExecContext(ctx, `INSERT INTO interview_technologies(interview_id,technology_id) VALUES($1,$2)`, id, tech); e != nil {
			return usecase.SessionContentDTO{}, e
		}
	}
	counters := map[string]int{}
	for _, item := range items {
		counters[item.ItemType]++
		options, e := json.Marshal(item.Options)
		if e != nil {
			return usecase.SessionContentDTO{}, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO interview_session_items(id,interview_id,question_id,item_type,sort_order,text,technology,difficulty,category,options) VALUES($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10)`, uuid.NewString(), id, item.QuestionID, item.ItemType, counters[item.ItemType], item.Text, item.Technology, item.Difficulty, item.Category, options); e != nil {
			return usecase.SessionContentDTO{}, e
		}
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO interview_quota_log(user_id,interview_id) VALUES($1,$2)`, r.UserId, id); e != nil {
		return usecase.SessionContentDTO{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO review_training_requests(user_id,request_id,source_report_id,interview_id) VALUES($1,$2,$3,$4)`, r.UserId, r.RequestId, r.SourceReportId, id); e != nil {
		return usecase.SessionContentDTO{}, e
	}
	if e = tx.Commit(); e != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("commit review training: %w", e)
	}
	return s.interview.GetSessionContent(ctx, id, r.UserId)
}
