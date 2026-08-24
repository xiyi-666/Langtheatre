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
	"github.com/linguaquest/server/internal/ielts"
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
	dialogues := validCantoneseConversation()
	if err := validateCantoneseDialogues(dialogues); err != nil {
		t.Fatalf("validateCantoneseDialogues() error = %v", err)
	}
}

func TestValidateCantoneseDialoguesRejectsEnglishText(t *testing.T) {
	dialogues := validCantoneseConversation()
	dialogues[2].Text = "我哋可以用 MTR 去中環，陣間再傾啦。"
	err := validateCantoneseDialogues(dialogues)
	if !errors.Is(err, errGeneratedCantoneseQuality) {
		t.Fatalf("validateCantoneseDialogues() error = %v, want Cantonese quality error", err)
	}
}

func TestNormalizeGeneratedCantoneseDialoguesForSpeech(t *testing.T) {
	dialogues := []domain.Dialogue{{Speaker: "**梁姐（女侍應）**", Gender: "female", Text: "搭 MTR 去中環……到時再傾；好唔好？？"}}
	got := normalizeGeneratedDialogues("CANTONESE", dialogues)
	want := "搭港鐵去中環，到時再傾，好唔好？"
	if got[0].Text != want {
		t.Fatalf("normalizeGeneratedDialogues() = %q, want %q", got[0].Text, want)
	}
	if got[0].Speaker != "梁姐（女侍應）" || got[0].Gender != "FEMALE" {
		t.Fatalf("speaker identity was not normalized: %+v", got[0])
	}
}

func TestValidateCantoneseDialoguesRejectsMonologue(t *testing.T) {
	dialogues := validCantoneseConversation()
	for index := range dialogues {
		dialogues[index].Speaker = "Lecturer"
	}
	err := validateCantoneseDialogues(dialogues)
	if !errors.Is(err, errGeneratedCantoneseQuality) {
		t.Fatalf("validateCantoneseDialogues() error = %v, want Cantonese quality error", err)
	}
}

func TestValidateCantoneseDialoguesRejectsThirdSpeaker(t *testing.T) {
	dialogues := validCantoneseConversation()
	dialogues[4].Speaker = "陳經理（男經理）"
	dialogues[4].Gender = "MALE"
	err := validateCantoneseDialogues(dialogues)
	if !errors.Is(err, errGeneratedCantoneseQuality) || !strings.Contains(err.Error(), "speaker_count=3") {
		t.Fatalf("validateCantoneseDialogues() error = %v, want three-speaker quality error", err)
	}
}

func TestValidateCantoneseDialoguesRejectsMissingOrInconsistentGender(t *testing.T) {
	dialogues := validCantoneseConversation()
	dialogues[0].Gender = ""
	if err := validateCantoneseDialogues(dialogues); !errors.Is(err, errGeneratedCantoneseQuality) {
		t.Fatalf("missing gender error = %v, want Cantonese quality error", err)
	}

	dialogues = validCantoneseConversation()
	dialogues[2].Gender = "MALE"
	if err := validateCantoneseDialogues(dialogues); !errors.Is(err, errGeneratedCantoneseQuality) || !strings.Contains(err.Error(), "inconsistent gender") {
		t.Fatalf("inconsistent gender error = %v, want speaker gender mismatch", err)
	}
}

func TestCantoneseListeningControlsKeepHighDifficultyInteractive(t *testing.T) {
	profile := ielts.ListeningProfile{Section: 4}
	controls := listeningControlsForGeneration("CANTONESE", profile, 7.5)
	form := listeningFormInstruction("CANTONESE", profile)
	for _, want := range []string{"exactly two speakers", "competing viewpoints", "exactly two named speakers", "Every dialogue item must include gender", "Never use a Lecturer"} {
		if !strings.Contains(controls+form, want) {
			t.Fatalf("Cantonese controls = %q, want %q", controls+form, want)
		}
	}
}

func TestRewriteDialoguesToCantonese(t *testing.T) {
	rewrittenPayload, err := json.Marshal(map[string]any{
		"dialogues": validCantoneseConversation(),
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
	source := validCantoneseConversation()
	for index := range source {
		source[index].Text = "请问现在还有位置吗？"
	}
	got, err := generator.rewriteDialoguesToCantonese(context.Background(), source)
	if err != nil {
		t.Fatalf("rewriteDialoguesToCantonese() error = %v", err)
	}
	if len(got) != 8 || !strings.Contains(got[0].Text, "唔該") || !strings.Contains(got[1].Text, "你哋") {
		t.Fatalf("unexpected rewritten dialogues: %+v", got)
	}
}

func validCantoneseConversation() []domain.Dialogue {
	return []domain.Dialogue{
		{Speaker: "阿欣（女顧客）", Gender: "FEMALE", Text: "唔該，我想問下而家仲有冇位呀？", ZhSubtitle: "请问现在还有位置吗？"},
		{Speaker: "阿明（男店員）", Gender: "MALE", Text: "有呀，你哋兩位可以坐嗰邊張枱。", ZhSubtitle: "有，你们两位可以坐那边。"},
		{Speaker: "阿欣（女顧客）", Gender: "FEMALE", Text: "好呀，咁我哋可唔可以先睇下餐牌？", ZhSubtitle: "好，我们能先看看菜单吗？"},
		{Speaker: "阿明（男店員）", Gender: "MALE", Text: "梗係得啦，陣間想落單再叫我。", ZhSubtitle: "当然可以，稍后想点单再叫我。"},
		{Speaker: "阿欣（女顧客）", Gender: "FEMALE", Text: "我哋趕時間，有冇啲快啲嘅套餐？", ZhSubtitle: "我们赶时间，有没有快一点的套餐？"},
		{Speaker: "阿明（男店員）", Gender: "MALE", Text: "有，不過而家菠蘿油要等十分鐘，粉麵會快啲。", ZhSubtitle: "有，不过菠萝油要等十分钟，粉面会更快。"},
		{Speaker: "阿欣（女顧客）", Gender: "FEMALE", Text: "咁我哋要兩份粉麵，飲品凍檸茶走甜，得唔得？", ZhSubtitle: "那我们要两份粉面，饮料冰柠茶少糖，可以吗？"},
		{Speaker: "阿明（男店員）", Gender: "MALE", Text: "得呀，我而家幫你哋落單，大概八分鐘送到。", ZhSubtitle: "可以，我现在帮你们下单，大约八分钟送到。"},
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
