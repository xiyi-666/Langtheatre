package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/linguaquest/server/internal/domain"
)

type roleplayTurnPayload struct {
	AssistantReply string `json:"assistantReply"`
	AssistantZhSub string `json:"assistantZhSub"`
	Relevance      int    `json:"relevance"`
	Accuracy       int    `json:"accuracy"`
	Naturalness    int    `json:"naturalness"`
	Total          int    `json:"total"`
	Feedback       string `json:"feedback"`
}

type roleplaySummaryPayload struct {
	Summary string `json:"summary"`
}

func (g *OpenAIGenerator) RoleplayTurn(ctx context.Context, theater domain.Theater, userRole string, transcript []domain.Dialogue, userReply string) (domain.RoleplayTurnEval, error) {
	sys := "You are an AI roleplay coach for language practice. Return one JSON object only."
	if strings.EqualFold(strings.TrimSpace(theater.Language), "CANTONESE") {
		sys = "你係語言學習角色扮演教練。對話請用港式粵語（繁體），但字幕與評語必須用簡體中文。僅返回JSON。"
	}
	maxTurns := 12
	if len(transcript) < maxTurns {
		maxTurns = len(transcript)
	}
	history := transcript[len(transcript)-maxTurns:]
	historyBytes, _ := json.Marshal(history)
	prompt := fmt.Sprintf(`Theater topic: %s
Scene: %s
Language: %s
User role: %s
Conversation history JSON: %s
Latest user reply: %s

Task:
1) Continue roleplay as the partner character and ask ONE natural follow-up question that connects to history.
1.1) Provide Simplified Chinese subtitle for the assistant reply.
2) Score latest user reply:
   - relevance: 0-40
   - accuracy: 0-30
   - naturalness: 0-30
   - total: 0-100
3) Give one concise actionable feedback in Simplified Chinese.
4) If latest user reply is empty, produce an opening question and all scores should be 0.

JSON format:
{"assistantReply":"...","assistantZhSub":"...","relevance":0,"accuracy":0,"naturalness":0,"total":0,"feedback":"..."}`,
		theater.Topic,
		theater.SceneDescription,
		theater.Language,
		userRole,
		string(historyBytes),
		strings.TrimSpace(userReply),
	)
	content, err := g.callJSONCompletion(ctx, sys, prompt)
	if err != nil {
		return domain.RoleplayTurnEval{}, err
	}
	var parsed roleplayTurnPayload
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return domain.RoleplayTurnEval{}, err
	}
	parsed.Relevance = clamp(parsed.Relevance, 0, 40)
	parsed.Accuracy = clamp(parsed.Accuracy, 0, 30)
	parsed.Naturalness = clamp(parsed.Naturalness, 0, 30)
	if parsed.Total == 0 {
		parsed.Total = parsed.Relevance + parsed.Accuracy + parsed.Naturalness
	}
	parsed.Total = clamp(parsed.Total, 0, 100)
	if strings.TrimSpace(parsed.AssistantReply) == "" {
		return domain.RoleplayTurnEval{}, fmt.Errorf("empty assistant reply")
	}
	if strings.TrimSpace(parsed.AssistantZhSub) == "" {
		parsed.AssistantZhSub = strings.TrimSpace(parsed.AssistantReply)
	}
	return domain.RoleplayTurnEval{
		AssistantReply: strings.TrimSpace(parsed.AssistantReply),
		AssistantZhSub: strings.TrimSpace(parsed.AssistantZhSub),
		Relevance:      parsed.Relevance,
		Accuracy:       parsed.Accuracy,
		Naturalness:    parsed.Naturalness,
		Total:          parsed.Total,
		Feedback:       strings.TrimSpace(parsed.Feedback),
	}, nil
}

func (g *OpenAIGenerator) RoleplaySummary(ctx context.Context, theater domain.Theater, transcript []domain.Dialogue, currentScore int) (string, error) {
	sys := "你是角色扮演总结教练。总结必须使用简体中文。仅返回JSON。"
	historyBytes, _ := json.Marshal(transcript)
	prompt := fmt.Sprintf(`Topic: %s
Language: %s
Current score: %d
Transcript JSON: %s

Write a concise final report with:
- overall performance
- strengths
- top 2 improvement actions
- one improved sample sentence

JSON format:
{"summary":"..."}`,
		theater.Topic,
		theater.Language,
		currentScore,
		string(historyBytes),
	)
	content, err := g.callJSONCompletion(ctx, sys, prompt)
	if err != nil {
		return "", err
	}
	var parsed roleplaySummaryPayload
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return "", err
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return "", fmt.Errorf("empty summary")
	}
	return summary, nil
}

func (g *OpenAIGenerator) callJSONCompletion(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.5,
	}
	raw, _ := json.Marshal(payload)
	chatURL := g.BaseURL + "/v1/chat/completions"
	if strings.HasSuffix(g.BaseURL, "/v1") {
		chatURL = g.BaseURL + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(g.APIKey)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("model API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("model API returned empty choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content), nil
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
