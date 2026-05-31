package service

import (
	"strings"
	"testing"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/ielts"
)

func TestSmallSampleQualityGate(t *testing.T) {
	listeningTopics := []string{
		"[Band 5.0][Section 1] course registration phone call",
		"[Band 6.0][Section 2] library orientation map",
		"[Band 6.5][Section 3] student research discussion",
		"[Band 7.0][Section 4] adaptive governance lecture",
		"[Band 7.0][Section 1] accommodation booking form",
	}
	for _, topic := range listeningTopics {
		profile := ielts.ListeningProfileFromTopic(topic, 6.5)
		dialogues, quiz := fallbackGeneratedContent("ENGLISH", topic, profile.QuizCount)
		if len(dialogues) < 8 {
			t.Fatalf("listening sample %q dialogues len = %d, want at least 8", topic, len(dialogues))
		}
		if len(quiz) != profile.QuizCount {
			t.Fatalf("listening sample %q quiz len = %d, want %d", topic, len(quiz), profile.QuizCount)
		}
		for _, d := range dialogues {
			if err := contentquality.ValidateReadableText("listening dialogue", d.Text, true); err != nil {
				t.Fatalf("listening sample %q failed text quality: %v", topic, err)
			}
		}
		for _, q := range quiz {
			if q.Type == "" {
				t.Fatalf("listening sample %q missing quiz type: %+v", topic, q)
			}
			if contentquality.IsGenericReadingQuestion(q.Question) {
				t.Fatalf("listening sample %q has generic question: %q", topic, q.Question)
			}
		}
	}

	readingSamples := []struct {
		topic string
		level string
	}{
		{"[Band 5.0][Multiple Choice] urban transport policy", "intermediate"},
		{"[Band 6.0][Matching Information] school meal design", "upper-intermediate"},
		{"[Band 6.5][TFNG] renewable energy storage", "upper-intermediate"},
		{"[Band 7.0][Summary Completion] adaptive governance", "advanced"},
		{"[Band 7.0][Mixed Question Set] public health data", "advanced"},
	}
	for _, sample := range readingSamples {
		meta := ielts.ReadingMetadataFromTopic("IELTS", sample.topic, sample.level)
		dialogues, quiz := fallbackReadingContentWithMetadata(sample.topic, meta, 5)
		passage := joinDialogueText(dialogues)
		limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", sample.topic, meta)
		if err := validateReadingMaterialText(passage, quiz, limits.MinWords, limits.MinSegments); err != nil {
			t.Fatalf("reading sample %q failed material quality: %v", sample.topic, err)
		}
		assertReadingQuestionShapes(t, meta.QuestionType, quiz)
		assertReadingEvidenceAnchored(t, passage, quiz)
	}
}

func joinDialogueText(dialogues []domain.Dialogue) string {
	parts := make([]string, 0, len(dialogues))
	for _, d := range dialogues {
		parts = append(parts, d.Text)
	}
	return strings.Join(parts, "\n")
}

func assertReadingQuestionShapes(t *testing.T, expectedType string, quiz []domain.QuizQuestion) {
	t.Helper()
	expectedKey := ielts.QuestionTypeKey(expectedType)
	for _, q := range quiz {
		key := ielts.QuestionTypeKey(q.Type)
		if expectedKey != "mixed" && key != expectedKey {
			t.Fatalf("question type = %q, want %q", q.Type, expectedType)
		}
		switch key {
		case "matching_headings":
			if q.ParagraphRef == "" || len(q.Headings) < 4 {
				t.Fatalf("bad matching headings shape: %+v", q)
			}
		case "matching_information":
			if len(q.Options) < 3 || q.AnswerKey == "" || q.ParagraphRef == "" {
				t.Fatalf("bad matching information shape: %+v", q)
			}
		case "tfng":
			if q.AnswerKey != "TRUE" && q.AnswerKey != "FALSE" && q.AnswerKey != "NOT GIVEN" {
				t.Fatalf("bad TFNG answer: %+v", q)
			}
		case "summary_completion":
			if q.SummaryText == "" || len(q.WordBank) < 3 || q.AnswerKey == "" {
				t.Fatalf("bad summary completion shape: %+v", q)
			}
		default:
			if len(q.Options) != 4 || q.AnswerKey == "" {
				t.Fatalf("bad multiple choice shape: %+v", q)
			}
		}
	}
}

func assertReadingEvidenceAnchored(t *testing.T, passage string, quiz []domain.QuizQuestion) {
	t.Helper()
	normalizedPassage := strings.ToLower(strings.Join(strings.Fields(passage), " "))
	for _, q := range quiz {
		evidence := strings.TrimSpace(q.Evidence)
		if evidence == "" || strings.EqualFold(evidence, "not stated") {
			continue
		}
		normalizedEvidence := strings.ToLower(strings.Join(strings.Fields(evidence), " "))
		if !strings.Contains(normalizedPassage, normalizedEvidence) {
			t.Fatalf("evidence %q for question %q was not found in passage", q.Evidence, q.Question)
		}
	}
}
