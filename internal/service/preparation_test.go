package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/repository"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPreparationValidation(t *testing.T) {
	valid := func() Preparation {
		return Preparation{Title: "Python developer", VacancyKey: "hh:123", URL: "https://hh.ru/vacancy/123", Level: "junior", Steps: []PreparationStep{{Technology: "Python", Specialization: "backend"}}}
	}
	for _, tc := range []struct {
		name   string
		change func(*Preparation)
	}{{"duplicate", func(p *Preparation) {
		p.Steps = append(p.Steps, PreparationStep{Technology: " python ", Specialization: "backend"})
	}}, {"unsafe URL", func(p *Preparation) { p.URL = "javascript:alert(1)" }}, {"empty stack", func(p *Preparation) { p.Steps = nil }}, {"bad direction", func(p *Preparation) { p.Steps[0].Specialization = "other" }}, {"bad level", func(p *Preparation) { p.Level = "unknown" }}} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.change(&p)
			if validatePreparation(&p) == nil {
				t.Fatal("invalid preparation accepted")
			}
		})
	}
	p := valid()
	if err := validatePreparation(&p); err != nil {
		t.Fatal(err)
	}
}
func TestPreparationPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("PREPARATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PREPARATION_TEST_DATABASE_URL for isolated schema integration")
	}
	ctx := context.Background()
	admin, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "prep_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec(`CREATE SCHEMA ` + schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, e := sql.Open("postgres", dsn+sep+"search_path="+schema)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ddl := `CREATE TABLE interviews(id UUID PRIMARY KEY,user_id BIGINT,title VARCHAR(255),description TEXT,status TEXT,scheduled_at TIMESTAMP,created_at TIMESTAMP,updated_at TIMESTAMP,level TEXT,specialization TEXT,task_mode TEXT); CREATE TABLE interview_technologies(interview_id UUID REFERENCES interviews(id) ON DELETE CASCADE,technology_id TEXT); CREATE TABLE interview_quota_log(user_id BIGINT,interview_id UUID,created_at TIMESTAMP DEFAULT NOW());`
	if _, e = db.Exec(ddl); e != nil {
		t.Fatal(e)
	}
	migration, e := os.ReadFile("../../migrations/000008_vacancy_preparations.up.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(migration)); e != nil {
		t.Fatal(e)
	}
	s := NewPreparationService(db, nil)
	create := func(user string) Preparation {
		t.Helper()
		p := Preparation{Title: "Python developer", VacancyKey: "hh:123", Level: "junior", Steps: []PreparationStep{{Technology: "Python", Specialization: "backend"}, {Technology: "SQL", Specialization: "backend"}}}
		raw, _ := json.Marshal(p)
		resp, e := s.Execute(ctx, &pb.VacancyPreparationRequest{UserId: user, Action: "create", SubscriptionPlan: "paid", PayloadJson: string(raw)})
		if e != nil {
			t.Fatal(e)
		}
		json.Unmarshal([]byte(resp.PayloadJson), &p)
		return p
	}
	p := create("1")
	if dup := create("1"); dup.ID != p.ID {
		t.Fatal("duplicate vacancy created a second path")
	}
	if other := create("2"); other.ID == p.ID {
		t.Fatal("paths shared across users")
	}
	var quota int
	if e = db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota); e != nil || quota != 2 {
		t.Fatal("each path should consume exactly one quota unit", e, quota)
	}
	req := func(user, request string, idx int32) *pb.VacancyPreparationRequest {
		return &pb.VacancyPreparationRequest{UserId: user, Action: "prepare_step", PreparationId: p.ID, StepIndex: idx, RequestId: request, SubscriptionPlan: "paid"}
	}
	if _, e = s.Execute(ctx, req("2", uuid.NewString(), 0)); status.Code(e) != codes.NotFound {
		t.Fatal("foreign path accessible", e)
	}
	// Concurrent clicks with distinct request IDs must still share one active training.
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, e := s.Execute(ctx, req("1", uuid.NewString(), 0))
			if e != nil {
				errs <- e
				return
			}
			var data map[string]string
			json.Unmarshal([]byte(resp.PayloadJson), &data)
			ids <- data["interview_id"]
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	id := ""
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("duplicate active attempts")
		}
		id = got
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota)
	if quota != 2 {
		t.Fatalf("quota=%d want 2", quota)
	}
	// Completion permits a deliberate new attempt, but retrying its key does not.
	if _, e = db.Exec(`UPDATE interviews SET status='completed' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	key := uuid.NewString()
	one, e := s.Execute(ctx, req("1", key, 0))
	if e != nil {
		t.Fatal(e)
	}
	two, e := s.Execute(ctx, req("1", key, 0))
	if e != nil || one.PayloadJson != two.PayloadJson {
		t.Fatal("request not idempotent", e)
	}
	if _, e = s.Execute(ctx, req("1", key, 1)); status.Code(e) != codes.FailedPrecondition {
		t.Fatal("key reused for another step", e)
	}
	// A failed transaction never leaves quota or orphan interview rows.
	if _, e = db.Exec(`ALTER TABLE interview_technologies ADD CONSTRAINT reject_sql CHECK(technology_id <> 'SQL')`); e != nil {
		t.Fatal(e)
	}
	_, e = s.Execute(ctx, req("1", uuid.NewString(), 1))
	if e == nil {
		t.Fatal("expected forced database failure")
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&quota)
	if quota != 2 {
		t.Fatal("failed transaction consumed quota", quota)
	}
	list, e := s.Execute(ctx, &pb.VacancyPreparationRequest{UserId: "1", Action: "list"})
	if e != nil {
		t.Fatal(e)
	}
	var paths []Preparation
	if e = json.Unmarshal([]byte(list.PayloadJson), &paths); e != nil || len(paths) != 1 || len(paths[0].Attempts) != 2 {
		t.Fatal("path persistence/attempt history broken", fmt.Sprint(paths), e)
	}
	statsService := NewInterviewService(repository.NewInterviewRepository(db), nil)
	stats, err := statsService.GetTrainingStats(ctx, "1", "paid")
	if err != nil || stats.QuotaUsed != 1 || stats.Total != 1 {
		t.Fatalf("one path must be one unit: %+v %v", stats, err)
	}
	// At the daily cap existing path stages remain available, while a new path is rejected.
	if _, e = db.Exec(`ALTER TABLE interview_technologies DROP CONSTRAINT reject_sql; INSERT INTO interview_quota_log(user_id,interview_id) SELECT 1,gen_random_uuid() FROM generate_series(1,14)`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Execute(ctx, req("1", uuid.NewString(), 1)); e != nil {
		t.Fatal("existing path blocked by quota", e)
	}
	raw, _ := json.Marshal(Preparation{Title: "Other", VacancyKey: "hh:other", Level: "junior", Steps: []PreparationStep{{Technology: "Python", Specialization: "backend"}}})
	if _, e = s.Execute(ctx, &pb.VacancyPreparationRequest{UserId: "1", Action: "create", SubscriptionPlan: "paid", PayloadJson: string(raw)}); status.Code(e) != codes.ResourceExhausted {
		t.Fatal("new path bypassed daily cap", e)
	}

}
