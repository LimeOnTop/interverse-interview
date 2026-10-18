package service

import (
	"context"
	"database/sql"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/repository"
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

func TestReviewTrainingPostgres(t *testing.T) {
	dsn := os.Getenv("PREPARATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	admin, e := sql.Open("postgres", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "review_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	ddl := `CREATE TABLE interviews(id UUID PRIMARY KEY,user_id BIGINT,title VARCHAR(255),description TEXT,status TEXT,scheduled_at TIMESTAMP,created_at TIMESTAMP,updated_at TIMESTAMP,level TEXT,specialization TEXT,task_mode TEXT); CREATE TABLE interview_technologies(interview_id UUID REFERENCES interviews(id) ON DELETE CASCADE,technology_id TEXT);CREATE TABLE interview_quota_log(user_id BIGINT,interview_id UUID,created_at TIMESTAMP DEFAULT now());`
	if _, e = db.Exec(ddl); e != nil {
		t.Fatal(e)
	}
	for _, file := range []string{"000003_training_sessions.up.sql", "000004_session_item_options.up.sql", "000008_vacancy_preparations.up.sql", "000009_review_training.up.sql"} {
		raw, e := os.ReadFile("../../migrations/" + file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	repo := repository.NewInterviewRepository(db)
	is := NewInterviewService(repo, nil)
	s := NewReviewTrainingService(db, is)
	source, e := repo.Create(ctx, entity.Interview{UserID: "1", Title: "Fixture", Status: "completed", Level: "junior", Specialization: "frontend", TaskMode: "few", ScheduledAt: time.Now()})
	if e != nil {
		t.Fatal(e)
	}
	wrong, right, task := uuid.NewString(), uuid.NewString(), uuid.NewString()
	opts := []entity.SessionOption{{Text: "correct", IsCorrect: true}, {Text: "wrong"}}
	if e = repo.SaveSessionItems(ctx, []entity.SessionItem{{ID: wrong, InterviewID: source.ID, QuestionID: uuid.NewString(), ItemType: "question", SortOrder: 1, Text: "Targeted question", Technology: "React", Options: opts}, {ID: right, InterviewID: source.ID, ItemType: "question", SortOrder: 2, Text: "Do not repeat this correct question", Technology: "React", Options: opts}, {ID: task, InterviewID: source.ID, ItemType: "task", SortOrder: 1, Text: "Targeted task", Technology: "React"}}); e != nil {
		t.Fatal(e)
	}
	request := func() *pb.ReviewTrainingRequest {
		return &pb.ReviewTrainingRequest{UserId: "1", SourceInterviewId: source.ID, SourceReportId: uuid.NewString(), StepIds: []string{wrong, task}, RequestId: uuid.NewString(), SubscriptionPlan: "paid"}
	}
	r := request()
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, e := s.Create(ctx, r)
			if e != nil {
				t.Error(e)
				return
			}
			if len(got.Questions) != 1 || len(got.Tasks) != 1 || got.Questions[0].Text != "Targeted question" || got.Tasks[0].Text != "Targeted task" {
				t.Errorf("wrong selection %+v", got)
			}
			ids <- got.Interview.ID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("duplicate training on retry")
		}
	}
	var count int
	if e = db.QueryRow(`SELECT count(*) FROM interview_quota_log WHERE interview_id=$1`, first).Scan(&count); e != nil || count != 1 {
		t.Fatal("quota must be charged once", e, count)
	}
	selected, _ := repo.GetSessionItems(ctx, first)
	if len(selected) != 2 || !selected[0].Options[0].IsCorrect {
		t.Fatal("correct option metadata lost")
	}
	bad := request()
	bad.UserId = "2"
	if _, e = s.Create(ctx, bad); status.Code(e) != codes.NotFound {
		t.Fatal("foreign source allowed", e)
	}
	bad = request()
	bad.SubscriptionPlan = "free"
	if _, e = s.Create(ctx, bad); status.Code(e) != codes.PermissionDenied {
		t.Fatal("Basic allowed", e)
	}
	bad = request()
	bad.StepIds = []string{uuid.NewString()}
	if _, e = s.Create(ctx, bad); status.Code(e) != codes.FailedPrecondition {
		t.Fatal("foreign step allowed", e)
	}
	if _, e = db.Exec(`INSERT INTO interview_quota_log(user_id,interview_id) SELECT 1,gen_random_uuid() FROM generate_series(1,15)`); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, request()); status.Code(e) != codes.ResourceExhausted {
		t.Fatal("quota bypass", e)
	}
	// A stage retake remains inside its paid-for preparation, even at the daily cap.
	prepID := uuid.NewString()
	if _, e = db.Exec(`INSERT INTO vacancy_preparations(id,user_id,vacancy_key,payload) VALUES($1,1,'fixture','{}');`, prepID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO vacancy_preparation_attempts(preparation_id,step_index,request_id,interview_id) VALUES($1,0,$2,$3)`, prepID, uuid.NewString(), source.ID); e != nil {
		t.Fatal(e)
	}
	var before, after int
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&before)
	pathRequest := request()
	retake, e := s.Create(ctx, pathRequest)
	if e != nil {
		t.Fatal("stage retake blocked at quota cap", e)
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&after)
	if after != before {
		t.Fatal("stage retake charged another attempt")
	}
	var parent string
	var stage int
	if e = db.QueryRow(`SELECT preparation_id::text,step_index FROM vacancy_preparation_attempts WHERE interview_id=$1`, retake.Interview.ID).Scan(&parent, &stage); e != nil || parent != prepID || stage != 0 {
		t.Fatal("retake lost stage membership", e)
	}
	again, e := s.Create(ctx, pathRequest)
	if e != nil || again.Interview.ID != retake.Interview.ID {
		t.Fatal("retake retry not idempotent", e)
	}
	// A retake of a retake must preserve membership too.
	nextItems, _ := repo.GetSessionItems(ctx, retake.Interview.ID)
	nested := request()
	nested.SourceInterviewId = retake.Interview.ID
	nested.StepIds = []string{nextItems[0].ID}
	nestedResult, e := s.Create(ctx, nested)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(`SELECT preparation_id::text FROM vacancy_preparation_attempts WHERE interview_id=$1`, nestedResult.Interview.ID).Scan(&parent); e != nil || parent != prepID {
		t.Fatal("nested retake lost membership", e)
	}
	db.QueryRow(`SELECT count(*) FROM interview_quota_log`).Scan(&after)
	if after != before {
		t.Fatal("nested retake charged quota")
	}

}
