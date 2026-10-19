package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
	"strings"
	"time"
)

var PublishedNews = []string{"vacancy-path-20261017", "practice-reminders-20261017"}

type PracticePayload struct {
	Technologies      []string `json:"technologies"`
	Specialization    string   `json:"specialization"`
	Level             string   `json:"level"`
	TaskMode          string   `json:"task_mode"`
	SourceInterviewID string   `json:"source_interview_id"`
	StepIDs           []string `json:"step_ids"`
}
type PracticeReminder struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	TargetID      string          `json:"target_id"`
	StepIndex     int             `json:"step_index"`
	Title         string          `json:"title"`
	Payload       PracticePayload `json:"payload"`
	ScheduledAt   time.Time       `json:"scheduled_at"`
	Timezone      string          `json:"timezone"`
	EmailEnabled  bool            `json:"email_enabled"`
	LeadMinutes   int             `json:"lead_minutes"`
	Revision      int             `json:"revision"`
	EmailFailed   bool            `json:"email_failed"`
	EmailSent     bool            `json:"email_sent"`
	EmailRetryAt  *time.Time      `json:"email_retry_at,omitempty"`
	EmailError    string          `json:"email_error"`
	EmailAttempts int             `json:"email_attempts"`
}
type PracticeMail struct {
	UserID, Title, Path string
	ScheduledAt         time.Time
	Timezone            string
}
type PracticeSender interface {
	Send(context.Context, PracticeMail) error
}
type PracticeStateService struct {
	db           *sql.DB
	interview    *InterviewService
	preparations *PreparationService
	review       *ReviewTrainingService
	sender       PracticeSender
}

