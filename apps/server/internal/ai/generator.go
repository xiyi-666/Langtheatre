package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		scenarioBrief, scenarioErr := g.expandConversationScenario(ctx, language, topic, difficulty, mode)
		if scenarioErr != nil {
			scenarioBrief = strings.TrimSpace(topic)
		}
		user = fmt.Sprintf(
			`Learning language code: %s. Topic: %s. Scenario brief: %s. Difficulty: %.1f. Mode: %s.
Scene must be realistic and specific (place, time, roles). Use natural spoken lines for the target language.
Do NOT use classroom/meta narration such as "today's topic is...", "we are discussing...", "welcome to mini theater", or direct topic announcements.
The first turn must immediately enter a concrete real-life situation with actionable context (for example at a counter, station, office desk, clinic, or phone call).
Each turn should either ask for concrete information, provide clarification, confirm details, or make a practical decision.
If the topic is written in Simplified Chinese and the language is CANTONESE, first reinterpret the topic into a natural Hong Kong Cantonese life scenario internally, then write the dialogue in authentic Hong Kong Cantonese.
All dialogue turns must stay consistent with the provided scenario brief.
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
5) Avoid generic/meta questions like "主题是什么" unless anchored by concrete dialogue details.
6) Prefer realistic detail questions: numbers, time, location, preference, constraints, next-step decisions.`,
			language, topic, scenarioBrief, difficulty, mode, quizCount,
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
	if err != nil && strings.Contains(err.Error(), "no parsable text") && !strings.EqualFold(model, "gpt-4o-mini") {
		log.Printf("model %s returned empty content, retry with fallback model gpt-4o-mini", model)
		payload["model"] = "gpt-4o-mini"
		content, err = g.callModelJSONPayload(ctx, payload)
	}
	if err != nil {
		return nil, nil, err
	}

	dialogues, quiz := parseModelOutput(content)
	if len(dialogues) == 0 {
		snippet := content
		if len(snippet) > 320 {
			snippet = snippet[:320]
		}
		log.Printf("model output parse failed snippet=%q", snippet)
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

func (g *OpenAIGenerator) expandConversationScenario(ctx context.Context, language string, topic string, difficulty float64, mode string) (string, error) {
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	system := "You expand a learning topic into one concrete, realistic conversation scenario. Return JSON only."
	user := fmt.Sprintf(`Language: %s
Topic: %s
Difficulty: %.1f
Mode: %s

Task:
Turn the topic into one concrete real-life conversation setup with:
- place
- time or timing pressure
- two roles
- immediate problem to solve
- one practical goal

Rules:
- One scenario only.
- No teaching narration.
- If Language is CANTONESE and Topic is Simplified Chinese, rewrite internally into a Hong Kong Cantonese life context.
- Keep it concise but specific.

JSON:
{"scenario":"..."}`, language, topic, difficulty, mode)
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.7,
	}
	content, err := g.callModelJSONPayload(ctx, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		Scenario string `json:"scenario"`
	}
	if err := json.Unmarshal([]byte(sanitizeJSONLikeContent(content)), &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Scenario), nil
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
				body, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					lastErr = readErr
					return
				}
				content, lastErr = extractModelTextFromResponse(body)
				if lastErr != nil {
					return
				}
				content = sanitizeJSONLikeContent(content)
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

func extractModelTextFromResponse(body []byte) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}

	if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if message, ok := first["message"].(map[string]any); ok {
				if content := asString(message["content"]); content != "" {
					return content, nil
				}
				if parts, ok := message["content"].([]any); ok {
					var chunks []string
					for _, p := range parts {
						if m, ok := p.(map[string]any); ok {
							if txt := asString(firstNonNil(m["text"], m["content"])); txt != "" {
								chunks = append(chunks, txt)
							}
						}
					}
					if len(chunks) > 0 {
						return strings.Join(chunks, "\n"), nil
					}
				}
				if reasoning := asString(firstNonNil(message["reasoning_content"], message["reasoning"])); reasoning != "" {
					return reasoning, nil
				}
			}
			if text := asString(first["text"]); text != "" {
				return text, nil
			}
		}
	}

	if output, ok := raw["output"].([]any); ok && len(output) > 0 {
		var chunks []string
		for _, item := range output {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if content, ok := entry["content"].([]any); ok {
				for _, c := range content {
					block, ok := c.(map[string]any)
					if !ok {
						continue
					}
					if txt := asString(firstNonNil(block["text"], block["content"])); txt != "" {
						chunks = append(chunks, txt)
					}
				}
			}
		}
		if len(chunks) > 0 {
			return strings.Join(chunks, "\n"), nil
		}
	}

	rawSnippet := string(body)
	if len(rawSnippet) > 320 {
		rawSnippet = rawSnippet[:320]
	}
	return "", fmt.Errorf("model API returned no parsable text, snippet=%q", rawSnippet)
}

