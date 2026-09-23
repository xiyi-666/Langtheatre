package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/questionbank"
	"log"
	"os"
	"strings"
	"time"
)

type speakingExaminerEngine interface {
	NextSpeakingPrompt(ctx context.Context, session domain.SpeakingSession, latestTurn domain.SpeakingTurn) (domain.SpeakingExaminerReply, error)
}

const maxSpeakingPromptSelectionAttempts = 3

func defaultSpeakingPrompts() []domain.SpeakingPrompt {
	return []domain.SpeakingPrompt{
		{Part: 1, Question: "Where do you live, and what do you like about that place?", PreparationSec: 0, AnswerSec: 30},
		{Part: 1, Question: "How do you usually spend your weekends?", PreparationSec: 0, AnswerSec: 30},
		{Part: 1, Question: "Do you prefer studying alone or with other people? Why?", PreparationSec: 0, AnswerSec: 30},
		{Part: 2, CueCard: "Describe a skill you would like to learn. You should say what it is, why you want to learn it, how you would learn it, and explain how it would help you.", PreparationSec: 60, AnswerSec: 120},
		{Part: 3, Question: "Why do some people continue learning new skills as adults?", PreparationSec: 0, AnswerSec: 45},
		{Part: 3, Question: "Should schools focus more on practical skills or academic knowledge?", PreparationSec: 0, AnswerSec: 45},
		{Part: 3, Question: "How might technology change the way people learn in the future?", PreparationSec: 0, AnswerSec: 45},
	}
}

func (s *Service) StartSpeakingSession(userID string) (domain.SpeakingSession, error) {
	if strings.TrimSpace(userID) == "" {
		return domain.SpeakingSession{}, errors.New("未登录，无法开始口语模拟")
	}
	prompts := defaultSpeakingPrompts()
	path := os.Getenv("SPEAKING_BANK_PATH")
	explicit := path != ""
	if !explicit {
		path = "data/question-bank/speaking.json"
	}
	bank, err := questionbank.Load(path)
	if err == nil {
		bank, err = filterPlayableSpeakingBank(bank, s.mediaDir)
		if err == nil {
			prompts, err = bank.SelectExcluding(nil)
		}
	}
	if err != nil {
		log.Printf("speaking_bank_unplayable review_path=%s media_dir=%s err=%v", path, s.mediaDir, err)
		if strings.Contains(err.Error(), "完整可播放") {
			return domain.SpeakingSession{}, errors.New("基础口语题库没有完整可播放题组，请联系管理员检查题库音频")
		}
		return domain.SpeakingSession{}, errors.New("基础口语题库读取或校验失败，请联系管理员检查 SPEAKING_BANK_PATH")
	}
	if s.store == nil {
		return domain.SpeakingSession{}, errors.New("口语存储服务不可用")
	}
	if _, ok := s.generator.(speakingExaminerEngine); !ok || s.tts == nil {
		return domain.SpeakingSession{}, errors.New("请先配置口语对话模型和 TTS 服务")
	}
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.SpeakingSession{}, err
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	background := false
	defer func() {
		if !background {
			release()
		}
	}()
	reviewID := uuid.NewString()
	reviewer, err := productionReviewer[speakingProductionReviewer](s.generator)
	if err != nil {
		return domain.SpeakingSession{}, speakingPromptReviewError(reviewID, err)
	}
	prompts, approvals, err := reviewSpeakingPromptSelections(context.Background(), reviewer, bank, prompts, reviewID)
	if err != nil {
		return domain.SpeakingSession{}, speakingPromptReviewError(reviewID, err)
	}
	for i := range prompts {
		prompts[i].ProductionApproval = &approvals[i]
	}
	now := time.Now().UTC()
	session := domain.SpeakingSession{ID: uuid.NewString(), UserID: userID, Status: "ACTIVE", Part: 1, PromptIndex: 0, PendingPromptIndex: -1, Prompts: prompts, Turns: []domain.SpeakingTurn{}, CreatedAt: now, UpdatedAt: now}
	if len(prompts) == 0 {
		return domain.SpeakingSession{}, errors.New("口语题目为空")
	}
	if err = ensureSpeakingPromptsProductionApproved(session); err != nil {
		return domain.SpeakingSession{}, speakingPromptReviewError(reviewID, err)
	}
	if prompts[0].AudioURL == "" {
		session.Status, session.ProcessingMessage = "PREPARING", "正在准备第一题考官音频"
	}
	if _, err := s.store.CreateSpeakingSession(session); err != nil {
		return domain.SpeakingSession{}, err
	}
	if session.Status == "PREPARING" {
		if !s.tasks.enqueue(func(ctx context.Context) { defer release(); s.prepareSpeakingAudio(ctx, session) }) {
			session.Status, session.ProcessingMessage = "PREPARATION_FAILED", "音频任务队列繁忙，请稍后重新开始"
			_, _ = s.store.UpdateSpeakingSession(session)
			return domain.SpeakingSession{}, errors.New(session.ProcessingMessage)
		}
		background = true
	}
	s.trackFeature("SPEAKING_SESSION_STARTED")
	return session, nil
}

