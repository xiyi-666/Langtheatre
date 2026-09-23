package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
)

const mockExamVersion = "IELTS-Academic-v2"

type mockExamEngine interface {
	GenerateMockExam(context.Context, float64) (domain.MockExam, error)
}

type cetMockExamEngine interface {
	GenerateCETMockExam(context.Context, string) (domain.MockExam, error)
}

func mockGenerationCost() int {
	return 3*aiCreditAmount(AICreditActionReadingGeneration) + 4*aiCreditAmount(AICreditActionTheaterGeneration)
}

func (s *Service) MockExamGenerationCost() int {
	if s == nil || !s.CommercialFeaturesEnabled() || s.MiniProgramFeaturesEnabled() || !s.billing.options.Enabled {
		return 0
	}
	return mockGenerationCost()
}

// 生成与评分走已有有界任务队列；音频全部准备好后才允许开始计时。
func (s *Service) StartMockExam(userID, exam string, targets ...float64) (domain.MockExam, error) {
	exam = normalizeMockExamName(exam)
	if exam == "" {
		return domain.MockExam{}, errors.New("目前仅支持 IELTS Academic、CET4 和 CET6 模拟考试")
	}
	target := 7.0
	if len(targets) > 0 {
		target = targets[0]
	}
	if exam == "IELTS" && (math.IsNaN(target) || math.IsInf(target, 0) || target < 6 || target > 8 || target*2 != math.Trunc(target*2)) {
		return domain.MockExam{}, errors.New("目标分数须为 6.0–8.0，步长为 0.5")
	}
	if err := s.rejectDemoAccountAI(userID); err != nil {
		return domain.MockExam{}, err
	}
	engine, ok := s.generator.(mockExamEngine)
	cetEngine, cetOK := s.generator.(cetMockExamEngine)
	if ((exam == "IELTS" && !ok) || (exam != "IELTS" && !cetOK)) || s.tts == nil {
		return domain.MockExam{}, errors.New("模拟考试需要配置出题模型和听力 TTS 服务")
	}
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	items, err := s.store.ListMockExams(userID)
	if err != nil {
		return domain.MockExam{}, err
	}
	for _, item := range items {
		if item.Status == "GENERATING" && time.Since(item.UpdatedAt) < s.tasks.timeout+time.Minute {
			if normalizeMockExamName(item.Exam) != exam {
				return domain.MockExam{}, fmt.Errorf("已有 %s 模拟考试正在生成，请等待完成或失败后再创建 %s", item.Exam, exam)
			}
			item = applyMockReadyEstimate(items, item)
			return cloneMockExam(item), nil
		}
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.MockExam{}, err
	}
	now := time.Now().UTC()
	version := mockExamVersion
	if exam != "IELTS" {
		version = ai.CETMockExamPaperVersion(exam)
	}
	paper := domain.MockExam{ID: uuid.NewString(), UserID: userID, Exam: exam, Status: "GENERATING", CurrentSection: "正在生成试卷并逐题审核，请稍候", TotalDurationSeconds: 0, CreatedAt: now, UpdatedAt: now, Sections: []domain.MockExamSection{{TargetBand: target, PaperVersion: version}}}
	if err = s.ConsumeAIConfidence(userID, "MOCK_EXAM_GENERATION", paper.ID, mockGenerationCost()); err != nil {
		release()
		return domain.MockExam{}, err
	}
	saved, err := s.store.SaveMockExam(paper)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, "MOCK_EXAM_GENERATION", paper.ID, mockGenerationCost())
		return domain.MockExam{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		if exam == "IELTS" {
			s.generateMockExam(ctx, saved, target, engine)
			return
		}
		s.generateCETMockExam(ctx, saved, cetEngine)
	}) {
		release()
		s.RefundAIConfidence(userID, "MOCK_EXAM_GENERATION", paper.ID, mockGenerationCost())
		saved.Status, saved.CurrentSection = "FAILED", "出题队列繁忙，请稍后重新创建"
		return s.store.UpdateMockExam(saved)
	}
	s.trackFeature("MOCK_EXAM_GENERATION_REQUESTED")
	return cloneMockExam(applyMockReadyEstimate(items, saved)), nil
}

func normalizeMockExamName(exam string) string {
	switch strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(exam), "-", "")) {
	case "IELTS":
		return "IELTS"
	case "CET4", "CET四级", "CET四":
		return "CET4"
	case "CET6", "CET六级", "CET六":
		return "CET6"
	default:
		return ""
	}
}

