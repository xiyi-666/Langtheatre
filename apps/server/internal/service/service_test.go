package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/ielts"
	"github.com/linguaquest/server/internal/store"
)

type sequenceTTS struct {
	urls  []string
	errs  []error
	calls int
}

func (s *sequenceTTS) Synthesize(_ context.Context, _ string, _ string, _ string) (string, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return "", s.errs[i]
	}
	if i < len(s.urls) {
		return s.urls[i], nil
	}
	return "", nil
}

func TestFallbackReadingContentMatchesQuestionType(t *testing.T) {
	meta := ielts.ReadingMetadataFromTopic("IELTS", "[Band 7.0][Matching Headings] urban transport", "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata("urban transport", meta, 5)
	limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", "[Band 7.0][Matching Headings] urban transport", meta)
	passage := joinDialogueText(dialogues)
	if got := contentquality.WordCount(passage); got < limits.MinWords {
		t.Fatalf("fallback word count = %d, want at least %d", got, limits.MinWords)
	}
	if len(dialogues) < limits.MinSegments {
		t.Fatalf("dialogues len = %d, want at least %d", len(dialogues), limits.MinSegments)
	}
	if len(quiz) != 5 {
		t.Fatalf("quiz len = %d, want 5", len(quiz))
	}
	for _, q := range quiz {
		if q.Type != "Matching Headings" {
			t.Fatalf("question type = %q, want Matching Headings", q.Type)
		}
		if len(q.Headings) < 4 || q.ParagraphRef == "" {
			t.Fatalf("question missing heading structure: %+v", q)
		}
		if contentquality.IsGenericReadingQuestion(q.Question) {
			t.Fatalf("fallback produced generic question: %q", q.Question)
		}
	}
}

func TestFallbackMixedReadingContentHasExpectedQuestionMix(t *testing.T) {
	meta := ielts.ReadingMetadataFromTopic("IELTS", "[Band 7.0][Mixed Question Set] public health data", "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata("[Band 7.0][Mixed Question Set] public health data", meta, 5)
	passage := joinDialogueText(dialogues)
	limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", "[Band 7.0][Mixed Question Set] public health data", meta)
	if err := validateReadingMaterialText(passage, quiz, limits.MinWords, limits.MinSegments); err != nil {
		t.Fatalf("mixed fallback failed material quality: %v", err)
	}
	wantTypes := []string{"Multiple Choice", "Matching Information", "TFNG", "Summary Completion", "Multiple Choice"}
	if len(quiz) != len(wantTypes) {
		t.Fatalf("quiz len = %d, want %d", len(quiz), len(wantTypes))
	}
	seenQuestions := map[string]bool{}
	for i, wantType := range wantTypes {
		if quiz[i].Type != wantType {
			t.Fatalf("quiz[%d].Type = %q, want %q", i, quiz[i].Type, wantType)
		}
		if seenQuestions[quiz[i].Question] {
			t.Fatalf("mixed fallback repeated question %q", quiz[i].Question)
		}
		seenQuestions[quiz[i].Question] = true
	}
	assertReadingQuestionShapes(t, meta.QuestionType, quiz)
	assertReadingEvidenceAnchored(t, passage, quiz)
}

func TestGenerateReadingAudioPreservesAndResumesChunks(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:          "reading-1",
		UserID:      "user-1",
		Exam:        "IELTS",
		Language:    "ENGLISH",
		Level:       "advanced",
		Topic:       "urban transport",
		Title:       "Urban transport",
		Passage:     longAudioPassage(),
		AudioStatus: "PENDING",
		CreatedAt:   time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	firstTTS := &sequenceTTS{urls: []string{"audio-1"}, errs: []error{nil, errors.New("temporary timeout")}}
	svc := New(mem, nil, nil, firstTTS, "secret")
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)
	partial, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if partial.AudioStatus != "PENDING" {
		t.Fatalf("AudioStatus after partial failure = %q, want PENDING", partial.AudioStatus)
	}
	if len(partial.AudioURLs) != 1 {
		t.Fatalf("AudioURLs len after partial failure = %d, want 1", len(partial.AudioURLs))
	}

	secondTTS := &sequenceTTS{urls: []string{"audio-2"}}
	svc.tts = secondTTS
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)
	ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.AudioStatus != "READY" {
		t.Fatalf("AudioStatus after resume = %q, want READY", ready.AudioStatus)
	}
	if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
		t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
	}
	if secondTTS.calls != 1 {
		t.Fatalf("resume TTS calls = %d, want 1", secondTTS.calls)
	}
}

func longAudioPassage() string {
	return strings.Repeat("governance ", 35) + ". " + strings.Repeat("evidence ", 35) + "."
}
