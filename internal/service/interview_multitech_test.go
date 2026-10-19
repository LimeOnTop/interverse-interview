package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"reflect"
	"testing"
)

type multiTechBank struct {
	pools map[string][]usecase.QuestionRef
}

func (b multiTechBank) GetByTechnology(_ context.Context, tech, level, category string, limit int) ([]usecase.QuestionRef, error) {
	items := b.pools[tech+"/"+level+"/"+category]
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
func (b multiTechBank) GetByID(context.Context, string) (usecase.QuestionRef, error) {
	return usecase.QuestionRef{}, nil
}
func refs(tech, level string, n int) []usecase.QuestionRef {
	out := []usecase.QuestionRef{}
	for i := 0; i < n; i++ {
		out = append(out, usecase.QuestionRef{ID: fmt.Sprintf("%s/%s/%d", tech, level, i), Technology: tech, Difficulty: level, Category: "question"})
	}
	return out
}
func TestEverySelectedTechnologyHasThreeBalancedQuestions(t *testing.T) {
	for _, n := range []int{1, 2, 4, 7, 12} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			techs := []string{}
			bank := multiTechBank{pools: map[string][]usecase.QuestionRef{}}
			for i := 0; i < n; i++ {
				tech := fmt.Sprintf("tech-%d", i)
				techs = append(techs, tech)
				bank.pools[tech+"/junior/question"] = refs(tech, "junior", 30)
			}
			svc := NewInterviewService(nil, bank)
			for run := 0; run < 100; run++ {
				questions, _, err := svc.collectSessionQuestions(context.Background(), techs, "junior", "backend", "theory")
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				seen := map[string]bool{}
				for _, q := range questions {
					if seen[q.ID] {
						t.Fatal("duplicate")
					}
					seen[q.ID] = true
					counts[q.Technology]++
				}
				low, high := len(questions), 0
				for _, tech := range techs {
					count := counts[tech]
					if count < 3 {
						t.Fatalf("technology missing: %v", counts)
					}
					if count < low {
						low = count
					}
					if count > high {
						high = count
					}
				}
				if high-low > 1 {
					t.Fatalf("uneven questions: %v", counts)
				}
			}
		})
	}
}
func TestThinTechnologyLimitsTotalWithoutLosingItsQuestions(t *testing.T) {
	bank := multiTechBank{pools: map[string][]usecase.QuestionRef{"Go/junior/question": refs("Go", "junior", 80), "Redis/junior/question": refs("Redis", "junior", 3)}}
	s := NewInterviewService(nil, bank)
	for i := 0; i < 50; i++ {
		questions, _, e := s.collectSessionQuestions(context.Background(), []string{"Go", "Redis"}, "junior", "backend")
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		for _, q := range questions {
			counts[q.Technology]++
		}
		if counts["Redis"] != 3 || counts["Go"] < 3 || counts["Go"] > 4 {
			t.Fatal(counts)
		}
	}
}
func TestMissingTechnologyRejectedBeforeAnyRepositoryOrQuotaAccess(t *testing.T) {
	s := NewInterviewService(nil, multiTechBank{pools: map[string][]usecase.QuestionRef{"Go/junior/question": refs("Go", "junior", 30)}})
	_, err := s.Create(context.Background(), entity.Interview{UserID: "1", Level: "junior", Specialization: "backend"}, []string{"Go", "Kafka"}, "free")
	var shortage *usecase.InsufficientQuestionsError
	if !errors.As(err, &shortage) || !reflect.DeepEqual(shortage.Technologies, []string{"Kafka"}) || shortage.MinQuestions != 3 {
		t.Fatal(err)
	}
	// nil repository deliberately panics if Create reaches quota or persistence.
}
func TestFallbackFillsMinimumWithoutBorrowingTechnologyOrDuplicating(t *testing.T) {
	bank := multiTechBank{pools: map[string][]usecase.QuestionRef{
		"Redis/junior/question": refs("Redis", "junior", 2), "Redis/intern/question": refs("Redis", "intern", 5),
		"Kafka/junior/question": refs("Go", "junior", 30),
	}}
	s := NewInterviewService(nil, bank)
	got, err := s.fetchByTechnology(context.Background(), "Redis", "junior", "question", 80)
	if err != nil || len(got) != 3 || got[0].Difficulty != "junior" || got[2].Difficulty != "intern" {
		t.Fatal(got, err)
	}
	wrong, err := s.fetchByTechnology(context.Background(), "Kafka", "junior", "question", 80)
	if err != nil || len(wrong) != 0 {
		t.Fatal("wrong technology accepted", wrong, err)
	}
}
func TestAliasesDoNotCountSameTechnologyTwice(t *testing.T) {
	got := uniqueTechnologies([]string{"Go", " golang ", "go", "Postgres", "PostgreSQL", "redis", "Kafka"})
	if !reflect.DeepEqual(got, []string{"Go", "PostgreSQL", "Redis", "Kafka"}) {
		t.Fatal(got)
	}
}
