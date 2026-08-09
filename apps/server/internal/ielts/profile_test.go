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

func TestNormalizeReadingMetadataPrefersExplicitValues(t *testing.T) {
	meta := NormalizeReadingMetadata(
		"IELTS",
		"[Band 6.0][Stage 2][Matching Headings] urban resilience",
		"upper-intermediate",
		ReadingMetadata{
			Band:           7.26,
			Stage:          "Stage 9",
			Section:        "Section 3",
			SkillFocus:     "author stance",
			QuestionType:   "TFNG",
			ScenarioFamily: "urban policy",
		},
	)
	if meta.Band != 7.3 || meta.Stage != "Stage 9" || meta.Section != "Section 3" {
		t.Fatalf("normalized numeric/stage metadata = %+v", meta)
	}
	if meta.SkillFocus != "author stance" || meta.QuestionType != "TFNG" || meta.ScenarioFamily != "urban policy" {
		t.Fatalf("explicit metadata did not win: %+v", meta)
	}
}

func TestReadingMetadataParsesExplicitScenarioTag(t *testing.T) {
	meta := ReadingMetadataFromTopic("IELTS", "[Scenario: coastal planning] [TFNG] flood resilience", "advanced")
	if meta.ScenarioFamily != "coastal planning" {
		t.Fatalf("ScenarioFamily = %q, want coastal planning", meta.ScenarioFamily)
	}
}
