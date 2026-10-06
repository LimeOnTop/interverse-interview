package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/LimeOnTop/interverse-interview/internal/entity"
	"github.com/LimeOnTop/interverse-interview/internal/usecase"
)

const (
	defaultStatus          = entity.StatusScheduled
	minQuestionsPerSession = 10
	maxQuestionsPerSession = 20
	minTasksPerSession     = 1
	maxTasksPerSession     = 3
	subscriptionPlanFree   = "free"
	subscriptionPlanPaid   = "paid"
	// Basic gets a single training per account, ever.
	freeTrainingsTotal = 1
	// Pro: one training costs ~0.4 ₽ of DeepSeek tokens, so 5/day keeps the
	// worst case (~150/month) near 60 ₽, a small share of the subscription.
	paidTrainingsPerDay = 5
)

func moscowLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

type InterviewService struct {
	repository   usecase.InterviewRepository
	questionBank usecase.QuestionBank
}

func NewInterviewService(repository usecase.InterviewRepository, questionBank usecase.QuestionBank) *InterviewService {
	return &InterviewService{
		repository:   repository,
		questionBank: questionBank,
	}
}

var _ usecase.Interview = (*InterviewService)(nil)

func (s *InterviewService) Create(ctx context.Context, interview entity.Interview, technologies []string, subscriptionPlan string) (usecase.InterviewDTO, error) {
	if interview.Status == "" {
		interview.Status = defaultStatus
	}

	if err := s.ensureTrainingQuota(ctx, interview.UserID, subscriptionPlan); err != nil {
		return usecase.InterviewDTO{}, err
	}

	created, err := s.repository.Create(ctx, interview)
	if err != nil {
		return usecase.InterviewDTO{}, fmt.Errorf("create interview: %w", err)
	}

	for _, techID := range technologies {
		if err := s.repository.AddTechnology(ctx, created.ID, techID); err != nil {
			return usecase.InterviewDTO{}, fmt.Errorf("add interview technology: %w", err)
		}
	}

	dto := toDTO(created)
	dto.Technologies = technologies
	return dto, nil
}

func (s *InterviewService) ensureTrainingQuota(ctx context.Context, userID, subscriptionPlan string) error {
	now := time.Now().In(moscowLocation())
	plan := strings.ToLower(strings.TrimSpace(subscriptionPlan))
	if plan != subscriptionPlanPaid {
		plan = subscriptionPlanFree
	}

	var since time.Time
	var limit int
	var period string

	if plan == subscriptionPlanPaid {
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		limit = paidTrainingsPerDay
		period = "day"
	} else {
		since = time.Time{}
		limit = freeTrainingsTotal
		period = "total"
	}

	used, err := s.repository.CountCreatedSince(ctx, userID, since.UTC())
	if err != nil {
		return fmt.Errorf("check training quota: %w", err)
	}
	if used >= int64(limit) {
		return fmt.Errorf(
			"training_limit_exceeded: plan=%s limit=%d period=%s used=%d",
			plan, limit, period, used,
		)
	}
	return nil
}

func (s *InterviewService) GetByID(ctx context.Context, id string) (usecase.InterviewDTO, error) {
	interview, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return usecase.InterviewDTO{}, fmt.Errorf("get interview: %w", err)
	}

	technologies, err := s.repository.GetTechnologies(ctx, id)
	if err != nil {
		return usecase.InterviewDTO{}, fmt.Errorf("get interview technologies: %w", err)
	}

	dto := toDTO(interview)
	dto.Technologies = technologies
	return dto, nil
}

func (s *InterviewService) GetByUser(ctx context.Context, userID, status string, limit, offset int64) ([]usecase.InterviewDTO, error) {
	interviews, err := s.repository.GetByUser(ctx, userID, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get interviews: %w", err)
	}

	return s.toDTOsWithTechnologies(ctx, interviews)
}