func (s *Service) generateMockExam(ctx context.Context, saved domain.MockExam, target float64, engine mockExamEngine) {
	defer func() {
		if r := recover(); r != nil {
			s.failMockExam(saved, "FAILED", fmt.Errorf("generation panic: %v", r))
		}
	}()
	paper, err := engine.GenerateMockExam(ctx, target)
	if err == nil {
		err = ai.ValidateMockExamPaper(paper)
	}
	if err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	paper.ID, paper.UserID, paper.Exam = saved.ID, saved.UserID, "IELTS"
	paper.CreatedAt, paper.UpdatedAt = saved.CreatedAt, time.Now().UTC()
	paper.Status, paper.CurrentSection = "GENERATING", "题目审核完成，正在生成听力音频"
	paper.StartedAt = time.Time{}
	if err = s.updateMockJob(paper, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	for i := range paper.Sections {
		section := &paper.Sections[i]
		if section.Skill != "LISTENING" {
			continue
		}
		paper.CurrentSection = fmt.Sprintf("题目审核完成，正在生成第 %d/4 段听力音频", i+1)
		if err = s.updateMockJob(paper, "GENERATING"); err != nil {
			s.failMockExam(saved, "FAILED", err)
			return
		}
		section.AudioURLs, err = s.mockListeningAudio(ctx, paper.ID, *section)
		if err != nil {
			s.failMockExam(saved, "FAILED", fmt.Errorf("%s audio: %w", section.Key, err))
			return
		}
		section.AudioURL = section.AudioURLs[0]
	}
	if err = finalizeReviewedMockExam(&paper); err != nil {
		s.failMockExam(saved, "FAILED", fmt.Errorf("production review gate: %w", err))
		return
	}
	paper.Status, paper.CurrentSection = "READY", paper.Sections[0].Key
	paper = recordMockGenerationDuration(paper, time.Since(saved.CreatedAt))
	if err = s.updateMockJob(paper, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
	}
}

func (s *Service) generateCETMockExam(ctx context.Context, saved domain.MockExam, engine cetMockExamEngine) {
	defer func() {
		if r := recover(); r != nil {
			s.failMockExam(saved, "FAILED", fmt.Errorf("generation panic: %v", r))
		}
	}()
	examName := normalizeMockExamName(saved.Exam)
	paper, err := engine.GenerateCETMockExam(ctx, examName)
	if err == nil {
		err = ai.ValidateCETMockExamPaper(paper)
	}
	if err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	paper.ID, paper.UserID, paper.Exam = saved.ID, saved.UserID, examName
	paper.CreatedAt, paper.UpdatedAt = saved.CreatedAt, time.Now().UTC()
	paper.Status, paper.CurrentSection = "GENERATING", "题目审核完成，正在生成听力音频"
	paper.StartedAt = time.Time{}
	if err = s.updateMockJob(paper, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
		return
	}
	for i := range paper.Sections {
		section := &paper.Sections[i]
		if section.Skill != "LISTENING" {
			continue
		}
		paper.CurrentSection = "题目审核完成，正在生成 CET 听力音频"
		if err = s.updateMockJob(paper, "GENERATING"); err != nil {
			s.failMockExam(saved, "FAILED", err)
			return
		}
		section.AudioURLs, err = s.mockListeningAudio(ctx, paper.ID, *section)
		if err != nil {
			s.failMockExam(saved, "FAILED", fmt.Errorf("%s audio: %w", section.Key, err))
			return
		}
		section.AudioURL = section.AudioURLs[0]
	}
	if err = finalizeReviewedMockExam(&paper); err != nil {
		s.failMockExam(saved, "FAILED", fmt.Errorf("production review gate: %w", err))
		return
	}
	paper.Status, paper.CurrentSection = "READY", paper.Sections[0].Key
	paper = recordMockGenerationDuration(paper, time.Since(saved.CreatedAt))
	if err = s.updateMockJob(paper, "GENERATING"); err != nil {
		s.failMockExam(saved, "FAILED", err)
	}
}

var mockSpeakerLine = regexp.MustCompile(`^([A-Za-z][A-Za-z .'-]{0,40}):\s*(.*)$`)

func mockListeningTurns(script string) ([]domain.Dialogue, error) {
	var turns []domain.Dialogue
	for _, line := range strings.Split(strings.ReplaceAll(script, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if match := mockSpeakerLine.FindStringSubmatch(line); len(match) == 3 {
			turns = append(turns, domain.Dialogue{Speaker: strings.TrimSpace(match[1]), Text: strings.TrimSpace(match[2])})
		} else {
			if len(turns) == 0 {
				return nil, errors.New("listening script must start with a named speaker")
			}
			turns[len(turns)-1].Text += "\n" + line
		}
	}
	if len(turns) == 0 {
		return nil, errors.New("empty listening script")
	}
	for _, turn := range turns {
		if strings.TrimSpace(turn.Text) == "" {
			return nil, errors.New("empty listening speaker turn")
		}
	}
	return turns, nil
}

func (s *Service) mockListeningAudio(ctx context.Context, examID string, section domain.MockExamSection) ([]string, error) {
	turns, err := mockListeningTurns(section.AudioScript)
	if err != nil {
		return nil, err
	}
	styles := assignListeningVoiceStyles(turns, [2]string{"御姐音色", "沉稳大叔"})
	urls := make([]string, 0, len(turns))
	// TTS 自身有并发槽位限制；这里并行提交 speaker turn，避免整卷 40 个
	// 片段串行生成导致后台任务接近超时。按索引写回，保证播放顺序不变。
	urls = make([]string, len(turns))
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, len(turns))
	var workers sync.WaitGroup
	for i, turn := range turns {
		i, turn := i, turn
		workers.Add(1)
		go func() {
			defer workers.Done()
			if workCtx.Err() != nil {
				return
			}
			// 角色名不进入朗读文本，同一角色在整段录音内保持相同音色。
			rawURL, synthErr := s.synthesizeWithTTSContextLimit(workCtx, turn.Text, "ENGLISH", styles[i], buildDialogueTTSContext(section.Title, turns, i))
			if synthErr != nil {
				errs <- fmt.Errorf("speaker turn %d TTS: %w", i+1, synthErr)
				cancel()
				return
			}
			if strings.TrimSpace(rawURL) == "" {
				errs <- fmt.Errorf("speaker turn %d TTS returned empty audio", i+1)
				cancel()
				return
			}
			materialized, materializeErr := s.materializeAudioURL(rawURL, "mock-exam", fmt.Sprintf("%s-%s-%d", examID, section.Key, i))
			if materializeErr != nil {
				errs <- fmt.Errorf("speaker turn %d audio persistence: %w", i+1, materializeErr)
				cancel()
				return
			}
			urls[i] = materialized
		}()
	}
	workers.Wait()
	close(errs)
	for workerErr := range errs {
		if workerErr != nil {
			return nil, workerErr
		}
	}
	for i, url := range urls {
		if strings.TrimSpace(url) == "" {
			return nil, fmt.Errorf("speaker turn %d produced no audio", i+1)
		}
	}
	return urls, nil
}

// Listening recordings intentionally use at most two stable voices. Generated
// scripts can include a narrator label in addition to two participants; the
// general dialogue allocator would otherwise introduce a third timbre.
func assignListeningVoiceStyles(turns []domain.Dialogue, pair [2]string) []string {
	styles := make([]string, len(turns))
	bySpeaker := make(map[string]string, 2)
	for index, turn := range turns {
		speaker := dialogueSpeakerKey(turn, index)
		style, exists := bySpeaker[speaker]
		if !exists {
			style = pair[len(bySpeaker)%len(pair)]
			bySpeaker[speaker] = style
		}
		styles[index] = style
	}
	return styles
}

func (s *Service) updateMockJob(paper domain.MockExam, expected string) error {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	current, err := s.store.GetMockExam(paper.ID, paper.UserID)
	if err != nil {
		return err
	}
	if current.Status != expected {
		return errors.New("模拟考试状态已改变，请重新打开记录")
	}
	paper.UpdatedAt = time.Now().UTC()
	_, err = s.store.UpdateMockExam(cloneMockExam(paper))
	return err
}

func (s *Service) failMockExam(exam domain.MockExam, status string, cause error) {
	kind := mockExamFailureKind(status, cause)
	log.Printf("mock_exam_failed exam_id=%s user_id=%s status=%s failure_kind=%s err=%v", exam.ID, exam.UserID, status, kind, cause)
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	current, err := s.store.GetMockExam(exam.ID, exam.UserID)
	if err != nil {
		return
	}
	if current.Status != "GENERATING" && current.Status != "EVALUATING" {
		return
	}
	current.Status, current.Result, current.UpdatedAt = status, nil, time.Now().UTC()
	current.CurrentSection = mockExamFailureMessage(exam.ID, status, kind)
	if errors.Is(cause, ai.ErrListeningDifficulty) {
		current.CurrentSection = "听力题目未通过目标难度审核，本次内容未开放，已触发本次点数或使用次数退回。请重新生成。记录编号：" + exam.ID
	}
	if ListeningTrainingPart(exam.Exam) > 0 && (errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, context.Canceled) || errors.Is(cause, ai.ErrListeningReviewUnavailable) || errors.Is(cause, ai.ErrListeningReviewInvalid)) {
		current.CurrentSection = "听力生成或审核服务超时、暂不可用或返回异常，尚未完成难度检验，并非你的操作问题。本次内容未开放，已触发本次点数或使用次数退回。请稍后重试。记录编号：" + exam.ID
	}
	if status == "EVALUATION_FAILED" {
		current.CurrentSection = "评分服务或评分校验失败，答案已保存，请重试评分。记录编号：" + exam.ID
	}
	if _, err = s.store.UpdateMockExam(current); err != nil {
		log.Printf("mock_exam_failure_persist exam_id=%s err=%v", exam.ID, err)
	}
	if status == "FAILED" {
		action, cost := mockGenerationCharge(exam)
		s.RefundAIConfidence(exam.UserID, action, exam.ID, cost)
	}
}

