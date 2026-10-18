package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type practiceSenderFake struct {
	mu    sync.Mutex
	mails []PracticeMail
	fail  bool
}

func (f *practiceSenderFake) Send(_ context.Context, m PracticeMail) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mails = append(f.mails, m)
	if f.fail {
		return errors.New("smtp unavailable")
	}
	return nil
}
func practiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PREPARATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PREPARATION_TEST_DATABASE_URL")
	}
	admin, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "reminder_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec(`CREATE SCHEMA ` + schema); e != nil {
		t.Fatal(e)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, e := sql.Open("postgres", dsn+sep+"search_path="+schema)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	ddl := `CREATE TABLE interviews(id UUID PRIMARY KEY,user_id BIGINT,title TEXT,description TEXT,status TEXT,scheduled_at TIMESTAMP,created_at TIMESTAMP,updated_at TIMESTAMP,level TEXT,specialization TEXT,task_mode TEXT);CREATE TABLE interview_technologies(interview_id UUID,technology_id TEXT);CREATE TABLE interview_quota_log(user_id BIGINT,interview_id UUID,created_at TIMESTAMP DEFAULT NOW());`
	if _, e = db.Exec(ddl); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("../../migrations/000008_vacancy_preparations.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("../../migrations/000010_practice_reminders.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("../../migrations/000011_practice_delivery_retry.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(raw)); e != nil {
		t.Fatal(e)
	}
	return db
}
func practiceReq(t *testing.T, s *PracticeStateService, user, action string, p any, plan string) ([]byte, error) {
	t.Helper()
	raw, _ := json.Marshal(p)
	out, e := s.Execute(context.Background(), &pb.PracticeStateRequest{UserId: user, Action: action, PayloadJson: string(raw), SubscriptionPlan: plan})
	if e != nil {
		return nil, e
	}
	return []byte(out.PayloadJson), nil
}
func draftReminder() PracticeReminder {
	return PracticeReminder{Kind: "draft", TargetID: uuid.NewString(), ScheduledAt: time.Now().Add(time.Hour), Timezone: "Europe/Moscow", Payload: PracticePayload{Technologies: []string{"Python"}, Specialization: "backend", Level: "junior", TaskMode: "theory"}}
}
func TestPracticeScheduleValidation(t *testing.T) {
	p := draftReminder()
	now := time.Now()
	if e := validateSchedule(p, now); e != nil {
		t.Fatal(e)
	}
	p.ScheduledAt = now.Add(-time.Second)
	if validateSchedule(p, now) == nil {
		t.Fatal("past accepted")
	}
	p.ScheduledAt = now.Add(5 * time.Minute)
	p.EmailEnabled = true
	p.LeadMinutes = 10
	if validateSchedule(p, now) == nil {
		t.Fatal("mail would be sent in past")
	}
	p.LeadMinutes = 0
	p.Timezone = "unknown/zone"
	if validateSchedule(p, now) == nil {
		t.Fatal("invalid zone accepted")
	}
}
func TestPracticePostgresLifecycle(t *testing.T) {
	db := practiceTestDB(t)
	s := NewPracticeStateService(db, nil, nil, nil, &practiceSenderFake{})
	ctx := context.Background()
	p := draftReminder()
	raw, e := practiceReq(t, s, "1", "save", p, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &p)
	var quota int
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota)
	if quota != 0 {
		t.Fatal("planning consumed quota")
	}
	old := p
	p.ScheduledAt = p.ScheduledAt.Add(time.Hour)
	raw, e = practiceReq(t, s, "1", "save", p, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &p)
	if p.ID != old.ID || p.Revision != old.Revision+1 {
		t.Fatal("reschedule duplicated reminder")
	}
	if _, e = practiceReq(t, s, "1", "save", old, ""); status.Code(e) != codes.Aborted {
		t.Fatal("stale version accepted", e)
	}
	if _, e = practiceReq(t, s, "2", "start", p, "free"); status.Code(e) != codes.NotFound {
		t.Fatal("foreign reminder started", e)
	}
	// Two simultaneous start clicks create one training and spend one quota unit.
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, e := practiceReq(t, s, "1", "start", p, "free")
			if e != nil {
				errs <- e
				return
			}
			ids <- string(raw)
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	first := ""
	for id := range ids {
		if first != "" && id != first {
			t.Fatal("duplicate starts")
		}
		first = id
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota)
	if quota != 1 {
		t.Fatalf("start quota=%d", quota)
	}
	// Fill Basic allowance: scheduling is still free, starting checks current quota.
	if _, e = db.ExecContext(ctx, `INSERT INTO interview_quota_log(user_id,interview_id) SELECT 1,gen_random_uuid() FROM generate_series(1,2)`); e != nil {
		t.Fatal(e)
	}
	next := draftReminder()
	raw, e = practiceReq(t, s, "1", "save", next, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &next)
	if _, e = practiceReq(t, s, "1", "start", next, "free"); status.Code(e) != codes.ResourceExhausted {
		t.Fatal("quota bypass", e)
	}
	if _, e = practiceReq(t, s, "1", "dismiss", next, ""); e != nil {
		t.Fatal(e)
	}
	var interviews int
	db.QueryRow(`SELECT count(*) FROM interviews`).Scan(&interviews)
	if interviews != 1 {
		t.Fatal("failed start left an interview")
	}
}
func TestPracticePostgresMailRevisionAndFailure(t *testing.T) {
	db := practiceTestDB(t)
	sender := &practiceSenderFake{}
	s := NewPracticeStateService(db, nil, nil, nil, sender)
	now := time.Now()
	p := draftReminder()
	p.ScheduledAt = now.Add(20 * time.Minute)
	p.EmailEnabled = true
	p.LeadMinutes = 10
	raw, e := practiceReq(t, s, "1", "save", p, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &p)
	p.ScheduledAt = now.Add(2 * time.Hour)
	raw, e = practiceReq(t, s, "1", "save", p, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &p)
	if sent, e := s.DispatchOne(context.Background(), now.Add(11*time.Minute)); e != nil || sent {
		t.Fatal("old schedule sent", e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.DispatchOne(context.Background(), now.Add(111*time.Minute))
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(sender.mails) != 1 || !sender.mails[0].ScheduledAt.Equal(p.ScheduledAt) {
		t.Fatal("duplicate or stale mail", sender.mails)
	}
	// A failed SMTP delivery is visible and is never blindly retried.
	bad := draftReminder()
	bad.ScheduledAt = now.Add(time.Hour)
	bad.EmailEnabled = true
	raw, e = practiceReq(t, s, "2", "save", bad, "")
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(raw, &bad)
	sender.fail = true
	if _, e = s.DispatchOne(context.Background(), now.Add(61*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if sent, e := s.DispatchOne(context.Background(), now.Add(62*time.Minute)); e != nil || sent {
		t.Fatal("SMTP failure retried", e)
	}
	var failed bool
	db.QueryRow(`SELECT email_failed FROM practice_reminders WHERE id=$1`, bad.ID).Scan(&failed)
	if !failed {
		t.Fatal("delivery failure hidden")
	}
}
func TestNewsPostgresAccountIsolation(t *testing.T) {
	s := NewPracticeStateService(practiceTestDB(t), nil, nil, nil, nil)
	if _, e := practiceReq(t, s, "1", "read_news", map[string]any{"ids": []string{PublishedNews[0]}}, ""); e != nil {
		t.Fatal(e)
	}
	one, _ := practiceReq(t, s, "1", "news", nil, "")
	two, _ := practiceReq(t, s, "2", "news", nil, "")
	if !strings.Contains(string(one), PublishedNews[0]) || strings.Contains(string(two), PublishedNews[0]) {
		t.Fatal("read state shared between accounts")
	}
	if _, e := practiceReq(t, s, "1", "read_news", map[string]any{"ids": []string{"future-invented-news"}}, ""); status.Code(e) != codes.InvalidArgument {
		t.Fatal("unknown news accepted")
	}
}