func (s *InterviewService) Update(ctx context.Context, interview entity.Interview, technologies []string) (usecase.InterviewDTO, error) {
	updated, err := s.repository.Update(ctx, interview)
	if err != nil {
		return usecase.InterviewDTO{}, fmt.Errorf("update interview: %w", err)
	}

	// An empty list means "keep the current stack": status-only updates (for
	// example completing a training from the report service) must not wipe the
	// technologies the user selected.
	if len(technologies) == 0 {
		existing, err := s.repository.GetTechnologies(ctx, interview.ID)
		if err != nil {
			return usecase.InterviewDTO{}, fmt.Errorf("get interview technologies: %w", err)
		}

		dto := toDTO(updated)
		dto.Technologies = existing
		return dto, nil
	}

	if err := s.repository.DeleteTechnologies(ctx, interview.ID); err != nil {
		return usecase.InterviewDTO{}, fmt.Errorf("delete interview technologies: %w", err)
	}

	for _, techID := range technologies {
		if err := s.repository.AddTechnology(ctx, interview.ID, techID); err != nil {
			return usecase.InterviewDTO{}, fmt.Errorf("add interview technology: %w", err)
		}
	}

	dto := toDTO(updated)
	dto.Technologies = technologies
	return dto, nil
}

func (s *InterviewService) Delete(ctx context.Context, id string) error {
	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete interview: %w", err)
	}
	return nil
}

func (s *InterviewService) GetScheduled(ctx context.Context, userID string, date time.Time) ([]usecase.InterviewDTO, error) {
	interviews, err := s.repository.GetScheduled(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("get scheduled interviews: %w", err)
	}

	return s.toDTOsWithTechnologies(ctx, interviews)
}

func (s *InterviewService) StartSession(ctx context.Context, interviewID, userID string) (usecase.SessionContentDTO, error) {
	interview, err := s.repository.GetByID(ctx, interviewID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get interview: %w", err)
	}

	if interview.UserID != userID {
		return usecase.SessionContentDTO{}, fmt.Errorf("start session: forbidden")
	}

	if interview.Status == entity.StatusCompleted {
		return usecase.SessionContentDTO{}, fmt.Errorf("start session: training already completed")
	}

	technologies, err := s.repository.GetTechnologies(ctx, interviewID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get interview technologies: %w", err)
	}

	if len(technologies) == 0 {
		return usecase.SessionContentDTO{}, fmt.Errorf("start session: technologies are required")
	}

	existingItems, err := s.repository.GetSessionItems(ctx, interviewID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get session items: %w", err)
	}

	if len(existingItems) > 0 && interview.Status == entity.StatusInProgress {
		return s.buildSessionContentDTO(ctx, interview)
	}

	if len(existingItems) > 0 {
		if err := s.repository.DeleteSessionItems(ctx, interviewID); err != nil {
			return usecase.SessionContentDTO{}, fmt.Errorf("reset stale session items: %w", err)
		}
	}

	questionRefs, taskRefs, err := s.collectSessionQuestions(ctx, technologies, interview.Level, interview.Specialization)
	if err != nil {
		return usecase.SessionContentDTO{}, err
	}

	items := make([]entity.SessionItem, 0, len(questionRefs)+len(taskRefs))
	for index, ref := range questionRefs {
		items = append(items, entity.SessionItem{
			InterviewID: interviewID,
			QuestionID:  ref.ID,
			ItemType:    entity.ItemTypeQuestion,
			SortOrder:   index + 1,
			Text:        ref.Text,
			Technology:  ref.Technology,
			Difficulty:  ref.Difficulty,
			Category:    ref.Category,
			Options:     shuffledSessionOptions(ref.Options),
		})
	}

	for index, ref := range taskRefs {
		items = append(items, entity.SessionItem{
			InterviewID: interviewID,
			QuestionID:  ref.ID,
			ItemType:    entity.ItemTypeTask,
			SortOrder:   index + 1,
			Text:        ref.Text,
			Technology:  ref.Technology,
			Difficulty:  ref.Difficulty,
			Category:    ref.Category,
		})
	}

	if err := s.repository.SaveSessionItems(ctx, items); err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("save session items: %w", err)
	}

	interview.Status = entity.StatusInProgress
	if _, err := s.repository.Update(ctx, interview); err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("update interview status: %w", err)
	}

	return s.buildSessionContentDTO(ctx, interview)
}

