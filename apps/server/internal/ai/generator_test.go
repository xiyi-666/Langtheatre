package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linguaquest/server/internal/domain"
)

func TestReadingMinWordsProgressesByBand(t *testing.T) {
	low := readingMinWords("[IELTS Reading][Band 5.0] urban transport")
	high := readingMinWords("[IELTS Reading][Band 7.0] urban transport")
	if high <= low {
		t.Fatalf("high band min words = %d, want greater than low band %d", high, low)
	}
}

func TestReadingLengthLimitsCapLowerBands(t *testing.T) {
	low := readingLengthLimitsForTopic("[IELTS Reading][Band 5.0] urban transport")
	mid := readingLengthLimitsForTopic("[IELTS Reading][Band 6.0] urban transport")
	high := readingLengthLimitsForTopic("[IELTS Reading][Band 7.0] urban transport")
	if low.MaxWords >= mid.MaxWords {
		t.Fatalf("Band 5 max words = %d, want lower than Band 6 max %d", low.MaxWords, mid.MaxWords)
	}
	if mid.MaxWords >= high.MaxWords {
		t.Fatalf("Band 6 max words = %d, want lower than Band 7 max %d", mid.MaxWords, high.MaxWords)
	}
	if low.MaxWords > 800 {
		t.Fatalf("Band 5 max words = %d, want capped for lower-band training", low.MaxWords)
	}
	if high.MinSegments <= low.MinSegments {
		t.Fatalf("Band 7 min segments = %d, want more than Band 5 min %d", high.MinSegments, low.MinSegments)
	}
}

func TestApplyReadingQuestionDefaultsForTFNG(t *testing.T) {
	quiz := applyReadingQuestionDefaults("TFNG", []domain.QuizQuestion{{Question: "The pilot scheme produced identical results everywhere.", AnswerKey: "FALSE"}})
	if quiz[0].Type != "TFNG" {
		t.Fatalf("Type = %q, want TFNG", quiz[0].Type)
	}
	if len(quiz[0].Options) != 3 {
		t.Fatalf("Options len = %d, want 3", len(quiz[0].Options))
	}
	if err := validateReadingQuestionShape("TFNG", quiz); err != nil {
		t.Fatalf("validateReadingQuestionShape() error = %v", err)
	}
}