type genDialogue struct {
	Speaker    string `json:"speaker"`
	Text       string `json:"text"`
	ZhSubtitle string `json:"zhSubtitle"`
	SubtitleZh string `json:"subtitleZh"`
	ZhText     string `json:"zhText"`
}

type genQuiz struct {
	Question  string   `json:"question"`
	Options   []string `json:"options"`
	AnswerKey string   `json:"answerKey"`
}

type combinedOut struct {
	Dialogues []genDialogue `json:"dialogues"`
	Quiz      []genQuiz     `json:"quiz"`
}

type readingAltOut struct {
	Passage    string    `json:"passage"`
	Paragraphs []string  `json:"paragraphs"`
	Quiz       []genQuiz `json:"quiz"`
}

func parseModelOutput(content string) ([]domain.Dialogue, []domain.QuizQuestion) {
	content = sanitizeJSONLikeContent(content)
	var out combinedOut
	if err := json.Unmarshal([]byte(content), &out); err != nil || len(out.Dialogues) == 0 {
		if extracted := extractFirstJSONObject(content); extracted != "" {
			content = extracted
			if err2 := json.Unmarshal([]byte(content), &out); err2 == nil && len(out.Dialogues) > 0 {
				dialogues := toDialogues(out.Dialogues)
				quiz := normalizeQuiz(out.Quiz)
				return dialogues, quiz
			}
		}
		if dialogues, quiz := parseDialogueAliases(content); len(dialogues) > 0 {
			return dialogues, quiz
		}
		var alt readingAltOut
		if errAlt := json.Unmarshal([]byte(content), &alt); errAlt == nil {
			paragraphs := make([]string, 0)
			if strings.TrimSpace(alt.Passage) != "" {
				paragraphs = append(paragraphs, splitPassageForDialogues(alt.Passage)...)
			}
			for _, p := range alt.Paragraphs {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					paragraphs = append(paragraphs, trimmed)
				}
			}
			if len(paragraphs) > 0 {
				dialogues := make([]domain.Dialogue, 0, len(paragraphs))
				for i, p := range paragraphs {
					dialogues = append(dialogues, domain.Dialogue{Speaker: "Passage", Text: p, Timestamp: float64(i) * 2.3})
				}
				quiz := normalizeQuiz(alt.Quiz)
				return dialogues, quiz
			}
		}
		var legacy []genDialogue
		if err2 := json.Unmarshal([]byte(content), &legacy); err2 != nil || len(legacy) == 0 {
			return nil, nil
		}
		return toDialogues(legacy), nil
	}
	dialogues := toDialogues(out.Dialogues)
	quiz := normalizeQuiz(out.Quiz)
	return dialogues, quiz
}

func extractFirstJSONObject(content string) string {
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(content); i++ {
		ch := content[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			depth++
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				return strings.TrimSpace(content[start : i+1])
			}
		}
	}
	return ""
}

func parseDialogueAliases(content string) ([]domain.Dialogue, []domain.QuizQuestion) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, nil
	}
	dialogueAny := firstNonNil(raw["dialogues"], raw["dialogue"], raw["dialogs"], raw["conversation"], raw["turns"], raw["messages"])
	dialogueList, ok := dialogueAny.([]any)
	if !ok || len(dialogueList) == 0 {
		return nil, nil
	}
	dialogues := make([]domain.Dialogue, 0, len(dialogueList))
	for i, item := range dialogueList {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		speaker := asString(firstNonNil(entry["speaker"], entry["role"], entry["character"], entry["name"]))
		text := asString(firstNonNil(entry["text"], entry["content"], entry["utterance"], entry["line"], entry["message"], entry["reply"]))
		zh := asString(firstNonNil(entry["zhSubtitle"], entry["subtitle"], entry["translation"], entry["zh"], entry["中文"]))
		if text == "" {
			continue
		}
		if speaker == "" {
			speaker = fmt.Sprintf("Speaker%d", i+1)
		}
		dialogues = append(dialogues, domain.Dialogue{
			Speaker:    speaker,
			Text:       text,
			ZhSubtitle: zh,
			Timestamp:  float64(i) * 2.0,
		})
	}
	quiz := parseQuizAliases(raw)
	return dialogues, quiz
}

func parseQuizAliases(raw map[string]any) []domain.QuizQuestion {
	quizAny := firstNonNil(raw["quiz"], raw["questions"], raw["quizQuestions"], raw["questionSet"])
	quizList, ok := quizAny.([]any)
	if !ok || len(quizList) == 0 {
		return nil
	}
	out := make([]domain.QuizQuestion, 0, len(quizList))
	for _, item := range quizList {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		question := asString(firstNonNil(entry["question"], entry["prompt"], entry["title"]))
		options := toStringSlice(firstNonNil(entry["options"], entry["choices"], entry["candidates"]))
		answer := asString(firstNonNil(entry["answerKey"], entry["answer"], entry["correct"], entry["correctAnswer"]))
		if question == "" || len(options) == 0 {
			continue
		}
		if answer == "" {
			answer = options[0]
		}
		valid := false
		for _, opt := range options {
			if opt == answer {
				valid = true
				break
			}
		}
		if !valid {
			continue
		}
		out = append(out, domain.QuizQuestion{Question: question, Options: options, AnswerKey: answer})
	}
	return out
}