func NewPracticeStateService(db *sql.DB, interview *InterviewService, prep *PreparationService, review *ReviewTrainingService, sender PracticeSender) *PracticeStateService {
	return &PracticeStateService{db, interview, prep, review, sender}
}
func (s *PracticeStateService) Execute(ctx context.Context, r *pb.PracticeStateRequest) (*pb.PracticeStateResponse, error) {
	if id, e := strconv.ParseInt(r.UserId, 10, 64); e != nil || id <= 0 {
		return nil, status.Error(codes.Unauthenticated, "Войдите в аккаунт")
	}
	var result any
	var err error
	switch r.Action {
	case "check":
		var p struct {
			PracticePayload
			IncludeLevels bool `json:"include_levels"`
		}
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректные параметры")
		}
		result, err = s.interview.practiceAvailability(ctx, p.PracticePayload, p.IncludeLevels)
	case "list":
		result, err = s.listReminders(ctx, r.UserId)
	case "save":
		var p PracticeReminder
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректное напоминание")
		}
		result, err = s.saveReminder(ctx, r.UserId, p)
	case "restore":
		var p PracticeReminder
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректный запрос")
		}
		result, err = s.restoreReminder(ctx, r.UserId, p)
	case "retry_email":
		var p PracticeReminder
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректный запрос")
		}
		result, err = s.retryMail(ctx, r.UserId, p)
	case "dismiss":
		var p PracticeReminder
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректный запрос")
		}
		result, err = s.dismiss(ctx, r.UserId, p)
	case "start":
		var p PracticeReminder
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil {
			return nil, status.Error(codes.InvalidArgument, "Некорректный запрос")
		}
		result, err = s.start(ctx, r.UserId, r.SubscriptionPlan, p)
	case "news":
		var ids = []string{}
		rows, e := s.db.QueryContext(ctx, `SELECT news_id FROM user_news_reads WHERE user_id=$1`, r.UserId)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		result = map[string]any{"read_ids": ids}
	case "read_news":
		var p struct {
			IDs []string `json:"ids"`
		}
		if json.Unmarshal([]byte(r.PayloadJson), &p) != nil || len(p.IDs) > len(PublishedNews) {
			return nil, status.Error(codes.InvalidArgument, "Некорректные новости")
		}
		for _, id := range p.IDs {
			known := false
			for _, n := range PublishedNews {
				if n == id {
					known = true
				}
			}
			if !known {
				return nil, status.Error(codes.InvalidArgument, "Новость не найдена")
			}
		}
		for _, id := range p.IDs {
			if _, err = s.db.ExecContext(ctx, `INSERT INTO user_news_reads(user_id,news_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, r.UserId, id); err != nil {
				return nil, err
			}
		}
		result = map[string]bool{"ok": true}
	default:
		return nil, status.Error(codes.InvalidArgument, "Неизвестное действие")
	}
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	return &pb.PracticeStateResponse{PayloadJson: string(raw)}, err
}

const reminderColumns = `id,kind,target_id,step_index,title,payload,scheduled_at,timezone,email_enabled,lead_minutes,revision,email_failed,(email_sent_at IS NOT NULL),email_retry_at,email_error,email_attempts`

func scanReminder(row interface{ Scan(...any) error }) (PracticeReminder, error) {
	var p PracticeReminder
	var raw []byte
	e := row.Scan(&p.ID, &p.Kind, &p.TargetID, &p.StepIndex, &p.Title, &raw, &p.ScheduledAt, &p.Timezone, &p.EmailEnabled, &p.LeadMinutes, &p.Revision, &p.EmailFailed, &p.EmailSent, &p.EmailRetryAt, &p.EmailError, &p.EmailAttempts)
	if e == nil {
		e = json.Unmarshal(raw, &p.Payload)
	}
	return p, e
}
func (s *PracticeStateService) listReminders(ctx context.Context, user string) ([]PracticeReminder, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT `+reminderColumns+` FROM practice_reminders WHERE user_id=$1 AND state='scheduled' ORDER BY scheduled_at LIMIT 50`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []PracticeReminder{}
	for rows.Next() {
		p, e := scanReminder(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func validateSchedule(p PracticeReminder, now time.Time) error {
	if !p.ScheduledAt.After(now) || p.ScheduledAt.After(now.AddDate(1, 0, 0)) {
		return status.Error(codes.InvalidArgument, "Выберите время впереди, не дальше чем через год")
	}
	if p.EmailEnabled && p.LeadMinutes != 60 {
		return status.Error(codes.InvalidArgument, "Напоминание отправляется за час до практики")
	}
	if p.EmailEnabled && !p.ScheduledAt.Add(-time.Duration(p.LeadMinutes)*time.Minute).After(now) {
		return status.Error(codes.InvalidArgument, "До практики меньше часа. Сохраните план без письма или выберите другое время")
	}
	if _, e := time.LoadLocation(p.Timezone); e != nil {
		return status.Error(codes.InvalidArgument, "Выберите часовой пояс")
	}
	return nil
}
func (s *PracticeStateService) resolve(ctx context.Context, user string, p *PracticeReminder) error {
	bad := status.Error(codes.NotFound, "Практика не найдена или больше недоступна")
	if p.Kind != "draft" {
		if _, e := uuid.Parse(p.TargetID); e != nil {
			return bad
		}
	}
	switch p.Kind {
	case "interview":
		i, e := s.interview.GetByID(ctx, p.TargetID)
		if e != nil || i.UserID != user {
			return bad
		}
		if i.Status == "completed" || i.Status == "cancelled" {
			return status.Error(codes.FailedPrecondition, "Эта тренировка завершена. Запланируйте повтор из отчёта")
		}
		p.Title = i.Title
		p.Payload = PracticePayload{}
	case "preparation":
		var raw []byte
		if e := s.db.QueryRowContext(ctx, `SELECT payload FROM vacancy_preparations WHERE id=$1 AND user_id=$2`, p.TargetID, user).Scan(&raw); e != nil {
			return bad
		}
		var prep Preparation
		if json.Unmarshal(raw, &prep) != nil || prep.Archived || p.StepIndex < 0 || p.StepIndex >= len(prep.Steps) {
			return bad
		}
		p.Title = prep.Steps[p.StepIndex].Technology + " · " + prep.Title
		p.Payload = PracticePayload{}
	case "review":
		i, e := s.interview.GetByID(ctx, p.Payload.SourceInterviewID)
		if e != nil || i.UserID != user || len(p.Payload.StepIDs) == 0 || len(p.Payload.StepIDs) > 40 {
			return bad
		}
		p.Title = "Повторить ошибки · " + i.Title
	case "draft":
		if _, e := uuid.Parse(p.TargetID); e != nil {
			return status.Error(codes.InvalidArgument, "Некорректный черновик")
		}
		probe := Preparation{VacancyKey: "draft", Title: "draft", Level: p.Payload.Level}
		for _, t := range p.Payload.Technologies {
			probe.Steps = append(probe.Steps, PreparationStep{Technology: t, Specialization: p.Payload.Specialization})
		}
		if e := validatePreparation(&probe); e != nil {
			return e
		}
		if p.Payload.TaskMode != "theory" && p.Payload.TaskMode != "few" && p.Payload.TaskMode != "many" {
			return status.Error(codes.InvalidArgument, "Выберите формат задач")
		}
		availability, e := s.interview.practiceAvailability(ctx, p.Payload, false)
		if e != nil {
			return e
		}
		if !availability.Available {
			return status.Error(codes.FailedPrecondition, "Для выбранных параметров недостаточно вопросов. Выберите другую технологию или уровень — попытка не потрачена")
		}
		p.Title = "Практика " + strings.Join(p.Payload.Technologies, ", ") + " · " + p.Payload.Level
	default:
		return status.Error(codes.InvalidArgument, "Неизвестный тип практики")
	}
	if len([]rune(p.Title)) > 250 {
		p.Title = string([]rune(p.Title)[:250])
	}
	return nil
}
func (s *PracticeStateService) saveReminder(ctx context.Context, user string, p PracticeReminder) (PracticeReminder, error) {
	p.LeadMinutes = 60
	if p.EmailEnabled && s.sender == nil {
		return p, status.Error(codes.FailedPrecondition, "Письма временно недоступны. Сохраните практику без письма")
	}
	if e := validateSchedule(p, time.Now()); e != nil {
		return p, e
	}
	if e := s.resolve(ctx, user, &p); e != nil {
		return p, e
	}
	key := fmt.Sprintf("%s:%s:%d", p.Kind, p.TargetID, p.StepIndex)
	raw, _ := json.Marshal(p.Payload)
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return p, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "reminder:"+user); e != nil {
		return p, e
	}
	if p.ID != "" {
		if _, e = uuid.Parse(p.ID); e != nil {
			return p, status.Error(codes.NotFound, "Напоминание не найдено")
		}
		var revision int
		var oldKey, state string
		e = tx.QueryRowContext(ctx, `SELECT revision,target_key,state FROM practice_reminders WHERE id=$1 AND user_id=$2 FOR UPDATE`, p.ID, user).Scan(&revision, &oldKey, &state)
		if e != nil {
			return p, status.Error(codes.NotFound, "Напоминание не найдено")
		}
		if p.Revision != revision || state != "scheduled" || oldKey != key {
			return p, status.Error(codes.Aborted, "Напоминание изменилось. Обновите страницу и попробуйте ещё раз")
		}
	}
	var active int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM practice_reminders WHERE user_id=$1 AND state='scheduled'`, user).Scan(&active); e != nil {
		return p, e
	}
	if active >= 20 && p.ID == "" {
		return p, status.Error(codes.ResourceExhausted, "Можно сохранить до 20 напоминаний. Уберите ненужное")
	}
	result, e := scanReminder(tx.QueryRowContext(ctx, `INSERT INTO practice_reminders(id,user_id,target_key,kind,target_id,step_index,title,payload,scheduled_at,timezone,email_enabled,lead_minutes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(user_id,target_key) DO UPDATE SET scheduled_at=EXCLUDED.scheduled_at,timezone=EXCLUDED.timezone,email_enabled=EXCLUDED.email_enabled,lead_minutes=EXCLUDED.lead_minutes,title=EXCLUDED.title,payload=EXCLUDED.payload,state='scheduled',revision=practice_reminders.revision+1,email_sent_at=practice_reminders.email_sent_at,email_attempts=0,email_failed=(practice_reminders.email_error='uncertain'),email_retry_at=NULL,email_error=CASE WHEN practice_reminders.email_error='uncertain' THEN 'uncertain' ELSE '' END,updated_at=NOW() RETURNING `+reminderColumns, uuid.NewString(), user, key, p.Kind, p.TargetID, p.StepIndex, p.Title, string(raw), p.ScheduledAt, p.Timezone, p.EmailEnabled, p.LeadMinutes))
	if e != nil {
		return p, e
	}
	return result, tx.Commit()
}
func (s *PracticeStateService) dismiss(ctx context.Context, user string, p PracticeReminder) (any, error) {
	if _, e := uuid.Parse(p.ID); e != nil {
		return nil, status.Error(codes.NotFound, "Напоминание не найдено")
	}
	r, e := s.db.ExecContext(ctx, `UPDATE practice_reminders SET state='dismissed',revision=revision+1,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND revision=$3 AND state='scheduled'`, p.ID, user, p.Revision)
	if e != nil {
		return nil, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return nil, status.Error(codes.Aborted, "Напоминание изменилось. Обновите список")
	}
	return map[string]bool{"ok": true}, nil
}
func (s *PracticeStateService) start(ctx context.Context, user, plan string, p PracticeReminder) (any, error) {
	if _, e := uuid.Parse(p.ID); e != nil {
		return nil, status.Error(codes.NotFound, "Напоминание не найдено")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "reminder:"+user); e != nil {
		return nil, e
	}
	var state string
	var started sql.NullString
	e = tx.QueryRowContext(ctx, `SELECT state,started_interview_id::text FROM practice_reminders WHERE id=$1 AND user_id=$2 FOR UPDATE`, p.ID, user).Scan(&state, &started)
	if e != nil {
		return nil, status.Error(codes.NotFound, "Напоминание не найдено")
	}
	if state == "started" && started.Valid {
		return map[string]string{"interview_id": started.String}, nil
	}
	if state != "scheduled" {
		return nil, status.Error(codes.FailedPrecondition, "Напоминание уже снято")
	}
	saved, e := scanReminder(tx.QueryRowContext(ctx, `SELECT `+reminderColumns+` FROM practice_reminders WHERE id=$1`, p.ID))
	if e != nil {
		return nil, e
	}
	if p.Revision != saved.Revision {
		return nil, status.Error(codes.Aborted, "Время изменилось. Обновите список")
	}
	id := ""
	switch saved.Kind {
	case "interview":
		i, e := s.interview.GetByID(ctx, saved.TargetID)
		if e != nil || i.UserID != user {
			return nil, status.Error(codes.NotFound, "Тренировка недоступна")
		}
		id = i.ID
	case "preparation":
		res, e := s.preparations.Execute(ctx, &pb.VacancyPreparationRequest{UserId: user, Action: "prepare_step", PreparationId: saved.TargetID, StepIndex: int32(saved.StepIndex), RequestId: saved.ID, SubscriptionPlan: plan})
		if e != nil {
			return nil, e
		}
		var data map[string]string
		json.Unmarshal([]byte(res.PayloadJson), &data)
		id = data["interview_id"]
	case "review":
		content, e := s.review.Create(ctx, &pb.ReviewTrainingRequest{UserId: user, SourceReportId: saved.TargetID, SourceInterviewId: saved.Payload.SourceInterviewID, StepIds: saved.Payload.StepIDs, RequestId: saved.ID, SubscriptionPlan: plan})
		if e != nil {
			return nil, e
		}
		id = content.Interview.ID
	case "draft":
		id, e = s.createDraft(ctx, tx, user, plan, saved)
		if e != nil {
			return nil, e
		}
	}
	if id == "" {
		return nil, status.Error(codes.Internal, "Не удалось начать практику")
	}
	if _, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET state='started',started_interview_id=$2,revision=revision+1,updated_at=NOW() WHERE id=$1`, saved.ID, id); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]string{"interview_id": id}, nil
}
func (s *PracticeStateService) createDraft(ctx context.Context, tx *sql.Tx, user, plan string, r PracticeReminder) (string, error) {
	var e error
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "training-quota:"+user); e != nil {
		return "", e
	}
	window := trainingQuotaWindow(plan, time.Now())
	var used int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM interview_quota_log q WHERE q.user_id=$1 AND q.created_at>=$2 AND NOT EXISTS(SELECT 1 FROM vacancy_preparation_attempts a WHERE a.interview_id=q.interview_id)`, user, window.since.UTC()).Scan(&used); e != nil {
		return "", e
	}
	if used >= window.limit {
		return "", status.Error(codes.ResourceExhausted, "training_limit_exceeded: Лимит тренировок исчерпан")
	}
	if s.interview != nil {
		if _, _, e = s.interview.collectSessionQuestions(ctx, r.Payload.Technologies, r.Payload.Level, r.Payload.Specialization, r.Payload.TaskMode); e != nil {
			return "", status.Error(codes.FailedPrecondition, "Для этих параметров пока недостаточно вопросов. Измените параметры тренировки")
		}
	}
	id := uuid.NewString()
	_, e = tx.ExecContext(ctx, `INSERT INTO interviews(id,user_id,title,description,status,scheduled_at,created_at,updated_at,level,specialization,task_mode) VALUES($1,$2,$3,'Запланированная практика','scheduled',NOW(),NOW(),NOW(),$4,$5,$6)`, id, user, r.Title, r.Payload.Level, r.Payload.Specialization, r.Payload.TaskMode)
	if e != nil {
		return "", e
	}
	for _, t := range r.Payload.Technologies {
		if _, e = tx.ExecContext(ctx, `INSERT INTO interview_technologies(interview_id,technology_id) VALUES($1,$2)`, id, t); e != nil {
			return "", e
		}
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO interview_quota_log(user_id,interview_id) VALUES($1,$2)`, user, id)
	return id, e
}

