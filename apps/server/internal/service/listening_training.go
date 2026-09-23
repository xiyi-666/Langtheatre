package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/domain"
)

const listeningGenerationAction = "LISTENING_TRAINING_GENERATION"

type listeningTrainingEngine interface {
	GenerateListeningTraining(context.Context, int, float64) (domain.MockExamSection, error)
}

// 复用现有三种数据库的考试记录存储，用独立标识隔离训练与整卷。
func ListeningTrainingPart(exam string) int {
	for part := 1; part <= 4; part++ {
		if exam == fmt.Sprintf("IELTS_LISTENING_PART_%d", part) {
			return part
		}
	}
	return 0
}

func (s *Service) ListeningTrainingGenerationCost() int {
	if s.MockExamGenerationCost() == 0 {
		return 0
	}
	return aiCreditAmount(AICreditActionTheaterGeneration)
}

func (s *Service) StartListeningTraining(userID string, part int, band float64) (domain.MockExam, error) {
	if userID == "" {
		return domain.MockExam{}, errors.New("请先登录后再开始听力训练")
	}
	if part < 1 || part > 4 || math.IsNaN(band) || math.IsInf(band, 0) || band < 6 || band > 8 || band*2 != math.Trunc(band*2) {
		return domain.MockExam{}, errors.New("请选择 Part 1–4，目标难度为 6.0–8.0，步长 0.5")
	}
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.MockExam{}, err
	}
	engine, ok := s.generator.(listeningTrainingEngine)
	if !ok || s.tts == nil {
		return domain.MockExam{}, errors.New("听力训练需要管理员配置出题模型和 TTS 服务")
	}
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	history, err := s.store.ListMockExams(userID)
	if err != nil {
		return domain.MockExam{}, err
	}
	kind := fmt.Sprintf("IELTS_LISTENING_PART_%d", part)
	for _, existing := range history {
		if existing.Status == "GENERATING" && time.Since(existing.UpdatedAt) < s.tasks.timeout+time.Minute {
			if existing.Exam == kind && len(existing.Sections) > 0 && existing.Sections[0].TargetBand == band {
				return cloneMockExam(applyMockReadyEstimate(history, existing)), nil
			}
			return domain.MockExam{}, errors.New("已有听力或模拟考试任务正在生成，请等待完成后再创建")
		}
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.MockExam{}, err
	}
	now := time.Now().UTC()
	paper := domain.MockExam{ID: uuid.NewString(), UserID: userID, Exam: kind, Status: "GENERATING", CurrentSection: "正在生成听力原文与题目，需通过题目和目标难度独立审核后才可使用", CreatedAt: now, UpdatedAt: now,
		Sections: []domain.MockExamSection{{Key: fmt.Sprintf("LISTENING_%d", part), TargetBand: band, PaperVersion: mockExamVersion}}}
	cost := aiCreditAmount(AICreditActionTheaterGeneration)
	if err := s.ConsumeAIConfidence(userID, listeningGenerationAction, paper.ID, cost); err != nil {
		release()
		return domain.MockExam{}, err
	}
	saved, err := s.store.SaveMockExam(paper)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, listeningGenerationAction, paper.ID, cost)
		return domain.MockExam{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.generateListeningTraining(ctx, saved, part, band, engine)
	}) {
		release()
		s.RefundAIConfidence(userID, listeningGenerationAction, paper.ID, cost)
		saved.Status, saved.CurrentSection = "FAILED", "生成队列繁忙，请稍后重新创建"
		return s.store.UpdateMockExam(saved)
	}
	s.trackFeature("LISTENING_TRAINING_GENERATION_REQUESTED")
	return cloneMockExam(applyMockReadyEstimate(history, saved)), nil
}

