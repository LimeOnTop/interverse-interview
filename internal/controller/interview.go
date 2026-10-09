package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/apperr"
	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
)

type InterviewController struct {
	pb.UnimplementedInterviewServiceServer
	interview usecase.Interview
}

func NewInterviewController(interview usecase.Interview) *InterviewController {
	return &InterviewController{interview: interview}
}

func (c *InterviewController) CreateInterview(ctx context.Context, req *pb.CreateInterviewRequest) (*pb.CreateInterviewResponse, error) {
	scheduledAt, err := parseScheduledAt(req.ScheduledAt)
	if err != nil {
		return &pb.CreateInterviewResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	created, err := c.interview.Create(ctx, entity.Interview{
		UserID:         req.UserId,
		Title:          req.Title,
		Description:    req.Description,
		Status:         entity.StatusScheduled,
		ScheduledAt:    scheduledAt,
		Level:          req.Level,
		Specialization: req.Specialization,
	}, req.Technologies, req.GetSubscriptionPlan())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.CreateInterviewResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	return &pb.CreateInterviewResponse{
		Response:  &pb.Response{Success: true, Message: "Training session created successfully"},
		Interview: toProtoInterview(created),
	}, nil
}

func (c *InterviewController) GetTrainingStats(ctx context.Context, req *pb.GetTrainingStatsRequest) (*pb.GetTrainingStatsResponse, error) {
	stats, err := c.interview.GetTrainingStats(ctx, req.GetUserId(), req.GetSubscriptionPlan())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.GetTrainingStatsResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	resetsAt := ""
	if stats.QuotaResetsAt != nil {
		resetsAt = stats.QuotaResetsAt.UTC().Format(time.RFC3339)
	}
	return &pb.GetTrainingStatsResponse{
		Response:      &pb.Response{Success: true},
		QuotaUsed:     int32(stats.QuotaUsed),
		QuotaLimit:    int32(stats.QuotaLimit),
		QuotaPeriod:   stats.QuotaPeriod,
		QuotaResetsAt: resetsAt,
		Total:         int32(stats.Total),
		Completed:     int32(stats.Completed),
		InProgress:    int32(stats.InProgress),
		Scheduled:     int32(stats.Scheduled),
	}, nil
}

func (c *InterviewController) GetInterview(ctx context.Context, req *pb.GetInterviewRequest) (*pb.GetInterviewResponse, error) {
	interview, err := c.interview.GetByID(ctx, req.InterviewId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.GetInterviewResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	return &pb.GetInterviewResponse{
		Response:  &pb.Response{Success: true},
		Interview: toProtoInterview(interview),
	}, nil
}

func (c *InterviewController) GetInterviews(ctx context.Context, req *pb.GetInterviewsRequest) (*pb.GetInterviewsResponse, error) {
	limit := int64(10)
	offset := int64(0)

	if req.Pagination != nil {
		limit = int64(req.Pagination.Limit)
		offset = int64(req.Pagination.Page-1) * int64(req.Pagination.Limit)
	}

	interviews, err := c.interview.GetByUser(ctx, req.UserId, req.Status, limit, offset)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.GetInterviewsResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	return &pb.GetInterviewsResponse{
		Response:   &pb.Response{Success: true},
		Interviews: toProtoInterviews(interviews),
		Pagination: &pb.Pagination{
			Page:  req.Pagination.GetPage(),
			Limit: req.Pagination.GetLimit(),
			Total: int32(len(interviews)),
		},
	}, nil
}

func (c *InterviewController) UpdateInterview(ctx context.Context, req *pb.UpdateInterviewRequest) (*pb.UpdateInterviewResponse, error) {
	existing, err := c.interview.GetByID(ctx, req.InterviewId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.UpdateInterviewResponse{
			Response: &pb.Response{Success: false, Error: fmt.Sprintf("failed to get existing interview: %v", err)},
		}, nil
	}

	scheduledAt := existing.ScheduledAt
	if req.ScheduledAt != "" {
		scheduledAt, err = parseScheduledAt(req.ScheduledAt)
		if err != nil {
			return &pb.UpdateInterviewResponse{
				Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
			}, nil
		}
	}

	status := req.Status
	if status == "" {
		status = existing.Status
	}

	updated, err := c.interview.Update(ctx, entity.Interview{
		ID:             req.InterviewId,
		UserID:         existing.UserID,
		Title:          req.Title,
		Description:    req.Description,
		Status:         status,
		ScheduledAt:    scheduledAt,
		Level:          req.Level,
		Specialization: req.Specialization,
	}, req.Technologies)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.UpdateInterviewResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	return &pb.UpdateInterviewResponse{
		Response:  &pb.Response{Success: true, Message: "Training session updated successfully"},
		Interview: toProtoInterview(updated),
	}, nil
}

func (c *InterviewController) DeleteInterview(ctx context.Context, req *pb.DeleteInterviewRequest) (*pb.Response, error) {
	err := c.interview.Delete(ctx, req.InterviewId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.Response{Success: false, Error: apperr.Message(err, "request failed")}, nil
	}

	return &pb.Response{Success: true, Message: "Training session deleted successfully"}, nil
}

func (c *InterviewController) GetScheduledInterviews(ctx context.Context, req *pb.GetScheduledInterviewsRequest) (*pb.GetScheduledInterviewsResponse, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return &pb.GetScheduledInterviewsResponse{
			Response: &pb.Response{Success: false, Error: "Invalid date format"},
		}, nil
	}

	interviews, err := c.interview.GetScheduled(ctx, req.UserId, date)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.GetScheduledInterviewsResponse{
			Response: &pb.Response{Success: false, Error: apperr.Message(err, "request failed")},
		}, nil
	}

	return &pb.GetScheduledInterviewsResponse{
		Response:   &pb.Response{Success: true},
		Interviews: toProtoInterviews(interviews),
	}, nil
}

