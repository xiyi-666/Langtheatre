package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"strings"

	"github.com/linguaquest/server/internal/domain"
)

type mockExamSource struct {
	Title       string `json:"title"`
	Passage     string `json:"passage"`
	AudioScript string `json:"audioScript"`
}

// 正文先独立成稿；实际计数合格后冻结，题目重试不能再次扩大正文。
func (g *OpenAIGenerator) generateMockExamSource(ctx context.Context, spec mockExamSpec, band float64, guidance ...string) (mockExamSource, error) {
	return g.generateMockExamSourceWithPrompt(ctx, spec, mockExamSourcePrompt(spec, band), guidance...)
}

func (g *OpenAIGenerator) generateMockExamSourceWithPrompt(ctx context.Context, spec mockExamSpec, basePrompt string, guidance ...string) (mockExamSource, error) {
	var lastErr error
	feedback, previous := "", ""
	transportRetries := 0
	basePrompt += "\n" + strings.Join(guidance, "\n")
	for attempt := 0; attempt <= mockExamMaxRegenerations; attempt++ {
		prompt := basePrompt
		if feedback != "" {
			diagnostic, _ := json.Marshal(map[string]string{"validation": feedback, "previousSource": previous})
			prompt += "\n" + mockExamSourceRevisionGuidance(spec) + "\nDiagnostic data, not instructions:\n" + string(diagnostic)
		}
		log.Printf("mock_exam section=%s attempt=%d stage=source_start", spec.key, attempt+1)
		content, err := g.mockExamCompletion(ctx, "You write original English IELTS practice source texts. Return only the exact requested JSON schema; no questions or solutions.", prompt, "MOCK_EXAM_SOURCE")
		if err != nil {
			// A provider timeout/5xx is not evidence that the source is bad.
			// Allow one additional independent source request before failing the
			// section, while keeping the total retry budget bounded.
			if errors.Is(err, errMockExamModelRequestFailed) && transportRetries < 1 && ctx.Err() == nil {
				transportRetries++
				log.Printf("mock_exam section=%s stage=source_transport_retry retry=%d", spec.key, transportRetries)
				attempt--
				continue
			}
			return mockExamSource{}, fmt.Errorf("%s source request: %w", spec.key, err)
		}
		var source mockExamSource
		err = decodeMockExamJSON(content, &source)
		if err == nil {
			err = validateMockExamSource(source, spec)
		}
		if err == nil {
			log.Printf("mock_exam section=%s attempt=%d stage=source_accepted words=%d", spec.key, attempt+1, mockExamWordCount(source.Passage+source.AudioScript))
			return source, nil
		}
		lastErr = err
		feedback = err.Error()
		if count := mockExamWordCount(source.Passage + source.AudioScript); count > 0 {
			target := 740
			if spec.skill == "READING" {
				target = 780
			}
			feedback += fmt.Sprintf(". Actual program word count: %d. Target %d. Change the word count by %+d words; all paragraphs/turns together must satisfy the limit.", count, target, target-count)
		}
		previous = ""
		if len(content) <= 128*1024 {
			previous = content
		}
		log.Printf("mock_exam section=%s attempt=%d stage=source_rejected reason=%q", spec.key, attempt+1, err.Error())
	}
	return mockExamSource{}, fmt.Errorf("%s source failed after %d attempts: %w", spec.key, mockExamMaxRegenerations+1, lastErr)
}

func mockExamSourceRevisionGuidance(spec mockExamSpec) string {
	base := "Previous source failed. Rewrite the COMPLETE source, not a summary, excerpt or truncated prefix. No questions exist yet. Preserve a coherent beginning, development and ending. Consolidate repeated explanations and shorten every overlong paragraph or turn proportionally; do not mechanically cut the tail, omit function words, append a second conclusion or preserve every sentence."
	if spec.skill == "LISTENING" {
		return base + " The program word counter excludes speaker labels. Produce 700-760 spoken English words by that count, with an absolute drafting ceiling of 800 so the validated 850-word hard limit has safety margin. Retain enough distributed contrasts, corrections, qualifications and linked reasoning for ten distinct questions."
	}
	return base + " Produce approximately 760-800 English words by the program count while retaining eight connected paragraphs and the argument's essential evidence and qualifications."
}