func reviewSpeakingPromptSelections(ctx context.Context, reviewer speakingProductionReviewer, bank questionbank.Bank, prompts []domain.SpeakingPrompt, reviewID string) ([]domain.SpeakingPrompt, []domain.ProductionApproval, error) {
	excluded := make(map[string]struct{}, maxSpeakingPromptSelectionAttempts)
	excludedItemIDs := make(map[string]struct{})
	for attempt := 1; attempt <= maxSpeakingPromptSelectionAttempts; attempt++ {
		fingerprint := questionbank.SpeakingSelectionFingerprint(prompts)
		if fingerprint != "" {
			excluded[fingerprint] = struct{}{}
		}
		approvals, err := reviewer.ReviewSpeakingPrompts(ctx, prompts)
		if err == nil && len(approvals) == len(prompts) {
			if attempt > 1 {
				log.Printf("speaking_prompt_review_recovered review_id=%s attempt=%d/%d prompt_ids=%s", reviewID, attempt, maxSpeakingPromptSelectionAttempts, speakingPromptIDsForLog(prompts))
			}
			return prompts, approvals, nil
		}
		if err == nil {
			err = errors.New("口语题目审核结果不完整")
		}
		log.Printf("speaking_prompt_review_attempt_failed review_id=%s attempt=%d/%d prompt_ids=%s err=%v", reviewID, attempt, maxSpeakingPromptSelectionAttempts, speakingPromptIDsForLog(prompts), err)
		if !errors.Is(err, ai.ErrProductionReviewRejected) || attempt == maxSpeakingPromptSelectionAttempts {
			return nil, nil, err
		}
		for index, approval := range approvals {
			if index >= len(prompts) || !speakingApprovalRejected(approval) {
				continue
			}
			if id := strings.TrimSpace(prompts[index].QuestionID); id != "" {
				excludedItemIDs[id] = struct{}{}
			}
		}
		next, selectErr := bank.SelectExcludingItems(excluded, excludedItemIDs)
		if selectErr != nil {
			log.Printf("speaking_prompt_reselection_failed review_id=%s attempt=%d/%d err=%v", reviewID, attempt, maxSpeakingPromptSelectionAttempts, selectErr)
			return nil, nil, err
		}
		prompts = next
	}
	return nil, nil, errors.New("口语题目审核未完成")
}

func speakingApprovalRejected(approval domain.ProductionApproval) bool {
	if strings.EqualFold(strings.TrimSpace(approval.Status), contentquality.ApprovalRejected) {
		return true
	}
	for _, check := range approval.Checks {
		if strings.EqualFold(strings.TrimSpace(check.Status), contentquality.CheckFailed) {
			return true
		}
	}
	return false
}

func speakingPromptIDsForLog(prompts []domain.SpeakingPrompt) string {
	ids := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		id := strings.TrimSpace(prompt.QuestionID)
		if len(id) > 12 {
			id = id[:12]
		}
		if id != "" {
			ids = append(ids, id)
		}
	}
	return strings.Join(ids, ",")
}

