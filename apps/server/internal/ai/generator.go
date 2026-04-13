package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

type OpenAIGenerator struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

const (
	modelAPIMaxRetries = 2
)

func NewOpenAIGenerator(apiKey string, model string, baseURL string) *OpenAIGenerator {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &OpenAIGenerator{
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 45 * time.Second},
	}
}

func languageDirective(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		return "Target language: English. Every speaker name, dialogue line, and quiz item MUST be in English only. Do not use Chinese or other languages."
	case "CANTONESE":
		return "Target: Hong Kong Cantonese. Write dialogue text in Traditional Chinese (口語化、粵語表達). But quiz questions and options must be in Simplified Chinese (standard Mandarin phrasing) for learner readability."
	default:
		return "Follow the language field strictly for all dialogue and quiz text."
	}
}

func requiredQuizCount(difficulty float64) int {
	if difficulty >= 7.0 {
		return 3
	}
	return 2
}

func isReadingGeneration(mode string, topic string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), "APPRECIATION") && strings.Contains(strings.ToLower(topic), "reading")
}

func readingMinWords(topic string) int {
	t := strings.ToUpper(topic)
	switch {
	case strings.Contains(t, "[IELTS READING]"):
		return 620
	case strings.Contains(t, "[CET READING]"):
		return 480
	default:
		return 520
	}
}

// Generate returns dialogues and comprehension questions with options and reference answers for server-side grading.
func (g *OpenAIGenerator) Generate(ctx context.Context, language string, topic string, difficulty float64, mode string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	apiKey := strings.TrimSpace(g.APIKey)
	if apiKey == "" {
		return nil, nil, fmt.Errorf("OPENAI_API_KEY is empty")
	}
	quizCount := requiredQuizCount(difficulty)
	readingMode := isReadingGeneration(mode, topic)
	if readingMode {
		quizCount = 5
	}
	sys := languageDirective(language) + " Output one JSON object only, no markdown fences."
	user := ""
	if readingMode {
		minWords := readingMinWords(topic)
		user = fmt.Sprintf(
			`Learning language code: %s. Topic: %s. Difficulty: %.1f. Mode: %s.
Create an exam-style long reading passage suitable for IELTS/CET practice.
Length requirement: total English word count MUST be at least %d words.
Structure:
- 8 to 10 passage segments in "dialogues" array.
- Each segment text should be a coherent paragraph (not chat turns), around 65-95 words.
- Keep style formal and information-dense, like real exam materials.
JSON shape:
{"dialogues":[{"speaker":"Passage","text":"...","zhSubtitle":"..."}],"quiz":[{"question":"...","options":["...","...","...","..."],"answerKey":"..."}]}
Rules for dialogues.zhSubtitle:
- Must be concise Simplified Chinese explanation of that paragraph's core idea.
- Do not translate word-by-word.
Rules for quiz:
1) Create exactly %d multiple-choice questions.
2) Cover varied skills: main idea, detail locating, inference, vocabulary-in-context, author attitude/structure.
3) options must contain exactly 4 choices, only one correct.
4) answerKey must be exactly one of the 4 option strings (verbatim match).`,
			language, topic, difficulty, mode, minWords, quizCount,
		)
	} else {
		user = fmt.Sprintf(
			`Learning language code: %s. Topic: %s. Difficulty: %.1f. Mode: %s.
Scene must be realistic and specific (place, time, roles). Use natural spoken lines for the target language.
Produce exactly 8 dialogue turns and exactly %d listening comprehension single-choice questions based ONLY on those dialogues.
Use clear speaker roles like 店员/顾客 for Cantonese or Barista/Customer for English.
JSON shape:
{"dialogues":[{"speaker":"...","text":"...","zhSubtitle":"..."}],"quiz":[{"question":"...","options":["...","...","...","..."],"answerKey":"..."}]}
Rules for dialogues.zhSubtitle: must be Simplified Chinese subtitle for the same line.
For ENGLISH text, zhSubtitle should be natural Chinese translation.
For CANTONESE text, zhSubtitle should be concise Mandarin-style Chinese paraphrase.
Rules for quiz:
1) Every question must test specific details from the generated dialogue and be answerable from dialogue evidence.
2) options must contain exactly 4 choices, only one correct.
3) answerKey must be exactly one of the 4 option strings (verbatim match).
4) For CANTONESE, dialogue stays Traditional Chinese, but quiz question and options must use Simplified Chinese.
5) Avoid generic/meta questions like "主题是什么" unless anchored by concrete dialogue details.`,
			language, topic, difficulty, mode, quizCount,
		)
	}
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
		"temperature": 0.65,
	}
	content, err := g.callModelJSONPayload(ctx, payload)
	if err != nil {
		return nil, nil, err
	}

	dialogues, quiz := parseModelOutput(content)
	if len(dialogues) == 0 {
		return nil, nil, fmt.Errorf("model output parsing failed: missing dialogues")
	}
	if len(quiz) < quizCount {
		return nil, nil, fmt.Errorf("model output parsing failed: missing quiz questions")
	}
	if readingMode {
		wordCount := 0
		for _, d := range dialogues {
			wordCount += len(strings.Fields(strings.TrimSpace(d.Text)))
		}
		if wordCount < readingMinWords(topic) {
			return nil, nil, fmt.Errorf("model output too short: got %d words", wordCount)
		}
	}
	return dialogues, quiz[:quizCount], nil
}