func TestValidateReadingQuestionShapeRejectsMismatchedType(t *testing.T) {
	quiz := []domain.QuizQuestion{{
		Type:      "Multiple Choice",
		Question:  "Which claim is supported?",
		Options:   []string{"A", "B", "C", "D"},
		AnswerKey: "A",
	}}
	if err := validateReadingQuestionShape("Matching Headings", quiz); err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestValidateReadingQuestionShapeRejectsBroadMatchingInformation(t *testing.T) {
	quiz := []domain.QuizQuestion{{
		Type:         "Matching Information",
		Question:     "Match each statement to the paragraph where it appears.",
		Options:      []string{"Paragraph 1", "Paragraph 2", "Paragraph 3"},
		AnswerKey:    "Paragraph 2",
		ParagraphRef: "Paragraphs 1-3",
		Evidence:     "broad explanation",
	}}
	if err := validateReadingQuestionShape("Mixed Question Set", quiz); err == nil {
		t.Fatal("expected broad matching information paragraphRef to be rejected")
	}
}

func TestNormalizeQuizAcceptsSummaryCompletionWithWordBankOnly(t *testing.T) {
	quiz := normalizeQuiz([]genQuiz{{
		Type:        "Summary Completion",
		Question:    "Complete the summary using the word bank.",
		SummaryText: "Future planning depends on _____ management.",
		WordBank:    []string{"adaptive", "routine", "temporary"},
		AnswerKey:   "adaptive",
	}})
	if len(quiz) != 1 {
		t.Fatalf("quiz len = %d, want 1", len(quiz))
	}
	if len(quiz[0].Options) != 3 {
		t.Fatalf("Options len = %d, want copied word bank", len(quiz[0].Options))
	}
	if err := validateReadingQuestionShape("Summary Completion", quiz); err != nil {
		t.Fatalf("validateReadingQuestionShape() error = %v", err)
	}
}

func TestApplyListeningQuestionDefaultsAddsSectionType(t *testing.T) {
	quiz := applyListeningQuestionDefaults(3, []domain.QuizQuestion{{
		Question:  "Which concern does the student raise?",
		Options:   []string{"Sample size", "Room booking", "Ticket price", "Start date"},
		AnswerKey: "Sample size",
	}})
	if quiz[0].Type != "Opinion Matching" {
		t.Fatalf("Type = %q, want Opinion Matching", quiz[0].Type)
	}
}

func TestApplyListeningQuestionDefaultsPreservesModelType(t *testing.T) {
	quiz := applyListeningQuestionDefaults(1, []domain.QuizQuestion{{
		Question:  "What is the corrected booking reference?",
		Options:   []string{"A", "B", "C", "D"},
		AnswerKey: "B",
		Type:      "Spelling",
	}})
	if quiz[0].Type != "Spelling" {
		t.Fatalf("Type = %q, want model-provided type", quiz[0].Type)
	}
}

func TestParseModelOutputKeepsMixedQuizWhenSummaryHasNoOptions(t *testing.T) {
	content := `{
		"dialogues":[{"speaker":"Passage","text":"Paragraph text.","zhSubtitle":"段落说明"}],
		"quiz":[
			{"type":"Multiple Choice","question":"Which claim is supported?","options":["A","B","C","D"],"answerKey":"A","paragraphRef":"Paragraph 1","evidence":"Paragraph text"},
			{"type":"Matching Information","question":"Which paragraph gives the example?","options":["Paragraph 1","Paragraph 2","Paragraph 3"],"answerKey":"Paragraph 1","paragraphRef":"Paragraph 1","evidence":"Paragraph text"},
			{"type":"TFNG","question":"The passage includes paragraph text.","options":["TRUE","FALSE","NOT GIVEN"],"answerKey":"TRUE","evidence":"Paragraph text"},
			{"type":"Summary Completion","question":"Complete the summary.","summaryText":"The paragraph contains _____.","wordBank":["text","data","policy"],"answerKey":"text","evidence":"Paragraph text"},
			{"type":"Multiple Choice","question":"What is mentioned directly?","options":["Text","A date","A price","A name"],"answerKey":"Text","paragraphRef":"Paragraph 1","evidence":"Paragraph text"}
		]
	}`
	_, quiz := parseModelOutput(content)
	if len(quiz) != 5 {
		t.Fatalf("quiz len = %d, want 5", len(quiz))
	}
	if quiz[3].Type != "Summary Completion" {
		t.Fatalf("quiz[3].Type = %q, want Summary Completion", quiz[3].Type)
	}
	if len(quiz[3].Options) != len(quiz[3].WordBank) {
		t.Fatalf("summary options len = %d, wordBank len = %d", len(quiz[3].Options), len(quiz[3].WordBank))
	}
}

func TestExtractModelTextFromStreamResponse(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"dialogues\\\":\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"ignore me\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"[]\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" }\"}}]}\n\n" +
		"data: [DONE]\n")

	got, err := extractModelTextFromStreamResponse(body)
	if err != nil {
		t.Fatalf("extractModelTextFromStreamResponse() error = %v", err)
	}
	if got != "{\"dialogues\":[] }" {
		t.Fatalf("stream text = %q", got)
	}
}

func TestExtractModelTextFromStreamResponsePreservesSpaces(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Good\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\" afternoon\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\", \"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Sara.\"}}]}\n\n" +
		"data: [DONE]\n")

	got, err := extractModelTextFromStreamResponse(body)
	if err != nil {
		t.Fatalf("extractModelTextFromStreamResponse() error = %v", err)
	}
	if got != "Good afternoon, Sara." {
		t.Fatalf("stream text = %q", got)
	}
}

