package ielts

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type ListeningProfile struct {
	Band                float64
	Section             int
	SkillFocus          string
	TaskDesign          string
	QuizCount           int
	Pace                string
	SentenceComplexity  string
	Vocabulary          string
	Paraphrase          string
	DistractorDensity   string
	QuestionInstruction string
	InteractionFormat   string
}

type ReadingMetadata struct {
	Band           float64
	Stage          string
	Section        string
	SkillFocus     string
	QuestionType   string
	ScenarioFamily string
}

type ReadingLengthLimits struct {
	MinWords        int
	MaxWords        int
	MinSegments     int
	MaxSegments     int
	SegmentGuidance string
	BandGuidance    string
}

var bandRE = regexp.MustCompile(`(?i)\bband\s*([0-9](?:\.[0-9])?)`)
var stageRE = regexp.MustCompile(`(?i)\bstage\s*[-_ ]*([0-9]{1,3})`)
var sectionRE = regexp.MustCompile(`(?i)\bsection\s*([1-4])`)
var focusRE = regexp.MustCompile(`(?i)\bfocus\s*[:=-]?\s*([^\]\|;]+)`)
var taskDesignRE = regexp.MustCompile(`(?i)\btask\s*design\s*[:=-]?\s*([^\]\|;]+)`)
var bracketRE = regexp.MustCompile(`\[[^\]]+\]`)

func ListeningProfileFromTopic(topic string, difficulty float64) ListeningProfile {
	band := extractBand(topic, difficulty)
	section := extractSection(topic, band)
	profile := ListeningProfile{
		Band:       band,
		Section:    section,
		SkillFocus: extractFocus(topic),
		TaskDesign: extractTaskDesign(topic),
		QuizCount:  listeningQuizCount(band),
	}
	switch {
	case band < 5.8:
		profile.Pace = "slow and explicit, with short information chunks"
		profile.SentenceComplexity = "mostly simple clauses with one detail per turn"
		profile.Vocabulary = "everyday words plus a few common IELTS service terms"
		profile.Paraphrase = "minimal paraphrase; repeat key information naturally"
		profile.DistractorDensity = "one mild distractor that is corrected immediately"
	case band < 6.8:
		profile.Pace = "natural but controlled, with occasional clarification"
		profile.SentenceComplexity = "compound sentences and short dependent clauses"
		profile.Vocabulary = "topic-specific vocabulary with familiar paraphrases"
		profile.Paraphrase = "moderate paraphrase between dialogue and questions"
		profile.DistractorDensity = "two plausible distractors, including one self-correction"
	default:
		profile.Pace = "exam-realistic and information-dense"
		profile.SentenceComplexity = "longer turns with embedded reasons and contrasts"
		profile.Vocabulary = "academic or semi-formal topic vocabulary where appropriate"
		profile.Paraphrase = "strong paraphrase; avoid copying answer wording into questions"
		profile.DistractorDensity = "dense distractors with corrections, competing details, and delayed answers"
	}
	profile.QuestionInstruction = listeningSectionInstruction(section)
	profile.InteractionFormat = listeningInteractionFormat(section)
	if profile.SkillFocus == "" {
		profile.SkillFocus = defaultListeningFocus(section)
	}
	if profile.TaskDesign == "" {
		profile.TaskDesign = defaultListeningTaskDesign(section)
	}
	return profile
}

func (p ListeningProfile) PromptBlock() string {
	return fmt.Sprintf(`IELTS Listening controls:
- Band %.1f: pace should be %s; sentence style should use %s; vocabulary should use %s.
- Paraphrase: %s.
- Distractors: %s.
- Section %d task: %s.
- Interaction format: %s.
- Focus: %s.
- Task design: %s.
- Questions must follow the Section task type, not generic main-idea questions.`,
		p.Band,
		p.Pace,
		p.SentenceComplexity,
		p.Vocabulary,
		p.Paraphrase,
		p.DistractorDensity,
		p.Section,
		p.QuestionInstruction,
		p.InteractionFormat,
		p.SkillFocus,
		p.TaskDesign,
	)
}

