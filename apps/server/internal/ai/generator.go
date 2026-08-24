package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/linguaquest/server/internal/analytics"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/ielts"
)

type OpenAIGenerator struct {
	mu        sync.RWMutex
	Provider  string
	APIKey    string
	Model     string
	BaseURL   string
	Client    *http.Client
	analytics *analytics.Reporter
}

func (g *OpenAIGenerator) SetUsageReporter(reporter *analytics.Reporter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.analytics = reporter
}

const (
	modelAPIMaxRetries = 2
	modelAPITimeout    = 180 * time.Second
)

const (
	defaultModelProvider = "OPENAI"
	defaultModelName     = "gpt-5.4"
	defaultBaseURL       = "http://43.172.5.210:3000/v1"
)

var errGeneratedCantoneseQuality = errors.New("generated dialogue is not authentic Hong Kong Cantonese")

func NewOpenAIGenerator(apiKey string, model string, baseURL string) *OpenAIGenerator {
	generator := &OpenAIGenerator{
		Client: &http.Client{Timeout: modelAPITimeout},
	}
	generator.UpdateModelConfig(domain.ModelConfig{
		Provider: defaultModelProvider,
		APIKey:   apiKey,
		Model:    model,
		BaseURL:  baseURL,
	})
	return generator
}

func (g *OpenAIGenerator) GetModelConfig() domain.ModelConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return domain.ModelConfig{
		Provider: normalizedModelProvider(g.Provider),
		APIKey:   strings.TrimSpace(g.APIKey),
		Model:    normalizedModelName(g.Model),
		BaseURL:  normalizedBaseURL(g.BaseURL),
	}
}

func (g *OpenAIGenerator) UpdateModelConfig(config domain.ModelConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Provider = normalizedModelProvider(config.Provider)
	g.APIKey = strings.TrimSpace(config.APIKey)
	g.Model = normalizedModelName(config.Model)
	g.BaseURL = normalizedBaseURL(config.BaseURL)
}

func normalizedModelProvider(provider string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(provider))
	if cleaned == "" {
		return defaultModelProvider
	}
	return cleaned
}

func normalizedModelName(model string) string {
	cleaned := strings.TrimSpace(model)
	if cleaned == "" {
		return defaultModelName
	}
	return cleaned
}

func normalizedBaseURL(baseURL string) string {
	cleaned := strings.TrimSpace(baseURL)
	if cleaned == "" {
		cleaned = defaultBaseURL
	}
	return strings.TrimRight(cleaned, "/")
}

func (g *OpenAIGenerator) apiKey() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return strings.TrimSpace(g.APIKey)
}

func (g *OpenAIGenerator) modelName() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return normalizedModelName(g.Model)
}

func (g *OpenAIGenerator) baseURL() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return normalizedBaseURL(g.BaseURL)
}

func languageDirective(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		return "Target language: English. Every speaker name, dialogue line, and quiz item MUST be in English only. Do not use Chinese or other languages."
	case "CANTONESE":
		return "Target: Hong Kong Cantonese. Write dialogue text in Traditional Chinese (口語化、粵語表達), with no Latin letters or unnecessary English words. Use exactly two stable speakers. Every dialogue item must include gender as FEMALE or MALE, matching its gender-identifying speaker name or role such as 阿晴（女店員） or 陳先生（男顧客）. Quiz questions and options must be in Simplified Chinese (standard Mandarin phrasing) for learner readability."
	default:
		return "Follow the language field strictly for all dialogue and quiz text."
	}
}

func isCantonese(language string) bool {
	return strings.EqualFold(strings.TrimSpace(language), "CANTONESE")
}

// listeningControlsForGeneration keeps the IELTS listening profile for English,
// while Cantonese follows its own conversation-first learning progression.
func listeningControlsForGeneration(language string, profile ielts.ListeningProfile, difficulty float64) string {
	if !isCantonese(language) {
		return profile.PromptBlock()
	}

	switch {
	case difficulty <= 4.5:
		return `Cantonese conversation controls:
- Level 4.0-4.5: two speakers in a concrete daily-life situation. Use complete but accessible spoken sentences, including time, place, price or quantity details.
- Keep the exchange natural rather than a textbook word-by-word drill. Include one straightforward confirmation or immediate correction.
- Questions should check useful facts and the final practical arrangement.`
	case difficulty <= 5.5:
		return `Cantonese conversation controls:
- Level 5.0-5.5: two speakers handle a practical situation with schedules, quantities, alternative choices and one self-correction.
- Use natural colloquial Cantonese and let information emerge across the exchange instead of stating every answer immediately.
- Questions should test corrected details, constraints and the agreed next step.`
	case difficulty <= 6.5:
		return `Cantonese conversation controls:
- Level 6.0-6.5: two speakers explain reasons, conditions and polite preferences. Add delayed information, clarification and a realistic trade-off.
- Use longer but still conversational turns, with contextual vocabulary from work, services or community life.
- Questions should require connecting a reason, condition or correction to the decision.`
	case difficulty <= 7.5:
		return `Cantonese conversation controls:
- Level 7.0-7.5: exactly two speakers coordinate a realistic workplace, community or social decision. Include competing viewpoints, follow-up questions, concessions and a justified choice.
- Increase difficulty through implicit constraints, comparison and polite disagreement; do not make any speaker give a speech.
- Questions should test who holds which view, why an option changes, and the final negotiated arrangement.`
	default:
		return `Cantonese conversation controls:
- Level 8.0: exactly two speakers discuss an abstract workplace or social issue through a real decision. Include inference, nuanced stance-taking, a counterpoint, concession, synthesis and a concrete next action.
- Keep it an authentic interaction: speakers respond to each other, refine claims and reach a decision. Never turn it into a lecture, narration or monologue.
- Questions should test inference, contrasted viewpoints, conditions and the final decision.`
	}
}

