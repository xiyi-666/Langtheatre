package service

import (
	"github.com/linguaquest/server/internal/domain"
	"math"
	"testing"
	"time"
)

func TestCETObjectiveWeights(t *testing.T) {
	for _, skill := range []string{"LISTENING", "READING"} {
		n := 25
		if skill == "READING" {
			n = 30
		}
		section := domain.MockExamSection{Skill: skill, Questions: make([]domain.QuizQuestion, n), Answers: make([]string, n)}
		for i := range section.Questions {
			section.Questions[i].AnswerKey = "A"
		}
		for _, tc := range []struct {
			index  int
			points float64
		}{{0, 1}, {15, 2}, {24, 2}} {
			points := tc.points
			if skill == "READING" {
				points = 0.5
				if tc.index >= 10 {
					points = 1
				}
				if tc.index >= 20 {
					points = 2
				}
			}
			section.Answers[tc.index] = "A"
			score, err := cetObjectiveScore([]domain.MockExamSection{section}, skill)
			if err != nil || math.Abs(score-points/35*100) > 0.00001 {
				t.Fatalf("%s index %d: score=%v err=%v", skill, tc.index, score, err)
			}
			section.Answers[tc.index] = ""
		}
		for i := range section.Answers {
			section.Answers[i] = "A"
		}
		score, err := cetObjectiveScore([]domain.MockExamSection{section}, skill)
		if err != nil || score != 100 {
			t.Fatal(score, err)
		}
		if _, err = cetObjectiveScore(nil, skill); err == nil {
			t.Fatal("missing section scored")
		}
	}
}

func TestMockEstimateUsesImmutableGenerationDuration(t *testing.T) {
	now := time.Now().UTC()
	history := []domain.MockExam{{ID: "old", Exam: "CET4", Status: "COMPLETED", UpdatedAt: now, CreatedAt: now.Add(-48 * time.Hour), Sections: []domain.MockExamSection{{GenerationDurationSeconds: 300}}}}
	current := domain.MockExam{ID: "new", Exam: "CET4", Status: "GENERATING", CreatedAt: now.Add(-100 * time.Second)}
	seconds, n := estimateMockReadySeconds(history, current)
	if seconds < 198 || seconds > 200 || n != 1 {
		t.Fatal(seconds, n)
	}
	current.Exam = "CET6"
	if seconds, n = estimateMockReadySeconds(history, current); seconds != 0 || n != 0 {
		t.Fatal("cross-exam sample used")
	}
	current.Exam = "CET4"
	current.CreatedAt = now.Add(-10 * time.Minute)
	if seconds, n = estimateMockReadySeconds(history, current); seconds != -1 || n != 1 {
		t.Fatal("overdue ETA fabricated", seconds, n)
	}
}