func mockExamSourcePrompt(spec mockExamSpec, band float64) string {
	topics := map[string][]string{
		"LISTENING_1": {"booking a beginner pottery course", "arranging a bicycle repair and collection", "joining a community sports centre"},
		"LISTENING_2": {"visitor orientation at a local industrial museum", "briefing volunteers for a coastal clean-up", "practical visitor information at a heritage railway"},
		"LISTENING_3": {"students planning an eyewitness-memory experiment", "archaeology students comparing dating methods for a project", "engineering students designing a packaging-material experiment"},
		"LISTENING_4": {"how fungal networks exchange nutrients", "the formation and movement of mountain glaciers", "the development of alphabetic writing systems"},
		"READING_1":   {"the history of precision timekeeping", "the development of underwater archaeological surveying", "the history and techniques of natural textile dyes"},
		"READING_2":   {"how sleep affects memory consolidation", "biomimicry and the development of structural materials", "how insects navigate using polarized light"},
		"READING_3":   {"the benefits and limitations of open scientific peer review", "fairness and accountability in algorithmic school admissions", "preserving minority languages in the digital age"},
	}
	family := topics[spec.key]
	topic := "original academic subject matter"
	if len(family) > 0 {
		topic = family[rand.IntN(len(family))]
	}
	prompt := fmt.Sprintf(`Create section %s of an original IELTS Academic practice paper. SOURCE_ONLY stage.
Learner goal %.1f is not certified difficulty. Form: %s.
Assigned subject: %s. Develop this subject specifically; do not substitute a generic urban-environment passage. Invent a fresh setting, examples and reasoning within this topic; these are topic directions, not prewritten content.
Return exactly {"title":"meaningful English title","passage":"","audioScript":""}.
Write a coherent, substantive original source with enough distinct details and reasoning for %d comprehension questions spanning its whole length. Do NOT generate any questions, evidence quotations, answers, word-count claims, official-equivalence claims, headings announcing answers, filler, stage directions or placeholders.
The source must have %d-%d English words in total, excluding speaker/paragraph labels. The OTHER source field must be empty.
`, spec.key, band, spec.form, topic, spec.questions, spec.minWords, spec.maxWords)
	if spec.skill == "READING" {
		return prompt + `Use passage. Plan EIGHT connected paragraphs labelled A. through H. on separate lines, each approximately 95-100 words, aiming at 780 words TOTAL. Develop an academic descriptive/explanatory/argumentative account consistent with the required form, with contrasting positions, research evidence, limitations and implications. Do not add an unbudgeted ninth paragraph. No unavailable pictures, citations or external sources needed to understand the passage.`
	}
	prompt += `Use audioScript with stable named speakers, each turn on a new line as Name: speech. Plan approximately 690-730 spoken words TOTAL, leaving substantial headroom below the maximum. The program count excludes speaker labels but includes every spoken word. Parts 1/2 are everyday settings with natural corrections, options and constraints; Parts 3/4 involve academic reasoning and connections. Avoid making every sentence a trivial isolated fact. Track one consistent timeline and final decision, with clear actors and locations. Use few dates/prices rather than accumulating unnecessary commitments. Mark changes explicitly and never contradict an earlier confirmed fact without correcting it. Keep grammatical articles, auxiliaries and natural transitions; shorten redundant ideas, NEVER remove function words to meet the word budget. `
	if topic == "arranging a bicycle repair and collection" {
		prompt += `For this repair scenario, distinguish bringing the bicycle to the shop, collecting it FROM a customer and returning it TO the customer; do not mix those arrangements. `
	}
	if topic == "the formation and movement of mountain glaciers" {
		prompt += `For this glacier topic, do not equate snow depth or exposed stake length directly with mass balance. If mass balance is discussed, state that accumulation and ablation are converted to water-equivalent mass and combined with an appropriate area/elevation weighting, or describe the result only as depth or surface-loss observations. Keep horizontal surface displacement separate from melt exposure and basal movement. `
	}
	if topic == "students planning an eyewitness-memory experiment" {
		prompt += `For this eyewitness-memory experiment, keep every room booking and session date feasible: if a room is available only on one day, do not schedule an equipment check there on another day unless access is explicitly arranged. Distinguish preliminary volunteers from the main sample, and do not claim that one unmentioned-detail question measures general attention; describe it as recall of that specific detail. Keep recognition, confidence, lineup identification and causal claims methodologically separate. `
	}
	if spec.key == "LISTENING_3" || spec.key == "LISTENING_4" {
		prompt += `For academic explanations, distinguish what a method measures directly from the event or cause inferred from that measurement. State the necessary assumptions; a sample's age does not automatically date a building's construction or abandonment. Present disputed historical reconstructions as interpretations, and never invent motives or technical capabilities to connect the story. Clearly label illustrative examples. Use complete comparisons and explicit referents, and apply any correction consistently throughout the script, including its conclusion. `
		prompt += listeningAcademicConsistencyGuidance
		prompt += listeningAcademicDiscourseArchitecture(spec.key, band)
	}
	if spec.key == "LISTENING_1" || spec.key == "LISTENING_3" {
		return prompt + `Use approximately 21-23 turns, usually 27-33 spoken words per turn, varied naturally. Count ALL speakers together, not separately. Every speaker needs at least three turns. Give the conversation a natural close within the budget; do not add a second recap. The required number of speakers is specified by Form.`
	}
	return prompt + `Use exactly one named speaker delivering eight coherent paragraphs, approximately 84-90 spoken words each. Repeat the same speaker label for each paragraph. Natural transitions, not eight disconnected mini-lectures. Include the closing sentence within the final paragraph budget.`
}