func normalizeQuiz(input []genQuiz) []domain.QuizQuestion {
	quiz := make([]domain.QuizQuestion, 0, len(input))
	for _, q := range input {
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
	return quiz
}

func splitPassageForDialogues(passage string) []string {
	clean := strings.TrimSpace(passage)
	if clean == "" {
		return nil
	}
	chunks := strings.Split(clean, "\n")
	result := make([]string, 0, len(chunks))
	for _, c := range chunks {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) > 0 {
		return result
	}
	return []string{clean}
}

func (g *OpenAIGenerator) AnalyzeReading(ctx context.Context, exam string, topic string, passage string, vocabulary []string) (domain.ReadingAnalysis, error) {
	apiKey := strings.TrimSpace(g.APIKey)
	if apiKey == "" {
		return domain.ReadingAnalysis{}, fmt.Errorf("OPENAI_API_KEY is empty")
	}
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	vocabList := strings.Join(vocabulary, ", ")
	prompt := fmt.Sprintf(
		`You are an English learning assistant. Analyze the passage and return JSON only.
Exam: %s
Topic: %s
Vocabulary candidates: %s
Passage:
%s

Output schema:
{
  "vocabularyItems": [
    {"word":"...","pos":"n./v./adj./adv.","meanings":["中文义1","中文义2"]}
  ],
  "associationSentences": ["英文句子1","英文句子2","英文句子3"],
  "grammarInsights": [
    {
      "sentence":"原句",
      "difficultyPoints":["难点1","难点2"],
      "studySuggestions":["建议1","建议2"]
    }
  ]
}

Rules:
1) vocabularyItems length must be >= 15.
2) meanings are Simplified Chinese and should include polysemy when applicable.
3) associationSentences must be exactly 3 complete English sentences and naturally include key vocabulary.
4) grammarInsights should include 3-4 representative long/complex sentences with practical learning advice.
5) Do not output markdown fences.`,
		exam,
		topic,
		vocabList,
		passage,
	)
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Return valid JSON only."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.35,
	}
	content, err := g.callModelJSONPayload(ctx, payload)
	if err != nil {
		return domain.ReadingAnalysis{}, err
	}
	content = sanitizeJSONLikeContent(content)

	var out domain.ReadingAnalysis
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		aliased, aliasErr := parseReadingAnalysisAliases(content)
		if aliasErr != nil {
			return domain.ReadingAnalysis{}, fmt.Errorf("parse reading analysis failed: %w", err)
		}
		out = aliased
	}
	return out, nil
}

func sanitizeJSONLikeContent(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func parseReadingAnalysisAliases(content string) (domain.ReadingAnalysis, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return domain.ReadingAnalysis{}, err
	}

	out := domain.ReadingAnalysis{}
	vocabAny := firstNonNil(raw["vocabularyItems"], raw["vocabulary"], raw["words"])
	if vocabList, ok := vocabAny.([]any); ok {
		for _, item := range vocabList {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			word := asString(firstNonNil(m["word"], m["term"], m["token"]))
			pos := asString(firstNonNil(m["pos"], m["partOfSpeech"], m["词性"]))
			meanings := toStringSlice(firstNonNil(m["meanings"], m["definitions"], m["中文释义"], m["释义"]))
			if word == "" {
				continue
			}
			out.VocabularyItems = append(out.VocabularyItems, domain.VocabularyItem{Word: word, POS: pos, Meanings: meanings})
		}
	}

	out.AssociationSentences = toStringSlice(firstNonNil(raw["associationSentences"], raw["memorySentences"], raw["联想句"], raw["联想记忆"]))

	grammarAny := firstNonNil(raw["grammarInsights"], raw["grammar"], raw["语法解析"], raw["longSentenceAnalysis"])
	if grammarList, ok := grammarAny.([]any); ok {
		for _, item := range grammarList {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			sentence := asString(firstNonNil(m["sentence"], m["original"], m["原句"]))
			difficulty := toStringSlice(firstNonNil(m["difficultyPoints"], m["difficulties"], m["难点"]))
			suggestion := toStringSlice(firstNonNil(m["studySuggestions"], m["suggestions"], m["learningTips"], m["学习建议"]))
			if sentence == "" {
				continue
			}
			out.GrammarInsights = append(out.GrammarInsights, domain.GrammarInsight{
				Sentence:         sentence,
				DifficultyPoints: difficulty,
				StudySuggestions: suggestion,
			})
		}
	}

	return out, nil
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s := asString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if listStr, ok := v.([]string); ok {
		out := make([]string, 0, len(listStr))
		for _, item := range listStr {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	if single := asString(v); single != "" {
		return []string{single}
	}
	return nil
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
