package client

import (
	"context"
	"fmt"

	questionpb "github.com/LimeOnTop/interverse-contracts/question/gen"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QuestionClient struct {
	client questionpb.QuestionServiceClient
}

func NewQuestionClient(questionServiceURL string) (*QuestionClient, error) {
	conn, err := grpc.NewClient(questionServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to question service: %w", err)
	}

	return &QuestionClient{
		client: questionpb.NewQuestionServiceClient(conn),
	}, nil
}

func (c *QuestionClient) GetByTechnology(ctx context.Context, technology, difficulty, category string, limit int) ([]usecase.QuestionRef, error) {
	resp, err := c.client.GetQuestionsByTechnology(ctx, &questionpb.GetQuestionsByTechnologyRequest{
		Technology: technology,
		Difficulty: difficulty,
		Category:   category,
		Pagination: &questionpb.Pagination{
			Page:  1,
			Limit: int32(limit),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get questions by technology: %w", err)
	}

	if resp.Response != nil && !resp.Response.Success {
		return nil, fmt.Errorf("get questions by technology: %s", resp.Response.Error)
	}

	result := make([]usecase.QuestionRef, 0, len(resp.Questions))
	for _, question := range resp.Questions {
		result = append(result, usecase.QuestionRef{
			ID:         question.Id,
			Text:       question.Text,
			Category:   question.Category,
			Difficulty: question.Difficulty,
			Technology: question.Technology,
			Tags:       question.Tags,
			Options:    mapQuestionOptions(question.Options),
		})
	}

	return result, nil
}

func (c *QuestionClient) GetByID(ctx context.Context, id string) (usecase.QuestionRef, error) {
	resp, err := c.client.GetQuestion(ctx, &questionpb.GetQuestionRequest{
		QuestionId: id,
	})
	if err != nil {
		return usecase.QuestionRef{}, fmt.Errorf("get question: %w", err)
	}

	if resp.Response != nil && !resp.Response.Success {
		return usecase.QuestionRef{}, fmt.Errorf("get question: %s", resp.Response.Error)
	}

	question := resp.GetQuestion()
	if question == nil {
		return usecase.QuestionRef{}, fmt.Errorf("get question: empty response")
	}

	return usecase.QuestionRef{
		ID:         question.Id,
		Text:       question.Text,
		Category:   question.Category,
		Difficulty: question.Difficulty,
		Technology: question.Technology,
		Tags:       question.Tags,
		Options:    mapQuestionOptions(question.Options),
	}, nil
}

func mapQuestionOptions(options []*questionpb.QuestionOption) []usecase.QuestionOptionRef {
	result := make([]usecase.QuestionOptionRef, 0, len(options))
	for _, option := range options {
		result = append(result, usecase.QuestionOptionRef{
			Text:      option.GetText(),
			IsCorrect: option.GetIsCorrect(),
			SortOrder: int(option.GetSortOrder()),
		})
	}
	return result
}