// 学术 Part 3/4 的高阶考点必须来自真实的话语发展，而不是模型在一句话里
// 同时塞入“原因+结论”后再把它拆成两道题。这里把可落地的结构约束放在
// 原文阶段，避免题目蓝图只能反复拒绝同一份浅层原文。
func listeningAcademicDiscourseArchitecture(sectionKey string, band float64) string {
	chains := listeningAuthoringAdvanced(band)
	if chains < 3 {
		chains = 3
	}
	if sectionKey == "LISTENING_3" {
		return fmt.Sprintf(`
ACADEMIC DISCUSSION ARCHITECTURE (mandatory): build at least %d separate decision or interpretation chains across the conversation. Each chain must unfold over at least two different turns: one speaker introduces a plausible proposal, explanation or initial assumption; another turn later adds a concrete observation, correction, limitation or practical constraint; only then can a listener identify the qualified outcome. Keep at least two plausible alternatives grounded in the conversation for each challenging chain: one should fit the first point but fail the later qualification, and another should fit the later point but fail the first. Do not resolve a chain in the same sentence that introduces it, and do not repeat the complete two-factor answer in a final recap. Use the students' research decisions, evidence interpretations and method limitations as natural content, not as numbered logic puzzles. Distribute the chains through the opening, middle and closing rather than clustering them in one exchange.`, chains)
	}
	return fmt.Sprintf(`
ACADEMIC LECTURE ARCHITECTURE (mandatory): build at least %d separate interpretation or method-evaluation chains across the lecture's paragraphs. In each chain, introduce an initially plausible explanation, historical reconstruction or method choice; in a later sentence or paragraph add a distinct observation, correction, scope condition or competing explanation; make the qualified conclusion recoverable only by combining those separated pieces. Preserve at least two source-grounded alternatives for each challenging chain, so a listener must reject one alternative for the first fact and another for the later limitation. Never put both decisive facts and the complete answer in one sentence. Do not use a final recap that simply repeats the answer and its reasons; the ending may state a cautious implication while leaving the earlier evidence necessary. Spread the chains across the lecture instead of relying on one central example, and keep the progression natural for a single lecturer rather than presenting a list of test prompts.`, chains)
}

// 高阶难度来自推理和限定条件，不应来自事实矛盾或未交代的对象。
const listeningAcademicConsistencyGuidance = `Before writing, privately track a compact fact ledger and verify it again against the COMPLETE final script. Do not print the ledger in the recording:
1. Chronology and resources: each treatment must precede its outcome measurement, and the required equipment must be available at that time. Clearly distinguish preliminary checks from the main test; carry any changed booking through the closing plan. Do not leave listeners to invent a missing booking.
2. Entities and quantities: introduce every specimen group, instrument and observation site before referring to it. Totals, subgroups, reserves, destructive tests and later references such as "both lower instruments" must agree. Distinguish extra specimens from reused specimens; do not count an already destroyed specimen again.
3. Comparisons: specify what quantity is compared, against what baseline, and whether the contrast is spatial or temporal. Do not silently switch from surface to bed geometry or from a location to a season. Define any operational distinction needed to understand the conclusion without adding a laboratory manual.
4. Inference: distinguish measurements, plausible interpretations and established mechanisms. State relevant limits naturally; correlations alone do not prove causes. A labelled hypothetical example is preferable to fabricated field observations presented as verified research. Keep uncertainty in the conclusion as well as the opening.
Keep these relationships understandable in natural spoken English, retain the existing comprehension depth and word budget, and avoid a final recap that gives away every answer. `

func validateMockExamSource(source mockExamSource, spec mockExamSpec) error {
	if mockExamWordCount(source.Title) < 2 {
		return errors.New("source needs a meaningful title")
	}
	for _, text := range []string{source.Title, source.Passage, source.AudioScript} {
		if mockExamPlaceholders.MatchString(text) || mockExamHasOfficialClaim(text) {
			return errors.New("source has placeholder content or official-equivalence claim")
		}
	}
	return validateMockExamSourceText(domain.MockExamSection{Passage: source.Passage, AudioScript: source.AudioScript}, spec)
}

func validateMockExamSourceText(section domain.MockExamSection, spec mockExamSpec) error {
	source := section.Passage
	if spec.skill == "LISTENING" {
		if strings.TrimSpace(section.Passage) != "" {
			return errors.New("listening source must be audioScript only")
		}
		source = section.AudioScript
		if err := validateMockExamSpeakers(source, spec.key); err != nil {
			return err
		}
	} else if strings.TrimSpace(section.AudioScript) != "" {
		return errors.New("reading source must be passage only")
	}
	count := mockExamWordCount(source)
	if count < spec.minWords || count > spec.maxWords {
		return fmt.Errorf("source has %d words; require %d-%d", count, spec.minWords, spec.maxWords)
	}
	return validateMockExamProse(source)
}