func speakingPromptReviewError(reviewID string, cause error) error {
	log.Printf("speaking_prompt_review_failed review_id=%s err=%v", reviewID, cause)
	if errors.Is(cause, ai.ErrProductionReviewRejected) {
		return fmt.Errorf("本次抽取的口语题目未通过质量审核，请重新开始。记录编号：%s", reviewID)
	}
	if errors.Is(cause, ai.ErrProductionReviewInconclusive) || errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, context.Canceled) {
		return fmt.Errorf("口语题目审核服务暂时不可用，请稍后重试。记录编号：%s", reviewID)
	}
	return fmt.Errorf("口语题目质量校验失败，请联系管理员并提供记录编号：%s", reviewID)
}

func (s *Service) GetSpeakingSession(userID, id string) (domain.SpeakingSession, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return domain.SpeakingSession{}, errors.New("口语会话不存在或无权访问")
	}
	if (session.Status == "PREPARING" || session.Status == "PROCESSING" || session.Status == "EVALUATING") && time.Since(session.UpdatedAt) > s.tasks.timeout+time.Minute {
		if session.Status == "PROCESSING" {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, fmt.Sprintf("%s:%d", id, session.PromptIndex), aiCreditAmount(AICreditActionRoleplayTurn))
			session.Status = "TURN_FAILED"
		} else if session.Status == "EVALUATING" {
			s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, id, aiCreditAmount(AICreditActionWritingEvaluation))
			session.Status = "EVALUATION_FAILED"
			session.Evaluation = nil
		} else {
			session.Status = "PREPARATION_FAILED"
		}
		session.ProcessingMessage = "处理超时或服务重启，请重新提交。记录编号：" + id
		session.LastError = session.ProcessingMessage
		session.UpdatedAt = time.Now().UTC()
		if _, err = s.store.UpdateSpeakingSession(session); err != nil {
			return domain.SpeakingSession{}, err
		}
	}
	if err = ensureSpeakingPromptsProductionApproved(session); err != nil {
		session.Prompts = nil
		session.Evaluation = nil
		if session.Status != "PREPARATION_FAILED" && session.Status != "TURN_FAILED" && session.Status != "EVALUATION_FAILED" {
			session.Status = "QUALITY_REVIEW_PENDING"
		}
		session.ProcessingMessage = "口语题目尚未通过生产质量审核，暂未开放"
		return speakingSnapshot(session), nil
	}
	if session.Evaluation != nil {
		if err = ensureSpeakingEvaluationProductionApproved(session); err != nil {
			session.Evaluation = nil
			if session.Status == "COMPLETED" {
				session.Status = "QUALITY_REVIEW_PENDING"
			}
			session.ProcessingMessage = "口语评分尚未通过生产质量审核，暂不展示"
		}
	}
	return speakingSnapshot(session), nil
}

func (s *Service) LatestSpeakingSession(userID string) (*domain.SpeakingSession, error) {
	session, err := s.store.LatestSpeakingSession(userID)
	if err != nil {
		return nil, errors.New("恢复口语会话失败，请稍后重试")
	}
	if session == nil {
		return nil, nil
	}
	resolved, err := s.GetSpeakingSession(userID, session.ID)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (s *Service) AbandonSpeakingSession(userID, id string) error {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return errors.New("口语会话不存在或无权访问")
	}
	if session.Status == "COMPLETED" || session.Status == "ABANDONED" {
		return nil
	}
	session.Status = "ABANDONED"
	session.PendingPromptIndex = -1
	session.ProcessingMessage = "用户已结束本次口语会话"
	session.LastError = ""
	session.UpdatedAt = time.Now().UTC()
	_, err = s.store.UpdateSpeakingSession(session)
	if err != nil {
		return errors.New("退出口语会话失败，请稍后重试")
	}
	s.trackFeature("SPEAKING_SESSION_ABANDONED")
	return nil
}

