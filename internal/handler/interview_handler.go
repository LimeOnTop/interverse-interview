package handler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "github.com/inter-verse/services/interview-service/gen"
	"github.com/inter-verse/services/interview-service/internal/models"
	"github.com/inter-verse/services/interview-service/internal/service"
	common "github.com/inter-verse/services/proto/gen"
)

type InterviewHandler struct {
	pb.UnimplementedInterviewServiceServer
	interviewService *service.InterviewService
}

func NewInterviewHandler(interviewService *service.InterviewService) *InterviewHandler {
	return &InterviewHandler{
		interviewService: interviewService,
	}
}

func (h *InterviewHandler) CreateInterview(ctx context.Context, req *pb.CreateInterviewRequest) (*pb.CreateInterviewResponse, error) {
	// Parse scheduled time
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return &pb.CreateInterviewResponse{
			Response: &common.Response{
				Success: false,
				Error:   "Invalid scheduled time format",
			},
		}, nil
	}

	interview := &models.Interview{
		CandidateID:    req.CandidateId,
		InterviewerID:  req.InterviewerId,
		Title:          req.Title,
		Description:    req.Description,
		ScheduledAt:    scheduledAt,
		Status:         "scheduled",
		Level:          req.Level,
		Specialization: req.Specialization,
	}

	createdInterview, err := h.interviewService.CreateInterview(interview, req.Technologies)
	if err != nil {
		return &pb.CreateInterviewResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	return &pb.CreateInterviewResponse{
		Response: &common.Response{
			Success: true,
			Message: "Interview created successfully",
		},
		Interview: &pb.Interview{
			Id:             createdInterview.ID,
			CandidateId:    createdInterview.CandidateID,
			InterviewerId:  createdInterview.InterviewerID,
			Title:          createdInterview.Title,
			Description:    createdInterview.Description,
			Status:         createdInterview.Status,
			ScheduledAt:    createdInterview.ScheduledAt.Format(time.RFC3339),
			CreatedAt:      createdInterview.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      createdInterview.UpdatedAt.Format(time.RFC3339),
			Technologies:   req.Technologies,
			Level:          createdInterview.Level,
			Specialization: createdInterview.Specialization,
		},
	}, nil
}

