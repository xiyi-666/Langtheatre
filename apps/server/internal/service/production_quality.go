package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
)

const productionApprovalMaxAge = 0

type theaterProductionReviewer interface {
	ReviewTheater(context.Context, domain.Theater) (domain.ProductionApproval, error)
}

type readingProductionReviewer interface {
	ReviewReading(context.Context, domain.ReadingMaterial) (domain.ProductionApproval, error)
}

type writingProductionReviewer interface {
	ReviewWritingPrompt(context.Context, domain.WritingSession) (domain.ProductionApproval, error)
	ReviewWritingEvaluation(context.Context, domain.WritingSession) (domain.ProductionApproval, error)
}

type speakingProductionReviewer interface {
	ReviewSpeakingPrompts(context.Context, []domain.SpeakingPrompt) ([]domain.ProductionApproval, error)
	ReviewSpeakingEvaluation(context.Context, domain.SpeakingSession) (domain.ProductionApproval, error)
}

type mockEvaluationProductionReviewer interface {
	ReviewMockExamEvaluation(context.Context, domain.MockExam) (domain.ProductionApproval, error)
}

func inconclusiveApproval(rubric, contentHash, reason string) domain.ProductionApproval {
	checks := []domain.QualityCheck{}
	if strings.TrimSpace(reason) != "" {
		checks = append(checks, domain.QualityCheck{Key: "review", Status: contentquality.CheckFailed, Reason: strings.TrimSpace(reason)})
	}
	return domain.ProductionApproval{
		Status: contentquality.ApprovalInconclusive, RubricVersion: rubric,
		Reviewer: "review-service-unavailable", ContentHash: contentHash,
		ReviewedAt: time.Now().UTC(), Checks: checks,
	}
}

func productionReviewer[T any](generator TheaterGenerator) (T, error) {
	reviewer, ok := any(generator).(T)
	if !ok {
		var zero T
		return zero, errors.New("独立生产质量审核服务未配置")
	}
	return reviewer, nil
}

func productionPolicy(rubric string, checks []string) contentquality.ApprovalPolicy {
	return contentquality.ApprovalPolicy{
		RubricVersion:  rubric,
		RequiredChecks: checks,
		MaxAge:         productionApprovalMaxAge,
		Now:            time.Now().UTC(),
	}
}

func ensureTheaterProductionApproved(theater domain.Theater) error {
	hash, err := contentquality.TheaterContentHash(theater)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(theater.ProductionApproval, hash, productionPolicy(contentquality.TheaterRubricVersion, contentquality.TheaterRequiredChecks))
}

func ensureReadingProductionApproved(material domain.ReadingMaterial) error {
	hash, err := contentquality.ReadingContentHash(material)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(material.ProductionApproval, hash, productionPolicy(contentquality.ReadingRubricVersion, contentquality.ReadingRequiredChecks))
}

func ensureWritingPromptProductionApproved(session domain.WritingSession) error {
	hash, err := contentquality.WritingPromptContentHash(session.Exam, session.TimeLimitSeconds, session.Prompt)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(session.PromptApproval, hash, productionPolicy(contentquality.WritingPromptRubricVersion, contentquality.WritingPromptRequiredChecks))
}

func ensureWritingEvaluationProductionApproved(session domain.WritingSession) error {
	hash, err := contentquality.WritingEvaluationContentHash(session)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(session.EvaluationApproval, hash, productionPolicy(contentquality.WritingEvaluationRubricVersion, contentquality.WritingEvaluationRequiredChecks))
}

func ensureMockExamProductionApproved(exam domain.MockExam) error {
	hash, err := contentquality.MockExamContentHash(exam)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(exam.ProductionApproval, hash, productionPolicy(contentquality.MockExamRubricVersion, contentquality.MockExamRequiredChecks))
}

func ensureMockExamEvaluationProductionApproved(exam domain.MockExam) error {
	hash, err := contentquality.MockExamEvaluationContentHash(exam)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(exam.EvaluationApproval, hash, productionPolicy(contentquality.MockEvaluationRubricVersion, contentquality.MockEvaluationRequiredChecks))
}

func approveDeterministicListeningEvaluation(exam *domain.MockExam) error {
	if exam == nil || ListeningTrainingPart(exam.Exam) == 0 || exam.Result == nil {
		return contentquality.ErrProductionApprovalRequired
	}
	hash, err := contentquality.MockExamEvaluationContentHash(*exam)
	if err != nil {
		return err
	}
	exam.EvaluationApproval = contentquality.ApprovedApproval(contentquality.MockEvaluationRubricVersion, "server-objective-scoring-v1", hash, time.Now().UTC(), []domain.QualityCheck{
		{Key: "input_integrity", Status: contentquality.CheckPassed, Reason: "answers are bound to the approved listening revision"},
		{Key: "objective_scoring", Status: contentquality.CheckPassed, Reason: "answers were scored by deterministic exact-match rules"},
		{Key: "subjective_review", Status: contentquality.CheckPassed, Reason: "this objective-only result contains no subjective AI score"},
		{Key: "score_consistency", Status: contentquality.CheckPassed, Reason: "reported correct count and percentage were recomputed server-side"},
	})
	return ensureMockExamEvaluationProductionApproved(*exam)
}