func ReadingMetadataFromTopic(exam string, topic string, level string) ReadingMetadata {
	band := extractBand(topic, defaultReadingBand(exam, level))
	questionType := extractQuestionType(topic)
	if questionType == "" {
		questionType = defaultQuestionType(level)
	}
	stage := extractStage(topic)
	sectionText := ""
	if section := extractSection(topic, band); section > 0 {
		sectionText = fmt.Sprintf("Section %d", section)
	}
	focus := extractFocus(topic)
	if focus == "" {
		focus = defaultReadingFocus(questionType)
	}
	return ReadingMetadata{
		Band:           band,
		Stage:          stage,
		Section:        sectionText,
		SkillFocus:     focus,
		QuestionType:   questionType,
		ScenarioFamily: scenarioFamily(topic, questionType),
	}
}

func ReadingLengthLimitsFromMetadata(exam string, topic string, meta ReadingMetadata) ReadingLengthLimits {
	t := strings.ToUpper(topic)
	switch {
	case strings.EqualFold(strings.TrimSpace(exam), "IELTS") || strings.Contains(t, "[IELTS READING]"):
		switch {
		case meta.Band >= 7.0:
			return ReadingLengthLimits{
				MinWords:        820,
				MaxWords:        1200,
				MinSegments:     8,
				MaxSegments:     9,
				SegmentGuidance: "mostly 100-130 words per paragraph, with compact academic development",
				BandGuidance:    "use abstract relationships, concessions, dense paraphrase, and plausible but fair distractors",
			}
		case meta.Band >= 6.5:
			return ReadingLengthLimits{
				MinWords:        700,
				MaxWords:        1050,
				MinSegments:     7,
				MaxSegments:     8,
				SegmentGuidance: "mostly 90-120 words per paragraph, with clear paragraph functions",
				BandGuidance:    "use moderate abstraction, cause-effect links, paraphrase, and detail-based distractors",
			}
		case meta.Band >= 6.0:
			return ReadingLengthLimits{
				MinWords:        620,
				MaxWords:        880,
				MinSegments:     7,
				MaxSegments:     8,
				SegmentGuidance: "mostly 80-105 words per paragraph, with explicit transitions",
				BandGuidance:    "use familiar academic vocabulary, concrete evidence, and limited inference load",
			}
		default:
			return ReadingLengthLimits{
				MinWords:        520,
				MaxWords:        760,
				MinSegments:     6,
				MaxSegments:     7,
				SegmentGuidance: "mostly 75-100 words per paragraph, with one main idea per paragraph",
				BandGuidance:    "use clear topic sentences, concrete examples, lighter paraphrase, and avoid dense nominalisation",
			}
		}
	case strings.EqualFold(strings.TrimSpace(exam), "CET") || strings.Contains(t, "[CET READING]"):
		return ReadingLengthLimits{
			MinWords:        480,
			MaxWords:        720,
			MinSegments:     6,
			MaxSegments:     8,
			SegmentGuidance: "mostly 70-100 words per paragraph, with direct evidence for questions",
			BandGuidance:    "use college-level vocabulary with clear sentence logic and practical distractors",
		}
	default:
		return ReadingLengthLimits{
			MinWords:        520,
			MaxWords:        780,
			MinSegments:     6,
			MaxSegments:     8,
			SegmentGuidance: "mostly 75-105 words per paragraph, with direct paragraph evidence",
			BandGuidance:    "keep progression controlled and avoid generic exam-coaching prose",
		}
	}
}