func listeningFormInstruction(language string, profile ielts.ListeningProfile) string {
	if isCantonese(language) {
		return `For CANTONESE, this must always be a real two-person conversation at every difficulty.
Use exactly 8 turns with exactly two named speakers. Each speaker must speak at least 3 times and respond directly to the other.
Use the exact same two speaker labels throughout. Do not add a third person, narrator, temporary role or off-screen speaker.
Every dialogue item must include gender with the exact value FEMALE or MALE, consistent for that speaker. Speaker labels must be plain text without Markdown such as **, __, backticks or headings.
Never use a Lecturer, Narrator, Guide, Host, 旁白, 講者, 講解員 or any one-person speech. Higher difficulty must add interaction complexity, not change the conversation into a monologue.`
	}
	if profile.Section == 4 {
		return `If the IELTS controls specify a single-speaker monologue, split it into 8 consecutive lecture chunks from the same speaker; each chunk should develop notes, contrasts, causes, evidence, or examples instead of asking or answering questions.
For a single-speaker monologue, start directly with lecture content. Do not mention recording booths, upload deadlines, timers, worksheets, classroom logistics, apologies, or interruptions.`
	}
	return "For conversational listening sections, each turn should either ask for concrete information, provide clarification, confirm details, or make a practical decision."
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

func readingLengthLimitsForTopic(topic string) ielts.ReadingLengthLimits {
	meta := ielts.ReadingMetadataFromTopic(readingExamFromTopic(topic), topic, "")
	return ielts.ReadingLengthLimitsFromMetadata(readingExamFromTopic(topic), topic, meta)
}

func readingMinWords(topic string) int {
	return readingLengthLimitsForTopic(topic).MinWords
}

func readingExamFromTopic(topic string) string {
	if strings.Contains(strings.ToUpper(topic), "CET") {
		return "CET"
	}
	return "IELTS"
}

// Generate returns dialogues and comprehension questions with options and reference answers for server-side grading.
func (g *OpenAIGenerator) Generate(ctx context.Context, language string, topic string, difficulty float64, mode string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	apiKey := g.apiKey()
	if apiKey == "" {
		return nil, nil, fmt.Errorf("OPENAI_API_KEY is empty")
	}
	quizCount := requiredQuizCount(difficulty)
	readingMode := isReadingGeneration(mode, topic)
	listeningProfile := ielts.ListeningProfile{}
	if !readingMode {
		listeningProfile = ielts.ListeningProfileFromTopic(topic, difficulty)
		if isCantonese(language) {
			// Section 3 keeps question defaults conversational. Cantonese difficulty is
			// defined by its own controls below, not IELTS Section 4's monologue rule.
			listeningProfile.Section = 3
			listeningProfile.QuizCount = requiredQuizCount(difficulty)
		}
		quizCount = listeningProfile.QuizCount
	}
	if readingMode {
		quizCount = 5
	}
	sys := languageDirective(language) + " Output one JSON object only, no markdown fences."
	user := ""
	if readingMode {
		lengthLimits := readingLengthLimitsForTopic(topic)
		metadata := ielts.ReadingMetadataFromTopic(readingExamFromTopic(topic), topic, "")
		questionInstruction := readingQuestionInstruction(metadata.QuestionType)
		user = fmt.Sprintf(
			`Learning language code: %s.
Exam/topic brief: %s.
Difficulty/Band: %.1f.
Mode: %s.

Create one original IELTS/CET-style academic reading drill. Do not copy official test content.

Reading controls:
- Band: %.1f.
- Stage: %s.
- Question type: %s.
- Skill focus: %s.
- Scenario family: %s.
- Length requirement: total English word count MUST be between %d and %d words. Do not go below or above this range.
- Structure: %d to %d passage segments in "dialogues" array.
- Each segment text should be a coherent paragraph, not a chat turn.
- Paragraph length: %s.
- Band-specific difficulty: %s.
- Passage text must never include prompt labels, bracket tags, task instructions, or metadata.

JSON shape:
{"dialogues":[{"speaker":"Passage","text":"...","zhSubtitle":"..."}],"quiz":[{"type":"...","question":"...","paragraphRef":"...","evidence":"...","options":["..."],"answerKey":"...","headings":["..."],"summaryText":"...","wordBank":["..."],"answers":["..."],"statements":[{"id":"...","text":"...","answer":"..."}]}]}
Required top-level keys:
- The JSON object MUST contain both "dialogues" and "quiz".
- Do not stop after the "dialogues" array. The "quiz" array is mandatory.
- For Summary Completion, include BOTH "wordBank" and "options" with the same entries.
- For Matching Headings, include BOTH "headings" and "options" with the same heading bank.
Rules for dialogues.zhSubtitle:
- Must be concise Simplified Chinese explanation of that paragraph's core idea.
- Do not translate word-by-word.
Rules for quiz:
1) Create exactly %d questions.
2) Every question must be answerable from paragraph evidence and use paraphrase.
3) Avoid generic questions such as "What is the main focus of the passage?"
4) Use this exact task structure: %s
5) For any item with options, answerKey must exactly match one option.
6) evidence must be an exact short quote copied from the passage text, usually 4-18 words, not an explanation.
7) paragraphRef must be one paragraph label such as "Paragraph 4", not a range.`,
			language,
			topic,
			difficulty,
			mode,
			metadata.Band,
			metadata.Stage,
			metadata.QuestionType,
			metadata.SkillFocus,
			metadata.ScenarioFamily,
			lengthLimits.MinWords,
			lengthLimits.MaxWords,
			lengthLimits.MinSegments,
			lengthLimits.MaxSegments,
			lengthLimits.SegmentGuidance,
			lengthLimits.BandGuidance,
			quizCount,
			questionInstruction,
		)
	} else {
		cleanTopic := ielts.CleanTopic(topic)
		if cleanTopic == "" {
			cleanTopic = strings.TrimSpace(topic)
		}
		scenarioBrief, scenarioErr := g.expandConversationScenario(ctx, language, cleanTopic, difficulty, mode)
		if scenarioErr != nil {
			scenarioBrief = cleanTopic
		}
		user = fmt.Sprintf(
			`Learning language code: %s. Topic: %s. Scenario brief: %s. Difficulty: %.1f. Mode: %s.
Scene must be realistic and specific (place, time, roles). Use natural spoken lines for the target language.
%s
%s
Do NOT use classroom/meta narration such as "today's topic is...", "we are discussing...", "welcome to mini theater", or direct topic announcements.
The first turn must immediately enter a concrete real-life situation with actionable context (for example at a counter, station, office desk, clinic, or phone call).
If the topic is written in Simplified Chinese and the language is CANTONESE, first reinterpret the topic into a natural Hong Kong Cantonese life scenario internally, then write the dialogue in authentic Hong Kong Cantonese.
For CANTONESE, dialogue text must use genuinely colloquial Hong Kong Cantonese wording and grammar, with natural expressions such as 我哋、你哋、而家、唔該、冇、係咪、喺、嘅、咗、啲、嗰個 when contextually appropriate.
Do not write Mandarin sentences and merely convert them to Traditional Chinese. For example, avoid dialogue patterns such as 我們現在、請問您、可以嗎、沒有問題、這裡是; rewrite them as natural spoken Cantonese instead.
For CANTONESE, dialogues[].text must not contain A-Z letters, English words, English abbreviations or bracketed English explanations. Convert common terms into natural Hong Kong Chinese before returning JSON.
For CANTONESE, keep each turn to one or two naturally connected short sentences. Do not use semicolons, ellipses, bracketed asides or repeated punctuation; use commas only where a real speaker would take a very short breath.
All dialogue turns must stay consistent with the provided scenario brief.
Produce exactly 8 dialogue turns and exactly %d listening comprehension single-choice questions based ONLY on those dialogues.
For CANTONESE, use exactly two stable gender-identifying speaker labels such as 阿晴（女店員） and 陳先生（男顧客） instead of neutral labels like 店員/顧客. Use the exact same two labels for all 8 turns, with no third person or narrator. Speaker labels must be plain text without Markdown.
For CANTONESE, every dialogue item must set gender to exactly FEMALE or MALE. The value must match the speaker and remain identical for every turn by that speaker. For English, use clear roles such as Barista/Customer.
JSON shape:
{"dialogues":[{"speaker":"阿晴（女店員）","gender":"FEMALE","text":"...","zhSubtitle":"..."}],"quiz":[{"question":"...","options":["...","...","...","..."],"answerKey":"..."}]}
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
			language, cleanTopic, scenarioBrief, difficulty, mode, listeningControlsForGeneration(language, listeningProfile, difficulty), listeningFormInstruction(language, listeningProfile), quizCount,
		)
	}
	model := g.modelName()
	attempts := 2
	if readingMode {
		attempts = 3
	}
	operation := "THEATER_GENERATION"
	if readingMode {
		operation = "READING_GENERATION"
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		attemptUser := user
		if attempt > 0 {
			if readingMode {
				attemptUser += readingRegenerationInstruction(quizCount, topic)
			} else {
				attemptUser += listeningRegenerationInstruction(quizCount)
				if strings.EqualFold(strings.TrimSpace(language), "CANTONESE") {
					attemptUser += cantoneseRegenerationInstruction()
				}
			}
		}
		payload := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": sys},
				{"role": "user", "content": attemptUser},
			},
			"temperature": 0.65,
		}
		if readingMode {
			payload["max_tokens"] = 6000
		} else {
			payload["max_tokens"] = 4000
		}
		content, err := g.callModelJSONPayload(ctx, payload, operation)
		if err != nil && strings.Contains(err.Error(), "no parsable text") && !strings.EqualFold(model, defaultModelName) {
			log.Printf("model %s returned empty content, retry with fallback model %s", model, defaultModelName)
			payload["model"] = defaultModelName
			content, err = g.callModelJSONPayload(ctx, payload, operation)
		}
		if err != nil {
			lastErr = err
		} else {
			dialogues, quiz, parseErr := parseAndValidateModelContent(language, topic, readingMode, quizCount, content)
			if parseErr == nil {
				return dialogues, quiz[:quizCount], nil
			}
			if errors.Is(parseErr, errGeneratedCantoneseQuality) && len(dialogues) > 0 {
				rewritten, rewriteErr := g.rewriteDialoguesToCantonese(ctx, dialogues)
				if rewriteErr == nil {
					if validationErr := validateGeneratedOutput(language, false, rewritten, quiz); validationErr == nil {
						log.Printf("rewrote model dialogue into authentic Hong Kong Cantonese turns=%d", len(rewritten))
						return rewritten, quiz[:quizCount], nil
					} else {
						rewriteErr = validationErr
					}
				}
				log.Printf("cantonese dialogue rewrite failed attempt=%d err=%v", attempt+1, rewriteErr)
			}
			lastErr = parseErr
		}
		if attempt+1 < attempts {
			if readingMode {
				log.Printf("reading model output failed quality gate, retrying attempt=%d err=%v", attempt+1, lastErr)
				if isTransientModelError(lastErr) {
					time.Sleep(time.Duration(attempt+1) * 1200 * time.Millisecond)
				}
			} else {
				log.Printf("listening model output failed quality gate, retrying attempt=%d err=%v", attempt+1, lastErr)
			}
			continue
		}
	}
	return nil, nil, lastErr
}

func (g *OpenAIGenerator) GenerateReading(ctx context.Context, request domain.ReadingGenerationRequest) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	metadata := ielts.NormalizeReadingMetadata(request.Exam, request.Topic, request.Level, ielts.ReadingMetadata{
		Band: request.Band, Stage: request.Stage, Section: request.Section, SkillFocus: request.SkillFocus,
		QuestionType: request.QuestionType, ScenarioFamily: request.ScenarioFamily,
	})
	parts := []string{fmt.Sprintf("[%s Reading]", strings.ToUpper(strings.TrimSpace(request.Exam)))}
	if metadata.Stage != "" {
		parts = append(parts, "["+metadata.Stage+"]")
	}
	if metadata.Band > 0 {
		parts = append(parts, fmt.Sprintf("[Band %.1f]", metadata.Band))
	}
	if metadata.Section != "" {
		parts = append(parts, "["+metadata.Section+"]")
	}
	if metadata.QuestionType != "" {
		parts = append(parts, "["+metadata.QuestionType+"]")
	}
	if metadata.SkillFocus != "" {
		parts = append(parts, "[Focus: "+metadata.SkillFocus+"]")
	}
	if metadata.ScenarioFamily != "" {
		parts = append(parts, "[Scenario: "+metadata.ScenarioFamily+"]")
	}
	cleanTopic := ielts.CleanTopic(request.Topic)
	if cleanTopic == "" {
		cleanTopic = strings.TrimSpace(request.Topic)
	}
	parts = append(parts, cleanTopic)
	language := strings.TrimSpace(request.Language)
	if language == "" {
		language = "ENGLISH"
	}
	return g.Generate(ctx, language, strings.Join(parts, " "), metadata.Band, "APPRECIATION")
}

func isTransientModelError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "502") ||
		strings.Contains(message, "503") ||
		strings.Contains(message, "504") ||
		strings.Contains(message, "bad gateway") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "unexpected eof")
}

func listeningRegenerationInstruction(quizCount int) string {
	return fmt.Sprintf(`

Regenerate the full JSON object because the previous listening response failed validation.
Critical requirements for this retry:
- Return one complete JSON object with BOTH "dialogues" and "quiz" at the top level.
- Return exactly 8 dialogue turns and exactly %d quiz items.
- Keep each dialogue turn compact, usually 18-34 English words for Section 1-3 or 25-45 words for Section 4.
- Do not quote or mention IELTS, Band, Section, Focus, Task design, metadata, prompt text, or the topic string.
- Start directly inside the scene; no welcome message, no "today's topic", and no mini-theater narration.
- Keep the same difficulty by using corrections, delayed answers, competing details, and paraphrased quiz wording.`, quizCount)
}

func cantoneseRegenerationInstruction() string {
	return `

The previous dialogue was rejected because it sounded like Mandarin written with Traditional Chinese characters.
Rewrite every dialogues[].text line as authentic colloquial Hong Kong Cantonese.
Use Cantonese vocabulary, sentence structure, pronouns and particles naturally (for example 我哋、你哋、而家、唔該、冇、係咪、喺、嘅、咗、啲、嗰個).
Replace every English word or abbreviation with a natural Cantonese/Chinese equivalent. dialogues[].text must contain no A-Z letters.
Use exactly two stable speaker labels across all 8 turns, with no Markdown, third person or narrator. Make each speaker's gender explicit through a name, title or role, for example 阿晴（女店員） or 陳先生（男顧客）.
Every dialogue item must include gender as exactly FEMALE or MALE, matching the speaker and staying consistent across that speaker's turns.
Keep dialogues[].zhSubtitle and all quiz text in readable Simplified Chinese. Do not put Mandarin wording into dialogues[].text.`
}

func readingRegenerationInstruction(quizCount int, topic string) string {
	metadata := ielts.ReadingMetadataFromTopic(readingExamFromTopic(topic), topic, "")
	mixedInstruction := ""
	if ielts.QuestionTypeKey(metadata.QuestionType) == "mixed" {
		mixedInstruction = `
- For Mixed Question Set, create this exact quiz sequence: Multiple Choice, Matching Information, TFNG, Summary Completion, Multiple Choice.`
	}
	return fmt.Sprintf(`

Regenerate the full JSON object because the previous model response failed validation.
Critical requirements for this retry:
- Return BOTH "dialogues" and "quiz" at the top level.
- Return exactly %d quiz items.
- Keep every quiz item faithful to the requested question type and include all required fields.
- Do not shorten the passage to make room for quiz items.%s`, quizCount, mixedInstruction)
}

func parseAndValidateModelContent(language string, topic string, readingMode bool, quizCount int, content string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	dialogues, quiz := parseModelOutput(content)
	dialogues = normalizeGeneratedDialogues(language, dialogues)
	quiz = normalizeGeneratedQuiz(language, quiz)
	if readingMode {
		metadata := ielts.ReadingMetadataFromTopic(readingExamFromTopic(topic), topic, "")
		quiz = applyReadingQuestionDefaults(metadata.QuestionType, quiz)
	} else {
		profile := ielts.ListeningProfileFromTopic(topic, 0)
		if isCantonese(language) {
			profile.Section = 3
		}
		quiz = applyListeningQuestionDefaults(profile.Section, quiz)
	}
	if len(dialogues) == 0 {
		log.Printf("model output parse failed snippet=%q", modelOutputSnippet(content))
		return nil, nil, fmt.Errorf("model output parsing failed: missing dialogues")
	}
	if len(quiz) < quizCount {
		log.Printf("model output parse failed quiz_count=%d want=%d snippet=%q", len(quiz), quizCount, modelOutputSnippet(content))
		return nil, nil, fmt.Errorf("model output parsing failed: missing quiz questions")
	}
	if err := validateGeneratedOutput(language, readingMode, dialogues, quiz); err != nil {
		if errors.Is(err, errGeneratedCantoneseQuality) {
			return dialogues, quiz, err
		}
		return nil, nil, err
	}
	if readingMode {
		metadata := ielts.ReadingMetadataFromTopic(readingExamFromTopic(topic), topic, "")
		if err := validateReadingQuestionShape(metadata.QuestionType, quiz); err != nil {
			return nil, nil, err
		}
		lengthLimits := readingLengthLimitsForTopic(topic)
		if len(dialogues) < lengthLimits.MinSegments {
			return nil, nil, fmt.Errorf("model output has too few reading segments: got %d, want at least %d", len(dialogues), lengthLimits.MinSegments)
		}
		if len(dialogues) > lengthLimits.MaxSegments {
			return nil, nil, fmt.Errorf("model output has too many reading segments: got %d, want at most %d", len(dialogues), lengthLimits.MaxSegments)
		}
		wordCount := 0
		for _, d := range dialogues {
			wordCount += contentquality.WordCount(d.Text)
		}
		if wordCount < lengthLimits.MinWords {
			return nil, nil, fmt.Errorf("model output too short: got %d words, want at least %d", wordCount, lengthLimits.MinWords)
		}
		if wordCount > lengthLimits.MaxWords {
			return nil, nil, fmt.Errorf("model output too long: got %d words, want at most %d", wordCount, lengthLimits.MaxWords)
		}
	}
	return dialogues, quiz, nil
}

func applyListeningQuestionDefaults(section int, quiz []domain.QuizQuestion) []domain.QuizQuestion {
	defaultType := listeningQuestionType(section)
	for i := range quiz {
		if strings.TrimSpace(quiz[i].Type) == "" {
			quiz[i].Type = defaultType
		}
	}
	return quiz
}

func listeningQuestionType(section int) string {
	switch section {
	case 1:
		return "Form/Table Completion"
	case 2:
		return "Map/Instruction Detail"
	case 3:
		return "Opinion Matching"
	default:
		return "Note/Summary Completion"
	}
}

func modelOutputSnippet(content string) string {
	if len(content) > 320 {
		return content[:320]
	}
	return content
}

func readingQuestionInstruction(questionType string) string {
	switch ielts.QuestionTypeKey(questionType) {
	case "matching_headings":
		return `Matching Headings. Each quiz item should target one paragraph. Set type to "Matching Headings", paragraphRef to the paragraph label, headings to a shared-style heading bank of at least 5 headings, options to the same headings, and answerKey to the correct heading.`
	case "matching_information":
		return `Matching Information. Each quiz item should ask where ONE specific piece of information appears, not a multi-statement matching set. Set type to "Matching Information", options to paragraph labels, paragraphRef to the one correct paragraph, evidence to an exact quote from that paragraph, and answerKey to the same paragraph label as paragraphRef.`
	case "tfng":
		return `TFNG. Each quiz item should be a claim. Set type to "TFNG", options to ["TRUE","FALSE","NOT GIVEN"], evidence to the relevant sentence or "not stated", and answerKey to exactly TRUE, FALSE, or NOT GIVEN.`
	case "summary_completion":
		return `Summary Completion. Each quiz item should contain a short summary sentence with one blank. Set type to "Summary Completion", summaryText to the summary with a blank, wordBank to at least 5 words or phrases, and answerKey to the correct word or phrase from the wordBank.`
	case "mixed":
		return `Mixed Question Set. Create exactly five quiz items in this order: 1) Multiple Choice with four options, paragraphRef, evidence and answerKey; 2) Matching Information asking where ONE specific detail appears, with paragraph-label options, paragraphRef, exact-quote evidence, and answerKey equal to paragraphRef; 3) TFNG with options ["TRUE","FALSE","NOT GIVEN"]; 4) Summary Completion with summaryText, wordBank, options copied from wordBank, and answerKey; 5) Multiple Choice with four options, paragraphRef, evidence and answerKey. Set type on every item and keep each item's structure faithful to that type.`
	default:
		return `Multiple Choice. Each quiz item must have four plausible options, paragraphRef, evidence, and one answerKey that exactly matches an option.`
	}
}