func (s *InterviewService) GetSessionContent(ctx context.Context, interviewID, userID string) (usecase.SessionContentDTO, error) {
	interview, err := s.repository.GetByID(ctx, interviewID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get interview: %w", err)
	}

	if interview.UserID != userID {
		return usecase.SessionContentDTO{}, fmt.Errorf("get session content: forbidden")
	}

	return s.buildSessionContentDTO(ctx, interview)
}

func (s *InterviewService) collectSessionQuestions(ctx context.Context, technologies []string, level, specialization string) ([]usecase.QuestionRef, []usecase.QuestionRef, error) {
	difficulty := mapLevelToDifficulty(level)
	questionPool := make([]usecase.QuestionRef, 0)
	taskPool := make([]usecase.QuestionRef, 0)
	seenQuestions := make(map[string]struct{})
	seenTasks := make(map[string]struct{})

	perTechnologyLimit := 80

	for _, technology := range technologies {
		questions, err := s.fetchByTechnology(ctx, technology, difficulty, entity.ItemTypeQuestion, perTechnologyLimit)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch questions for %s: %w", technology, err)
		}

		for _, question := range questions {
			if !matchesSpecialization(question, specialization) {
				continue
			}
			if _, exists := seenQuestions[question.ID]; exists {
				continue
			}
			seenQuestions[question.ID] = struct{}{}
			questionPool = append(questionPool, question)
		}

		tasks, err := s.fetchByTechnology(ctx, technology, difficulty, entity.ItemTypeTask, perTechnologyLimit)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch tasks for %s: %w", technology, err)
		}

		for _, task := range tasks {
			if !matchesSpecialization(task, specialization) {
				continue
			}
			if _, exists := seenTasks[task.ID]; exists {
				continue
			}
			seenTasks[task.ID] = struct{}{}
			taskPool = append(taskPool, task)
		}
	}

	questionCount, taskCount, err := sessionItemCounts(len(questionPool), len(taskPool))
	if err != nil {
		questionPool, taskPool = s.collectPoolsWithoutSpecialization(ctx, technologies, difficulty)
		questionCount, taskCount, err = sessionItemCounts(len(questionPool), len(taskPool))
	}

	// Never substitute another stack: if the selected technologies do not have
	// enough material, tell the user instead of silently serving other questions.
	if err != nil {
		return nil, nil, &usecase.InsufficientQuestionsError{
			Technologies: technologies,
			Level:        level,
			Questions:    len(questionPool),
			Tasks:        len(taskPool),
			MinQuestions: minQuestionsPerSession,
			MinTasks:     minTasksPerSession,
		}
	}

	selectedQuestions := pickItems(questionPool, questionCount)
	selectedTasks := pickItems(taskPool, taskCount)

	return selectedQuestions, selectedTasks, nil
}

func (s *InterviewService) collectPoolsWithoutSpecialization(ctx context.Context, technologies []string, difficulty string) ([]usecase.QuestionRef, []usecase.QuestionRef) {
	questionPool := make([]usecase.QuestionRef, 0)
	taskPool := make([]usecase.QuestionRef, 0)
	seenQuestions := make(map[string]struct{})
	seenTasks := make(map[string]struct{})

	for _, technology := range technologies {
		questions, err := s.fetchByTechnology(ctx, technology, difficulty, entity.ItemTypeQuestion, 80)
		if err == nil {
			for _, question := range questions {
				if _, exists := seenQuestions[question.ID]; exists {
					continue
				}
				seenQuestions[question.ID] = struct{}{}
				questionPool = append(questionPool, question)
			}
		}

		tasks, err := s.fetchByTechnology(ctx, technology, difficulty, entity.ItemTypeTask, 80)
		if err == nil {
			for _, task := range tasks {
				if _, exists := seenTasks[task.ID]; exists {
					continue
				}
				seenTasks[task.ID] = struct{}{}
				taskPool = append(taskPool, task)
			}
		}
	}

	return questionPool, taskPool
}