func approveReviewedListeningTraining(exam *domain.MockExam) error {
	if exam == nil || len(exam.Sections) != 1 {
		return contentquality.ErrProductionApprovalRequired
	}
	section := &exam.Sections[0]
	if err := ai.ValidateListeningDifficulty(*section); err != nil {
		return err
	}
	if len(section.AudioURLs) == 0 || strings.TrimSpace(section.AudioURL) == "" {
		return errors.New("听力音频未完整生成")
	}
	for _, audioURL := range section.AudioURLs {
		if strings.TrimSpace(audioURL) == "" {
			return errors.New("听力音频未完整生成")
		}
	}
	sectionHash, err := contentquality.MockExamSectionContentHash(*section)
	if err != nil {
		return err
	}
	audit := section.ListeningDifficulty
	checks := []domain.QualityCheck{
		{Key: "completeness", Status: contentquality.CheckPassed},
		{Key: "answer_integrity", Status: contentquality.CheckPassed},
		{Key: "difficulty", Status: contentquality.CheckPassed},
		{Key: "media", Status: contentquality.CheckPassed},
	}
	section.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, audit.ReviewerModel, sectionHash, audit.ReviewedAt, checks)
	examHash, err := contentquality.MockExamContentHash(*exam)
	if err != nil {
		return err
	}
	exam.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, audit.ReviewerModel, examHash, audit.ReviewedAt, checks)
	return nil
}

func finalizeReviewedMockExam(exam *domain.MockExam) error {
	if exam == nil || len(exam.Sections) == 0 {
		return contentquality.ErrProductionApprovalRequired
	}
	reviewers := make([]string, 0, len(exam.Sections))
	for i := range exam.Sections {
		section := &exam.Sections[i]
		hash, err := contentquality.MockExamSectionContentHash(*section)
		if err != nil {
			return err
		}
		approval := section.ProductionApproval
		if strings.EqualFold(approval.Status, contentquality.ApprovalApproved) {
			if err = contentquality.ValidateProductionApproval(approval, hash, productionPolicy(contentquality.MockExamRubricVersion, contentquality.MockExamRequiredChecks)); err != nil {
				return err
			}
			reviewers = append(reviewers, approval.Reviewer)
			continue
		}
		if !strings.EqualFold(approval.Status, contentquality.ApprovalPending) || approval.RubricVersion != contentquality.MockExamRubricVersion || strings.TrimSpace(approval.Reviewer) == "" || !strings.EqualFold(approval.ContentHash, hash) || approval.ReviewedAt.IsZero() {
			return contentquality.ErrProductionApprovalRequired
		}
		passed := map[string]bool{}
		allowed := map[string]bool{"completeness": true, "answer_integrity": true, "difficulty": true}
		for _, check := range approval.Checks {
			if !allowed[check.Key] || check.Status != contentquality.CheckPassed || passed[check.Key] {
				return contentquality.ErrProductionApprovalRequired
			}
			passed[check.Key] = true
		}
		for _, key := range []string{"completeness", "answer_integrity", "difficulty"} {
			if !passed[key] {
				return contentquality.ErrProductionApprovalRequired
			}
		}
		if section.Skill == "LISTENING" {
			if strings.TrimSpace(section.AudioURL) == "" || len(section.AudioURLs) == 0 {
				return errors.New("听力音频未完整生成")
			}
			for _, audioURL := range section.AudioURLs {
				if strings.TrimSpace(audioURL) == "" {
					return errors.New("听力音频未完整生成")
				}
			}
		}
		checks := append([]domain.QualityCheck(nil), approval.Checks...)
		checks = append(checks, domain.QualityCheck{Key: "media", Status: contentquality.CheckPassed, Reason: "required media assets were generated and persisted before release"})
		section.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, approval.Reviewer, hash, time.Now().UTC(), checks)
		reviewers = append(reviewers, approval.Reviewer)
	}
	examHash, err := contentquality.MockExamContentHash(*exam)
	if err != nil {
		return err
	}
	exam.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, "section-review-chain:"+strings.Join(reviewers, ","), examHash, time.Now().UTC(), []domain.QualityCheck{
		{Key: "completeness", Status: contentquality.CheckPassed, Reason: "every required section has a valid revision-bound production review"},
		{Key: "answer_integrity", Status: contentquality.CheckPassed, Reason: "every scored section passed independent blind answer verification"},
		{Key: "difficulty", Status: contentquality.CheckPassed, Reason: "every section passed its configured difficulty and form review"},
		{Key: "media", Status: contentquality.CheckPassed, Reason: "all required listening media assets are complete"},
	})
	return ensureMockExamProductionApproved(*exam)
}

