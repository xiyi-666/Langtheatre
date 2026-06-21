package contentquality

import "testing"

func TestNormalizeEnglishSpacingRepairsCollapsedExample(t *testing.T) {
	input := "Goodafternoon,BrookdaleLanguageCentre."
	got := NormalizeEnglishSpacing(input)
	want := "Good afternoon, Brookdale Language Centre."
	if got != want {
		t.Fatalf("NormalizeEnglishSpacing() = %q, want %q", got, want)
	}
}

func TestPromptLeakDetection(t *testing.T) {
	if !ContainsPromptLeak("[IELTS Academic Stage 01]\nTask design\nCreate an IELTS Academic reading drill") {
		t.Fatal("expected prompt leak to be detected")
	}
	if !ContainsPromptLeak("Welcome to today's mini-theater. Our topic is [IELTS Listening][Stage 09].") {
		t.Fatal("expected listening prompt leak to be detected")
	}
	if ContainsPromptLeak("Researchers compare several urban planning policies across regions.") {
		t.Fatal("did not expect clean passage to be flagged")
	}
}

func TestWordAndParagraphCount(t *testing.T) {
	text := "One short paragraph has five words.\n\nAnother paragraph has four words."
	if got := ParagraphCount(text); got != 2 {
		t.Fatalf("ParagraphCount() = %d, want 2", got)
	}
	if got := WordCount(text); got != 11 {
		t.Fatalf("WordCount() = %d, want 11", got)
	}
}

func TestGenericReadingQuestionDetection(t *testing.T) {
	if !IsGenericReadingQuestion("What is the main focus of the passage?") {
		t.Fatal("expected generic question to be detected")
	}
	if IsGenericReadingQuestion("Which regional factor explains the different outcomes in paragraph 4?") {
		t.Fatal("did not expect evidence-based question to be generic")
	}
}