func (s *Service) generateListeningTraining(ctx context.Context, saved domain.MockExam, part int, band float64, engine listeningTrainingEngine) {
	defer func() {
		if r := recover(); r != nil {
			s.failMockExam(saved, "FAILED", fmt.Errorf("listening generation panic: %v", r))
		}
	}()
	section, err := engine.GenerateListeningTraining(ctx, part, band)
	if err == nil {
		err = ai.ValidateListeningTraining(section, part)
	}
	if err == nil && section.TargetBand != band {
		err = errors.New("listening target band differs from request")
	}
	if err != nil {
		if section.ListeningDifficulty != nil || section.AudioScript != "" {
			// 审核接口失败时同样保留已生成草稿供排查；FAILED 不对学员返回正文或答案。
			saved.Sections = []domain.MockExamSection{section}
			if persistErr := s.updateMockJob(saved, "GENERATING"); persistErr != nil {
				log.Printf("listening_audit_persist_failed training_id=%s", saved.ID)
			}
		}
		s.failMockExam(saved, "FAILED", err)
		return
	}
	saved.Sections = []domain.MockExamSection{section}
	saved.TotalDurationSeconds = section.DurationSeconds
	saved.CurrentSection = "题目与目标难度审核通过，正在生成听力音频；可离开页面稍后查看"
	if err = s.updateMockJob(saved, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	urls, err := s.mockListeningAudio(ctx, saved.ID, section)
	if err != nil || len(urls) == 0 {
		if err == nil {
			err = errors.New("listening audio empty")
		}
		s.failMockExam(saved, "FAILED", err)
		return
	}
	saved.Sections[0].AudioURLs, saved.Sections[0].AudioURL = urls, urls[0]
	if err = approveReviewedListeningTraining(&saved); err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	saved.Status, saved.CurrentSection = "READY", section.Key
	saved = recordMockGenerationDuration(saved, time.Since(saved.CreatedAt))
	if err = s.updateMockJob(saved, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
	}
}

func (s *Service) ListeningTrainings(userID string) ([]domain.MockExam, error) {
	if userID == "" {
		return nil, errors.New("请先登录")
	}
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	history, err := s.store.ListMockExams(userID)
	if err != nil {
		return nil, err
	}
	result := []domain.MockExam{}
	for _, item := range history {
		if ListeningTrainingPart(item.Exam) == 0 {
			continue
		}
		item, err = s.recoverStaleMockExamLocked(userID, item)
		if err != nil {
			return nil, err
		}
		result = append(result, cloneMockExam(quarantineListeningTraining(applyMockReadyEstimate(history, item))))
	}
	return result, nil
}

func (s *Service) ListeningTraining(userID, id string) (domain.MockExam, error) {
	if userID == "" {
		return domain.MockExam{}, errors.New("请先登录")
	}
	paper, err := s.MockExam(userID, id)
	if err != nil || ListeningTrainingPart(paper.Exam) == 0 {
		return domain.MockExam{}, errors.New("听力训练不存在或无权访问")
	}
	return paper, nil
}

func (s *Service) BeginListeningTraining(userID, id string) (domain.MockExam, error) {
	if _, err := s.ListeningTraining(userID, id); err != nil {
		return domain.MockExam{}, err
	}
	return s.BeginMockExam(userID, id)
}

func (s *Service) SaveListeningAnswers(userID, id string, answers []string) (domain.MockExam, error) {
	paper, err := s.ListeningTraining(userID, id)
	if err != nil {
		return domain.MockExam{}, err
	}
	if len(paper.Sections) != 1 {
		return domain.MockExam{}, errors.New("听力训练内容不完整")
	}
	return s.SaveMockExamAnswers(userID, id, paper.Sections[0].Key, answers, nil)
}

func (s *Service) DeleteListeningTraining(userID, id string) error {
	if _, err := s.ListeningTraining(userID, id); err != nil {
		return err
	}
	// 专项训练支持放弃未完成练习；后台生成中禁止删除。
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	paper, err := s.store.GetMockExam(id, userID)
	if err != nil {
		return err
	}
	if paper.Status == "GENERATING" {
		return errors.New("听力仍在生成，请等待完成或失败后删除")
	}
	return s.store.DeleteMockExam(id, userID)
}

func ListeningAnswerMatches(answer string, q domain.QuizQuestion) bool {
	return mockAnswerMatches(answer, q)
}

// 旧内容和修改后审核不再匹配的内容不可继续使用；保留数据库原记录便于复核。
func quarantineListeningTraining(paper domain.MockExam) domain.MockExam {
	if ListeningTrainingPart(paper.Exam) == 0 || paper.Status == "GENERATING" || paper.Status == "FAILED" {
		return paper
	}
	if err := validateMockExamForService(paper); err != nil {
		paper.Status, paper.Result = "FAILED", nil
		paper.CurrentSection = "这份听力尚未通过当前难度审核，暂不能练习。请重新生成或联系管理员。记录编号：" + paper.ID
	}
	return paper
}

func (s *Service) FinishListeningTraining(userID, id string, answers []string) (domain.MockExam, error) {
	s.mockMu.Lock()
	paper, err := s.finishListeningTrainingLocked(userID, id, answers)
	s.mockMu.Unlock()
	if err == nil && paper.Result != nil {
		answered := false
		for _, answer := range paper.Sections[0].Answers {
			answered = answered || strings.TrimSpace(answer) != ""
		}
		if answered {
			if _, xpErr := s.awardLearningXP(userID, "LISTENING_PRACTICE", id, int(paper.Result.TotalScore)); xpErr != nil {
				log.Printf("listening_xp_failed training_id=%s err=%v", id, xpErr)
			}
		}
	}
	return paper, err
}

func (s *Service) finishListeningTrainingLocked(userID, id string, answers []string) (domain.MockExam, error) {
	if userID == "" {
		return domain.MockExam{}, errors.New("请先登录")
	}
	paper, err := s.store.GetMockExam(id, userID)
	if err != nil || ListeningTrainingPart(paper.Exam) == 0 {
		return domain.MockExam{}, errors.New("听力训练不存在或无权访问")
	}
	if err = validateMockExamForService(paper); err != nil {
		return domain.MockExam{}, errors.New("听力内容或难度校验未通过，请重新创建")
	}
	if err = ensureMockExamProductionApproved(paper); err != nil {
		return domain.MockExam{}, productionGateError("听力训练", err)
	}
	if paper.Status == "COMPLETED" {
		return cloneMockExam(paper), nil
	}
	if paper.Status != "IN_PROGRESS" {
		return domain.MockExam{}, errors.New("请先开始听力训练再提交")
	}
	paper = cloneMockExam(paper)
	if len(answers) != len(paper.Sections[0].Questions) {
		return domain.MockExam{}, errors.New("提交的答案数量与题目不一致，未作答请留空")
	}
	for _, answer := range answers {
		if len([]rune(answer)) > 200 {
			return domain.MockExam{}, errors.New("单题答案过长")
		}
	}
	paper.Sections[0].Answers = append([]string{}, answers...)
	correct, total := scoreMockSections(paper.Sections, "LISTENING")
	now := time.Now().UTC()
	result := &domain.MockExamResult{ListeningCorrect: correct, ListeningTotal: total, TotalScore: float64(correct) * 100 / float64(total), ScoreScale: "ACCURACY_PERCENT", QualityStatus: "PRACTICE_NOT_OFFICIAL_BAND", CompletedAt: now,
		Feedback: "按已审核答案核对；空白计错，忽略大小写及首尾、多余空格，拼写须正确。单段10题仅显示正确率，不换算雅思 Band。"}
	for _, kind := range []string{"multiple_choice", "sentence_completion", "matching_information"} {
		wrong := 0
		for i, q := range paper.Sections[0].Questions {
			if q.Type == kind && !mockAnswerMatches(answers[i], q) {
				wrong++
			}
		}
		if wrong > 0 {
			result.Recommendations = append(result.Recommendations, fmt.Sprintf("%s错答或未答 %d 题：先重听相关内容，再对照原文依据检查同义替换、干扰信息和拼写。", mockQuestionLabel(kind), wrong))
		}
	}
	if correct == total {
		result.Recommendations = []string{"本次全部答对。建议隐藏原文再听一遍，复述主要信息后挑战其他 Part。"}
	}
	paper.Result, paper.Status, paper.CurrentSection, paper.SubmittedAt, paper.UpdatedAt = result, "COMPLETED", "RESULT", now, now
	if err = approveDeterministicListeningEvaluation(&paper); err != nil {
		return domain.MockExam{}, productionGateError("听力评分", err)
	}
	saved, err := s.store.UpdateMockExam(paper)
	if err == nil {
		s.trackFeature("LISTENING_TRAINING_COMPLETED")
	}
	return cloneMockExam(saved), err
}

func mockGenerationCharge(exam domain.MockExam) (string, int) {
	if ListeningTrainingPart(exam.Exam) > 0 {
		return listeningGenerationAction, aiCreditAmount(AICreditActionTheaterGeneration)
	}
	return "MOCK_EXAM_GENERATION", mockGenerationCost()
}
