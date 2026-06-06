package service

import (
	"testing"

	"github.com/linguaquest/server/internal/domain"
)

func TestAnswerMatches(t *testing.T) {
	cases := []struct {
		name     string
		user     string
		expected string
		language string
		want     bool
	}{
		{"exact_match", "hello", "hello", "ENGLISH", true},
		{"case_insensitive_en", "Hello", "hello", "ENGLISH", true},
		{"substring_en", "A delayed train", "delayed train", "ENGLISH", true},
		{"reverse_substring_en", "delayed", "A delayed train", "ENGLISH", true},
		{"no_match_en", "hotel", "delayed train", "ENGLISH", false},
		{"empty_user", "", "hello", "ENGLISH", false},
		{"empty_expected", "hello", "", "ENGLISH", false},
		{"both_empty", "", "", "ENGLISH", false},
		{"whitespace_normalization", "  hello   world  ", "hello world", "ENGLISH", true},
		{"chinese_exact", "地铁延误", "地铁延误", "CANTONESE", true},
		{"chinese_substring", "地铁延误了", "地铁延误", "CANTONESE", true},
		{"chinese_no_match", "酒店订错", "地铁延误", "CANTONESE", false},
		{"substring_contained_en", "A", "AB", "ENGLISH", true},
		{"short_expected_no_match", "xyz", "AB", "ENGLISH", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := answerMatches(tc.user, tc.expected, tc.language)
			if got != tc.want {
				t.Errorf("answerMatches(%q, %q, %q) = %v, want %v",
					tc.user, tc.expected, tc.language, got, tc.want)
			}
		})
	}
}

func TestFallbackSceneAndCharacters_English(t *testing.T) {
	scene, chars := fallbackSceneAndCharacters("ENGLISH", "shopping", nil)
	if scene == "" {
		t.Error("scene should not be empty")
	}
	if len(chars) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(chars))
	}
	if chars[0].Name != "Alex" || chars[1].Name != "Sam" {
		t.Errorf("default English names should be Alex and Sam, got %q and %q", chars[0].Name, chars[1].Name)
	}
}

func TestFallbackSceneAndCharacters_Cantonese(t *testing.T) {
	scene, chars := fallbackSceneAndCharacters("CANTONESE", "", nil)
	if scene == "" {
		t.Error("scene should not be empty")
	}
	if len(chars) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(chars))
	}
	if chars[0].Name != "阿明" || chars[1].Name != "小美" {
		t.Errorf("default Cantonese names wrong, got %q and %q", chars[0].Name, chars[1].Name)
	}
}

func TestFallbackSceneAndCharacters_WithDialogueSpeakers(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "Bob"},
		{Speaker: "Carol"},
	}
	scene, chars := fallbackSceneAndCharacters("ENGLISH", "travel", dialogues)
	if scene == "" {
		t.Error("scene should not be empty")
	}
	if len(chars) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(chars))
	}
	if chars[0].Name != "Bob" {
		t.Errorf("first character should be Bob, got %q", chars[0].Name)
	}
	if chars[1].Name != "Carol" {
		t.Errorf("second character should be Carol, got %q", chars[1].Name)
	}
}

func TestFallbackSceneAndCharacters_EmptyTopic(t *testing.T) {
	scene, _ := fallbackSceneAndCharacters("CANTONESE", "", nil)
	// Default topic "日常沟通" should appear in the scene
	if scene == "" {
		t.Error("scene should not be empty for empty topic")
	}
}

func TestFallbackTheaterContent_English(t *testing.T) {
	dialogues, questions := fallbackTheaterContent("ENGLISH", "travel")
	if len(dialogues) == 0 {
		t.Error("expected dialogues")
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 quiz questions, got %d", len(questions))
	}
	if dialogues[0].Speaker != "Alex" {
		t.Errorf("first English speaker should be Alex, got %q", dialogues[0].Speaker)
	}
}

func TestFallbackTheaterContent_Cantonese(t *testing.T) {
	dialogues, questions := fallbackTheaterContent("CANTONESE", "日常")
	if len(dialogues) == 0 {
		t.Error("expected dialogues")
	}
	if len(questions) != 2 {
		t.Errorf("expected 2 quiz questions, got %d", len(questions))
	}
	if dialogues[0].Speaker != "阿明" {
		t.Errorf("first Cantonese speaker should be 阿明, got %q", dialogues[0].Speaker)
	}
}

func TestFallbackQuizOnly(t *testing.T) {
	questions := fallbackQuizOnly("ENGLISH", "any")
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
	for _, q := range questions {
		if q.Question == "" {
			t.Error("question text should not be empty")
		}
		if q.AnswerKey == "" {
			t.Error("answer key should not be empty")
		}
	}
}
