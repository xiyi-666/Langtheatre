package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

type writingEngine interface {
	GenerateWritingPrompt(ctx context.Context, exam string) (domain.WritingPrompt, error)
	EvaluateWriting(ctx context.Context, exam string, prompt domain.WritingPrompt, essay string, timeLimitSeconds int, elapsedSeconds int) (domain.WritingEvaluation, error)
}

func (s *Service) StartWritingSession(userID, exam string, timeLimitSeconds int) (domain.WritingSession, error) {
	exam = normalizeWritingExam(exam)
	if exam == "" {
		return domain.WritingSession{}, errors.New("exam must be IELTS, CET4, or CET6")
	}
	if timeLimitSeconds < 300 || timeLimitSeconds > 7200 {
		return domain.WritingSession{}, errors.New("time limit must be between 5 and 120 minutes")
	}
	sessionID := uuid.NewString()
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	defer release()
	if err = s.ConsumeAIConfidence(userID, AICreditActionWritingPrompt, sessionID, aiCreditAmount(AICreditActionWritingPrompt)); err != nil {
		return domain.WritingSession{}, err
	}
	prompt := fallbackWritingPrompt(exam)
	if engine, ok := any(s.generator).(writingEngine); ok {
		if generated, err := engine.GenerateWritingPrompt(context.Background(), exam); err == nil {
			prompt = generated
		}
	}
	now := time.Now().UTC()
	session := domain.WritingSession{ID: sessionID, UserID: userID, Exam: exam, TimeLimitSeconds: timeLimitSeconds, Prompt: prompt, Status: "WRITING", ProgressMessage: "计时已开始", StartedAt: now, CreatedAt: now, UpdatedAt: now}
	saved, err := s.store.SaveWritingSession(session)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionWritingPrompt, sessionID, aiCreditAmount(AICreditActionWritingPrompt))
	} else {
		s.trackFeature("WRITING_SESSION_STARTED")
	}
	return saved, err
}

func (s *Service) WritingSession(userID, sessionID string) (domain.WritingSession, error) {
	return s.store.GetWritingSession(sessionID, userID)
}
func (s *Service) WritingSessions(userID string) ([]domain.WritingSession, error) {
	return s.store.ListWritingSessions(userID)
}

func (s *Service) DeleteWritingSession(userID, sessionID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("unauthorized")
	}
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil {
		return err
	}
	if isActiveGeneratedMaterialStatus(session.Status) {
		return errors.New("writing evaluation is still in progress")
	}
	return s.store.DeleteWritingSession(userID, sessionID)
}

func (s *Service) SubmitWritingSession(userID, sessionID, essay string) (domain.WritingSession, error) {
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if session.Status != "WRITING" {
		return domain.WritingSession{}, errors.New("writing has already been submitted")
	}
	essay = strings.TrimSpace(essay)
	if len([]rune(essay)) < 30 {
		return domain.WritingSession{}, errors.New("essay must contain at least 30 characters")
	}
	if len([]rune(essay)) > 30000 {
		return domain.WritingSession{}, errors.New("essay is too long")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if err = s.ConsumeAIConfidence(userID, AICreditActionWritingEvaluation, sessionID, aiCreditAmount(AICreditActionWritingEvaluation)); err != nil {
		release()
		return domain.WritingSession{}, err
	}
	session.Essay = essay
	session.WordCount = len(strings.Fields(essay))
	session.SubmittedAt = time.Now().UTC()
	session.Status = "EVALUATING"
	session.ProgressMessage = "文章已提交，AI 正在按语法、词汇、连贯性和任务回应评分"
	saved, err := s.store.UpdateWritingSessionExisting(session)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, sessionID, aiCreditAmount(AICreditActionWritingEvaluation))
		return domain.WritingSession{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) { defer release(); s.evaluateWritingTask(ctx, userID, sessionID) }) {
		release()
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, sessionID, aiCreditAmount(AICreditActionWritingEvaluation))
		session.Status = "WRITING"
		session.ProgressMessage = "评分队列繁忙，请稍后再提交"
		_, _ = s.store.UpdateWritingSessionExisting(session)
		return domain.WritingSession{}, errors.New("writing evaluation queue is full")
	}
	s.trackFeature("WRITING_EVALUATION_REQUESTED")
	return saved, nil
}

func (s *Service) evaluateWritingTask(ctx context.Context, userID, sessionID string) {
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil || session.Status != "EVALUATING" {
		return
	}
	elapsed := int(session.SubmittedAt.Sub(session.StartedAt).Seconds())
	evaluation := fallbackWritingEvaluation(session)
	if engine, ok := any(s.generator).(writingEngine); ok {
		if generated, evalErr := engine.EvaluateWriting(ctx, session.Exam, session.Prompt, session.Essay, session.TimeLimitSeconds, elapsed); evalErr == nil {
			evaluation = generated
		}
	}
	session.Evaluation = &evaluation
	session.Status = "COMPLETED"
	session.ProgressMessage = "评分完成"
	_, err = s.store.UpdateWritingSessionExisting(session)
	if err == nil {
		_, _ = s.awardLearningXP(userID, "WRITING_COMPLETE", sessionID, int(evaluation.OverallScore))
	} else {
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, sessionID, aiCreditAmount(AICreditActionWritingEvaluation))
	}
}

func normalizeWritingExam(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IELTS", "CET4", "CET6":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}
func fallbackWritingPrompt(exam string) domain.WritingPrompt {
	if exam == "IELTS" {
		return domain.WritingPrompt{Title: "Remote work and community", Instructions: "Some people believe remote work improves quality of life, while others think it weakens local communities. Discuss both views and give your own opinion. Write in English.", SuggestedWordCount: 250}
	}
	return domain.WritingPrompt{Title: "A meaningful change on campus", Instructions: "Write an English essay describing one change that would improve university life. Explain the problem, your solution, and the expected benefits.", SuggestedWordCount: 150}
}
func fallbackWritingEvaluation(session domain.WritingSession) domain.WritingEvaluation {
	words := session.WordCount
	score := float64(min(82, 45+words/4))
	if words < 80 {
		score -= 12
	}
	if score < 20 {
		score = 20
	}
	return domain.WritingEvaluation{OverallScore: score, GrammarScore: score - 2, VocabularyScore: score, CoherenceScore: score - 1, TaskResponseScore: score, Strengths: []string{"已完成完整英文表达，并保持了基本的段落结构。"}, Issues: []string{"请检查长句中的主谓一致、冠词和连接词使用。"}, Suggestions: []string{"每段先给中心句，再用一个具体例子支撑观点。", "提交前用 2 分钟检查时态、单复数和拼写。"}, RevisedExcerpt: fmt.Sprintf("One practical way to improve this situation is to introduce a clear policy and explain its benefits to students. (%d words submitted)", words), Summary: "已生成基础评分；配置模型后可获得更细致的逐句分析。"}
}
