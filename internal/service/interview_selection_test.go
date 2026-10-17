package service

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/LimeOnTop/interverse-interview/internal/usecase"
)

func TestDifficultyFallbackOrderPrefersNearestLevel(t *testing.T) {
	cases := map[string][]string{
		"intern": {"intern", "junior", "middle", "senior"},
		"junior": {"junior", "intern", "middle", "senior"},
		"middle": {"middle", "junior", "senior", "intern"},
		"senior": {"senior", "middle", "junior", "intern"},
	}
	for requested, want := range cases {
		if got := difficultyFallbackOrder(requested); !reflect.DeepEqual(got, want) {
			t.Errorf("difficultyFallbackOrder(%q) = %v, want %v", requested, got, want)
		}
	}
}

func TestShuffledSessionOptionsMovesCorrectAnswer(t *testing.T) {
	options := []usecase.QuestionOptionRef{
		{Text: "correct", IsCorrect: true, SortOrder: 0},
		{Text: "b", SortOrder: 1},
		{Text: "c", SortOrder: 2},
		{Text: "d", SortOrder: 3},
	}

	positions := map[int]bool{}
	for i := 0; i < 200; i++ {
		shuffled := shuffledSessionOptions(options)
		if len(shuffled) != len(options) {
			t.Fatalf("got %d options, want %d", len(shuffled), len(options))
		}
		for index, option := range shuffled {
			if option.SortOrder != index {
				t.Fatalf("option %q has sort order %d at index %d", option.Text, option.SortOrder, index)
			}
			if option.IsCorrect {
				positions[index] = true
			}
		}
	}
	if len(positions) < 2 {
		t.Fatalf("correct option never moved from its original position: %v", positions)
	}
}

func TestInsufficientQuestionsErrorIsDetectable(t *testing.T) {
	var err error = &usecase.InsufficientQuestionsError{Technologies: []string{"React"}, Level: "junior", MinQuestions: 10, MinTasks: 1}
	var target *usecase.InsufficientQuestionsError
	if !errors.As(err, &target) || target.UserMessage() == "" {
		t.Fatal("expected InsufficientQuestionsError with a user message")
	}
}

func TestBalancedSelectionAndRedistribution(t *testing.T) {
	for run := 0; run < 100; run++ {
		pool := []usecase.QuestionRef{}
		for _, tech := range []string{"React", "JavaScript", "TypeScript"} {
			for i := 0; i < 30; i++ {
				pool = append(pool, usecase.QuestionRef{ID: fmt.Sprintf("%s-%d", tech, i), Technology: tech})
			}
		}
		selected := pickBalancedItems(pool, []string{"React", "JavaScript", "TypeScript", " React "}, 20)
		counts := map[string]int{}
		seen := map[string]bool{}
		for _, item := range selected {
			if seen[item.ID] {
				t.Fatal("duplicate question")
			}
			seen[item.ID] = true
			counts[item.Technology]++
		}
		if len(selected) != 20 {
			t.Fatal("incorrect count")
		}
		for _, n := range counts {
			if n < 6 || n > 7 {
				t.Fatalf("unbalanced: %v", counts)
			}
		}
	}
	pool := []usecase.QuestionRef{{ID: "r", Technology: "React"}}
	for i := 0; i < 20; i++ {
		pool = append(pool, usecase.QuestionRef{ID: fmt.Sprint(i), Technology: "JavaScript"})
	}
	selected := pickBalancedItems(pool, []string{"React", "JavaScript"}, 15)
	if len(selected) != 15 {
		t.Fatal("did not redistribute")
	}
	n := 0
	for _, item := range selected {
		if item.Technology == "React" {
			n++
		}
	}
	if n != 1 {
		t.Fatal("lost scarce technology")
	}
}

func TestTaskModesAllowEmptyBankAndRespectRanges(t *testing.T) {
	for _, mode := range []string{"theory", "few", "many"} {
		for pool := 0; pool <= 10; pool++ {
			for repeat := 0; repeat < 30; repeat++ {
				q, n, err := sessionItemCounts(50, pool, mode)
				if err != nil || q < 10 || q > 20 || n > pool {
					t.Fatalf("bad count %s %d: %d %d %v", mode, pool, q, n, err)
				}
				if mode == "theory" && n != 0 {
					t.Fatal("theory included task")
				}
				if mode == "few" && pool > 0 && (n < 1 || n > 3) {
					t.Fatal("few outside range")
				}
				if mode == "many" && pool >= 4 && (n < 4 || n > 6) {
					t.Fatal("many outside range")
				}
				if mode == "many" && pool < 4 && n != pool {
					t.Fatal("limited bank not used")
				}
			}
		}
	}
}