// mockExamFailureKind 将底层错误转换为稳定的内部分类。错误正文只写日志，
// 页面显示分类后的中文提示，避免把供应商响应或内部实现细节泄露给用户。
func mockExamFailureKind(status string, cause error) string {
	if status == "EVALUATION_FAILED" {
		return "EVALUATION"
	}
	if cause == nil {
		return "PROCESSING"
	}
	value := strings.ToLower(cause.Error())
	switch {
	case errors.Is(cause, context.DeadlineExceeded), errors.Is(cause, context.Canceled):
		return "TIMEOUT"
	case strings.Contains(value, "production review gate"), strings.Contains(value, "audio production review"):
		return "AUDIO_REVIEW"
	case strings.Contains(value, "audio"), strings.Contains(value, "speaker turn"), strings.Contains(value, "tts"):
		return "AUDIO_GENERATION"
	case strings.Contains(value, "review"), strings.Contains(value, "failed after"), strings.Contains(value, "generation"), strings.Contains(value, "source"):
		return "PAPER_REVIEW"
	default:
		return "PROCESSING"
	}
}

func mockExamFailureMessage(id, status, kind string) string {
	suffix := "记录编号：" + id
	if status == "EVALUATION_FAILED" || kind == "EVALUATION" {
		return "评分服务或评分校验失败，答案已保存，请重试评分。" + suffix
	}
	switch kind {
	case "TIMEOUT":
		return "试卷生成或听力音频处理超时，内容未开放，本次点数或使用次数已退回，请稍后重试。" + suffix
	case "PAPER_REVIEW":
		return "试卷内容生成或独立质量审核未通过，内容未开放，本次点数或使用次数已退回，请重新创建。" + suffix
	case "AUDIO_GENERATION":
		return "听力音频生成或保存失败，试卷未开放，本次点数或使用次数已退回，请稍后重试。" + suffix
	case "AUDIO_REVIEW":
		return "听力音频生产审核未通过，试卷未开放，本次点数或使用次数已退回，请重新创建。" + suffix
	default:
		return "试卷生成或音频审核失败，内容未开放，本次点数或使用次数已退回，请稍后重试。" + suffix
	}
}

