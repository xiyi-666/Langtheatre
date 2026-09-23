package contentquality

import "github.com/linguaquest/server/internal/domain"

const (
	TheaterRubricVersion            = "theater-production-v1"
	ReadingRubricVersion            = "reading-production-v1"
	WritingPromptRubricVersion      = "writing-prompt-production-v1"
	WritingEvaluationRubricVersion  = "writing-evaluation-production-v1"
	MockExamRubricVersion           = "mock-exam-production-v1"
	MockEvaluationRubricVersion     = "mock-evaluation-production-v1"
	SpeakingPromptRubricVersion     = "speaking-prompt-production-v1"
	SpeakingEvaluationRubricVersion = "speaking-evaluation-production-v1"
)

var (
	TheaterRequiredChecks            = []string{"structure", "language", "speaker_consistency", "audio"}
	ReadingRequiredChecks            = []string{"source_integrity", "answer_evidence", "difficulty", "language"}
	WritingPromptRequiredChecks      = []string{"task_validity", "difficulty", "timing"}
	WritingEvaluationRequiredChecks  = []string{"four_dimensions", "evidence", "score_consistency"}
	MockExamRequiredChecks           = []string{"completeness", "answer_integrity", "difficulty", "media"}
	MockEvaluationRequiredChecks     = []string{"input_integrity", "objective_scoring", "subjective_review", "score_consistency"}
	SpeakingPromptRequiredChecks     = []string{"source", "semantic_quality", "part_alignment", "audio"}
	SpeakingEvaluationRequiredChecks = []string{"completeness", "evidence", "band_consistency", "audio_basis"}
)

func TheaterContentHash(item domain.Theater) (string, error) {
	return HashCanonical(struct {
		Language         string
		Topic            string
		Difficulty       float64
		Mode             string
		SceneDescription string
		Characters       []domain.Character
		Dialogues        []domain.Dialogue
		Questions        []domain.QuizQuestion
	}{item.Language, item.Topic, item.Difficulty, item.Mode, item.SceneDescription, item.Characters, item.Dialogues, item.QuizQuestions})
}

func ReadingContentHash(item domain.ReadingMaterial) (string, error) {
	return HashCanonical(struct {
		Exam, Language, Level, Topic, Stage, Section, SkillFocus, QuestionType, ScenarioFamily string
		Band                                                                                   float64
		Title, Passage                                                                         string
		Vocabulary                                                                             []string
		Questions                                                                              []domain.QuizQuestion
		SourceIDs                                                                              []string
	}{item.Exam, item.Language, item.Level, item.Topic, item.Stage, item.Section, item.SkillFocus, item.QuestionType, item.ScenarioFamily, item.Band, item.Title, item.Passage, item.Vocabulary, item.Questions, item.SourceIDs})
}

func WritingPromptContentHash(exam string, timeLimitSeconds int, prompt domain.WritingPrompt) (string, error) {
	return HashCanonical(struct {
		Exam             string
		TimeLimitSeconds int
		Prompt           domain.WritingPrompt
	}{exam, timeLimitSeconds, prompt})
}

func WritingEvaluationContentHash(session domain.WritingSession) (string, error) {
	return HashCanonical(struct {
		Exam             string
		TimeLimitSeconds int
		Prompt           domain.WritingPrompt
		Essay            string
		Evaluation       *domain.WritingEvaluation
	}{session.Exam, session.TimeLimitSeconds, session.Prompt, session.Essay, session.Evaluation})
}

func MockExamContentHash(exam domain.MockExam) (string, error) {
	sections := make([]mockSectionRevision, 0, len(exam.Sections))
	for _, section := range exam.Sections {
		sections = append(sections, mockSectionRevisionValue(section))
	}
	return HashCanonical(struct {
		Exam                 string
		TotalDurationSeconds int
		Sections             []mockSectionRevision
	}{exam.Exam, exam.TotalDurationSeconds, sections})
}

type mockSectionRevision struct {
	PaperVersion, Key, Title, Skill, Instructions, Passage, AudioScript string
	TargetBand                                                          float64
	DurationSeconds                                                     int
	Questions                                                           []domain.QuizQuestion
	WritingPrompts                                                      []domain.WritingPrompt
}

func mockSectionRevisionValue(section domain.MockExamSection) mockSectionRevision {
	return mockSectionRevision{
		section.PaperVersion, section.Key, section.Title, section.Skill, section.Instructions,
		section.Passage, section.AudioScript, section.TargetBand, section.DurationSeconds,
		section.Questions, section.WritingPrompts,
	}
}

func MockExamSectionContentHash(section domain.MockExamSection) (string, error) {
	return HashCanonical(mockSectionRevisionValue(section))
}

func MockExamEvaluationContentHash(exam domain.MockExam) (string, error) {
	type responseRevision struct {
		Key       string
		Answers   []string
		Responses []string
	}
	responses := make([]responseRevision, 0, len(exam.Sections))
	for _, section := range exam.Sections {
		responses = append(responses, responseRevision{section.Key, section.Answers, section.Responses})
	}
	paperHash, err := MockExamContentHash(exam)
	if err != nil {
		return "", err
	}
	return HashCanonical(struct {
		PaperHash string
		Responses []responseRevision
		Result    *domain.MockExamResult
	}{paperHash, responses, exam.Result})
}

func SpeakingPromptContentHash(prompt domain.SpeakingPrompt) (string, error) {
	return HashCanonical(struct {
		QuestionID, Source, Question, CueCard string
		Part, PreparationSec, AnswerSec       int
	}{prompt.QuestionID, prompt.Source, prompt.Question, prompt.CueCard, prompt.Part, prompt.PreparationSec, prompt.AnswerSec})
}

func SpeakingEvaluationContentHash(session domain.SpeakingSession) (string, error) {
	type promptRevision struct {
		QuestionID, Source, Question, CueCard string
		Part, PreparationSec, AnswerSec       int
	}
	prompts := make([]promptRevision, 0, len(session.Prompts))
	for _, prompt := range session.Prompts {
		prompts = append(prompts, promptRevision{prompt.QuestionID, prompt.Source, prompt.Question, prompt.CueCard, prompt.Part, prompt.PreparationSec, prompt.AnswerSec})
	}
	return HashCanonical(struct {
		Prompts    []promptRevision
		Turns      []domain.SpeakingTurn
		Evaluation *domain.SpeakingEvaluation
	}{prompts, session.Turns, session.Evaluation})
}
