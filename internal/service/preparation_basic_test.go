package service

import (
	"context"
	"encoding/json"
	"fmt"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/repository"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync"
	"testing"
)

func TestBasicPreparationSharesLifetimeQuotaAndAllStagesCostOne(t *testing.T) {
	t.Setenv("FREE_TRAININGS_TOTAL", "3")
	db := practiceTestDB(t)
	ctx := context.Background()
	bank := multiTechBank{pools: map[string][]usecase.QuestionRef{}}
	for _, tech := range []string{"Go", "Redis"} {
		bank.pools[tech+"/junior/question"] = refs(tech, "junior", 20)
	}
	interview := NewInterviewService(repository.NewInterviewRepository(db), bank)
	service := NewPreparationService(db, interview)
	create := func(key string) (Preparation, error) {
		p := Preparation{Title: "Backend developer", VacancyKey: key, Level: "junior", Steps: []PreparationStep{{Technology: "Go", Specialization: "backend"}, {Technology: "Redis", Specialization: "backend"}}}
		raw, _ := json.Marshal(p)
		resp, err := service.Execute(ctx, &pb.VacancyPreparationRequest{UserId: "1", SubscriptionPlan: "free", Action: "create", PayloadJson: string(raw)})
		if err == nil {
			err = json.Unmarshal([]byte(resp.PayloadJson), &p)
		}
		return p, err
	}
	// Concurrent duplicate submissions must consume just one lifetime attempt.
	var wg sync.WaitGroup
	results := make(chan Preparation, 6)
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := create("manual:basic")
			if e != nil {
				errs <- e
			} else {
				results <- p
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var p Preparation
	for got := range results {
		if p.ID != "" && p.ID != got.ID {
			t.Fatal("duplicate preparation")
		}
		p = got
	}
	assertQuota := func(want int) {
		t.Helper()
		stats, e := interview.GetTrainingStats(ctx, "1", "free")
		if e != nil || stats.QuotaUsed != want || stats.Total != want {
			t.Fatalf("quota %+v, want %d, %v", stats, want, e)
		}
	}
	assertQuota(1)
	// A regular interview and a second path use the same three-attempt allowance.
	if _, e := interview.Create(ctx, entity.Interview{UserID: "1", Title: "Regular", Level: "junior", Specialization: "backend", TaskMode: "theory"}, []string{"Go"}, "free"); e != nil {
		t.Fatal(e)
	}
	if _, e := create("manual:second"); e != nil {
		t.Fatal(e)
	}
	assertQuota(3)
	if _, e := create("manual:over-limit"); status.Code(e) != codes.ResourceExhausted {
		t.Fatal("new preparation bypassed Basic quota", e)
	}
	// Both stages, repeats, and request retries remain available at the cap.
	start := func(step int, key string) string {
		t.Helper()
		res, e := service.Execute(ctx, &pb.VacancyPreparationRequest{UserId: "1", SubscriptionPlan: "free", Action: "prepare_step", PreparationId: p.ID, StepIndex: int32(step), RequestId: key})
		if e != nil {
			t.Fatal(e)
		}
		var out map[string]string
		if e = json.Unmarshal([]byte(res.PayloadJson), &out); e != nil {
			t.Fatal(e)
		}
		return out["interview_id"]
	}
	firstKey := uuid.NewString()
	first := start(0, firstKey)
	if got := start(0, firstKey); got != first {
		t.Fatal("retry recreated stage")
	}
	if got := start(0, uuid.NewString()); got != first {
		t.Fatal("active stage recreated")
	}
	second := start(1, uuid.NewString())
	if first == second {
		t.Fatal("different stages shared an interview")
	}
	if _, e := db.Exec(`UPDATE interviews SET status='completed' WHERE id=$1`, first); e != nil {
		t.Fatal(e)
	}
	retake := start(0, uuid.NewString())
	if retake == first {
		t.Fatal("retake not created")
	}
	assertQuota(3)
	// Scope checks stay enforced after opening Basic access.
	_, e := service.Execute(ctx, &pb.VacancyPreparationRequest{UserId: "2", SubscriptionPlan: "free", Action: "prepare_step", PreparationId: p.ID, RequestId: uuid.NewString()})
	if status.Code(e) != codes.NotFound {
		t.Fatal("foreign preparation accessible", e)
	}
	var count int
	if e = db.QueryRow(`SELECT count(*) FROM vacancy_preparations WHERE user_id=1`).Scan(&count); e != nil || count != 2 {
		t.Fatal(fmt.Sprint(count), e)
	}
}