func applyReadingQuestionDefaults(questionType string, quiz []domain.QuizQuestion) []domain.QuizQuestion {
	defaultType := strings.TrimSpace(questionType)
	if defaultType == "" {
		defaultType = "Multiple Choice"
	}
	for i := range quiz {
		if strings.TrimSpace(quiz[i].Type) == "" {
			quiz[i].Type = defaultType
		}
		switch ielts.QuestionTypeKey(quiz[i].Type) {
		case "matching_headings":
			if len(quiz[i].Headings) == 0 && len(quiz[i].Options) > 0 {
				quiz[i].Headings = append([]string{}, quiz[i].Options...)
			}
			if len(quiz[i].Options) == 0 && len(quiz[i].Headings) > 0 {
				quiz[i].Options = append([]string{}, quiz[i].Headings...)
			}
		case "tfng":
			if len(quiz[i].Options) == 0 {
				quiz[i].Options = []string{"TRUE", "FALSE", "NOT GIVEN"}
			}
		case "summary_completion":
			if len(quiz[i].WordBank) == 0 && len(quiz[i].Options) > 0 {
				quiz[i].WordBank = append([]string{}, quiz[i].Options...)
			}
			if len(quiz[i].Options) == 0 && len(quiz[i].WordBank) > 0 {
				quiz[i].Options = append([]string{}, quiz[i].WordBank...)
			}
			if len(quiz[i].Answers) == 0 && strings.TrimSpace(quiz[i].AnswerKey) != "" {
				quiz[i].Answers = []string{quiz[i].AnswerKey}
			}
		}
	}
	return quiz
}