func (s *Service) prepareSpeakingAudio(ctx context.Context, session domain.SpeakingSession) {
	fail := func(cause error) {
		log.Printf("speaking_prepare_failed session_id=%s err=%v", session.ID, cause)
		session.Status, session.ProcessingMessage = "PREPARATION_FAILED", "考官音频准备失败，请稍后重新开始。记录编号："+session.ID
		_ = s.commitSpeakingJob(session, "PREPARING", 0)
	}
	defer func() {
		if r := recover(); r != nil {
			fail(fmt.Errorf("panic: %v", r))
		}
	}()
	session = speakingSnapshot(session)
	url, err := s.synthesizeWithTTSLimit(ctx, strings.TrimSpace(session.Prompts[0].Question+"\n"+session.Prompts[0].CueCard), "ENGLISH", "")
	if err != nil {
		fail(err)
		return
	}
	if url == "" {
		fail(errors.New("empty TTS audio"))
		return
	}
	url, err = s.materializeAudioURL(url, "speaking", session.ID+"-0")
	if err != nil {
		fail(err)
		return
	}
	session.Prompts[0].AudioURL = url
	session.Status, session.ProcessingMessage = "ACTIVE", ""
	if err = s.commitSpeakingJob(session, "PREPARING", 0); err != nil {
		fail(err)
	}
}

func (s *Service) SubmitSpeakingTurn(userID, id string, promptIndex int, audioDataURL, text string, language string) (domain.SpeakingSession, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.SpeakingSession{}, err
	}
	if len(text) > 12000 || len(audioDataURL) > 14*1024*1024 {
		return domain.SpeakingSession{}, errors.New("回答过长，请将文字控制在12000字节内、录音控制在两分钟内")
	}
	if language != "" && !strings.EqualFold(language, "ENGLISH") && !strings.EqualFold(language, "en") {
		return domain.SpeakingSession{}, errors.New("当前口语训练仅支持英语")
	}
	language = "ENGLISH"
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return domain.SpeakingSession{}, errors.New("口语会话不存在或无权访问")
	}
	if promptIndex < 0 {
		return domain.SpeakingSession{}, errors.New("缺少当前口语题号，请刷新页面后重试")
	}
	if err = ensureSpeakingPromptsProductionApproved(session); err != nil {
		return domain.SpeakingSession{}, productionGateError("口语题目", err)
	}
	if promptIndex < session.PromptIndex {
		return session, nil
	}
	if session.Status == "PROCESSING" {
		return session, nil
	}
	if session.Status != "ACTIVE" && session.Status != "TURN_FAILED" {
		return domain.SpeakingSession{}, errors.New("口语会话已结束或正在处理")
	}
	if promptIndex != session.PromptIndex {
		return domain.SpeakingSession{}, errors.New("口语题目索引已变化，请恢复会话后重试")
	}
	if session.PromptIndex >= len(session.Prompts) {
		return domain.SpeakingSession{}, errors.New("当前会话没有待回答题目")
	}
	if strings.TrimSpace(audioDataURL) == "" && strings.TrimSpace(text) == "" {
		return domain.SpeakingSession{}, errors.New("请录音或输入回答后再提交")
	}
	if strings.TrimSpace(audioDataURL) != "" && s.asr == nil {
		return domain.SpeakingSession{}, errors.New("ASR 未配置，无法识别录音")
	}
	if _, ok := s.generator.(speakingExaminerEngine); !ok || s.tts == nil {
		return domain.SpeakingSession{}, errors.New("AI 考官服务未配置")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	creditSource := fmt.Sprintf("%s:%d", id, promptIndex)
	if err = s.ConsumeAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn)); err != nil {
		release()
		return domain.SpeakingSession{}, err
	}
	session.Status = "PROCESSING"
	session.PendingPromptIndex = promptIndex
	session.ProcessingMessage = "正在处理录音与 AI 考官追问"
	session.LastError = ""
	session.UpdatedAt = time.Now().UTC()
	saved, err := s.store.UpdateSpeakingSession(session)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		return domain.SpeakingSession{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.processSpeakingTurn(ctx, userID, id, promptIndex, audioDataURL, text, language, creditSource)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		session.Status = "ACTIVE"
		session.PendingPromptIndex = -1
		session.ProcessingMessage = ""
		session.LastError = "口语处理队列已满，请稍后重试"
		_, _ = s.store.UpdateSpeakingSession(session)
		return domain.SpeakingSession{}, errors.New(session.LastError)
	}
	return saved, nil
}

