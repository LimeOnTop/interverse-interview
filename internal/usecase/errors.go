package usecase

import (
	"errors"
	"fmt"
	"strings"
)

// InsufficientQuestionsError means the question bank has too little material
// for the selected technologies and level to build a session.
type InsufficientQuestionsError struct {
	Technologies []string
	Level        string
	Questions    int
	Tasks        int
	MinQuestions int
	MinTasks     int
}

func (e *InsufficientQuestionsError) Error() string {
	return fmt.Sprintf(
		"not enough questions (level=%s, technologies=%s): questions=%d/%d, tasks=%d/%d",
		e.Level, strings.Join(e.Technologies, ", "),
		e.Questions, e.MinQuestions, e.Tasks, e.MinTasks,
	)
}

// UserMessage is safe to show to the end user.
func (e *InsufficientQuestionsError) UserMessage() string {
	return fmt.Sprintf(
		"По выбранным технологиям (%s) для уровня %s пока недостаточно материалов: нужно минимум %d вопросов и %d задач, найдено %d и %d. Выберите другие технологии или уровень.",
		strings.Join(e.Technologies, ", "), e.Level,
		e.MinQuestions, e.MinTasks, e.Questions, e.Tasks,
	)
}

var ErrNotFound = errors.New("not found")