func validateReadingQuestionShape(questionType string, quiz []domain.QuizQuestion) error {
	expectedKey := ielts.QuestionTypeKey(questionType)
	for i, q := range quiz {
		key := ielts.QuestionTypeKey(q.Type)
		if expectedKey != "mixed" && key != expectedKey {
			return fmt.Errorf("reading question %d type mismatch: got %q want %q", i+1, q.Type, questionType)
		}
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("reading question %d is empty", i+1)
		}
		switch key {
		case "matching_headings":
			if strings.TrimSpace(q.ParagraphRef) == "" {
				return fmt.Errorf("matching headings question %d missing paragraphRef", i+1)
			}
			if len(q.Headings) < 4 && len(q.Options) < 4 {
				return fmt.Errorf("matching headings question %d missing heading bank", i+1)
			}
		case "matching_information":
			if len(q.Options) < 3 || strings.TrimSpace(q.AnswerKey) == "" || strings.TrimSpace(q.ParagraphRef) == "" {
				return fmt.Errorf("matching information question %d missing paragraph options, paragraphRef, or answer", i+1)
			}
			if !answerInSet(q.AnswerKey, q.Options) {
				return fmt.Errorf("matching information question %d answerKey %q is not one of the paragraph options", i+1, q.AnswerKey)
			}
			if !strings.EqualFold(strings.TrimSpace(q.AnswerKey), strings.TrimSpace(q.ParagraphRef)) {
				return fmt.Errorf("matching information question %d paragraphRef %q must equal answerKey %q", i+1, q.ParagraphRef, q.AnswerKey)
			}
			if strings.Contains(strings.ToLower(q.ParagraphRef), "paragraphs") {
				return fmt.Errorf("matching information question %d paragraphRef must be one paragraph, got %q", i+1, q.ParagraphRef)
			}
		case "tfng":
			if !answerInSet(q.AnswerKey, []string{"TRUE", "FALSE", "NOT GIVEN"}) {
				return fmt.Errorf("tfng question %d has invalid answerKey %q", i+1, q.AnswerKey)
			}
		case "summary_completion":
			if strings.TrimSpace(q.SummaryText) == "" || len(q.WordBank) < 3 || strings.TrimSpace(q.AnswerKey) == "" {
				return fmt.Errorf("summary completion question %d missing summaryText, wordBank, or answer", i+1)
			}
		default:
			if len(q.Options) != 4 || strings.TrimSpace(q.AnswerKey) == "" {
				return fmt.Errorf("multiple choice question %d must have four options and answerKey", i+1)
			}
		}
	}
	return nil
}