func (s *Service) FinishSpeakingSession(userID, id string) (domain.SpeakingSession, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.SpeakingSession{}, err
	}
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return domain.SpeakingSession{}, errors.New("口语会话不存在或无权访问")
	}
	if session.Status == "EVALUATING" || session.Status == "COMPLETED" {
		return session, nil
	}
	if err = ensureSpeakingPromptsProductionApproved(session); err != nil {
		return domain.SpeakingSession{}, productionGateError("口语题目", err)
	}
	if session.Status == "PROCESSING" {
		return domain.SpeakingSession{}, errors.New("口语回答仍在处理中，请稍后再提交评分")
	}
	if len(session.Turns) < len(session.Prompts) {
		return domain.SpeakingSession{}, errors.New("请完成全部口语题目后再提交评分")
	}
	engine, ok := any(s.generator).(speakingEngine)
	if !ok {
		return domain.SpeakingSession{}, errors.New("口语评分服务未配置")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.SpeakingSession{}, err
	}
	if err = s.ConsumeAIConfidence(userID, AICreditActionWritingEvaluation, id, aiCreditAmount(AICreditActionWritingEvaluation)); err != nil {
		release()
		return domain.SpeakingSession{}, err
	}
	session.Status = "EVALUATING"
	session.ProcessingMessage = "正在依据回答原文评估词汇、语法与文字连贯性"
	session.LastError = ""
	session.UpdatedAt = time.Now().UTC()
	if _, err = s.store.UpdateSpeakingSession(session); err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, id, aiCreditAmount(AICreditActionWritingEvaluation))
		return domain.SpeakingSession{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.processSpeakingEvaluation(ctx, userID, id, engine)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, id, aiCreditAmount(AICreditActionWritingEvaluation))
		session.Status = "ACTIVE"
		session.ProcessingMessage = ""
		session.LastError = "口语评分队列已满，请稍后重试"
		_, _ = s.store.UpdateSpeakingSession(session)
		return domain.SpeakingSession{}, errors.New(session.LastError)
	}
	return s.store.GetSpeakingSession(id, userID)
}

