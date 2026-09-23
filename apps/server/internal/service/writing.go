package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
)

type writingEngine interface {
	GenerateWritingPrompt(ctx context.Context, exam string) (domain.WritingPrompt, error)
	EvaluateWriting(ctx context.Context, exam string, prompt domain.WritingPrompt, essay string, timeLimitSeconds int, elapsedSeconds int) (domain.WritingEvaluation, error)
}

func (s *Service) StartWritingSession(userID, exam string, timeLimitSeconds int) (domain.WritingSession, error) {
	return s.StartWritingSessionWithDifficulty(userID, exam, timeLimitSeconds, "")
}

func (s *Service) StartWritingSessionWithDifficulty(userID, exam string, timeLimitSeconds int, difficulty string) (domain.WritingSession, error) {
	exam = normalizeWritingExam(exam)
	if exam == "" {
		return domain.WritingSession{}, errors.New("exam must be IELTS, CET4, or CET6")
	}
	if timeLimitSeconds < 300 || timeLimitSeconds > 7200 {
		return domain.WritingSession{}, errors.New("time limit must be between 5 and 120 minutes")
	}
	if s.isDemoAccount(userID) {
		return s.generateDemoWriting(userID, exam, timeLimitSeconds, difficulty)
	}
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.WritingSession{}, err
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
	engine, ok := any(s.generator).(writingEngine)
	if !ok {
		return domain.WritingSession{}, errors.New("写作模型未配置，无法生成 IELTS 题目")
	}
	prompt, err := engine.GenerateWritingPrompt(context.Background(), exam)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionWritingPrompt, sessionID, aiCreditAmount(AICreditActionWritingPrompt))
		return domain.WritingSession{}, fmt.Errorf("写作题目生成失败: %w", err)
	}
	now := time.Now().UTC()
	session := domain.WritingSession{ID: sessionID, UserID: userID, Exam: exam, TimeLimitSeconds: timeLimitSeconds, Prompt: prompt, Status: "QUALITY_REVIEW_PENDING", ProgressMessage: "正在进行独立质量审核", PromptApproval: contentquality.PendingApproval(), CreatedAt: now, UpdatedAt: now}
	reviewer, reviewErr := productionReviewer[writingProductionReviewer](s.generator)
	if reviewErr == nil {
		session.PromptApproval, reviewErr = reviewer.ReviewWritingPrompt(context.Background(), session)
	} else {
		hash, _ := contentquality.WritingPromptContentHash(session.Exam, session.TimeLimitSeconds, session.Prompt)
		session.PromptApproval = inconclusiveApproval(contentquality.WritingPromptRubricVersion, hash, reviewErr.Error())
	}
	if reviewErr == nil {
		reviewErr = ensureWritingPromptProductionApproved(session)
	}
	if reviewErr != nil {
		session.Status = "FAILED"
		session.ProgressMessage = "题目未通过独立质量审核，已隔离且不会开始计时"
		saved, saveErr := s.store.SaveWritingSession(session)
		s.RefundAIConfidence(userID, AICreditActionWritingPrompt, sessionID, aiCreditAmount(AICreditActionWritingPrompt))
		if saveErr != nil {
			return domain.WritingSession{}, saveErr
		}
		return saved, productionGateError("写作题目", reviewErr)
	}
	session.Status = "WRITING"
	session.ProgressMessage = "题目已通过质量审核，计时已开始"
	session.StartedAt = time.Now().UTC()
	session.UpdatedAt = session.StartedAt
	saved, err := s.store.SaveWritingSession(session)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionWritingPrompt, sessionID, aiCreditAmount(AICreditActionWritingPrompt))
	} else {
		s.trackFeature("WRITING_SESSION_STARTED")
	}
	return saved, err
}

func (s *Service) WritingSession(userID, sessionID string) (domain.WritingSession, error) {
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if s.isDemoAccount(userID) && (approvalIsDemoOnly(session.PromptApproval) || approvalIsDemoOnly(session.EvaluationApproval)) {
		return session, nil
	}
	return redactWritingForQuality(session), nil
}
func (s *Service) WritingSessions(userID string) ([]domain.WritingSession, error) {
	if s.isDemoAccount(userID) {
		if err := s.ensureDemoLearningFixtures(userID, ""); err != nil {
			log.Printf("demo writing fixtures unavailable user_id=%s err=%v", userID, err)
		}
	}
	items, err := s.store.ListWritingSessions(userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if !(s.isDemoAccount(userID) && (approvalIsDemoOnly(items[i].PromptApproval) || approvalIsDemoOnly(items[i].EvaluationApproval))) {
			items[i] = redactWritingForQuality(items[i])
		}
	}
	return items, nil
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
	if s.isDemoAccount(userID) {
		return s.submitDemoWriting(userID, sessionID, essay)
	}
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.WritingSession{}, err
	}
	session, err := s.store.GetWritingSession(sessionID, userID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if session.Status != "WRITING" {
		return domain.WritingSession{}, errors.New("writing has already been submitted")
	}
	if err = ensureWritingPromptProductionApproved(session); err != nil {
		return domain.WritingSession{}, productionGateError("写作题目", err)
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
	engine, ok := any(s.generator).(writingEngine)
	if !ok {
		s.markWritingEvaluationFailed(userID, session, "评分系统未配置")
		return
	}
	evaluation, evalErr := engine.EvaluateWriting(ctx, session.Exam, session.Prompt, session.Essay, session.TimeLimitSeconds, elapsed)
	if evalErr != nil {
		s.markWritingEvaluationFailed(userID, session, fmt.Sprintf("AI 评分失败：%s", evalErr.Error()))
		return
	}
	session.Evaluation = &evaluation
	reviewer, reviewErr := productionReviewer[writingProductionReviewer](s.generator)
	if reviewErr == nil {
		session.EvaluationApproval, reviewErr = reviewer.ReviewWritingEvaluation(ctx, session)
	} else {
		hash, _ := contentquality.WritingEvaluationContentHash(session)
		session.EvaluationApproval = inconclusiveApproval(contentquality.WritingEvaluationRubricVersion, hash, reviewErr.Error())
	}
	if reviewErr == nil {
		reviewErr = ensureWritingEvaluationProductionApproved(session)
	}
	if reviewErr != nil {
		session.ProgressMessage = "评分未通过独立质量审核，结果已隔离，请稍后重试"
		s.markWritingEvaluationFailed(userID, session, session.ProgressMessage)
		return
	}
	session.Status = "COMPLETED"
	session.ProgressMessage = "评分完成并通过质量审核"
	_, err = s.store.UpdateWritingSessionExisting(session)
	if err == nil {
		_, _ = s.awardLearningXP(userID, "WRITING_COMPLETE", sessionID, int(evaluation.OverallScore))
	} else {
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, sessionID, aiCreditAmount(AICreditActionWritingEvaluation))
	}
}

func (s *Service) markWritingEvaluationFailed(userID string, session domain.WritingSession, message string) {
	session.Status = "EVALUATION_FAILED"
	session.ProgressMessage = message
	if _, err := s.store.UpdateWritingSessionExisting(session); err != nil {
		log.Printf("writing evaluation failure state persist failed session_id=%s err=%v", session.ID, err)
	}
	s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, session.ID, aiCreditAmount(AICreditActionWritingEvaluation))
}

func normalizeWritingExam(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IELTS", "CET4", "CET6":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}