func (g *OpenAIGenerator) chatCompletionsURL() string {
	if strings.HasSuffix(g.BaseURL, "/v1") {
		return g.BaseURL + "/chat/completions"
	}
	return g.BaseURL + "/v1/chat/completions"
}

func shouldRetryModelStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func (g *OpenAIGenerator) callModelJSONPayload(ctx context.Context, payload map[string]any) (string, error) {
	raw, _ := json.Marshal(payload)
	chatURL := g.chatCompletionsURL()
	apiKey := strings.TrimSpace(g.APIKey)
	var lastErr error

	for attempt := 0; attempt <= modelAPIMaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		// Some OpenAI-compatible gateways validate x-api-key instead of Authorization only.
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.Client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request model API failed: %w", err)
		} else {
			var retryable bool
			var content string
			func() {
				defer resp.Body.Close()
				if resp.StatusCode >= 400 {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					lastErr = fmt.Errorf("model API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
					retryable = shouldRetryModelStatus(resp.StatusCode)
					return
				}
				var parsed struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if decodeErr := json.NewDecoder(resp.Body).Decode(&parsed); decodeErr != nil {
					lastErr = decodeErr
					return
				}
				if len(parsed.Choices) == 0 {
					lastErr = fmt.Errorf("model API returned empty choices")
					return
				}
				content = strings.TrimSpace(parsed.Choices[0].Message.Content)
				content = strings.TrimPrefix(content, "```json")
				content = strings.TrimPrefix(content, "```")
				content = strings.TrimSuffix(content, "```")
				lastErr = nil
			}()
			if lastErr == nil {
				return strings.TrimSpace(content), nil
			}
			if !retryable {
				break
			}
		}

		if attempt == modelAPIMaxRetries {
			break
		}
		backoff := time.Duration(attempt+1) * 500 * time.Millisecond
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("request model API failed: unknown error")
	}
	return "", lastErr
}

type genDialogue struct {
	Speaker    string `json:"speaker"`
	Text       string `json:"text"`
	ZhSubtitle string `json:"zhSubtitle"`
	SubtitleZh string `json:"subtitleZh"`
	ZhText     string `json:"zhText"`
}

type genQuiz struct {
	Question  string `json:"question"`
	Options   []string `json:"options"`
	AnswerKey string `json:"answerKey"`
}

type combinedOut struct {
	Dialogues []genDialogue `json:"dialogues"`
	Quiz      []genQuiz     `json:"quiz"`
}

func parseModelOutput(content string) ([]domain.Dialogue, []domain.QuizQuestion) {
	var out combinedOut
	if err := json.Unmarshal([]byte(content), &out); err != nil || len(out.Dialogues) == 0 {
		var legacy []genDialogue
		if err2 := json.Unmarshal([]byte(content), &legacy); err2 != nil || len(legacy) == 0 {
			return nil, nil
		}
		return toDialogues(legacy), nil
	}
	dialogues := toDialogues(out.Dialogues)
	quiz := make([]domain.QuizQuestion, 0, len(out.Quiz))
	for _, q := range out.Quiz {
		if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.AnswerKey) == "" || len(q.Options) < 2 {
			continue
		}
		options := make([]string, 0, len(q.Options))
		for _, option := range q.Options {
			trimmed := strings.TrimSpace(option)
			if trimmed == "" {
				continue
			}
			options = append(options, trimmed)
		}
		answerKey := strings.TrimSpace(q.AnswerKey)
		validAnswer := false
		for _, option := range options {
			if option == answerKey {
				validAnswer = true
				break
			}
		}
		if !validAnswer {
			continue
		}
		quiz = append(quiz, domain.QuizQuestion{Question: strings.TrimSpace(q.Question), Options: options, AnswerKey: answerKey})
	}
	return dialogues, quiz
}

func toDialogues(items []genDialogue) []domain.Dialogue {
	result := make([]domain.Dialogue, 0, len(items))
	for index, item := range items {
		zhSubtitle := strings.TrimSpace(item.ZhSubtitle)
		if zhSubtitle == "" {
			zhSubtitle = strings.TrimSpace(item.SubtitleZh)
		}
		if zhSubtitle == "" {
			zhSubtitle = strings.TrimSpace(item.ZhText)
		}
		result = append(result, domain.Dialogue{
			Speaker:    item.Speaker,
			Text:       item.Text,
			ZhSubtitle: zhSubtitle,
			AudioURL:   "",
			Timestamp:  float64(index) * 2.3,
		})
	}
	return result
}
