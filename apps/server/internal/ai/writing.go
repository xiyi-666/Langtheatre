package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/linguaquest/server/internal/domain"
)

func (g *OpenAIGenerator) GenerateWritingPrompt(ctx context.Context, exam string) (domain.WritingPrompt, error) {
	prompt := fmt.Sprintf(`Create one English writing prompt for %s. Return JSON only:
{"title":"...","instructions":"...","suggestedWordCount":250}
The task must be realistic, self-contained, and appropriate for the exam. Instructions may be bilingual Chinese/English, but the writing task itself must be English.`, exam)
	content, err := g.callJSONCompletion(ctx, "You design fair English examination writing prompts. Return valid JSON only.", prompt, "WRITING_PROMPT")
	if err != nil {
		return domain.WritingPrompt{}, err
	}
	var result domain.WritingPrompt
	if err = json.Unmarshal([]byte(sanitizeJSONLikeContent(content)), &result); err != nil {
		return domain.WritingPrompt{}, err
	}
	result.Title = strings.TrimSpace(result.Title)
	result.Instructions = strings.TrimSpace(result.Instructions)
	if result.Title == "" || result.Instructions == "" {
		return domain.WritingPrompt{}, fmt.Errorf("empty writing prompt")
	}
	if result.SuggestedWordCount < 80 {
		result.SuggestedWordCount = defaultWritingWordCount(exam)
	}
	return result, nil
}

func (g *OpenAIGenerator) EvaluateWriting(ctx context.Context, exam string, prompt domain.WritingPrompt, essay string, timeLimitSeconds int, elapsedSeconds int) (domain.WritingEvaluation, error) {
	request := fmt.Sprintf(`Evaluate this English essay for %s. Writing prompt title: %s
Prompt: %s
Time limit: %d seconds; elapsed time: %d seconds.
Essay:
%s

Return JSON only:
{"overallScore":0,"grammarScore":0,"vocabularyScore":0,"coherenceScore":0,"taskResponseScore":0,"strengths":["..."],"issues":["quote a short fragment and explain"],"suggestions":["..."],"revisedExcerpt":"...","summary":"简体中文总结"}
All numeric scores are 0-100. Be specific, constructive, and never invent quoted text.`, exam, prompt.Title, prompt.Instructions, timeLimitSeconds, elapsedSeconds, essay)
	content, err := g.callJSONCompletion(ctx, "You are a meticulous English writing examiner. Give evidence-based feedback in Simplified Chinese. Return valid JSON only.", request, "WRITING_EVALUATION")
	if err != nil {
		return domain.WritingEvaluation{}, err
	}
	var result domain.WritingEvaluation
	if err = json.Unmarshal([]byte(sanitizeJSONLikeContent(content)), &result); err != nil {
		return domain.WritingEvaluation{}, err
	}
	result.OverallScore = clampWritingScore(result.OverallScore)
	result.GrammarScore = clampWritingScore(result.GrammarScore)
	result.VocabularyScore = clampWritingScore(result.VocabularyScore)
	result.CoherenceScore = clampWritingScore(result.CoherenceScore)
	result.TaskResponseScore = clampWritingScore(result.TaskResponseScore)
	if result.OverallScore == 0 {
		result.OverallScore = (result.GrammarScore + result.VocabularyScore + result.CoherenceScore + result.TaskResponseScore) / 4
	}
	return result, nil
}

func defaultWritingWordCount(exam string) int {
	if strings.EqualFold(exam, "IELTS") {
		return 250
	}
	return 150
}
func clampWritingScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
