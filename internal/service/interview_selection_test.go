package service

import (
	"errors"
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