func (h *InterviewHandler) GetInterview(ctx context.Context, req *pb.GetInterviewRequest) (*pb.GetInterviewResponse, error) {
	interview, err := h.interviewService.GetInterviewByID(req.InterviewId)
	if err != nil {
		return &pb.GetInterviewResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	return &pb.GetInterviewResponse{
		Response: &common.Response{
			Success: true,
		},
		Interview: &pb.Interview{
			Id:             interview.ID,
			CandidateId:    interview.CandidateID,
			InterviewerId:  interview.InterviewerID,
			Title:          interview.Title,
			Description:    interview.Description,
			Status:         interview.Status,
			ScheduledAt:    interview.ScheduledAt.Format(time.RFC3339),
			CreatedAt:      interview.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      interview.UpdatedAt.Format(time.RFC3339),
			Level:          interview.Level,
			Specialization: interview.Specialization,
		},
	}, nil
}

func (h *InterviewHandler) GetInterviews(ctx context.Context, req *pb.GetInterviewsRequest) (*pb.GetInterviewsResponse, error) {
	limit := 10
	offset := 0

	if req.Pagination != nil {
		limit = int(req.Pagination.Limit)
		offset = int(req.Pagination.Page-1) * int(req.Pagination.Limit)
	}

	interviews, err := h.interviewService.GetInterviewsByInterviewer(req.InterviewerId, limit, offset)
	if err != nil {
		return &pb.GetInterviewsResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	var pbInterviews []*pb.Interview
	for _, interview := range interviews {
		fmt.Printf("DEBUG: Handler processing interview %s, candidate: %v\n", interview.ID, interview.Candidate != nil)
		if interview.Candidate != nil {
			fmt.Printf("DEBUG: Handler found candidate %s for interview %s\n", interview.Candidate.Name, interview.ID)
		}

		pbInterview := &pb.Interview{
			Id:             interview.ID,
			CandidateId:    interview.CandidateID,
			InterviewerId:  interview.InterviewerID,
			Title:          interview.Title,
			Description:    interview.Description,
			Status:         interview.Status,
			ScheduledAt:    interview.ScheduledAt.Format(time.RFC3339),
			CreatedAt:      interview.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      interview.UpdatedAt.Format(time.RFC3339),
			Level:          interview.Level,
			Specialization: interview.Specialization,
		}

		// Add candidate information if available
		if interview.Candidate != nil {
			pbInterview.Candidate = &pb.Candidate{
				Id:              interview.Candidate.ID,
				Name:            interview.Candidate.Name,
				Email:           interview.Candidate.Email,
				Phone:           interview.Candidate.Phone,
				ExperienceYears: int32(interview.Candidate.ExperienceYears),
				CreatedAt:       interview.Candidate.CreatedAt.Format(time.RFC3339),
				UpdatedAt:       interview.Candidate.UpdatedAt.Format(time.RFC3339),
			}
			fmt.Printf("DEBUG: Set pbInterview.Candidate for interview %s, is nil: %v\n", interview.ID, pbInterview.Candidate == nil)
		} else {
			fmt.Printf("DEBUG: No candidate to add to protobuf for interview %s\n", interview.ID)
		}

		pbInterviews = append(pbInterviews, pbInterview)
	}

	fmt.Printf("DEBUG: Returning %d interviews\n", len(pbInterviews))
	if len(pbInterviews) > 0 {
		fmt.Printf("DEBUG: First interview has candidate: %v\n", pbInterviews[0].Candidate != nil)
	}

	return &pb.GetInterviewsResponse{
		Response: &common.Response{
			Success: true,
		},
		Interviews: pbInterviews,
		Pagination: &common.Pagination{
			Page:  req.Pagination.Page,
			Limit: req.Pagination.Limit,
			Total: int32(len(pbInterviews)),
		},
	}, nil
}

func (h *InterviewHandler) UpdateInterview(ctx context.Context, req *pb.UpdateInterviewRequest) (*pb.UpdateInterviewResponse, error) {
	// Parse scheduled time
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return &pb.UpdateInterviewResponse{
			Response: &common.Response{
				Success: false,
				Error:   "Invalid scheduled time format",
			},
		}, nil
	}

	interview := &models.Interview{
		ID:             req.InterviewId,
		Title:          req.Title,
		Description:    req.Description,
		Status:         req.Status,
		ScheduledAt:    scheduledAt,
		Level:          req.Level,
		Specialization: req.Specialization,
	}

	updatedInterview, err := h.interviewService.UpdateInterview(interview)
	if err != nil {
		return &pb.UpdateInterviewResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	return &pb.UpdateInterviewResponse{
		Response: &common.Response{
			Success: true,
			Message: "Interview updated successfully",
		},
		Interview: &pb.Interview{
			Id:             updatedInterview.ID,
			CandidateId:    updatedInterview.CandidateID,
			InterviewerId:  updatedInterview.InterviewerID,
			Title:          updatedInterview.Title,
			Description:    updatedInterview.Description,
			Status:         updatedInterview.Status,
			ScheduledAt:    updatedInterview.ScheduledAt.Format(time.RFC3339),
			CreatedAt:      updatedInterview.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      updatedInterview.UpdatedAt.Format(time.RFC3339),
			Level:          updatedInterview.Level,
			Specialization: updatedInterview.Specialization,
		},
	}, nil
}

func (h *InterviewHandler) DeleteInterview(ctx context.Context, req *pb.DeleteInterviewRequest) (*common.Response, error) {
	err := h.interviewService.DeleteInterview(req.InterviewId)
	if err != nil {
		return &common.Response{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &common.Response{
		Success: true,
		Message: "Interview deleted successfully",
	}, nil
}

func (h *InterviewHandler) GetScheduledInterviews(ctx context.Context, req *pb.GetScheduledInterviewsRequest) (*pb.GetScheduledInterviewsResponse, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return &pb.GetScheduledInterviewsResponse{
			Response: &common.Response{
				Success: false,
				Error:   "Invalid date format",
			},
		}, nil
	}

	interviews, err := h.interviewService.GetScheduledInterviews(req.InterviewerId, date)
	if err != nil {
		return &pb.GetScheduledInterviewsResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	var pbInterviews []*pb.Interview
	for _, interview := range interviews {
		pbInterview := &pb.Interview{
			Id:             interview.ID,
			CandidateId:    interview.CandidateID,
			InterviewerId:  interview.InterviewerID,
			Title:          interview.Title,
			Description:    interview.Description,
			Status:         interview.Status,
			ScheduledAt:    interview.ScheduledAt.Format(time.RFC3339),
			CreatedAt:      interview.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      interview.UpdatedAt.Format(time.RFC3339),
			Level:          interview.Level,
			Specialization: interview.Specialization,
		}

		// Add candidate information if available
		if interview.Candidate != nil {
			pbInterview.Candidate = &pb.Candidate{
				Id:              interview.Candidate.ID,
				Name:            interview.Candidate.Name,
				Email:           interview.Candidate.Email,
				Phone:           interview.Candidate.Phone,
				ExperienceYears: int32(interview.Candidate.ExperienceYears),
				CreatedAt:       interview.Candidate.CreatedAt.Format(time.RFC3339),
				UpdatedAt:       interview.Candidate.UpdatedAt.Format(time.RFC3339),
			}
		}

		pbInterviews = append(pbInterviews, pbInterview)
	}

	return &pb.GetScheduledInterviewsResponse{
		Response: &common.Response{
			Success: true,
		},
		Interviews: pbInterviews,
	}, nil
}

func (h *InterviewHandler) GenerateQuestions(ctx context.Context, req *pb.GenerateQuestionsRequest) (*pb.GenerateQuestionsResponse, error) {
	questions, err := h.interviewService.GenerateQuestions(req.InterviewId, req.Technologies, req.Level, req.Specialization)
	if err != nil {
		return &pb.GenerateQuestionsResponse{
			Response: &common.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	var pbQuestions []*pb.Question
	for i, questionText := range questions {
		pbQuestions = append(pbQuestions, &pb.Question{
			Id:         strconv.Itoa(i + 1),
			Text:       questionText,
			Category:   "technical",
			Difficulty: req.Level,
			Technology: req.Technologies[0], // Use first technology
			Tags:       req.Technologies,
		})
	}

	return &pb.GenerateQuestionsResponse{
		Response: &common.Response{
			Success: true,
			Message: "Questions generated successfully",
		},
		Questions: pbQuestions,
	}, nil
}
