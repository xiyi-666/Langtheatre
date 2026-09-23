package ai

import (
	"math"
	"testing"
)

func TestSpeakingEvidenceAndScoreValidation(t *testing.T) {
	turns := speakingQualityTurns()
	result := speakingQualityEvaluation()
	if err := ValidateSpeakingTextEvaluation(result, turns); err != nil {
		t.Fatal(err)
	}
	result.Evidence[0] = "I fabricated this entire sentence"
	if ValidateSpeakingTextEvaluation(result, turns) == nil {
		t.Fatal("invented quote accepted")
	}
	result.Evidence[0] = "I enjoy learning languages"
	bad := math.NaN()
	result.LexicalResource = &bad
	if ValidateSpeakingTextEvaluation(result, turns) == nil {
		t.Fatal("NaN accepted")
	}
	bad = 6.7
	if ValidateSpeakingTextEvaluation(result, turns) == nil {
		t.Fatal("invalid score step accepted")
	}
}
