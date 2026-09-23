package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

const cetMockQualityStatus = "TRAINING_PERCENTAGE_UNCALIBRATED"

func (s *Service) evaluateCETMockExam(ctx context.Context, exam domain.MockExam) (domain.MockExam, error) {
	examName := normalizeMockExamName(exam.Exam)
	if examName != "CET4" && examName != "CET6" {
		return domain.MockExam{}, errors.New("unsupported CET mock exam")
	}
	engine, ok := s.generator.(writingEngine)
	if !ok {
		return domain.MockExam{}, errors.New("写作评分模型未配置")
	}
	lc, lt := scoreMockSections(exam.Sections, "LISTENING")
	rc, rt := scoreMockSections(exam.Sections, "READING")
	listeningScore, err := cetObjectiveScore(exam.Sections, "LISTENING")
	if err != nil {
		return domain.MockExam{}, err
	}
	readingScore, err := cetObjectiveScore(exam.Sections, "READING")
	if err != nil {
		return domain.MockExam{}, err
	}
	writing, translation, err := cetWritingAndTranslationSections(exam.Sections)
	if err != nil {
		return domain.MockExam{}, err
	}
	writingEval, err := evaluateCETConstructedResponse(ctx, engine, examName, writing, "WRITING")
	if err != nil {
		return domain.MockExam{}, fmt.Errorf("writing: %w", err)
	}
	translationEval, err := evaluateCETConstructedResponse(ctx, engine, examName, translation, "TRANSLATION")
	if err != nil {
		return domain.MockExam{}, fmt.Errorf("translation: %w", err)
	}
	writingScore, translationScore := writingEval.OverallScore, translationEval.OverallScore
	if invalidPercent(writingScore) || invalidPercent(translationScore) {
		return domain.MockExam{}, errors.New("CET 主观题评分不在 0..100")
	}
	total := listeningScore*0.35 + readingScore*0.35 + writingScore*0.15 + translationScore*0.15
	result := &domain.MockExamResult{
		WritingEvaluations:    []domain.WritingEvaluation{writingEval},
		TranslationEvaluation: &translationEval,
		ReadingCorrect:        rc,
		ReadingTotal:          rt,
		ListeningCorrect:      lc,
		ListeningTotal:        lt,
		ReadingScore:          readingScore,
		ListeningScore:        listeningScore,
		WritingScore:          writingScore,
		WritingTask1Score:     writingScore,
		TranslationScore:      translationScore,
		TotalScore:            math.Round(total*10) / 10,
		ScoreScale:            "PERCENTAGE",
		QualityStatus:         cetMockQualityStatus,
		CompletedAt:           time.Now().UTC(),
	}
	result.Feedback = fmt.Sprintf("%s 训练估分：听力与阅读按各题型分值加权后归一至百分制；写作和翻译由模型评估；总分按听力35%%、阅读35%%、写作15%%、翻译15%%加权。此结果不是官方710分等值分。", examName)
	result.Recommendations = cetObjectiveRecommendations(exam.Sections)
	result.Strengths = append(result.Strengths, writingEval.Strengths...)
	result.Strengths = append(result.Strengths, translationEval.Strengths...)
	result.Weaknesses = append(result.Weaknesses, writingEval.Issues...)
	result.Weaknesses = append(result.Weaknesses, translationEval.Issues...)
	result.Recommendations = append(result.Recommendations, writingEval.Suggestions...)
	result.Recommendations = append(result.Recommendations, translationEval.Suggestions...)
	exam.Result, exam.Status, exam.CurrentSection, exam.UpdatedAt = result, "COMPLETED", "RESULT", time.Now().UTC()
	return exam, nil
}

func cetWritingAndTranslationSections(sections []domain.MockExamSection) (domain.MockExamSection, domain.MockExamSection, error) {
	var writing, translation domain.MockExamSection
	for _, section := range sections {
		switch section.Skill {
		case "WRITING":
			writing = section
		case "TRANSLATION":
			translation = section
		}
	}
	if len(writing.WritingPrompts) != 1 {
		return domain.MockExamSection{}, domain.MockExamSection{}, errors.New("CET 写作题目不完整")
	}
	if len(translation.WritingPrompts) != 1 {
		return domain.MockExamSection{}, domain.MockExamSection{}, errors.New("CET 翻译题目不完整")
	}
	return writing, translation, nil
}

func evaluateCETConstructedResponse(ctx context.Context, engine writingEngine, exam string, section domain.MockExamSection, skill string) (domain.WritingEvaluation, error) {
	response := ""
	if len(section.Responses) > 0 {
		response = strings.TrimSpace(section.Responses[0])
	}
	if response == "" {
		return domain.WritingEvaluation{OverallScore: 0, Issues: []string{section.Title + " 未作答，按 0 分计入。"}}, nil
	}
	prompt := section.WritingPrompts[0]
	var evaluation domain.WritingEvaluation
	var err error
	if skill == "TRANSLATION" {
		translator, ok := engine.(interface {
			EvaluateCETTranslation(context.Context, string, domain.WritingPrompt, string) (domain.WritingEvaluation, error)
		})
		if !ok {
			return domain.WritingEvaluation{}, errors.New("翻译评分服务未配置")
		}
		evaluation, err = translator.EvaluateCETTranslation(ctx, exam, prompt, response)
	} else {
		evaluation, err = engine.EvaluateWriting(ctx, exam, prompt, response, section.DurationSeconds, 0)
	}
	if err != nil {
		return domain.WritingEvaluation{}, err
	}
	if invalidPercent(evaluation.OverallScore) {
		return domain.WritingEvaluation{}, errors.New("score outside 0..100")
	}
	evaluation.BandEstimate = 0
	return evaluation, nil
}

// 每题占整卷百分比：听力前15题各1、后10题各2；阅读10选词各0.5、10匹配各1、10仔细阅读各2。
func cetObjectiveScore(sections []domain.MockExamSection, skill string) (float64, error) {
	count := 0
	points := 0.0
	for _, section := range sections {
		if section.Skill != skill {
			continue
		}
		for i, q := range section.Questions {
			weight := 1.0
			if skill == "LISTENING" && count >= 15 {
				weight = 2
			}
			if skill == "READING" {
				if count < 10 {
					weight = 0.5
				} else if count >= 20 {
					weight = 2
				}
			}
			if i < len(section.Answers) && mockAnswerMatches(section.Answers[i], q) {
				points += weight
			}
			count++
		}
	}
	expected := 25
	if skill == "READING" {
		expected = 30
	}
	if count != expected {
		return 0, errors.New("四六级客观题数量不完整，不能计分")
	}
	return points / 35 * 100, nil
}

func percentScore(correct, total int) float64 {
	if total <= 0 || correct < 0 || correct > total {
		return 0
	}
	return math.Round(float64(correct)*1000/float64(total)) / 10
}

func invalidPercent(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100
}

func cetObjectiveRecommendations(sections []domain.MockExamSection) []string {
	var recommendations []string
	for _, section := range sections {
		if section.Skill != "LISTENING" && section.Skill != "READING" {
			continue
		}
		wrong := 0
		for i, q := range section.Questions {
			if i >= len(section.Answers) || !mockAnswerMatches(section.Answers[i], q) {
				wrong++
			}
		}
		if wrong > 0 {
			recommendations = append(recommendations, fmt.Sprintf("%s：错误或未答 %d 题，建议按题型复盘定位、同义替换和干扰项。", section.Title, wrong))
		}
	}
	return recommendations
}