func TestModelPayloadWithStreamingDoesNotMutateInput(t *testing.T) {
	payload := map[string]any{"model": "test-model"}
	got := modelPayloadWithStreaming(payload)
	if got["stream"] != true {
		t.Fatalf("stream = %v, want true", got["stream"])
	}
	if _, ok := payload["stream"]; ok {
		t.Fatal("modelPayloadWithStreaming mutated input")
	}
}

func TestListeningRegenerationInstructionRequiresCompleteCompactJSON(t *testing.T) {
	got := listeningRegenerationInstruction(3)
	for _, want := range []string{
		`BOTH "dialogues" and "quiz"`,
		"exactly 8 dialogue turns",
		"exactly 3 quiz items",
		"18-34 English words",
		"Do not quote or mention IELTS",
		"no mini-theater narration",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("listeningRegenerationInstruction() = %q, want %q", got, want)
		}
	}
}

func TestValidateCantoneseDialoguesRejectsMandarinWrittenInTraditionalChinese(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Text: "我們現在去餐廳，可以嗎？"},
		{Text: "好的，這裡是菜單，請問您想吃什麼？"},
		{Text: "我沒有問題，你們先點餐。"},
		{Text: "知道了，我們現在開始。"},
	}
	err := validateCantoneseDialogues(dialogues)
	if !errors.Is(err, errGeneratedCantoneseQuality) {
		t.Fatalf("validateCantoneseDialogues() error = %v, want Cantonese quality error", err)
	}
}

func TestValidateCantoneseDialoguesAcceptsColloquialHongKongCantonese(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Text: "唔該，我想問下而家仲有冇位呀？"},
		{Text: "有呀，你哋兩位可以坐嗰邊張枱。"},
		{Text: "好呀，咁我哋可唔可以先睇下餐牌？"},
		{Text: "梗係得啦，陣間想落單再叫我。"},
	}
	if err := validateCantoneseDialogues(dialogues); err != nil {
		t.Fatalf("validateCantoneseDialogues() error = %v", err)
	}
}

func TestRewriteDialoguesToCantonese(t *testing.T) {
	rewrittenPayload, err := json.Marshal(map[string]any{
		"dialogues": []map[string]string{
			{"speaker": "顧客", "text": "唔該，我想問下而家仲有冇位呀？", "zhSubtitle": "请问现在还有位置吗？"},
			{"speaker": "店員", "text": "有呀，你哋可以坐嗰邊張枱。", "zhSubtitle": "有，你们可以坐那边。"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": string(rewrittenPayload)}}},
		})
	}))
	defer server.Close()

	generator := NewOpenAIGenerator("test-key", "test-model", server.URL)
	generator.Client = server.Client()
	got, err := generator.rewriteDialoguesToCantonese(context.Background(), []domain.Dialogue{
		{Speaker: "顧客", Text: "请问现在还有位置吗？", ZhSubtitle: "请问现在还有位置吗？"},
		{Speaker: "店員", Text: "有，你们可以坐那边。", ZhSubtitle: "有，你们可以坐那边。"},
	})
	if err != nil {
		t.Fatalf("rewriteDialoguesToCantonese() error = %v", err)
	}
	if len(got) != 2 || !strings.Contains(got[0].Text, "唔該") || !strings.Contains(got[1].Text, "你哋") {
		t.Fatalf("unexpected rewritten dialogues: %+v", got)
	}
}

func TestCantoneseRegenerationInstructionRejectsMandarinGrammar(t *testing.T) {
	got := cantoneseRegenerationInstruction()
	for _, want := range []string{"authentic colloquial Hong Kong Cantonese", "我哋", "zhSubtitle", "Mandarin wording"} {
		if !strings.Contains(got, want) {
			t.Fatalf("cantoneseRegenerationInstruction() = %q, want %q", got, want)
		}
	}
}