func (s *InterviewService) buildSessionContentDTO(ctx context.Context, interview entity.Interview) (usecase.SessionContentDTO, error) {
	technologies, err := s.repository.GetTechnologies(ctx, interview.ID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get interview technologies: %w", err)
	}

	items, err := s.repository.GetSessionItems(ctx, interview.ID)
	if err != nil {
		return usecase.SessionContentDTO{}, fmt.Errorf("get session items: %w", err)
	}

	for index, item := range items {
		if item.ItemType != entity.ItemTypeQuestion || len(item.Options) > 0 || item.QuestionID == "" {
			continue
		}

		question, err := s.questionBank.GetByID(ctx, item.QuestionID)
		if err != nil || len(question.Options) == 0 {
			continue
		}

		items[index].Options = shuffledSessionOptions(question.Options)
		if err := s.repository.UpdateSessionItemOptions(ctx, item.ID, items[index].Options); err != nil {
			return usecase.SessionContentDTO{}, fmt.Errorf("backfill session item options: %w", err)
		}
	}

	dto := toDTO(interview)
	dto.Technologies = technologies

	content := usecase.SessionContentDTO{
		Interview: dto,
		Questions: make([]usecase.SessionItemDTO, 0),
		Tasks:     make([]usecase.SessionItemDTO, 0),
	}

	for _, item := range items {
		sessionItem := usecase.SessionItemDTO{
			ID:         item.ID,
			QuestionID: item.QuestionID,
			ItemType:   item.ItemType,
			SortOrder:  item.SortOrder,
			Text:       item.Text,
			Technology: item.Technology,
			Difficulty: item.Difficulty,
			Category:   item.Category,
			Options:    optionTexts(item.Options),
		}

		switch item.ItemType {
		case entity.ItemTypeTask:
			content.Tasks = append(content.Tasks, sessionItem)
		default:
			content.Questions = append(content.Questions, sessionItem)
		}
	}

	return content, nil
}

func (s *InterviewService) toDTOsWithTechnologies(ctx context.Context, interviews []entity.Interview) ([]usecase.InterviewDTO, error) {
	result := make([]usecase.InterviewDTO, 0, len(interviews))

	for _, interview := range interviews {
		technologies, err := s.repository.GetTechnologies(ctx, interview.ID)
		if err != nil {
			return nil, fmt.Errorf("get interview technologies: %w", err)
		}

		dto := toDTO(interview)
		dto.Technologies = technologies
		result = append(result, dto)
	}

	return result, nil
}

func toDTO(interview entity.Interview) usecase.InterviewDTO {
	return usecase.InterviewDTO{
		ID:             interview.ID,
		UserID:         interview.UserID,
		Title:          interview.Title,
		Description:    interview.Description,
		Status:         interview.Status,
		ScheduledAt:    interview.ScheduledAt,
		CreatedAt:      interview.CreatedAt,
		UpdatedAt:      interview.UpdatedAt,
		Level:          interview.Level,
		Specialization: interview.Specialization,
	}
}

func mapLevelToDifficulty(level string) string {
	switch strings.ToLower(level) {
	case "intern":
		return "intern"
	case "junior":
		return "junior"
	case "middle":
		return "middle"
	case "senior", "lead":
		return "senior"
	default:
		return strings.ToLower(level)
	}
}

var difficultyLevels = []string{"intern", "junior", "middle", "senior"}

// difficultyFallbackOrder returns the requested difficulty followed by the
// nearest levels (closest first, easier first on ties), so an Intern session
// falls back to Junior material rather than Senior.
func difficultyFallbackOrder(requested string) []string {
	requestedIndex := -1
	for index, level := range difficultyLevels {
		if level == requested {
			requestedIndex = index
			break
		}
	}
	if requestedIndex == -1 {
		if requested == "" {
			return append([]string(nil), difficultyLevels...)
		}
		return append([]string{requested}, difficultyLevels...)
	}

	order := []string{requested}
	for distance := 1; distance < len(difficultyLevels); distance++ {
		if lower := requestedIndex - distance; lower >= 0 {
			order = append(order, difficultyLevels[lower])
		}
		if higher := requestedIndex + distance; higher < len(difficultyLevels) {
			order = append(order, difficultyLevels[higher])
		}
	}
	return order
}