func (s *Service) processSpeakingTurn(ctx context.Context, userID, id string, promptIndex int, audioDataURL, text, language, creditSource string) {
	defer func() {
		if r := recover(); r != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, fmt.Sprintf("worker panic: %v", r))
		}
	}()
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return
	}
	if session.PromptIndex != promptIndex || session.Status != "PROCESSING" {
		return
	}
	session = speakingSnapshot(session)
	transcript := strings.TrimSpace(text)
	audioEvidence := false
	asrProvider := ""
	asrModel := ""
	if strings.TrimSpace(audioDataURL) != "" {
		session.ProcessingMessage = "正在识别录音"
		result, e := s.asr.Transcribe(ctx, audioDataURL, language)
		if e != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "语音识别失败："+e.Error())
			return
		}
		transcript = strings.TrimSpace(result.Text)
		audioEvidence = true
		asrProvider = result.Provider
		asrModel = result.Model
	}
	if transcript == "" {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		s.markSpeakingTurnFailure(userID, id, promptIndex, "未识别到有效回答，请重新录音或输入文字后重试")
		return
	}
	prompt := session.Prompts[promptIndex]
	turn := domain.SpeakingTurn{
		Part:          prompt.Part,
		PromptIndex:   promptIndex,
		Prompt:        strings.TrimSpace(prompt.Question + "\n" + prompt.CueCard),
		Transcript:    transcript,
		AudioEvidence: audioEvidence,
		ASRProvider:   asrProvider,
		ASRModel:      asrModel,
		SubmittedAt:   time.Now().UTC(),
	}
	session.Turns = append(session.Turns, turn)
	session.PromptIndex++
	session.PendingPromptIndex = -1
	if session.PromptIndex < len(session.Prompts) {
		session.Part = session.Prompts[session.PromptIndex].Part
		session.ProcessingMessage = "AI 考官正在生成后续提问"
		examiner, ok := any(s.generator).(speakingExaminerEngine)
		if !ok {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "AI 考官追问服务未配置")
			return
		}
		reply, e := examiner.NextSpeakingPrompt(ctx, session, turn)
		if e == nil {
			e = ai.ValidateSpeakingExaminerReply(session, reply)
		}
		if e != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "AI 考官追问生成失败："+e.Error())
			return
		}
		next := session.Prompts[session.PromptIndex]
		if strings.TrimSpace(next.Question) != reply.Question || strings.TrimSpace(next.CueCard) != reply.CueCard {
			next.Source = "AI examiner follow-up based on this practice session"
			next.QuestionID = ""
		}
		next.Question = strings.TrimSpace(reply.Question)
		next.CueCard = strings.TrimSpace(reply.CueCard)
		next.AudioURL = ""
		session.Turns[len(session.Turns)-1].ExaminerText = reply.Text
		if s.tts == nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "TTS 未配置，无法播放 AI 考官后续提问")
			return
		}
		session.ProcessingMessage = "正在生成 AI 考官语音"
		audioURL, e := s.synthesizeWithTTSLimit(ctx, reply.Text, "ENGLISH", "")
		if e != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "AI 考官语音生成失败："+e.Error())
			return
		}
		if strings.TrimSpace(audioURL) == "" {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "TTS returned empty audio")
			return
		}
		audioURL, e = s.materializeAudioURL(audioURL, "speaking", fmt.Sprintf("%s-%d", id, session.PromptIndex))
		if e != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, e.Error())
			return
		}
		next.AudioURL = audioURL
		reviewer, e := productionReviewer[speakingProductionReviewer](s.generator)
		if e == nil {
			var approvals []domain.ProductionApproval
			approvals, e = reviewer.ReviewSpeakingPrompts(ctx, []domain.SpeakingPrompt{next})
			if e == nil && len(approvals) == 1 {
				next.ProductionApproval = &approvals[0]
			} else if e == nil {
				e = errors.New("口语追问审核结果不完整")
			}
		}
		if e != nil {
			s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
			s.markSpeakingTurnFailure(userID, id, promptIndex, "AI 考官追问未通过独立质量审核")
			return
		}
		session.Prompts[session.PromptIndex] = next
		session.Turns[len(session.Turns)-1].ExaminerAudioURL = audioURL
	}
	session.Status = "ACTIVE"
	session.ProcessingMessage = ""
	session.LastError = ""
	if session.PromptIndex >= len(session.Prompts) {
		session.Status = "READY_TO_FINISH"
		session.ProcessingMessage = "对话已完成，可以提交文本评估"
	}
	if err = s.commitSpeakingJob(session, "PROCESSING", promptIndex); err == nil {
		s.trackFeature("SPEAKING_TURN_COMPLETED")
	} else {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		s.markSpeakingTurnFailure(userID, id, promptIndex, err.Error())
	}
}

func (s *Service) markSpeakingTurnFailure(userID, id string, promptIndex int, message string) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	log.Printf("speaking_turn_failed session_id=%s prompt_index=%d detail=%s", id, promptIndex, message)
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return
	}
	if session.Status != "PROCESSING" || session.PromptIndex != promptIndex {
		return
	}
	if promptIndex >= 0 && promptIndex < len(session.Turns) {
		session.Turns = session.Turns[:promptIndex]
	}
	session.PromptIndex = promptIndex
	if promptIndex >= 0 && promptIndex < len(session.Prompts) {
		session.Part = session.Prompts[promptIndex].Part
	}
	session.Status = "TURN_FAILED"
	session.PendingPromptIndex = -1
	session.ProcessingMessage = "语音识别、考官追问或音频生成未完成，请重试本题。记录编号：" + id
	session.LastError = session.ProcessingMessage
	session.UpdatedAt = time.Now().UTC()
	_, _ = s.store.UpdateSpeakingSession(session)
}

