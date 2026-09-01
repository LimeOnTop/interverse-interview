package client

import (
	"context"
	"fmt"

	"github.com/LimeOnTop/interverse-interview/internal/usecase"
	questionpb "github.com/LimeOnTop/interverse-contracts/question/gen"
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
		})
	}

	return result, nil
}
