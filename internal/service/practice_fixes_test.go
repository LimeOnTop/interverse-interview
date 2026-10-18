package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/repository"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"testing"
	"time"
)

type availabilityBank struct{ unavailable bool }

func (b availabilityBank) GetByTechnology(_ context.Context, technology, difficulty, category string, _ int) ([]usecase.QuestionRef, error) {
	if b.unavailable {
		return nil, errors.New("question service offline")
	}
	if technology != "Python" || category != "question" {
		return nil, nil
	}
	out := []usecase.QuestionRef{}
	for i := 0; i < 10; i++ {
		out = append(out, usecase.QuestionRef{ID: fmt.Sprintf("q%d", i), Text: "question", Technology: technology, Difficulty: difficulty, Options: []usecase.QuestionOptionRef{{Text: "right", IsCorrect: true}, {Text: "wrong"}}})
	}
	return out, nil
}
func (availabilityBank) GetByID(context.Context, string) (usecase.QuestionRef, error) {
	return usecase.QuestionRef{}, nil
}
func TestPracticeAvailabilityRejectsImpossiblePlanBeforeQuota(t *testing.T) {
	db := practiceTestDB(t)
	s := NewPracticeStateService(db, NewInterviewService(nil, availabilityBank{}), nil, nil, nil)
	p := draftReminder()
	p.Payload.Technologies = []string{"Actix"}
	p.Payload.Level = "lead"
	if _, e := practiceReq(t, s, "1", "save", p, ""); status.Code(e) != codes.FailedPrecondition {
		t.Fatal("impossible plan saved", e)
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM practice_reminders`).Scan(&n)
	if n != 0 {
		t.Fatal("failed plan persisted")
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&n)
	if n != 0 {
		t.Fatal("failed plan spent quota")
	}
	p.Payload.Technologies = []string{"Python"}
	raw, e := practiceReq(t, s, "1", "save", p, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &p)
	p.Payload.Level = "junior"
	if _, e = practiceReq(t, s, "1", "save", p, ""); e != nil {
		t.Fatal("saved draft not editable", e)
	}
	availability, e := s.interview.practiceAvailability(context.Background(), p.Payload, true)
	if e != nil || !availability.Available || len(availability.AvailableLevels) != 5 {
		t.Fatal("valid levels missing", e, availability)
	}
	s.interview = NewInterviewService(nil, availabilityBank{unavailable: true})
	if _, e = s.interview.practiceAvailability(context.Background(), p.Payload, false); status.Code(e) != codes.Unavailable {
		t.Fatal("upstream failure disguised as missing questions", e)
	}
}
func TestPreparationEditingPreservesStageHistoryAndReminders(t *testing.T) {
	db := practiceTestDB(t)
	raw, e := os.ReadFile("../../migrations/000008_vacancy_preparations.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	s := NewPreparationService(db, nil)
	p, e := s.create(ctx, "1", "paid", Preparation{Title: "Developer", VacancyKey: "manual:edit", Level: "junior", Steps: []PreparationStep{{Technology: "Python", Specialization: "backend"}, {Technology: "Docker", Specialization: "devops"}}})
	if e != nil {
		t.Fatal(e)
	}
	attempted := uuid.NewString()
	if _, e = db.Exec(`INSERT INTO interviews(id,user_id,status) VALUES($1,1,'completed')`, attempted); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id) VALUES($1,0,$2,$3)`, p.ID, uuid.NewString(), attempted); e != nil {
		t.Fatal(e)
	}
	ps := NewPracticeStateService(db, nil, nil, nil, nil)
	plan := PracticeReminder{Kind: "preparation", TargetID: p.ID, StepIndex: 0, ScheduledAt: time.Now().Add(time.Hour), Timezone: "Europe/Moscow"}
	out, e := practiceReq(t, ps, "1", "save", plan, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(out, &plan)
	update := func(user string, next Preparation, action string) (Preparation, error) {
		raw, _ := json.Marshal(next)
		return s.manage(ctx, &pb.VacancyPreparationRequest{UserId: user, PreparationId: p.ID, Action: action, PayloadJson: string(raw)})
	}
	next := p
	next.Title = "Renamed developer"
	next.Steps = []PreparationStep{p.Steps[1], p.Steps[0]}
	edited, e := update("1", next, "update")
	if e != nil {
		t.Fatal(e)
	}
	var step int
	db.QueryRow(`SELECT step_index FROM vacancy_preparation_attempts WHERE interview_id=$1`, attempted).Scan(&step)
	if step != 1 {
		t.Fatal("history not reordered", step)
	}
	var key, title string
	db.QueryRow(`SELECT step_index,target_key,title FROM practice_reminders WHERE id=$1`, plan.ID).Scan(&step, &key, &title)
	if step != 1 || key != "preparation:"+p.ID+":1" || title != "Python · Renamed developer" {
		t.Fatal("reminder lost technology", step, key, title)
	}
	if _, e = update("1", next, "update"); status.Code(e) != codes.Aborted {
		t.Fatal("stale edit accepted", e)
	}
	remove := edited
	remove.Steps = []PreparationStep{edited.Steps[0]}
	if _, e = update("1", remove, "update"); status.Code(e) != codes.FailedPrecondition {
		t.Fatal("started stage deleted", e)
	}
	if _, e = update("2", edited, "update"); status.Code(e) != codes.NotFound {
		t.Fatal("foreign edit allowed", e)
	}
	edited.Archived = true
	archived, e := update("1", edited, "archive")
	if e != nil {
		t.Fatal(e)
	}
	var state string
	db.QueryRow(`SELECT state FROM practice_reminders WHERE id=$1`, plan.ID).Scan(&state)
	if state != "dismissed" {
		t.Fatal("archived practice still scheduled")
	}
	archived.Archived = false
	restored, e := update("1", archived, "archive")
	if e != nil || restored.Archived {
		t.Fatal("restore failed", e)
	}
	var quota int
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota)
	if quota != 1 {
		t.Fatal("edit/archive/restore spent quota", quota)
	}
	// Archiving a completed path must not erase all-time completed statistics.
	second := uuid.NewString()
	if _, e = db.Exec(`INSERT INTO interviews(id,user_id,status) VALUES($1,1,'completed')`, second); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id) VALUES($1,0,$2,$3)`, p.ID, uuid.NewString(), second); e != nil {
		t.Fatal(e)
	}
	restored.Archived = true
	if _, e = update("1", restored, "archive"); e != nil {
		t.Fatal(e)
	}
	counts, e := repository.NewInterviewRepository(db).CountByStatus(ctx, "1")
	if e != nil || counts["completed"] != 1 || counts["in_progress"] != 0 {
		t.Fatal("archive changed completed history", counts, e)
	}
}

type deliveryScript struct {
	calls   int
	failure *PracticeDeliveryError
}

func (s *deliveryScript) Send(context.Context, PracticeMail) error {
	s.calls++
	if s.calls == 1 {
		return s.failure
	}
	return nil
}
func TestReminderRetriesKnownTemporaryFailureOnly(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			db := practiceTestDB(t)
			sender := &deliveryScript{failure: &PracticeDeliveryError{Err: errors.New("connection failed"), Retryable: !uncertain, Uncertain: uncertain}}
			s := NewPracticeStateService(db, nil, nil, nil, sender)
			now := time.Now()
			p := draftReminder()
			p.EmailEnabled = true
			p.LeadMinutes = 10
			p.ScheduledAt = now.Add(20 * time.Minute)
			raw, e := practiceReq(t, s, "1", "save", p, "")
			if e != nil {
				t.Fatal(e)
			}
			json.Unmarshal(raw, &p)
			if _, e = s.DispatchOne(context.Background(), now.Add(11*time.Minute)); e != nil {
				t.Fatal(e)
			}
			_, e = s.DispatchOne(context.Background(), now.Add(11*time.Minute+30*time.Second))
			if e != nil || sender.calls != 1 {
				t.Fatal("retry ignored backoff", e, sender.calls)
			}
			_, e = s.DispatchOne(context.Background(), now.Add(13*time.Minute))
			if e != nil {
				t.Fatal(e)
			}
			if uncertain {
				if sender.calls != 1 {
					t.Fatal("uncertain accepted mail duplicated")
				}
			} else {
				if sender.calls != 2 {
					t.Fatal("temporary failure never retried")
				}
				var n int
				db.QueryRow(`SELECT count(*) FROM practice_reminder_deliveries`).Scan(&n)
				if n != 1 {
					t.Fatal("delivery not recorded")
				}
			}
		})
	}
}
func TestReminderEmailCapacityIsShownBeforeSavingSixth(t *testing.T) {
	s := NewPracticeStateService(practiceTestDB(t), nil, nil, nil, &practiceSenderFake{})
	for i := 0; i < 6; i++ {
		p := draftReminder()
		p.EmailEnabled = true
		_, e := practiceReq(t, s, "1", "save", p, "")
		if i < 5 && e != nil {
			t.Fatal(e)
		}
		if i == 5 && status.Code(e) != codes.FailedPrecondition {
			t.Fatal("sixth email not rejected before promise", e)
		}
	}
}

func TestHistoricalStageReviewsKeepAuditButStopChargingQuota(t *testing.T) {
	db := practiceTestDB(t)
	ctx := context.Background()
	raw, e := os.ReadFile("../../migrations/000009_review_training.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`CREATE TABLE reports(id UUID PRIMARY KEY,interview_id UUID);`); e != nil {
		t.Fatal(e)
	}
	p, e := NewPreparationService(db, nil).create(ctx, "1", "paid", Preparation{Title: "Old path", VacancyKey: "legacy:review", Level: "junior", Steps: []PreparationStep{{Technology: "Python", Specialization: "backend"}}})
	if e != nil {
		t.Fatal(e)
	}
	source, child, nested := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{source, child, nested} {
		if _, e = db.Exec(`INSERT INTO interviews(id,user_id,status) VALUES($1,1,'completed')`, id); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec(`INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id) VALUES($1,0,$2,$3)`, p.ID, uuid.NewString(), source); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{source, child}, {child, nested}} {
		report := uuid.NewString()
		if _, e = db.Exec(`INSERT INTO reports(id,interview_id) VALUES($1,$2)`, report, pair[0]); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(`INSERT INTO review_training_requests(user_id,request_id,source_report_id,interview_id) VALUES(1,$1,$2,$3)`, uuid.NewString(), report, pair[1]); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(`INSERT INTO interview_quota_log(user_id,interview_id) VALUES(1,$1)`, pair[1]); e != nil {
			t.Fatal(e)
		}
	}
	raw, e = os.ReadFile("../../migrations/000012_restore_vacancy_review_membership.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 2; n++ {
		if _, e = db.Exec(string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	used, e := repository.NewInterviewRepository(db).CountCreatedSince(ctx, "1", time.Unix(0, 0))
	if e != nil || used != 1 {
		t.Fatal("historical double charges still count", used, e)
	}
	var audit, members int
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&audit)
	db.QueryRow(`SELECT count(*) FROM vacancy_preparation_attempts`).Scan(&members)
	if audit != 3 || members != 3 {
		t.Fatal("history deleted or membership missing", audit, members)
	}
}