func (s *Service) processSpeakingEvaluation(ctx context.Context, userID, id string, engine speakingEngine) {
	defer func() {
		if r := recover(); r != nil {
			s.failSpeakingEvaluation(userID, id, fmt.Errorf("worker panic: %v", r))
		}
	}()
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil {
		return
	}
	evaluation, err := engine.EvaluateSpeaking(ctx, session.Prompts, session.Turns)
	if err == nil {
		err = ai.ValidateSpeakingTextEvaluation(evaluation, session.Turns)
	}
	if err != nil {
		s.failSpeakingEvaluation(userID, id, err)
		return
	}
	session.Evaluation = &evaluation
	reviewer, reviewErr := productionReviewer[speakingProductionReviewer](s.generator)
	if reviewErr == nil {
		session.EvaluationApproval, reviewErr = reviewer.ReviewSpeakingEvaluation(ctx, session)
	} else {
		hash, _ := contentquality.SpeakingEvaluationContentHash(session)
		session.EvaluationApproval = inconclusiveApproval(contentquality.SpeakingEvaluationRubricVersion, hash, reviewErr.Error())
	}
	if reviewErr == nil {
		reviewErr = ensureSpeakingEvaluationProductionApproved(session)
	}
	if reviewErr != nil {
		s.failSpeakingEvaluation(userID, id, reviewErr)
		return
	}
	session.Status = "COMPLETED"
	session.ProcessingMessage = ""
	session.LastError = ""
	session.UpdatedAt = time.Now().UTC()
	if err := s.commitSpeakingJob(session, "EVALUATING", session.PromptIndex); err == nil {
		s.trackFeature("SPEAKING_SESSION_COMPLETED")
	} else {
		s.failSpeakingEvaluation(userID, id, err)
	}
}

func speakingSnapshot(session domain.SpeakingSession) domain.SpeakingSession {
	raw, _ := json.Marshal(session)
	var copy domain.SpeakingSession
	_ = json.Unmarshal(raw, &copy)
	copy.PendingPromptIndex = -1
	if copy.Status == "PROCESSING" {
		copy.PendingPromptIndex = copy.PromptIndex
	}
	if strings.HasSuffix(copy.Status, "FAILED") {
		copy.LastError = copy.ProcessingMessage
	}
	return copy
}

func (s *Service) commitSpeakingJob(session domain.SpeakingSession, expected string, index int) error {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	current, err := s.store.GetSpeakingSession(session.ID, session.UserID)
	if err != nil {
		return err
	}
	if current.Status != expected || current.PromptIndex != index {
		return errors.New("口语状态已变化")
	}
	session.UpdatedAt = time.Now().UTC()
	_, err = s.store.UpdateSpeakingSession(speakingSnapshot(session))
	return err
}

func (s *Service) failSpeakingEvaluation(userID, id string, cause error) {
	log.Printf("speaking_evaluation_failed session_id=%s err=%v", id, cause)
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	session, err := s.store.GetSpeakingSession(id, userID)
	if err != nil || session.Status != "EVALUATING" {
		return
	}
	s.RefundAIConfidence(userID, AICreditActionWritingEvaluation, id, aiCreditAmount(AICreditActionWritingEvaluation))
	session.Status, session.Evaluation = "EVALUATION_FAILED", nil
	session.ProcessingMessage = "口语文本评估失败或未通过校验，回答已保存，请重试。记录编号：" + id
	session.LastError = session.ProcessingMessage
	session.UpdatedAt = time.Now().UTC()
	_, _ = s.store.UpdateSpeakingSession(session)
}
