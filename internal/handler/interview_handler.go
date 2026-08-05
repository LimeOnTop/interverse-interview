package handler

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	"github.com/LimeOnTop/interverse-interview/internal/models"
	"github.com/LimeOnTop/interverse-interview/internal/service"
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
	fmt.Printf("DEBUG: CreateInterview called with candidate_id=%s, interviewer_id=%s, scheduled_at=%s\n", req.CandidateId, req.InterviewerId, req.ScheduledAt)

	// Parse scheduled time
	var scheduledAt time.Time
	var err error
	if req.ScheduledAt != "" {
		scheduledAt, err = time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			fmt.Printf("DEBUG: Failed to parse scheduled_at '%s': %v\n", req.ScheduledAt, err)
			return &pb.CreateInterviewResponse{
				Response: &pb.Response{
					Success: false,
					Error:   fmt.Sprintf("Invalid scheduled time format: %v", err),
				},
			}, nil
		}
	} else {
		// Use current time if not specified
		scheduledAt = time.Now()
		fmt.Printf("DEBUG: No scheduled_at provided, using current time: %v\n", scheduledAt)
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

	fmt.Printf("DEBUG: Calling interviewService.CreateInterview with interview title=%s\n", interview.Title)
	createdInterview, err := h.interviewService.CreateInterview(interview, req.Technologies)
	if err != nil {
		fmt.Printf("DEBUG: CreateInterview failed: %v\n", err)
		return &pb.CreateInterviewResponse{
			Response: &pb.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	fmt.Printf("DEBUG: CreateInterview successful, interview ID=%s\n", createdInterview.ID)

	return &pb.CreateInterviewResponse{
		Response: &pb.Response{
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
			Response: &pb.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	// Get technologies for this interview
	technologies, err := h.interviewService.GetInterviewTechnologies(interview.ID)
	if err != nil {
		technologies = []string{} // Continue with empty technologies
	}

	return &pb.GetInterviewResponse{
		Response: &pb.Response{
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
			Technologies:   technologies,
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
			Response: &pb.Response{
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

		// Get technologies for this interview
		technologies, err := h.interviewService.GetInterviewTechnologies(interview.ID)
		if err != nil {
			fmt.Printf("DEBUG: Failed to get technologies for interview %s: %v\n", interview.ID, err)
			technologies = []string{} // Continue with empty technologies
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
			Technologies:   technologies,
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
		Response: &pb.Response{
			Success: true,
		},
		Interviews: pbInterviews,
		Pagination: &pb.Pagination{
			Page:  req.Pagination.Page,
			Limit: req.Pagination.Limit,
			Total: int32(len(pbInterviews)),
		},
	}, nil
}

func (h *InterviewHandler) UpdateInterview(ctx context.Context, req *pb.UpdateInterviewRequest) (*pb.UpdateInterviewResponse, error) {
	// Get existing interview to preserve fields not being updated
	existingInterview, err := h.interviewService.GetInterviewByID(req.InterviewId)
	if err != nil {
		return &pb.UpdateInterviewResponse{
			Response: &pb.Response{
				Success: false,
				Error:   fmt.Sprintf("Failed to get existing interview: %v", err),
			},
		}, nil
	}

	// Parse scheduled time if provided
	var scheduledAt time.Time
	if req.ScheduledAt != "" {
		var err error
		scheduledAt, err = time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			return &pb.UpdateInterviewResponse{
				Response: &pb.Response{
					Success: false,
					Error:   "Invalid scheduled time format",
				},
			}, nil
		}
	} else {
		scheduledAt = existingInterview.ScheduledAt
	}

	// Use existing status if not provided
	status := req.Status
	if status == "" {
		status = existingInterview.Status
	}

	interview := &models.Interview{
		ID:             req.InterviewId,
		Title:          req.Title,
		Description:    req.Description,
		Status:         status,
		ScheduledAt:    scheduledAt,
		Level:          req.Level,
		Specialization: req.Specialization,
	}

	updatedInterview, err := h.interviewService.UpdateInterview(interview, req.Technologies)
	if err != nil {
		return &pb.UpdateInterviewResponse{
			Response: &pb.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	// Get technologies for this interview
	technologies, err := h.interviewService.GetInterviewTechnologies(updatedInterview.ID)
	if err != nil {
		technologies = req.Technologies // Fallback to requested technologies
	}

	return &pb.UpdateInterviewResponse{
		Response: &pb.Response{
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
			Technologies:   technologies,
		},
	}, nil
}

func (h *InterviewHandler) DeleteInterview(ctx context.Context, req *pb.DeleteInterviewRequest) (*pb.Response, error) {
	err := h.interviewService.DeleteInterview(req.InterviewId)
	if err != nil {
		return &pb.Response{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.Response{
		Success: true,
		Message: "Interview deleted successfully",
	}, nil
}

func (h *InterviewHandler) GetScheduledInterviews(ctx context.Context, req *pb.GetScheduledInterviewsRequest) (*pb.GetScheduledInterviewsResponse, error) {
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return &pb.GetScheduledInterviewsResponse{
			Response: &pb.Response{
				Success: false,
				Error:   "Invalid date format",
			},
		}, nil
	}

	interviews, err := h.interviewService.GetScheduledInterviews(req.InterviewerId, date)
	if err != nil {
		return &pb.GetScheduledInterviewsResponse{
			Response: &pb.Response{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	var pbInterviews []*pb.Interview
	for _, interview := range interviews {
		// Get technologies for this interview
		technologies, err := h.interviewService.GetInterviewTechnologies(interview.ID)
		if err != nil {
			fmt.Printf("DEBUG: Failed to get technologies for interview %s: %v\n", interview.ID, err)
			technologies = []string{} // Continue with empty technologies
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
			Technologies:   technologies,
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
		Response: &pb.Response{
			Success: true,
		},
		Interviews: pbInterviews,
	}, nil
}

func (h *InterviewHandler) GenerateQuestions(ctx context.Context, req *pb.GenerateQuestionsRequest) (*pb.GenerateQuestionsResponse, error) {
	questions, err := h.interviewService.GenerateQuestions(req.InterviewId, req.Technologies, req.Level, req.Specialization)
	if err != nil {
		return &pb.GenerateQuestionsResponse{
			Response: &pb.Response{
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
		Response: &pb.Response{
			Success: true,
			Message: "Questions generated successfully",
		},
		Questions: pbQuestions,
	}, nil
}