func (s *Service) MockExam(userID, examID string) (domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	exam, err := s.store.GetMockExam(examID, userID)
	if err != nil {
		return domain.MockExam{}, errors.New("模拟考试记录不存在或无权访问")
	}
	exam, err = s.recoverStaleMockExamLocked(userID, exam)
	if err != nil {
		return domain.MockExam{}, err
	}
	items, _ := s.store.ListMockExams(userID)
	exam = applyMockReadyEstimate(items, exam)
	if exam.Status != "GENERATING" && exam.Status != "FAILED" {
		if approvalErr := ensureMockExamProductionApproved(exam); approvalErr != nil {
			return cloneMockExam(redactMockExamForQuality(exam)), nil
		}
	}
	if exam.Result != nil {
		if approvalErr := ensureMockExamEvaluationProductionApproved(exam); approvalErr != nil {
			exam = redactMockEvaluationForQuality(exam)
		}
	}
	return cloneMockExam(quarantineListeningTraining(exam)), nil
}
func (s *Service) MockExams(userID string) ([]domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	items, err := s.store.ListMockExams(userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		var recoverErr error
		items[i], recoverErr = s.recoverStaleMockExamLocked(userID, items[i])
		if recoverErr != nil {
			return nil, recoverErr
		}
		if items[i].Status != "GENERATING" && items[i].Status != "FAILED" {
			if approvalErr := ensureMockExamProductionApproved(items[i]); approvalErr != nil {
				items[i] = redactMockExamForQuality(items[i])
			}
		}
		if items[i].Result != nil {
			if approvalErr := ensureMockExamEvaluationProductionApproved(items[i]); approvalErr != nil {
				items[i] = redactMockEvaluationForQuality(items[i])
			}
		}
		items[i] = applyMockReadyEstimate(items, items[i])
		items[i] = cloneMockExam(items[i])
	}
	result := make([]domain.MockExam, 0, len(items))
	for _, item := range items {
		if ListeningTrainingPart(item.Exam) == 0 {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Service) recoverStaleMockExamLocked(userID string, exam domain.MockExam) (domain.MockExam, error) {
	// 查询不触发重启后假恢复；进程内队列丢失或超时的遗留任务显式失败，允许用户删除/重试。
	if (exam.Status != "GENERATING" && exam.Status != "EVALUATING") || time.Since(exam.UpdatedAt) <= s.tasks.timeout+time.Minute {
		return exam, nil
	}
	if exam.Status == "GENERATING" {
		exam.Status = "FAILED"
		action, cost := mockGenerationCharge(exam)
		s.RefundAIConfidence(userID, action, exam.ID, cost)
	} else {
		exam.Status = "EVALUATION_FAILED"
	}
	exam.CurrentSection = "处理超时或服务已重启，请返回后重试。记录编号：" + exam.ID
	exam.UpdatedAt = time.Now().UTC()
	return s.store.UpdateMockExam(exam)
}

func applyMockReadyEstimate(history []domain.MockExam, current domain.MockExam) domain.MockExam {
	remaining, samples := estimateMockReadySeconds(history, current)
	current.EstimatedReadySeconds = remaining
	current.GenerationEstimateSamples = samples
	return current
}

func estimateMockReadySeconds(history []domain.MockExam, current domain.MockExam) (int, int) {
	if current.Status != "GENERATING" {
		return 0, 0
	}
	var total int
	var count int
	for _, item := range history {
		if item.ID == current.ID || item.Exam != current.Exam {
			continue
		}
		if item.Status == "READY" || item.Status == "IN_PROGRESS" || item.Status == "EVALUATING" || item.Status == "EVALUATION_FAILED" || item.Status == "COMPLETED" {
			elapsed := mockGenerationDurationSeconds(item)
			if elapsed > 0 && elapsed < 7200 {
				total += elapsed
				count++
			}
		}
	}
	if count == 0 {
		return 0, 0
	}
	estimate := total / count
	remaining := estimate - int(time.Since(current.CreatedAt).Seconds())
	if remaining <= 0 {
		return -1, count
	}
	return remaining, count
}

func mockGenerationDurationSeconds(exam domain.MockExam) int {
	if len(exam.Sections) > 0 && exam.Sections[0].GenerationDurationSeconds > 0 {
		return exam.Sections[0].GenerationDurationSeconds
	}
	return 0
}

func recordMockGenerationDuration(exam domain.MockExam, elapsed time.Duration) domain.MockExam {
	if len(exam.Sections) == 0 {
		return exam
	}
	seconds := int(elapsed.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	exam.Sections[0].GenerationDurationSeconds = seconds
	return exam
}

// RetryMockExam creates an independent attempt from a completed, approved
// paper. The source result remains immutable and no generation work is queued.
func (s *Service) RetryMockExam(userID, id string) (domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	source, err := s.store.GetMockExam(id, userID)
	if err != nil {
		return domain.MockExam{}, errors.New("模拟考试记录不存在或无权访问")
	}
	if source.Status != "COMPLETED" {
		return domain.MockExam{}, errors.New("仅可重新挑战已完成的模拟考试")
	}
	if err = validateMockExamForService(source); err != nil {
		return domain.MockExam{}, errors.New("原试卷内容已失效，无法重新挑战，请生成新试卷")
	}
	if err = ensureMockExamProductionApproved(source); err != nil {
		return domain.MockExam{}, productionGateError("模拟考试", err)
	}
	if len(source.Sections) == 0 {
		return domain.MockExam{}, errors.New("原试卷内容不完整，无法重新挑战")
	}

	now := time.Now().UTC()
	retry := cloneMockExam(source)
	retry.ID = uuid.NewString()
	retry.Status = "READY"
	retry.CurrentSection = retry.Sections[0].Key
	retry.Result = nil
	retry.EvaluationApproval = domain.ProductionApproval{}
	retry.StartedAt = time.Time{}
	retry.SubmittedAt = time.Time{}
	retry.CreatedAt = now
	retry.UpdatedAt = now
	retry.EstimatedReadySeconds = 0
	retry.GenerationEstimateSamples = 0
	for index := range retry.Sections {
		retry.Sections[index].Answers = nil
		retry.Sections[index].Responses = nil
		retry.Sections[index].GenerationDurationSeconds = 0
	}
	saved, err := s.store.SaveMockExam(retry)
	if err != nil {
		return domain.MockExam{}, err
	}
	s.trackFeature("MOCK_EXAM_RETRY_CREATED")
	return cloneMockExam(saved), nil
}

func (s *Service) DeleteMockExam(userID, id string) error {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	item, err := s.store.GetMockExam(id, userID)
	if err != nil {
		return errors.New("考试记录不存在或无权访问")
	}
	if item.Status != "READY" && item.Status != "COMPLETED" && item.Status != "FAILED" && item.Status != "EVALUATION_FAILED" {
		return errors.New("仅可删除已就绪、已完成或失败的模拟考试记录")
	}
	return s.store.DeleteMockExam(id, userID)
}
func (s *Service) BeginMockExam(userID, examID string) (domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	exam, err := s.store.GetMockExam(examID, userID)
	if err != nil {
		return domain.MockExam{}, errors.New("模拟考试记录不存在或无权访问")
	}
	if exam.Status == "IN_PROGRESS" {
		if err = ensureMockExamProductionApproved(exam); err != nil {
			return domain.MockExam{}, productionGateError("模拟考试", err)
		}
		if ListeningTrainingPart(exam.Exam) > 0 {
			if err := validateMockExamForService(exam); err != nil {
				return domain.MockExam{}, errors.New("听力内容或难度审核未通过，请重新生成")
			}
		}
		return cloneMockExam(exam), nil
	}
	if exam.Status != "READY" {
		return domain.MockExam{}, errors.New("试卷和音频尚未就绪，暂时不能开始")
	}
	if err = validateMockExamForService(exam); err != nil {
		return domain.MockExam{}, errors.New("试卷质量校验未通过，请重新创建")
	}
	if err = ensureMockExamProductionApproved(exam); err != nil {
		return domain.MockExam{}, productionGateError("模拟考试", err)
	}
	for _, section := range exam.Sections {
		if section.Skill != "LISTENING" {
			continue
		}
		if strings.TrimSpace(section.AudioURL) == "" || len(section.AudioURLs) == 0 {
			return domain.MockExam{}, errors.New("听力音频尚未完整生成，请刷新后重试")
		}
		for _, audioURL := range section.AudioURLs {
			if strings.TrimSpace(audioURL) == "" {
				return domain.MockExam{}, errors.New("听力分段音频不完整，请重新生成试卷")
			}
		}
	}
	for _, section := range exam.Sections {
		if section.Skill == "LISTENING" && section.AudioURL == "" {
			return domain.MockExam{}, errors.New("听力音频未就绪")
		}
	}
	exam.Status, exam.CurrentSection = "IN_PROGRESS", exam.Sections[0].Key
	exam.StartedAt, exam.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	return s.store.UpdateMockExam(exam)
}
func (s *Service) SaveMockExamAnswers(userID, examID, sectionKey string, answers, responses []string) (domain.MockExam, error) {
	return s.saveMockAnswers(userID, examID, sectionKey, answers, responses, false)
}
func (s *Service) SubmitMockExamSection(userID, examID, sectionKey string, answers, responses []string) (domain.MockExam, error) {
	return s.saveMockAnswers(userID, examID, sectionKey, answers, responses, true)
}
func (s *Service) saveMockAnswers(userID, examID, sectionKey string, answers, responses []string, advance bool) (domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	exam, err := s.store.GetMockExam(examID, userID)
	if err != nil {
		return domain.MockExam{}, errors.New("模拟考试记录不存在或无权访问")
	}
	exam = cloneMockExam(exam)
	if ListeningTrainingPart(exam.Exam) > 0 {
		if err := validateMockExamForService(exam); err != nil {
			return domain.MockExam{}, errors.New("听力内容或难度审核未通过，暂不能作答")
		}
	}
	if exam.Status != "IN_PROGRESS" {
		return domain.MockExam{}, errors.New("当前考试状态不允许修改答案")
	}
	if exam.StartedAt.IsZero() || (ListeningTrainingPart(exam.Exam) == 0 && time.Since(exam.StartedAt) >= time.Duration(exam.TotalDurationSeconds)*time.Second) {
		return domain.MockExam{}, errors.New("考试时间已结束，已保存的答案可提交评分")
	}
	current := -1
	for i := range exam.Sections {
		if exam.Sections[i].Key == exam.CurrentSection {
			current = i
		}
	}
	for i := range exam.Sections {
		section := &exam.Sections[i]
		if section.Key != sectionKey {
			continue
		}
		if i > current {
			return domain.MockExam{}, errors.New("请按顺序完成考试")
		}
		if len(answers) > len(section.Questions) || len(responses) > len(section.WritingPrompts) {
			return domain.MockExam{}, errors.New("答案数量与试卷不符")
		}
		for _, answer := range answers {
			if len([]rune(answer)) > 200 {
				return domain.MockExam{}, errors.New("单题答案过长")
			}
		}
		for _, response := range responses {
			if len([]rune(response)) > 30000 {
				return domain.MockExam{}, errors.New("作文内容过长")
			}
		}
		section.Answers, section.Responses = append([]string{}, answers...), append([]string{}, responses...)
		if advance && i == current && i+1 < len(exam.Sections) {
			exam.CurrentSection = exam.Sections[i+1].Key
		}
		exam.UpdatedAt = time.Now().UTC()
		return s.store.UpdateMockExam(exam)
	}
	return domain.MockExam{}, errors.New("模拟考试部分不存在")
}
func (s *Service) FinishMockExam(userID, examID string) (domain.MockExam, error) {
	s.mockMu.Lock()
	defer s.mockMu.Unlock()
	exam, err := s.store.GetMockExam(examID, userID)
	if err != nil {
		return domain.MockExam{}, errors.New("模拟考试记录不存在或无权访问")
	}
	if ListeningTrainingPart(exam.Exam) > 0 {
		return domain.MockExam{}, errors.New("请在听力专项页面提交答案")
	}
	if exam.Status == "COMPLETED" || exam.Status == "EVALUATING" {
		return cloneMockExam(exam), nil
	}
	if exam.Status != "IN_PROGRESS" && exam.Status != "EVALUATION_FAILED" {
		return domain.MockExam{}, errors.New("该试卷尚不能提交评分")
	}
	if err = validateMockExamForService(exam); err != nil {
		return domain.MockExam{}, errors.New("该记录为旧版或未通过审核的试卷，请创建新试卷")
	}
	if _, ok := s.generator.(writingEngine); !ok {
		return domain.MockExam{}, errors.New("写作评分模型未配置")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.MockExam{}, err
	}
	exam.Status, exam.CurrentSection = "EVALUATING", "正在核对答案并评分"
	if exam.SubmittedAt.IsZero() {
		exam.SubmittedAt = time.Now().UTC()
	}
	exam.UpdatedAt = time.Now().UTC()
	saved, err := s.store.UpdateMockExam(exam)
	if err != nil {
		release()
		return domain.MockExam{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		defer func() {
			if r := recover(); r != nil {
				s.failMockExam(saved, "EVALUATION_FAILED", fmt.Errorf("assessment panic: %v", r))
			}
		}()
		completed, scoreErr := s.evaluateMockExam(ctx, cloneMockExam(saved))
		if scoreErr != nil {
			s.failMockExam(saved, "EVALUATION_FAILED", scoreErr)
			return
		}
		reviewer, reviewErr := productionReviewer[mockEvaluationProductionReviewer](s.generator)
		if reviewErr == nil {
			completed.EvaluationApproval, reviewErr = reviewer.ReviewMockExamEvaluation(ctx, completed)
		} else {
			hash, _ := contentquality.MockExamEvaluationContentHash(completed)
			completed.EvaluationApproval = inconclusiveApproval(contentquality.MockEvaluationRubricVersion, hash, reviewErr.Error())
		}
		if reviewErr == nil {
			reviewErr = ensureMockExamEvaluationProductionApproved(completed)
		}
		if reviewErr != nil {
			completed.Status = "EVALUATION_FAILED"
			completed.CurrentSection = "评分未通过独立质量审核，答案已保存，可稍后重试。记录编号：" + completed.ID
			if scoreErr = s.updateMockJob(completed, "EVALUATING"); scoreErr != nil {
				s.failMockExam(saved, "EVALUATION_FAILED", scoreErr)
			}
			return
		}
		if scoreErr = s.updateMockJob(completed, "EVALUATING"); scoreErr != nil {
			s.failMockExam(saved, "EVALUATION_FAILED", scoreErr)
		}
	}) {
		release()
		saved.Status, saved.CurrentSection = "EVALUATION_FAILED", "评分队列繁忙，答案已保存，请稍后重试"
		return s.store.UpdateMockExam(saved)
	}
	return cloneMockExam(saved), nil
}

func validateMockExamForService(exam domain.MockExam) error {
	if err := ensureMockExamProductionApproved(exam); err != nil {
		return err
	}
	if part := ListeningTrainingPart(exam.Exam); part > 0 {
		if len(exam.Sections) != 1 {
			return errors.New("听力训练必须包含一个完整部分")
		}
		return ai.ValidateListeningTraining(exam.Sections[0], part)
	}
	if normalizeMockExamName(exam.Exam) == "IELTS" {
		return ai.ValidateMockExamPaper(exam)
	}
	return ai.ValidateCETMockExamPaper(exam)
}

func (s *Service) evaluateMockExam(ctx context.Context, exam domain.MockExam) (domain.MockExam, error) {
	if normalizeMockExamName(exam.Exam) != "IELTS" {
		return s.evaluateCETMockExam(ctx, exam)
	}
	rc, rt := scoreMockSections(exam.Sections, "READING")
	lc, lt := scoreMockSections(exam.Sections, "LISTENING")
	var writing domain.MockExamSection
	for _, section := range exam.Sections {
		if section.Skill == "WRITING" {
			writing = section
		}
	}
	engine, ok := s.generator.(writingEngine)
	if !ok {
		return domain.MockExam{}, errors.New("写作评分模型未配置")
	}
	if len(writing.WritingPrompts) != 2 {
		return domain.MockExam{}, errors.New("写作题目不完整")
	}
	var evaluations [2]domain.WritingEvaluation
	for i := range evaluations {
		essay := ""
		if i < len(writing.Responses) {
			essay = strings.TrimSpace(writing.Responses[i])
		}
		// 未作答计零是明确规则，不作为模型失败时的替代评分。
		if essay == "" {
			evaluations[i].Issues = []string{fmt.Sprintf("Task %d 未作答，按未作答计 0 分", i+1)}
			continue
		}
		var err error
		evaluations[i], err = engine.EvaluateWriting(ctx, "IELTS", writing.WritingPrompts[i], essay, []int{1200, 2400}[i], 0)
		if err != nil {
			return domain.MockExam{}, fmt.Errorf("Task %d: %w", i+1, err)
		}
		if b := evaluations[i].BandEstimate; math.IsNaN(b) || math.IsInf(b, 0) || b < 0 || b > 9 {
			return domain.MockExam{}, errors.New("写作评分不在有效范围内")
		}
	}
	rb, lb := ieltsAcademicReadingBand(rc, rt), ieltsListeningBand(lc, lt)
	wb := roundHalf((evaluations[0].OverallScore + 2*evaluations[1].OverallScore) * 9 / 300)
	result := &domain.MockExamResult{ReadingCorrect: rc, ReadingTotal: rt, ListeningCorrect: lc, ListeningTotal: lt, ReadingScore: rb, ListeningScore: lb, WritingScore: (evaluations[0].OverallScore + 2*evaluations[1].OverallScore) / 3, WritingTask1Score: evaluations[0].OverallScore, WritingTask2Score: evaluations[1].OverallScore, WritingBand: wb, WritingTask1Band: evaluations[0].BandEstimate, WritingTask2Band: evaluations[1].BandEstimate, EstimatedBand: roundHalf((rb + lb + wb) / 3), QualityStatus: "TRAINING_ESTIMATE_UNCALIBRATED", CompletedAt: time.Now().UTC()}
	result.WritingEvaluations = append([]domain.WritingEvaluation{}, evaluations[:]...)
	result.Feedback = "听力、阅读按参考原始分区间估算；写作 Task 2 占双倍权重。本报告为三科训练估分，不含口语，试题尚未经考生样本等值校准。"
	for _, e := range evaluations {
		result.Strengths = append(result.Strengths, e.Strengths...)
		result.Weaknesses = append(result.Weaknesses, e.Issues...)
		result.Recommendations = append(result.Recommendations, e.Suggestions...)
	}
	for _, section := range exam.Sections {
		if len(section.Questions) == 0 {
			continue
		}
		wrong := map[string]int{}
		for i, q := range section.Questions {
			if i >= len(section.Answers) || !mockAnswerMatches(section.Answers[i], q) {
				wrong[q.Type]++
			}
		}
		for _, kind := range []string{"multiple_choice", "sentence_completion", "true_false_not_given", "matching_information"} {
			if n := wrong[kind]; n > 0 {
				result.Recommendations = append(result.Recommendations, fmt.Sprintf("%s：%s 错误或未答 %d 题，建议结合原文定位和同义替换复盘。", section.Title, mockQuestionLabel(kind), n))
			}
		}
	}
	exam.Result, exam.Status, exam.CurrentSection, exam.UpdatedAt = result, "COMPLETED", "RESULT", time.Now().UTC()
	return exam, nil
}
func mockQuestionLabel(kind string) string {
	switch kind {
	case "sentence_completion":
		return "填空题"
	case "true_false_not_given":
		return "判断题"
	case "matching_information":
		return "信息匹配题"
	default:
		return "选择题"
	}
}
func mockAnswerMatches(answer string, q domain.QuizQuestion) bool {
	normalize := func(v string) string { return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(v)), " ")) }
	if strings.TrimSpace(answer) == "" {
		return false
	}
	if q.Type == "sentence_completion" && len(strings.Fields(answer)) > 3 {
		return false
	}
	return normalize(answer) == normalize(q.AnswerKey)
}
func scoreMockSections(sections []domain.MockExamSection, skill string) (int, int) {
	correct, total := 0, 0
	for _, section := range sections {
		if section.Skill == skill {
			for i, q := range section.Questions {
				total++
				if i < len(section.Answers) && mockAnswerMatches(section.Answers[i], q) {
					correct++
				}
			}
		}
	}
	return correct, total
}

// 官方公布的典型参照点：23≈6、30≈7、35≈8；具体试卷需另行等值校准。
func ieltsAcademicReadingBand(correct, total int) float64 {
	return mockReferenceBand(correct, total, false)
}
func ieltsListeningBand(correct, total int) float64 { return mockReferenceBand(correct, total, true) }
func mockReferenceBand(correct, total int, listening bool) float64 {
	if total != 40 || correct < 0 || correct > 40 {
		return 0
	}
	thresholds := []int{0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 15, 19, 23, 27, 30, 33, 35, 37, 39}
	if listening {
		thresholds = []int{0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 18, 23, 26, 30, 32, 35, 37, 39}
	}
	for i := len(thresholds) - 1; i >= 0; i-- {
		if correct >= thresholds[i] {
			return float64(i) / 2
		}
	}
	return 0
}
func roundHalf(value float64) float64 { return math.Round(value*2) / 2 }
func cloneMockExam(exam domain.MockExam) domain.MockExam {
	// 隔离 MemoryStore 的浅拷贝，防止排队任务与查询共享切片。
	data, _ := json.Marshal(exam)
	var copy domain.MockExam
	_ = json.Unmarshal(data, &copy)
	return copy
}
