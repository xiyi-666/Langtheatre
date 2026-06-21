package ielts

import (
	"strings"
	"testing"
)

func TestSection4ListeningProfileRequiresMonologue(t *testing.T) {
	profile := ListeningProfileFromTopic("[IELTS Listening] [Band 7.0] [Section 4] urban heat islands", 7.0)
	block := profile.PromptBlock()
	if !strings.Contains(block, "single-speaker academic monologue") {
		t.Fatalf("PromptBlock() = %q, want single-speaker monologue constraint", block)
	}
	if !strings.Contains(block, `speaker "Lecturer"`) {
		t.Fatalf("PromptBlock() = %q, want Lecturer speaker constraint", block)
	}
	if !strings.Contains(block, "no students") {
		t.Fatalf("PromptBlock() = %q, want no-student constraint", block)
	}
}

func TestBand7ListeningProfileAddsCompactDenseTurnGuidance(t *testing.T) {
	profile := ListeningProfileFromTopic("[IELTS Listening][Band 7.0][Section 1] conference registration", 7.0)
	block := profile.PromptBlock()
	if !strings.Contains(block, "18-34 words") {
		t.Fatalf("PromptBlock() = %q, want compact turn length guidance", block)
	}
	if !strings.Contains(block, "never mention IELTS, Band, Section, Focus, Task design") {
		t.Fatalf("PromptBlock() = %q, want metadata leak guard", block)
	}
	if !strings.Contains(block, "corrections, delayed answers, or competing details") {
		t.Fatalf("PromptBlock() = %q, want Band 7 density guidance", block)
	}
}

func TestReadingMetadataPrefersMixedQuestionSetTagOverFocusHints(t *testing.T) {
	meta := ReadingMetadataFromTopic(
		"IELTS",
		"[IELTS Reading][Stage 18][Band 7.5][Section 3][Mixed Question Set][Focus: inference, paragraph evidence, author stance, summary completion, and dense paraphrase] climate adaptation planning",
		"academic",
	)
	if meta.QuestionType != "Mixed Question Set" {
		t.Fatalf("QuestionType = %q, want Mixed Question Set", meta.QuestionType)
	}
}