func answerInSet(answer string, allowed []string) bool {
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(answer), item) {
			return true
		}
	}
	return false
}

func normalizeGeneratedDialogues(language string, dialogues []domain.Dialogue) []domain.Dialogue {
	for i := range dialogues {
		dialogues[i].Speaker = contentquality.NormalizeSpeakerLabel(dialogues[i].Speaker)
		dialogues[i].Gender = normalizeDialogueGender(dialogues[i].Gender)
	}
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "ENGLISH":
		for i := range dialogues {
			dialogues[i].Text = contentquality.NormalizeEnglishSpacing(dialogues[i].Text)
		}
	case "CANTONESE":
		for i := range dialogues {
			dialogues[i].Text = contentquality.NormalizeCantoneseSpeechText(dialogues[i].Text)
		}
	}
	return dialogues
}

func normalizeDialogueGender(value string) string {
	clean := strings.ToUpper(strings.TrimSpace(value))
	clean = strings.Trim(clean, "`*_#：:()（）[]【】 ")
	switch clean {
	case "FEMALE", "F", "WOMAN", "GIRL", "女", "女性", "女聲", "女声":
		return "FEMALE"
	case "MALE", "M", "MAN", "BOY", "男", "男性", "男聲", "男声":
		return "MALE"
	default:
		return ""
	}
}

func normalizeGeneratedQuiz(language string, quiz []domain.QuizQuestion) []domain.QuizQuestion {
	if !strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		return quiz
	}
	for i := range quiz {
		quiz[i].Question = contentquality.NormalizeEnglishSpacing(quiz[i].Question)
		for j := range quiz[i].Options {
			quiz[i].Options[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Options[j])
		}
		quiz[i].AnswerKey = contentquality.NormalizeEnglishSpacing(quiz[i].AnswerKey)
		quiz[i].ParagraphRef = contentquality.NormalizeEnglishSpacing(quiz[i].ParagraphRef)
		quiz[i].Evidence = contentquality.NormalizeEnglishSpacing(quiz[i].Evidence)
		for j := range quiz[i].Headings {
			quiz[i].Headings[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Headings[j])
		}
		quiz[i].SummaryText = contentquality.NormalizeEnglishSpacing(quiz[i].SummaryText)
		for j := range quiz[i].WordBank {
			quiz[i].WordBank[j] = contentquality.NormalizeEnglishSpacing(quiz[i].WordBank[j])
		}
		for j := range quiz[i].Answers {
			quiz[i].Answers[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Answers[j])
		}
		for j := range quiz[i].Statements {
			quiz[i].Statements[j].Text = contentquality.NormalizeEnglishSpacing(quiz[i].Statements[j].Text)
			quiz[i].Statements[j].Answer = contentquality.NormalizeEnglishSpacing(quiz[i].Statements[j].Answer)
		}
	}
	return quiz
}

