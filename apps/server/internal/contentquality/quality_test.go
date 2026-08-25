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

func TestNormalizeSpeakerLabelRemovesMarkdown(t *testing.T) {
	input := " **梁姐（女侍應）** `"
	want := "梁姐（女侍應）"
	if got := NormalizeSpeakerLabel(input); got != want {
		t.Fatalf("NormalizeSpeakerLabel() = %q, want %q", got, want)
	}
}

func TestNormalizeCantoneseSpeechTextReducesPausesAndEnglishTerms(t *testing.T) {
	input := "我哋用 MTR 去中環……到埗之後（check in）再用 QR code 登記；好唔好？？"
	want := "我哋用港鐵去中環，到埗之後再用二維碼登記，好唔好？"
	if got := NormalizeCantoneseSpeechText(input); got != want {
		t.Fatalf("NormalizeCantoneseSpeechText() = %q, want %q", got, want)
	}
}

func TestNormalizeCantoneseSpeechTextConnectsDrinkModifiers(t *testing.T) {
	input := "如果賣晒，就要火腿通粉，凍奶茶少甜。"
	want := "如果賣晒，就要火腿通粉，一杯少甜嘅凍奶茶。"
	if got := NormalizeCantoneseSpeechText(input); got != want {
		t.Fatalf("NormalizeCantoneseSpeechText() = %q, want %q", got, want)
	}
}

func TestNormalizeCantoneseSpeechTextDoesNotDuplicateCupMeasure(t *testing.T) {
	input := "我要一杯凍奶茶要少甜。"
	want := "我要一杯少甜嘅凍奶茶。"
	if got := NormalizeCantoneseSpeechText(input); got != want {
		t.Fatalf("NormalizeCantoneseSpeechText() = %q, want %q", got, want)
	}
}

func TestContainsLatinLetters(t *testing.T) {
	if !ContainsLatinLetters("用 app 登記") {
		t.Fatal("expected Latin text to be detected")
	}
	if ContainsLatinLetters("用應用程式登記") {
		t.Fatal("did not expect Chinese text to contain Latin letters")
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