func (s *InterviewService) fetchByTechnology(ctx context.Context, technology, difficulty, category string, limit int) ([]usecase.QuestionRef, error) {
	for _, candidateDifficulty := range difficultyFallbackOrder(difficulty) {
		questions, err := s.questionBank.GetByTechnology(ctx, technology, candidateDifficulty, category, limit)
		if err != nil {
			return nil, err
		}
		if len(questions) > 0 {
			return questions, nil
		}
	}

	return nil, nil
}

func matchesSpecialization(question usecase.QuestionRef, specialization string) bool {
	if specialization == "" {
		return true
	}

	normalizedSpecialization := strings.ToLower(specialization)
	for _, tag := range question.Tags {
		if strings.EqualFold(tag, specialization) || strings.Contains(strings.ToLower(tag), normalizedSpecialization) {
			return true
		}
	}

	normalizedCategory := strings.ToLower(question.Category)
	return strings.Contains(normalizedCategory, normalizedSpecialization)
}

func sessionItemCounts(questionPoolSize, taskPoolSize int) (questionCount, taskCount int, err error) {
	if questionPoolSize < minQuestionsPerSession {
		return 0, 0, fmt.Errorf(
			"not enough questions in question bank: need at least %d, found %d",
			minQuestionsPerSession, questionPoolSize,
		)
	}
	if taskPoolSize < minTasksPerSession {
		return 0, 0, fmt.Errorf(
			"not enough tasks in question bank: need at least %d, found %d",
			minTasksPerSession, taskPoolSize,
		)
	}

	maxQuestions := maxQuestionsPerSession
	if questionPoolSize < maxQuestions {
		maxQuestions = questionPoolSize
	}
	maxTasks := maxTasksPerSession
	if taskPoolSize < maxTasks {
		maxTasks = taskPoolSize
	}

	return randomIntInclusive(minQuestionsPerSession, maxQuestions), randomIntInclusive(minTasksPerSession, maxTasks), nil
}

func randomIntInclusive(min, max int) int {
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min+1)
}

func pickItems(items []usecase.QuestionRef, count int) []usecase.QuestionRef {
	if count <= 0 || len(items) == 0 {
		return nil
	}
	if count > len(items) {
		count = len(items)
	}

	shuffled := append([]usecase.QuestionRef(nil), items...)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled[:count]
}

func mergeQuestionRefs(existing, extra []usecase.QuestionRef) []usecase.QuestionRef {
	if len(extra) == 0 {
		return existing
	}

	seen := make(map[string]struct{}, len(existing)+len(extra))
	merged := make([]usecase.QuestionRef, 0, len(existing)+len(extra))

	for _, item := range existing {
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		merged = append(merged, item)
	}

	for _, item := range extra {
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		merged = append(merged, item)
	}

	return merged
}

// shuffledSessionOptions randomizes answer order per session so the correct
// option is not always in the same position as in the question bank.
func shuffledSessionOptions(options []usecase.QuestionOptionRef) []entity.SessionOption {
	result := toSessionOptions(options)
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	for index := range result {
		result[index].SortOrder = index
	}
	return result
}

func toSessionOptions(options []usecase.QuestionOptionRef) []entity.SessionOption {
	result := make([]entity.SessionOption, 0, len(options))
	for _, option := range options {
		result = append(result, entity.SessionOption{
			Text:      option.Text,
			IsCorrect: option.IsCorrect,
			SortOrder: option.SortOrder,
		})
	}
	return result
}

func optionTexts(options []entity.SessionOption) []string {
	result := make([]string, 0, len(options))
	for _, option := range options {
		result = append(result, option.Text)
	}
	return result
}