func validateGeneratedOutput(language string, readingMode bool, dialogues []domain.Dialogue, quiz []domain.QuizQuestion) error {
	english := strings.EqualFold(strings.TrimSpace(language), "ENGLISH")
	for i, dialogue := range dialogues {
		if err := contentquality.ValidateReadableText(fmt.Sprintf("dialogue %d", i+1), dialogue.Text, english); err != nil {
			return err
		}
	}
	genericReadingQuestions := 0
	for i, question := range quiz {
		if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d", i+1), question.Question, english); err != nil {
			return err
		}
		for j, option := range question.Options {
			if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d option %d", i+1, j+1), option, english); err != nil {
				return err
			}
		}
		if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d answer", i+1), question.AnswerKey, english); err != nil {
			return err
		}
		if readingMode && contentquality.IsGenericReadingQuestion(question.Question) {
			genericReadingQuestions++
		}
	}
	if readingMode && genericReadingQuestions >= 2 {
		return fmt.Errorf("reading generation returned too many generic questions: %d", genericReadingQuestions)
	}
	if !readingMode && strings.EqualFold(strings.TrimSpace(language), "CANTONESE") {
		if err := validateCantoneseDialogues(dialogues); err != nil {
			return err
		}
	}
	return nil
}

func validateCantoneseDialogues(dialogues []domain.Dialogue) error {
	if len(dialogues) != 8 {
		return fmt.Errorf("%w: dialogue_turns=%d want=8", errGeneratedCantoneseQuality, len(dialogues))
	}
	forbiddenSpeakerTerms := []string{
		"lecturer", "narrator", "guide", "host", "speaker", "旁白", "講者", "讲者", "講解", "讲解", "主持",
	}
	speakerTurns := make(map[string]int)
	speakerGenders := make(map[string]string)
	markers := []string{
		"我哋", "你哋", "佢哋", "而家", "唔該", "唔好", "唔係", "唔知", "冇", "係咪",
		"喺", "嘅", "咗", "啲", "嗰個", "呢個", "邊個", "乜嘢", "點解", "點樣",
		"畀", "攞", "睇", "嚟", "返去", "落單", "埋單", "即刻", "陣間", "搞掂",
		"得唔得", "可唔可以", "使唔使", "好唔好", "啦", "喎", "啫", "吓",
	}
	mandarinPatterns := []string{
		"我們現在", "你們現在", "他們現在", "請問您", "可以嗎", "是不是", "沒有問題",
		"這裡是", "那裡是", "什麼時候", "怎麼辦", "知道了", "好的，", "好的。",
	}
	nonEmptyLines := 0
	markerLines := 0
	markerHits := 0
	mandarinHits := 0
	for _, dialogue := range dialogues {
		speaker := contentquality.NormalizeSpeakerLabel(dialogue.Speaker)
		if speaker == "" {
			return fmt.Errorf("%w: dialogue has an empty speaker", errGeneratedCantoneseQuality)
		}
		lowerSpeaker := strings.ToLower(speaker)
		for _, term := range forbiddenSpeakerTerms {
			if strings.Contains(lowerSpeaker, term) {
				return fmt.Errorf("%w: forbidden monologue speaker=%q", errGeneratedCantoneseQuality, speaker)
			}
		}
		speakerTurns[lowerSpeaker]++
		gender := normalizeDialogueGender(dialogue.Gender)
		if gender == "" {
			return fmt.Errorf("%w: speaker=%q has missing or invalid gender", errGeneratedCantoneseQuality, speaker)
		}
		if existing := speakerGenders[lowerSpeaker]; existing != "" && existing != gender {
			return fmt.Errorf("%w: speaker=%q has inconsistent gender %s/%s", errGeneratedCantoneseQuality, speaker, existing, gender)
		}
		speakerGenders[lowerSpeaker] = gender
		text := strings.TrimSpace(dialogue.Text)
		if text == "" {
			continue
		}
		if contentquality.ContainsLatinLetters(text) {
			return fmt.Errorf("%w: dialogue contains English text speaker=%q", errGeneratedCantoneseQuality, speaker)
		}
		nonEmptyLines++
		lineHasMarker := false
		for _, marker := range markers {
			if strings.Contains(text, marker) {
				markerHits += strings.Count(text, marker)
				lineHasMarker = true
			}
		}
		if lineHasMarker {
			markerLines++
		}
		for _, pattern := range mandarinPatterns {
			mandarinHits += strings.Count(text, pattern)
		}
	}
	if nonEmptyLines == 0 {
		return fmt.Errorf("%w: no dialogue text", errGeneratedCantoneseQuality)
	}
	if len(speakerTurns) != 2 {
		return fmt.Errorf("%w: speaker_count=%d want=2", errGeneratedCantoneseQuality, len(speakerTurns))
	}
	mainSpeakers := 0
	for _, turns := range speakerTurns {
		if turns >= 3 {
			mainSpeakers++
		}
	}
	if mainSpeakers != 2 {
		return fmt.Errorf("%w: main_speakers=%d want=2", errGeneratedCantoneseQuality, mainSpeakers)
	}
	minimumMarkerLines := max(2, (nonEmptyLines+1)/2)
	if markerLines < minimumMarkerLines || markerHits < minimumMarkerLines || mandarinHits >= 2 {
		return fmt.Errorf("%w: marker_lines=%d/%d marker_hits=%d mandarin_hits=%d", errGeneratedCantoneseQuality, markerLines, nonEmptyLines, markerHits, mandarinHits)
	}
	return nil
}

