package service_test

import (
	"context"
	"fmt"
	"github.com/LimeOnTop/interverse-interview/internal/client"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/service"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"os"
	"testing"
	"time"
)

// Opt-in replay against a real question service. The interview repository is
// strictly in memory: no production accounts, sessions or quotas are written.
func TestMultiTechnologyLiveBank(t *testing.T) {
	target := os.Getenv("MULTITECH_TEST_QUESTION_SERVICE")
	if target == "" {
		t.Skip("set MULTITECH_TEST_QUESTION_SERVICE for read-only live-bank replay")
	}
	bank, err := client.NewQuestionClient(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"intern", "junior", "middle", "senior", "lead"} {
		t.Run(level, func(t *testing.T) {
			for repeat := 0; repeat < 3; repeat++ {
				repo := &memorySessionRepository{interview: entity.Interview{ID: "dry-run", UserID: "dry-run", Level: level, Specialization: "backend", TaskMode: "theory", Status: entity.StatusScheduled}, technologies: []string{"Go", "Kafka", "PostgreSQL", "Redis"}}
				svc := service.NewInterviewService(repo, bank)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				dto, err := svc.StartSession(ctx, "dry-run", "dry-run")
				cancel()
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				for _, q := range dto.Questions {
					counts[q.Technology]++
					source, err := bank.GetByID(context.Background(), q.QuestionID)
					if err != nil || source.Technology != q.Technology || source.Text != q.Text {
						t.Fatalf("incorrect source badge: %+v %v", q, err)
					}
				}
				low, high := len(dto.Questions), 0
				for _, tech := range repo.technologies {
					n := counts[tech]
					if n < 3 {
						t.Fatal("missing technology", counts)
					}
					low = min(low, n)
					high = max(high, n)
				}
				if high-low > 1 {
					t.Fatal("uneven", counts)
				}
				t.Logf("%s replay %d: %v", level, repeat, counts)
			}
		})
	}
}

type memorySessionRepository struct {
	usecase.InterviewRepository
	interview    entity.Interview
	technologies []string
	items        []entity.SessionItem
}

func (r *memorySessionRepository) GetByID(context.Context, string) (entity.Interview, error) {
	return r.interview, nil
}
func (r *memorySessionRepository) GetTechnologies(context.Context, string) ([]string, error) {
	return r.technologies, nil
}
func (r *memorySessionRepository) GetSessionItems(context.Context, string) ([]entity.SessionItem, error) {
	return r.items, nil
}
func (r *memorySessionRepository) SaveSessionItems(_ context.Context, items []entity.SessionItem) error {
	r.items = items
	for i := range r.items {
		r.items[i].ID = fmt.Sprint(i)
	}
	return nil
}
func (r *memorySessionRepository) Update(_ context.Context, value entity.Interview) (entity.Interview, error) {
	r.interview = value
	return value, nil
}