func (c *InterviewController) StartSession(ctx context.Context, req *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	content, err := c.interview.StartSession(ctx, req.InterviewId, req.UserId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.StartSessionResponse{
			Response: &pb.Response{Success: false, Error: sessionError(err)},
		}, nil
	}

	return &pb.StartSessionResponse{
		Response:  &pb.Response{Success: true, Message: "Training session started successfully"},
		Interview: toProtoInterview(content.Interview),
		Questions: toProtoSessionItems(content.Questions),
		Tasks:     toProtoSessionItems(content.Tasks),
	}, nil
}

func (c *InterviewController) GetSessionContent(ctx context.Context, req *pb.GetSessionContentRequest) (*pb.GetSessionContentResponse, error) {
	content, err := c.interview.GetSessionContent(ctx, req.InterviewId, req.UserId)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("context canceled: %w", err)
		}
		return &pb.GetSessionContentResponse{
			Response: &pb.Response{Success: false, Error: sessionError(err)},
		}, nil
	}

	return &pb.GetSessionContentResponse{
		Response:  &pb.Response{Success: true},
		Interview: toProtoInterview(content.Interview),
		Questions: toProtoSessionItems(content.Questions),
		Tasks:     toProtoSessionItems(content.Tasks),
	}, nil
}

func sessionError(err error) string {
	var insufficient *usecase.InsufficientQuestionsError
	if errors.As(err, &insufficient) {
		return insufficient.UserMessage()
	}

	switch {
	case errors.Is(err, usecase.ErrNotFound):
		return apperr.Message(err, "Тренировка не найдена")
	case strings.Contains(err.Error(), "forbidden"):
		return apperr.Message(err, "Нет доступа к тренировке")
	case strings.Contains(err.Error(), "already completed"):
		return apperr.Message(err, "Тренировка уже завершена")
	case strings.Contains(err.Error(), "technologies are required"):
		return apperr.Message(err, "Добавьте технологии для тренировки")
	default:
		return apperr.Message(err, "Не удалось загрузить тренировку")
	}
}

func parseScheduledAt(value string) (time.Time, error) {
	if value == "" {
		return time.Now(), nil
	}

	scheduledAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid scheduled time format: %w", err)
	}

	return scheduledAt, nil
}

func toProtoInterview(interview usecase.InterviewDTO) *pb.Interview {
	return &pb.Interview{
		Id:             interview.ID,
		UserId:         interview.UserID,
		Title:          interview.Title,
		Description:    interview.Description,
		Status:         interview.Status,
		ScheduledAt:    interview.ScheduledAt.Format(time.RFC3339),
		CreatedAt:      interview.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      interview.UpdatedAt.Format(time.RFC3339),
		Level:          interview.Level,
		Specialization: interview.Specialization,
		Technologies:   interview.Technologies,
	}
}

func toProtoInterviews(interviews []usecase.InterviewDTO) []*pb.Interview {
	result := make([]*pb.Interview, 0, len(interviews))
	for _, interview := range interviews {
		result = append(result, toProtoInterview(interview))
	}
	return result
}

func toProtoSessionItems(items []usecase.SessionItemDTO) []*pb.SessionItem {
	result := make([]*pb.SessionItem, 0, len(items))
	for _, item := range items {
		result = append(result, &pb.SessionItem{
			Id:         item.ID,
			QuestionId: item.QuestionID,
			ItemType:   item.ItemType,
			SortOrder:  int32(item.SortOrder),
			Text:       item.Text,
			Technology: item.Technology,
			Difficulty: item.Difficulty,
			Category:   item.Category,
			Options:    item.Options,
		})
	}
	return result
}