func (g *OpenAIGenerator) rewriteDialoguesToCantonese(ctx context.Context, dialogues []domain.Dialogue) ([]domain.Dialogue, error) {
	type rewriteDialogue struct {
		Speaker    string `json:"speaker"`
		Gender     string `json:"gender"`
		Text       string `json:"text"`
		ZhSubtitle string `json:"zhSubtitle"`
	}
	input := make([]rewriteDialogue, 0, len(dialogues))
	for _, dialogue := range dialogues {
		input = append(input, rewriteDialogue{
			Speaker: dialogue.Speaker, Gender: dialogue.Gender, Text: dialogue.Text, ZhSubtitle: dialogue.ZhSubtitle,
		})
	}
	rawInput, err := json.Marshal(map[string]any{"dialogues": input})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model": g.modelName(),
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "你係專業香港粵語編劇。將對白改寫成自然、地道、口語化嘅香港粵語，唔可以只將普通話轉做繁體字，亦唔可以保留英文單字或縮寫。全程只可以有兩個角色，角色名稱要穩定並清楚反映性別。每輪都要返回 FEMALE 或 MALE。只返回完整 JSON。",
			},
			{
				"role":    "user",
				"content": `Rewrite the following dialogues[].text into authentic colloquial Hong Kong Cantonese in Traditional Chinese. Preserve every factual detail, intent, turn count and order. Use exactly two stable speakers and never add a third person or narrator. Replace every English word and abbreviation with a natural Cantonese/Chinese equivalent, so text contains no A-Z letters. Keep each speaker stable, plain text without Markdown, and use a gender-identifying name, title or role. Set every item's gender to exactly FEMALE or MALE and keep it consistent for that speaker. Use natural Cantonese grammar and expressions such as 我哋、你哋、而家、唔該、冇、係咪、喺、嘅、咗、啲 where appropriate. Keep zhSubtitle as concise Mandarin-style Simplified Chinese. Return exactly {"dialogues":[{"speaker":"阿晴（女店員）","gender":"FEMALE","text":"...","zhSubtitle":"..."}]}. Input: ` + string(rawInput),
			},
		},
		"temperature": 0.2,
		"max_tokens":  3000,
	}
	content, err := g.callModelJSONPayload(ctx, payload, "CANTONESE_REWRITE")
	if err != nil {
		return nil, err
	}
	rewritten, _ := parseModelOutput(content)
	if len(rewritten) != len(dialogues) {
		return nil, fmt.Errorf("cantonese rewrite changed turn count: got %d want %d", len(rewritten), len(dialogues))
	}
	for i := range rewritten {
		if strings.TrimSpace(rewritten[i].Speaker) == "" {
			rewritten[i].Speaker = dialogues[i].Speaker
		}
		if normalizeDialogueGender(rewritten[i].Gender) == "" {
			rewritten[i].Gender = dialogues[i].Gender
		}
		if strings.TrimSpace(rewritten[i].ZhSubtitle) == "" {
			rewritten[i].ZhSubtitle = dialogues[i].ZhSubtitle
		}
		rewritten[i].Timestamp = dialogues[i].Timestamp
	}
	rewritten = normalizeGeneratedDialogues("CANTONESE", rewritten)
	if err := validateCantoneseDialogues(rewritten); err != nil {
		return nil, err
	}
	return rewritten, nil
}

func (g *OpenAIGenerator) expandConversationScenario(ctx context.Context, language string, topic string, difficulty float64, mode string) (string, error) {
	model := g.modelName()
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
	content, err := g.callModelJSONPayload(ctx, payload, "THEATER_SCENARIO")
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
	baseURL := g.baseURL()
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}

	parsed, err := url.Parse(baseURL)
	if err == nil && strings.Trim(parsed.Path, "/") == "" {
		return strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	}
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

func shouldRetryModelStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func (g *OpenAIGenerator) callModelJSONPayload(ctx context.Context, payload map[string]any, operationValues ...string) (string, error) {
	raw, _ := json.Marshal(payload)
	chatURL := g.chatCompletionsURL()
	apiKey := g.apiKey()
	operation := "MODEL_COMPLETION"
	if len(operationValues) > 0 && strings.TrimSpace(operationValues[0]) != "" {
		operation = strings.TrimSpace(operationValues[0])
	}
	model, _ := payload["model"].(string)
	if strings.TrimSpace(model) == "" {
		model = g.modelName()
	}
	var lastErr error

	for attempt := 0; attempt <= modelAPIMaxRetries; attempt++ {
		startedAt := time.Now()
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
			g.recordModelUsage(model, operation, 0, 0, 0, false, true, time.Since(startedAt))
			lastErr = fmt.Errorf("request model API failed: %w", err)
		} else {
			var retryable bool
			var content string
			func() {
				defer resp.Body.Close()
				if resp.StatusCode >= 400 {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					g.recordModelUsage(model, operation, 0, 0, 0, false, true, time.Since(startedAt))
					lastErr = fmt.Errorf("model API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
					retryable = shouldRetryModelStatus(resp.StatusCode)
					return
				}
				body, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					g.recordModelUsage(model, operation, 0, 0, 0, false, true, time.Since(startedAt))
					lastErr = readErr
					return
				}
				promptTokens, completionTokens, totalTokens, usageReported := extractModelUsageFromResponse(body)
				content, lastErr = extractModelTextFromResponse(body)
				g.recordModelUsage(model, operation, promptTokens, completionTokens, totalTokens, usageReported, lastErr != nil, time.Since(startedAt))
				if lastErr != nil {
					content, lastErr = extractModelTextFromResponse(body)
					if lastErr != nil {
						return
					}
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

func (g *OpenAIGenerator) recordModelUsage(model string, operation string, promptTokens int64, completionTokens int64, totalTokens int64, usageReported bool, failed bool, latency time.Duration) {
	g.mu.RLock()
	reporter := g.analytics
	provider := normalizedModelProvider(g.Provider)
	g.mu.RUnlock()
	if reporter != nil {
		reporter.RecordModelUsage(provider, strings.TrimSpace(model), operation, promptTokens, completionTokens, totalTokens, usageReported, failed, latency)
	}
}

func extractModelUsageFromResponse(body []byte) (promptTokens int64, completionTokens int64, totalTokens int64, reported bool) {
	var payload struct {
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
			InputTokens      int64 `json:"input_tokens"`
			OutputTokens     int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Usage == nil {
		return 0, 0, 0, false
	}
	promptTokens = payload.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = payload.Usage.InputTokens
	}
	completionTokens = payload.Usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = payload.Usage.OutputTokens
	}
	totalTokens = payload.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	return promptTokens, completionTokens, totalTokens, true
}

func modelPayloadWithStreaming(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	out["stream"] = true
	return out
}

func extractModelTextFromStreamResponse(body []byte) (string, error) {
	text := strings.TrimSpace(string(body))
	if !strings.Contains(text, "data:") {
		return "", fmt.Errorf("model stream response has no data events")
	}

	var chunks []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		chunk, err := extractModelTextFromStreamChunk([]byte(data))
		if err != nil {
			return "", err
		}
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	if len(chunks) == 0 {
		snippet := text
		if len(snippet) > 320 {
			snippet = snippet[:320]
		}
		return "", fmt.Errorf("model stream response returned no text chunks, snippet=%q", snippet)
	}
	return strings.Join(chunks, ""), nil
}

func extractModelTextFromStreamChunk(body []byte) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}
	if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
		var chunks []string
		for _, choice := range choices {
			first, ok := choice.(map[string]any)
			if !ok {
				continue
			}
			if delta, ok := first["delta"].(map[string]any); ok {
				if content, ok := rawString(delta["content"]); ok && content != "" {
					chunks = append(chunks, content)
				}
			}
			if text, ok := rawString(first["text"]); ok && text != "" {
				chunks = append(chunks, text)
			}
		}
		if len(chunks) > 0 {
			return strings.Join(chunks, ""), nil
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
				for _, value := range content {
					block, ok := value.(map[string]any)
					if !ok {
						continue
					}
					if text, ok := rawString(firstNonNil(block["text"], block["content"])); ok && text != "" {
						chunks = append(chunks, text)
					}
				}
			}
		}
		if len(chunks) > 0 {
			return strings.Join(chunks, ""), nil
		}
	}
	return "", nil
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
	Gender     string `json:"gender"`
	Sex        string `json:"sex"`
	Text       string `json:"text"`
	ZhSubtitle string `json:"zhSubtitle"`
	SubtitleZh string `json:"subtitleZh"`
	ZhText     string `json:"zhText"`
}