func ensureSpeakingPromptsProductionApproved(session domain.SpeakingSession) error {
	if len(session.Prompts) == 0 {
		return contentquality.ErrProductionApprovalRequired
	}
	for _, prompt := range session.Prompts {
		if prompt.ProductionApproval == nil {
			return contentquality.ErrProductionApprovalRequired
		}
		if strings.TrimSpace(prompt.AudioURL) == "" {
			return fmt.Errorf("%w：口语题目音频缺失", contentquality.ErrProductionApprovalRequired)
		}
		hash, err := contentquality.SpeakingPromptContentHash(prompt)
		if err != nil {
			return err
		}
		if err = contentquality.ValidateProductionApproval(*prompt.ProductionApproval, hash, productionPolicy(contentquality.SpeakingPromptRubricVersion, contentquality.SpeakingPromptRequiredChecks)); err != nil {
			return err
		}
	}
	return nil
}

func ensureSpeakingEvaluationProductionApproved(session domain.SpeakingSession) error {
	hash, err := contentquality.SpeakingEvaluationContentHash(session)
	if err != nil {
		return err
	}
	return contentquality.ValidateProductionApproval(session.EvaluationApproval, hash, productionPolicy(contentquality.SpeakingEvaluationRubricVersion, contentquality.SpeakingEvaluationRequiredChecks))
}

func productionGateError(kind string, err error) error {
	if errors.Is(err, contentquality.ErrProductionApprovalRequired) {
		return fmt.Errorf("%s尚未通过生产质量审核，当前内容已隔离，不能练习、评分或分享", kind)
	}
	return fmt.Errorf("%s质量校验失败，请联系管理员并提供记录编号", kind)
}

func approvalIsDemoOnly(approval domain.ProductionApproval) bool {
	return strings.EqualFold(strings.TrimSpace(approval.Status), contentquality.ApprovalDemoOnly)
}

func redactTheaterForQuality(theater domain.Theater) domain.Theater {
	theater.SceneDescription = ""
	theater.Characters = nil
	theater.Dialogues = nil
	theater.QuizQuestions = nil
	theater.ShareCode = ""
	if theater.Status != "GENERATING" && theater.Status != "QUEUED" && theater.Status != "FAILED" {
		theater.Status = "QUALITY_REVIEW_PENDING"
	}
	theater.GenerationMessage = "内容尚未通过生产质量审核，暂未开放"
	return theater
}

func redactReadingForQuality(material domain.ReadingMaterial) domain.ReadingMaterial {
	material.Passage = ""
	material.Vocabulary = nil
	material.Questions = nil
	material.AudioURL = ""
	material.AudioURLs = nil
	material.VocabularyItems = nil
	material.AssociationSentences = nil
	material.GrammarInsights = nil
	if material.Status != "GENERATING" && material.Status != "QUEUED" && material.Status != "FAILED" {
		material.Status = "QUALITY_REVIEW_PENDING"
	}
	material.GenerationMessage = "内容尚未通过生产质量审核，暂未开放"
	return material
}

func redactWritingForQuality(session domain.WritingSession) domain.WritingSession {
	if err := ensureWritingPromptProductionApproved(session); err != nil {
		session.Prompt = domain.WritingPrompt{}
		if session.Status != "FAILED" && session.Status != "EVALUATION_FAILED" {
			session.Status = "QUALITY_REVIEW_PENDING"
		}
		session.ProgressMessage = "写作题目尚未通过生产质量审核，暂未开放"
	}
	if session.Evaluation != nil {
		if err := ensureWritingEvaluationProductionApproved(session); err != nil {
			session.Evaluation = nil
		}
	}
	return session
}

func redactMockExamForQuality(exam domain.MockExam) domain.MockExam {
	exam.Sections = nil
	exam.Result = nil
	if exam.Status != "GENERATING" && exam.Status != "FAILED" {
		exam.Status = "QUALITY_REVIEW_PENDING"
	}
	exam.CurrentSection = "试卷尚未通过生产质量审核，暂未开放"
	return exam
}

func redactMockEvaluationForQuality(exam domain.MockExam) domain.MockExam {
	exam.Result = nil
	if exam.Status == "COMPLETED" {
		exam.Status = "EVALUATION_FAILED"
	}
	exam.CurrentSection = "评分尚未通过生产质量审核，答案已保留，可稍后重试"
	return exam
}
