package service

import (
	"fmt"
	"time"

	"github.com/inter-verse/services/interview-service/internal/models"
	"github.com/inter-verse/services/interview-service/internal/repository"
)

type InterviewService struct {
	interviewRepo      repository.InterviewRepository
	userServiceURL     string
	questionServiceURL string
}

func NewInterviewService(interviewRepo repository.InterviewRepository, userServiceURL, questionServiceURL string) *InterviewService {
	return &InterviewService{
		interviewRepo:      interviewRepo,
		userServiceURL:     userServiceURL,
		questionServiceURL: questionServiceURL,
	}
}

func (s *InterviewService) CreateInterview(interview *models.Interview, technologies []string) (*models.Interview, error) {
	if err := s.interviewRepo.CreateInterview(interview); err != nil {
		return nil, fmt.Errorf("failed to create interview: %w", err)
	}

	// Add technologies to interview
	for _, techID := range technologies {
		if err := s.interviewRepo.AddInterviewTechnology(interview.ID, techID); err != nil {
			return nil, fmt.Errorf("failed to add technology to interview: %w", err)
		}
	}

	return interview, nil
}

func (s *InterviewService) GetInterviewByID(id string) (*models.Interview, error) {
	interview, err := s.interviewRepo.GetInterviewByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get interview: %w", err)
	}

	// Get technologies for this interview
	technologies, err := s.interviewRepo.GetInterviewTechnologies(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get interview technologies: %w", err)
	}

	// Note: In a real implementation, you might want to include technologies in the response
	_ = technologies

	return interview, nil
}

func (s *InterviewService) GetInterviewsByInterviewer(interviewerID string, limit, offset int) ([]*models.Interview, error) {
	interviews, err := s.interviewRepo.GetInterviewsByInterviewer(interviewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get interviews: %w", err)
	}

	// Log candidate information
	for _, interview := range interviews {
		if interview.Candidate != nil {
			fmt.Printf("DEBUG: Service found candidate %s for interview %s\n", interview.Candidate.Name, interview.ID)
		} else {
			fmt.Printf("DEBUG: Service found no candidate for interview %s\n", interview.ID)
		}
	}

	return interviews, nil
}

func (s *InterviewService) UpdateInterview(interview *models.Interview) (*models.Interview, error) {
	if err := s.interviewRepo.UpdateInterview(interview); err != nil {
		return nil, fmt.Errorf("failed to update interview: %w", err)
	}

	return interview, nil
}

func (s *InterviewService) DeleteInterview(id string) error {
	return s.interviewRepo.DeleteInterview(id)
}

func (s *InterviewService) GetScheduledInterviews(interviewerID string, date time.Time) ([]*models.Interview, error) {
	interviews, err := s.interviewRepo.GetScheduledInterviews(interviewerID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled interviews: %w", err)
	}

	return interviews, nil
}

func (s *InterviewService) GenerateQuestions(interviewID string, technologies []string, level, specialization string) ([]string, error) {
	// In a real implementation, this would call the Question Service via gRPC
	// For now, we'll return mock questions
	questions := []string{
		fmt.Sprintf("Explain the difference between %s and %s", technologies[0], technologies[1]),
		fmt.Sprintf("How would you implement a %s solution for %s?", level, specialization),
		"What are the best practices for %s development?", specialization,
	}

	return questions, nil
}

func (s *InterviewService) ValidateUser(userID string) error {
	// In a real implementation, this would call the User Service via gRPC
	// For now, we'll assume the user is valid
	return nil
}

func (s *InterviewService) GetQuestionsFromService(technologies []string, level, specialization string) ([]string, error) {
	// In a real implementation, this would make a gRPC call to the Question Service
	// For now, we'll return mock questions
	questions := []string{
		fmt.Sprintf("Explain the difference between %s and %s", technologies[0], technologies[1]),
		fmt.Sprintf("How would you implement a %s solution for %s?", level, specialization),
		"What are the best practices for %s development?", specialization,
	}

	return questions, nil
}