type genQuiz struct {
	Question           string                    `json:"question"`
	Prompt             string                    `json:"prompt"`
	Title              string                    `json:"title"`
	Options            []string                  `json:"options"`
	Choices            []string                  `json:"choices"`
	Candidates         []string                  `json:"candidates"`
	AnswerKey          string                    `json:"answerKey"`
	Answer             string                    `json:"answer"`
	Correct            string                    `json:"correct"`
	CorrectAnswer      string                    `json:"correctAnswer"`
	Type               string                    `json:"type"`
	QuestionType       string                    `json:"questionType"`
	ParagraphRef       string                    `json:"paragraphRef"`
	Paragraph          string                    `json:"paragraph"`
	Location           string                    `json:"location"`
	Evidence           string                    `json:"evidence"`
	Rationale          string                    `json:"rationale"`
	SupportingEvidence string                    `json:"supportingEvidence"`
	Headings           []string                  `json:"headings"`
	HeadingOptions     []string                  `json:"headingOptions"`
	Statements         []domain.ReadingStatement `json:"statements"`
	SummaryText        string                    `json:"summaryText"`
	Summary            string                    `json:"summary"`
	WordBank           []string                  `json:"wordBank"`
	Words              []string                  `json:"words"`
	Answers            []string                  `json:"answers"`
	AnswerKeys         []string                  `json:"answerKeys"`
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
		gender := asString(firstNonNil(entry["gender"], entry["sex"]))
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
			Gender:     normalizeDialogueGender(gender),
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
	aliased := make([]genQuiz, 0, len(quizList))
	for _, item := range quizList {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		aliased = append(aliased, genQuiz{
			Question:     asString(firstNonNil(entry["question"], entry["prompt"], entry["title"])),
			Options:      toStringSlice(firstNonNil(entry["options"], entry["choices"], entry["candidates"])),
			AnswerKey:    asString(firstNonNil(entry["answerKey"], entry["answer"], entry["correct"], entry["correctAnswer"])),
			Type:         asString(firstNonNil(entry["type"], entry["questionType"])),
			ParagraphRef: asString(firstNonNil(entry["paragraphRef"], entry["paragraph"], entry["location"])),
			Evidence:     asString(firstNonNil(entry["evidence"], entry["rationale"], entry["supportingEvidence"])),
			Headings:     toStringSlice(firstNonNil(entry["headings"], entry["headingOptions"])),
			SummaryText:  asString(firstNonNil(entry["summaryText"], entry["summary"])),
			WordBank:     toStringSlice(firstNonNil(entry["wordBank"], entry["words"])),
			Answers:      toStringSlice(firstNonNil(entry["answers"], entry["answerKeys"])),
		})
	}
	return normalizeQuiz(aliased)
}

func normalizeQuiz(input []genQuiz) []domain.QuizQuestion {
	quiz := make([]domain.QuizQuestion, 0, len(input))
	for _, q := range input {
		questionType := firstNonEmptyString(q.Type, q.QuestionType)
		questionKey := ielts.QuestionTypeKey(questionType)
		question := firstNonEmptyString(q.Question, q.Prompt, q.Title)
		options := firstNonEmptyStringSlice(q.Options, q.Choices, q.Candidates)
		headings := firstNonEmptyStringSlice(q.Headings, q.HeadingOptions)
		wordBank := firstNonEmptyStringSlice(q.WordBank, q.Words)
		answers := firstNonEmptyStringSlice(q.Answers, q.AnswerKeys)
		answerKey := firstNonEmptyString(q.AnswerKey, q.Answer, q.Correct, q.CorrectAnswer)
		paragraphRef := firstNonEmptyString(q.ParagraphRef, q.Paragraph, q.Location)
		evidence := firstNonEmptyString(q.Evidence, q.Rationale, q.SupportingEvidence)
		summaryText := firstNonEmptyString(q.SummaryText, q.Summary)

		if answerKey == "" && len(answers) > 0 {
			answerKey = answers[0]
		}
		switch questionKey {
		case "matching_headings":
			if len(headings) == 0 && len(options) > 0 {
				headings = append([]string{}, options...)
			}
			if len(options) == 0 && len(headings) > 0 {
				options = append([]string{}, headings...)
			}
			if question == "" && paragraphRef != "" {
				question = "Choose the best heading for " + paragraphRef + "."
			}
		case "tfng":
			if len(options) == 0 {
				options = []string{"TRUE", "FALSE", "NOT GIVEN"}
			}
		case "summary_completion":
			if len(wordBank) == 0 && len(options) > 0 {
				wordBank = append([]string{}, options...)
			}
			if len(options) == 0 && len(wordBank) > 0 {
				options = append([]string{}, wordBank...)
			}
			if len(answers) == 0 && answerKey != "" {
				answers = []string{answerKey}
			}
			if question == "" && summaryText != "" {
				question = "Complete the summary using the word bank."
			}
		}
		if answerKey != "" && len(options) > 0 {
			answerKey = alignAnswerKeyToOption(answerKey, options)
		}
		if strings.TrimSpace(question) == "" || strings.TrimSpace(answerKey) == "" {
			continue
		}
		if questionKey == "" && len(options) == 0 {
			continue
		}
		quiz = append(quiz, domain.QuizQuestion{
			Question:     question,
			Options:      options,
			AnswerKey:    answerKey,
			Type:         questionType,
			ParagraphRef: paragraphRef,
			Evidence:     evidence,
			Headings:     headings,
			Statements:   q.Statements,
			SummaryText:  summaryText,
			WordBank:     wordBank,
			Answers:      answers,
		})
	}
	return quiz
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyStringSlice(values ...[]string) []string {
	for _, value := range values {
		if trimmed := trimStringSlice(value); len(trimmed) > 0 {
			return trimmed
		}
	}
	return nil
}

func alignAnswerKeyToOption(answerKey string, options []string) string {
	clean := strings.TrimSpace(answerKey)
	for _, option := range options {
		if option == clean {
			return clean
		}
	}
	for _, option := range options {
		if strings.EqualFold(option, clean) {
			return option
		}
	}
	return clean
}

func trimStringSlice(input []string) []string {
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
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
	apiKey := g.apiKey()
	if apiKey == "" {
		return domain.ReadingAnalysis{}, fmt.Errorf("OPENAI_API_KEY is empty")
	}
	model := g.modelName()
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
	content, err := g.callModelJSONPayload(ctx, payload, "READING_ANALYSIS")
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

func rawString(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
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
			Gender:     normalizeDialogueGender(firstNonEmptyString(item.Gender, item.Sex)),
			Text:       item.Text,
			ZhSubtitle: zhSubtitle,
			AudioURL:   "",
			Timestamp:  float64(index) * 2.3,
		})
	}
	return result
}
