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