func CleanTopic(topic string) string {
	clean := bracketRE.ReplaceAllString(topic, " ")
	clean = focusRE.ReplaceAllString(clean, " ")
	clean = taskDesignRE.ReplaceAllString(clean, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.Trim(clean, " -|;")
}

func QuestionTypeKey(questionType string) string {
	clean := strings.ToLower(strings.TrimSpace(questionType))
	clean = strings.ReplaceAll(clean, "-", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	switch clean {
	case "matching headings", "headings matching":
		return "matching_headings"
	case "matching information":
		return "matching_information"
	case "tfng", "true false not given", "true/false/not given":
		return "tfng"
	case "summary completion", "complex summary":
		return "summary_completion"
	case "mixed question set", "mixed":
		return "mixed"
	default:
		return "multiple_choice"
	}
}

func extractBand(topic string, fallback float64) float64 {
	if match := bandRE.FindStringSubmatch(topic); len(match) == 2 {
		if parsed, err := strconv.ParseFloat(match[1], 64); err == nil {
			return roundBand(parsed)
		}
	}
	return roundBand(fallback)
}

func extractSection(topic string, band float64) int {
	if match := sectionRE.FindStringSubmatch(topic); len(match) == 2 {
		if parsed, err := strconv.Atoi(match[1]); err == nil && parsed >= 1 && parsed <= 4 {
			return parsed
		}
	}
	switch {
	case band < 5.8:
		return 1
	case band < 6.3:
		return 2
	case band < 6.8:
		return 3
	default:
		return 4
	}
}

func extractStage(topic string) string {
	if match := stageRE.FindStringSubmatch(topic); len(match) == 2 {
		return "Stage " + match[1]
	}
	return ""
}

func extractFocus(topic string) string {
	if match := focusRE.FindStringSubmatch(topic); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func extractTaskDesign(topic string) string {
	if match := taskDesignRE.FindStringSubmatch(topic); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func extractQuestionType(topic string) string {
	lower := strings.ToLower(topic)
	switch {
	case strings.Contains(lower, "matching headings"):
		return "Matching Headings"
	case strings.Contains(lower, "matching information"):
		return "Matching Information"
	case strings.Contains(lower, "tfng") || strings.Contains(lower, "true false not given") || strings.Contains(lower, "true/false/not given"):
		return "TFNG"
	case strings.Contains(lower, "summary completion") || strings.Contains(lower, "complex summary"):
		return "Summary Completion"
	case strings.Contains(lower, "mixed question set"):
		return "Mixed Question Set"
	case strings.Contains(lower, "multiple choice"):
		return "Multiple Choice"
	default:
		return ""
	}
}

func listeningQuizCount(band float64) int {
	if band >= 7.0 {
		return 3
	}
	return 2
}

func listeningSectionInstruction(section int) string {
	switch section {
	case 1:
		return "form/table completion with names, numbers, dates, prices, spelling, and short factual details"
	case 2:
		return "public announcement or orientation with map/route/location or facility description details"
	case 3:
		return "two or three speakers comparing opinions, evidence, and decisions in an academic discussion"
	default:
		return "monologue or lecture with note/summary completion and conceptual signposting"
	}
}

func listeningInteractionFormat(section int) string {
	switch section {
	case 1:
		return "two-speaker transactional conversation, such as caller/receptionist or customer/staff"
	case 2:
		return "mostly one public speaker or guide addressing listeners; avoid academic debate"
	case 3:
		return "two or three speakers in an academic discussion, with clear speaker-specific opinions"
	default:
		return `single-speaker academic monologue split into 8 consecutive lecture chunks; every dialogue item must use speaker "Lecturer"; no students, interruptions, interviews, receptionist/caller exchanges, Q&A turns, recording/upload logistics, timers, or worksheets`
	}
}

func defaultListeningFocus(section int) string {
	switch section {
	case 1:
		return "specific factual details"
	case 2:
		return "location and procedural information"
	case 3:
		return "speaker opinions and agreement"
	default:
		return "lecture notes and abstract relationships"
	}
}

func defaultListeningTaskDesign(section int) string {
	switch section {
	case 1:
		return "capture exact details from a transactional conversation"
	case 2:
		return "follow an explanation of places, services, or instructions"
	case 3:
		return "match speakers to viewpoints and identify decision reasons"
	default:
		return "complete notes from a structured academic monologue"
	}
}

func defaultReadingBand(exam string, level string) float64 {
	if strings.EqualFold(strings.TrimSpace(exam), "CET") {
		return 5.5
	}
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "foundation", "lower-intermediate", "intermediate":
		return 5.5
	case "advanced":
		return 7.0
	default:
		return 6.5
	}
}

func defaultQuestionType(level string) string {
	if strings.EqualFold(strings.TrimSpace(level), "advanced") {
		return "Mixed Question Set"
	}
	return "Multiple Choice"
}

func defaultReadingFocus(questionType string) string {
	switch QuestionTypeKey(questionType) {
	case "matching_headings":
		return "paragraph main ideas and heading discrimination"
	case "matching_information":
		return "detail location and paraphrase"
	case "tfng":
		return "claim verification and not-given traps"
	case "summary_completion":
		return "summary logic and lexical cohesion"
	case "mixed":
		return "combined IELTS Academic reading skills"
	default:
		return "evidence-based comprehension"
	}
}

func scenarioFamily(topic string, questionType string) string {
	clean := CleanTopic(topic)
	if clean == "" {
		clean = questionType
	}
	words := strings.Fields(clean)
	if len(words) > 4 {
		words = words[:4]
	}
	return strings.Join(words, " ")
}

func roundBand(value float64) float64 {
	if value <= 0 {
		return 6.5
	}
	return math.Round(value*10) / 10
}
