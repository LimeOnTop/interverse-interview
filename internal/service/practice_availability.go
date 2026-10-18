package service

import (
	"context"
	"errors"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PracticeAvailability struct {
	Available       bool     `json:"available"`
	AvailableLevels []string `json:"available_levels"`
}

func (s *InterviewService) practiceAvailability(ctx context.Context, p PracticePayload, includeLevels bool) (PracticeAvailability, error) {
	out := PracticeAvailability{AvailableLevels: []string{}}
	probe := Preparation{Title: "availability", VacancyKey: "availability", Level: p.Level, Steps: []PreparationStep{}}
	for _, t := range p.Technologies {
		probe.Steps = append(probe.Steps, PreparationStep{Technology: t, Specialization: p.Specialization})
	}
	if e := validatePreparation(&probe); e != nil {
		return out, e
	}
	if p.TaskMode != "theory" && p.TaskMode != "few" && p.TaskMode != "many" {
		return out, status.Error(codes.InvalidArgument, "Выберите формат задач")
	}
	levels := []string{p.Level}
	if includeLevels {
		levels = []string{"intern", "junior", "middle", "senior", "lead"}
	}
	checked := map[string]bool{}
	checkedResult := map[string]bool{}
	for _, level := range levels {
		difficulty := mapLevelToDifficulty(level)
		if checked[difficulty] {
			if checkedResult[difficulty] {
				out.AvailableLevels = append(out.AvailableLevels, level)
			}
			continue
		}
		checked[difficulty] = true
		if s == nil {
			checkedResult[difficulty] = true
			out.AvailableLevels = append(out.AvailableLevels, level)
			continue
		}
		_, _, err := s.collectSessionQuestions(ctx, p.Technologies, level, p.Specialization, p.TaskMode)
		if err == nil {
			checkedResult[difficulty] = true
			out.AvailableLevels = append(out.AvailableLevels, level)
			continue
		}
		var insufficient *usecase.InsufficientQuestionsError
		if !errors.As(err, &insufficient) {
			return out, status.Error(codes.Unavailable, "Не удалось проверить банк вопросов. Повторите проверку")
		}
	}
	for _, level := range out.AvailableLevels {
		if level == p.Level {
			out.Available = true
		}
	}
	return out, nil
}