func (s *PracticeStateService) MarkStarted(ctx context.Context, user, id string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE practice_reminders r SET state='started',started_interview_id=$2,revision=revision+1,updated_at=NOW() WHERE r.user_id=$1 AND r.state='scheduled' AND ((r.kind='interview' AND r.target_id=$2) OR (r.kind='preparation' AND EXISTS(SELECT 1 FROM vacancy_preparation_attempts a WHERE a.preparation_id::text=r.target_id AND a.step_index=r.step_index AND a.interview_id::text=$2)) OR (r.kind='review' AND EXISTS(SELECT 1 FROM review_training_requests q WHERE q.source_report_id::text=r.target_id AND q.interview_id::text=$2)))`, user, id)
}

func (s *PracticeStateService) restoreReminder(ctx context.Context, user string, p PracticeReminder) (any, error) {
	if _, e := uuid.Parse(p.ID); e != nil {
		return nil, status.Error(codes.NotFound, "План не найден")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "reminder:"+user); e != nil {
		return nil, e
	}
	saved, e := scanReminder(tx.QueryRowContext(ctx, `SELECT `+reminderColumns+` FROM practice_reminders WHERE id=$1 AND user_id=$2 AND revision=$3 AND state='dismissed' FOR UPDATE`, p.ID, user, p.Revision))
	if e != nil {
		return nil, status.Error(codes.Aborted, "План изменился. Обновите список")
	}
	if e = s.resolve(ctx, user, &saved); e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `UPDATE practice_reminders SET state='scheduled',revision=revision+1,lead_minutes=60,email_enabled=(email_enabled AND scheduled_at>NOW()+INTERVAL '1 hour'),updated_at=NOW() WHERE id=$1`, p.ID)
	if e != nil {
		return nil, e
	}
	return map[string]bool{"ok": true}, tx.Commit()
}
