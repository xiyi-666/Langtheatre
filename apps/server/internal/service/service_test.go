package service

import (
	"context"
	"errors"
	"fmt"
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

func TestFallbackReadingPassageUsesTopicSpecificFrame(t *testing.T) {
	topic := "[IELTS Reading][Stage 11][Band 6.5][Section 2][Matching Headings] why certain migratory birds alter their routes after changes in agricultural lighting"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "upper-intermediate")
	dialogues, _ := fallbackReadingContentWithMetadata(topic, meta, 5)
	passage := joinDialogueText(dialogues)
	if strings.Contains(passage, "Public discussion of why certain migratory birds") {
		t.Fatalf("fallback passage still uses generic public discussion frame: %q", passage[:min(180, len(passage))])
	}
	for _, want := range []string{"Ecologists", "agricultural landscapes", "artificial lighting"} {
		if !strings.Contains(passage, want) {
			t.Fatalf("fallback passage missing topic-specific marker %q in %q", want, passage[:min(240, len(passage))])
		}
	}
}

func TestFallbackAdvancedMixedReadingAvoidsLongTopicRepetition(t *testing.T) {
	topic := "[IELTS Reading][Stage 18][Band 7.5][Section 3][Mixed Question Set][Focus: inference and paragraph evidence] the limits of algorithmic decision-making in climate adaptation, public health triage, and urban infrastructure planning"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata(topic, meta, 5)
	passage := joinDialogueText(dialogues)
	cleanTopic := ielts.CleanTopic(topic)

	if strings.Contains(passage, "Public discussion of") {
		t.Fatalf("advanced fallback still uses generic public discussion frame: %q", passage[:min(180, len(passage))])
	}
	if strings.Count(strings.ToLower(passage), strings.ToLower(cleanTopic)) > 0 {
		t.Fatalf("advanced fallback repeats full raw topic instead of concise subject: %q", cleanTopic)
	}
	lowerPassage := strings.ToLower(passage)
	for _, want := range []string{"algorithmic decision systems", "policy analysts", "datasets, scoring rules and appeal procedures"} {
		if !strings.Contains(lowerPassage, want) {
			t.Fatalf("advanced fallback missing high-level topic marker %q in %q", want, passage[:min(260, len(passage))])
		}
	}
	assertReadingQuestionShapes(t, meta.QuestionType, quiz)
}

func TestReadingMaterialTitleRemovesMetadataTags(t *testing.T) {
	topic := "[IELTS Reading][Stage 18][Band 7.5][Section 3][Mixed Question Set][Focus: inference and paragraph evidence] the limits of algorithmic decision-making in climate adaptation"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "advanced")
	title := readingMaterialTitle("IELTS", topic, meta)

	for _, blocked := range []string{"[IELTS", "[Stage", "[Band", "[Focus", "Mixed Question Set"} {
		if strings.Contains(title, blocked) {
			t.Fatalf("title leaked metadata marker %q: %q", blocked, title)
		}
	}
	if !strings.Contains(title, "The limits of algorithmic decision-making") {
		t.Fatalf("title = %q, want cleaned human-readable topic", title)
	}
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

func TestGenerateReadingAudioClearsProgressNoteAfterReady(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:          "reading-2",
		UserID:      "user-1",
		Exam:        "IELTS",
		Language:    "ENGLISH",
		Level:       "advanced",
		Topic:       "public health",
		Title:       "Public health",
		Passage:     longAudioPassage(),
		AudioStatus: "PENDING",
		CreatedAt:   time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2"}}
	svc := New(mem, nil, nil, tts, "secret")
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)

	ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.AudioStatus != "READY" {
		t.Fatalf("AudioStatus = %q, want READY", ready.AudioStatus)
	}
	if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
		t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
	}
	if strings.Contains(ready.GenerationNote, "audio chunk") {
		t.Fatalf("GenerationNote = %q, want progress note cleared after ready", ready.GenerationNote)
	}
}

func TestReadingMaterialRetriesFallbackAudioAndDeduplicatesJobs(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:             "reading-fallback",
		UserID:         "user-1",
		Exam:           "IELTS",
		Language:       "ENGLISH",
		Level:          "advanced",
		Topic:          "[IELTS Reading][Band 6.5][Matching Headings] public health systems",
		Title:          "Public health systems",
		Passage:        longAudioPassage(),
		AudioStatus:    "PENDING",
		GenerationNote: "Generated via structured fallback after AI generation was unavailable or failed quality validation.",
		CreatedAt:      time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2"}}
	svc := New(mem, nil, nil, tts, "secret")

	first, err := svc.ReadingMaterial(material.UserID, material.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ReadingMaterial(material.UserID, material.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != material.ID || second.ID != material.ID {
		t.Fatalf("unexpected material ids: %q %q", first.ID, second.ID)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
		if err != nil {
			t.Fatal(err)
		}
		if ready.AudioStatus == "READY" {
			if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
				t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
			}
			if tts.calls != 2 {
				t.Fatalf("TTS calls = %d, want 2", tts.calls)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("fallback reading audio did not reach READY in time")
}

func TestReadingMaterialsQueuesLimitedFallbackAudioRetriesWithCooldown(t *testing.T) {
	mem := store.NewMemoryStore()
	now := time.Now()
	for i := 0; i < 3; i++ {
		material := domain.ReadingMaterial{
			ID:             fmt.Sprintf("reading-list-%d", i+1),
			UserID:         "user-1",
			Exam:           "IELTS",
			Language:       "ENGLISH",
			Level:          "advanced",
			Topic:          fmt.Sprintf("[IELTS Reading][Band 6.0][Matching Information] sample %d", i+1),
			Title:          "Sample",
			Passage:        "Short fallback passage for retry.",
			AudioStatus:    "PENDING",
			GenerationNote: "Generated via structured fallback after AI generation was unavailable or failed quality validation.",
			CreatedAt:      now.Add(time.Duration(i) * time.Second),
		}
		if _, err := mem.SaveReadingMaterial(material); err != nil {
			t.Fatal(err)
		}
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2", "audio-3"}}
	svc := New(mem, nil, nil, tts, "secret")

	items, err := svc.ReadingMaterials("user-1", "IELTS")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("ReadingMaterials len = %d, want 3", len(items))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tts.calls >= maxReadingAudioListRetries {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tts.calls != maxReadingAudioListRetries {
		t.Fatalf("initial TTS calls = %d, want %d", tts.calls, maxReadingAudioListRetries)
	}

	if _, err := svc.ReadingMaterials("user-1", "IELTS"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if tts.calls != maxReadingAudioListRetries {
		t.Fatalf("TTS calls after second list = %d, want still %d", tts.calls, maxReadingAudioListRetries)
	}
}

func longAudioPassage() string {
	return strings.Repeat("governance ", 35) + ". " + strings.Repeat("evidence ", 35) + "."
}
