package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/linguaquest/server/internal/domain"
)

func TestListeningUnfixedRepairReceivesNeighboursAndPreservesThem(t *testing.T) {
	section := mockExamFixtureSection(2, 6.5)
	conformListeningTrainingFixtureToPlan(&section)
	fixed := make(map[int]domain.QuizQuestion)
	for i, question := range section.Questions {
		if i != 4 {
			fixed[i] = question
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
			t.Error("invalid repair request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, encoded, ok := strings.Cut(req.Messages[1].Content, "DIFFICULTY_REPAIR_DATA:\n")
		var data struct {
			Context []listeningQuestionRepairContextItem `json:"orderedQuestionContext"`
			Mutable []listeningQuestionRepairItem        `json:"questionsToReplace"`
		}
		if err := json.Unmarshal([]byte(encoded), &data); err != nil || !ok || len(data.Context) != 10 || len(data.Mutable) != 1 {
			t.Errorf("repair lost whole-set context: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for i, item := range data.Context {
			if item.Number != i+1 || item.Frozen != (i != 4) || item.Evidence != section.Questions[i].Evidence {
				t.Errorf("invalid neighbouring evidence context for item %d", i+1)
			}
		}
		data.Mutable[0].Question = "Which choice meets the requirements for the fifth decision?"
		content, _ := json.Marshal(listeningQuestionRepair{Items: data.Mutable})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()
	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	draft, err := g.repairListeningUnfixedQuestions(t.Context(), mockExamSource{Title: section.Title, AudioScript: section.AudioScript}, section, fixed, "item 5 is too easy")
	if err != nil {
		t.Fatal(err)
	}
	for i, question := range draft.Questions {
		if i != 4 && !reflect.DeepEqual(question, section.Questions[i]) {
			t.Fatalf("frozen item %d changed", i+1)
		}
	}
	if draft.Questions[4].Question == section.Questions[4].Question {
		t.Fatal("mutable question was not repaired")
	}
}

func TestListeningPlanRepairOnlyReplacesFailedItems(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	section := domain.MockExamSection{Title: "Repair fixture", Instructions: "Answer all questions.", AudioScript: source, Questions: make([]domain.QuizQuestion, 10)}
	fixed := map[int]domain.QuizQuestion{}
	for i, item := range plan.Items {
		section.Questions[i] = domain.QuizQuestion{Type: item.Type, Question: "original question", Options: []string{"A. old", "B. old", "C. old", "D. old"}, AnswerKey: "A", Evidence: item.Evidence}
		if i != 4 {
			fixed[i] = section.Questions[i]
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Messages) != 2 || !strings.Contains(req.Messages[1].Content, "LISTENING_FAILED_ITEM_REPAIR:") {
			t.Error("targeted repair prompt was not used")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Messages[1].Content, "separate sentences or speaker turns") || !strings.Contains(req.Messages[1].Content, "never invent another speaker") {
			t.Error("targeted repair must also support single-speaker lecture evidence")
		}
		var data struct {
			Context []listeningQuestionRepairContextItem `json:"orderedQuestionContext"`
		}
		_, encoded, _ := strings.Cut(req.Messages[1].Content, "LISTENING_FAILED_ITEM_REPAIR:\n")
		if err := json.Unmarshal([]byte(encoded), &data); err != nil || len(data.Context) != 10 {
			t.Errorf("missing full repair context: %v", err)
		} else {
			for i, item := range data.Context {
				if item.Number != i+1 || item.Frozen != (i != 4) || item.Evidence != section.Questions[i].Evidence || item.Question != section.Questions[i].Question {
					t.Errorf("incorrect context for position %d", i+1)
				}
			}
		}
		response := listeningQuestionRepair{Items: []listeningQuestionRepairItem{{
			Number: 5, Type: plan.Items[4].Type, Question: "Which option reflects the stated budget decision?", Options: []string{"A. Cedar only", "B. Oak only", "C. Both", "D. Neither"}, AnswerKey: "B", Evidence: plan.Items[4].Evidence,
		}}}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()
	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	draft, err := g.repairListeningPlanQuestions(t.Context(), mockExamSource{Title: section.Title, AudioScript: source}, section, plan, fixed, "item 5 failed")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Questions[4].Question == section.Questions[4].Question {
		t.Fatal("failed question was not replaced")
	}
	for i := range draft.Questions {
		if i != 4 && draft.Questions[i].Question != section.Questions[i].Question {
			t.Fatalf("fixed question %d was changed", i+1)
		}
	}
}

func listeningQuestionPlanFixture() (listeningQuestionPlan, string) {
	segments := make([]string, 10)
	for i := range segments {
		segments[i] = "Maya: Option cedar meets the stated budget for decision " + string(rune('A'+i)) + ". Daniel: However, only oak survives the required wet-weather test for that decision."
	}
	source := strings.Join(segments, "\n")
	plan := listeningQuestionPlan{}
	for i, evidence := range segments {
		item := listeningQuestionPlanItem{
			Number:   i + 1,
			Type:     "multiple_choice",
			Level:    "ADVANCED",
			Evidence: evidence,
			FactA:    "cedar meets the stated budget",
			FactB:    "oak survives the required wet-weather test",
			Mechanisms: []string{
				"integration", "constraint",
			},
			Design: "Combine the selected object with its stated comparison condition to reject each partial alternative.",
		}
		if i >= 4 && i < 9 {
			item.Level = "STANDARD"
			item.FactB = ""
			item.Mechanisms = []string{"paraphrase"}
		}
		if i == 9 {
			item.Level = "BASIC"
			item.Type = "sentence_completion"
			item.FactA = "stated budget"
			item.FactB = ""
			item.Mechanisms = []string{"detail"}
		}
		if i == 6 {
			item.Type = "sentence_completion"
			item.FactA = "stated budget"
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, source
}

func TestListeningRepairRequestsUseStrictSchemaOnlyForResponses(t *testing.T) {
	for _, responses := range []bool{false, true} {
		for _, operation := range []string{"LISTENING_DIFFICULTY_ITEM_REPAIR", "LISTENING_PLAN_ITEM_REPAIR"} {
			t.Run(fmt.Sprintf("%t/%s", responses, operation), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					if responses {
						textConfig, _ := payload["text"].(map[string]any)
						format, _ := textConfig["format"].(map[string]any)
						if format["type"] != "json_schema" || format["name"] != "listening_question_repair" || format["strict"] != true {
							t.Errorf("missing repair schema: %#v", format)
						}
						schema, _ := format["schema"].(map[string]any)
						props, _ := schema["properties"].(map[string]any)
						items, _ := props["items"].(map[string]any)
						item, _ := items["items"].(map[string]any)
						required, _ := item["required"].([]any)
						fields, _ := item["properties"].(map[string]any)
						if len(required) != 6 || len(fields) != 6 || item["additionalProperties"] != false {
							t.Error("repair fields are not strictly bounded")
						}
					} else if payload["text"] != nil || payload["response_format"] != nil {
						t.Error("legacy chat provider format changed")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"items":[]}`}}}})
				}))
				defer server.Close()
				endpoint := server.URL
				if responses {
					endpoint += "/v1/responses"
				}
				g := NewOpenAIGenerator("test-key", "test-model", endpoint)
				if _, err := g.mockExamCompletion(t.Context(), "JSON only", "repair questions", operation); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestListeningQuestionPlanRequiresGroundedHighBandMix(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	if err := validateListeningQuestionPlan(plan, source, 7.5); err != nil {
		t.Fatal(err)
	}
	oneWordCompletion := plan
	oneWordCompletion.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
	oneWordCompletion.Items[6].FactA = "budget"
	if err := validateListeningQuestionPlan(oneWordCompletion, source, 7.5); err != nil {
		t.Fatalf("one-word completion answer phrase was rejected: %v", err)
	}
	invalid := plan
	invalid.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
	invalid.Items[0].FactB = ""
	if err := validateListeningQuestionPlan(invalid, source, 7.5); err == nil {
		t.Fatal("advanced item without two grounded facts was accepted")
	}
	invalid = plan
	invalid.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
	invalid.Items[9].Level = "STANDARD"
	invalid.Items[9].Mechanisms = []string{"paraphrase"}
	if err := validateListeningQuestionPlan(invalid, source, 7.5); err == nil {
		t.Fatal("wrong high-band item mix was accepted")
	}
	invalid = plan
	invalid.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
	invalid.Items[0].Type = "sentence_completion"
	invalid.Items[6].Type = "multiple_choice"
	if err := validateListeningQuestionPlan(invalid, source, 7.5); err == nil {
		t.Fatal("advanced sentence completion was accepted")
	}
	invalid = plan
	invalid.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
	invalid.Items[6].FactA = "cedar meets the stated budget"
	if err := validateListeningQuestionPlan(invalid, source, 7.5); err == nil {
		t.Fatal("overlong completion answer phrase was accepted")
	}
}

func TestListeningQuestionPlanOrderIsNormalizedFromExactEvidence(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	plan.Items[1], plan.Items[7] = plan.Items[7], plan.Items[1]
	if !normalizeListeningQuestionPlanOrder(&plan, source) {
		t.Fatal("valid exact plan evidence was not ordered")
	}
	if err := validateListeningQuestionPlan(plan, source, 7.5); err != nil {
		t.Fatal(err)
	}
	plan.Items[1].Evidence = plan.Items[0].Evidence
	if normalizeListeningQuestionPlanOrder(&plan, source) {
		t.Fatal("duplicate evidence was normalized instead of rejected")
	}
}

func TestListeningPlanStructurallyConformingItemsUnlocksOnlyDriftedQuestion(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	section := domain.MockExamSection{AudioScript: source, Questions: make([]domain.QuizQuestion, len(plan.Items))}
	for i, item := range plan.Items {
		section.Questions[i] = domain.QuizQuestion{
			Type: item.Type, Question: "question", Options: []string{"A. one", "B. two", "C. three", "D. four"}, AnswerKey: "A", Evidence: item.Evidence,
		}
		if item.Type == "sentence_completion" {
			section.Questions[i].Question = "Complete the statement with NO MORE THAN THREE WORDS: ____"
			section.Questions[i].Options = nil
			section.Questions[i].AnswerKey = item.FactA
		}
	}
	section.Questions[2].Type = "sentence_completion"
	section.Questions[2].Options = nil
	section.Questions[2].AnswerKey = "stated budget"

	fixed := listeningPlanStructurallyConformingItems(section, plan)
	if len(fixed) != 9 {
		t.Fatalf("expected only the drifted question to remain mutable, got %d fixed items", len(fixed))
	}
	if _, ok := fixed[2]; ok {
		t.Fatal("question with a changed planned type was frozen")
	}
}

func TestListeningQuestionPlanReviewsViabilityAtBand65(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	// Band 6.5 authoring uses the formal minimum of two ADVANCED items and one
	// BASIC item; the independent release gate remains 2-4 ADVANCED and at most 3 BASIC.
	plan.Items[3].Level = "STANDARD"
	plan.Items[3].FactB = ""
	plan.Items[3].Mechanisms = []string{"paraphrase"}
	plan.Items[2].Level = "STANDARD"
	plan.Items[2].FactB = ""
	plan.Items[2].Mechanisms = []string{"paraphrase"}
	viabilityCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
			t.Error("invalid model request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var response any = plan
		if strings.Contains(req.Messages[1].Content, "LISTENING_PLAN_VIABILITY:") {
			viabilityCalls++
			review := listeningPlanConformanceReview{Approved: true, Feedback: "Every planned item has sufficient source support for its stated listening operation."}
			for i := 1; i <= 10; i++ {
				review.Items = append(review.Items, listeningPlanConformanceItem{Number: i, Follows: true, Reason: "The source evidence supports the planned facts and required listening operation."})
			}
			response = review
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	if _, err := g.planHighBandListeningQuestions(t.Context(), mockExamSource{Title: "Fixture", AudioScript: source}, 1, 6.5, ""); err != nil {
		t.Fatal(err)
	}
	if viabilityCalls != 1 {
		t.Fatalf("band 6.5 plan received %d viability reviews, want 1", viabilityCalls)
	}
}

func TestListeningQuestionPlanRetryIncludesPreviousPlan(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
			t.Error("invalid model request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		prompt := req.Messages[1].Content
		var response any
		if strings.Contains(prompt, "LISTENING_PLAN_VIABILITY:") {
			response = approvedListeningPlanReview()
		} else {
			requests++
			response = plan
			if requests == 1 {
				invalid := plan
				invalid.Items = append([]listeningQuestionPlanItem(nil), plan.Items...)
				invalid.Items[7].Evidence = "an invented quotation that is absent from the frozen source"
				response = invalid
			} else if !strings.Contains(prompt, `"previousPlan"`) || !strings.Contains(prompt, "plan item 8 lacks 4-60 word continuous exact evidence") {
				t.Error("plan retry omitted the previous plan or exact validator diagnostic")
			}
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	if _, err := g.planHighBandListeningQuestions(t.Context(), mockExamSource{Title: "Fixture", AudioScript: source}, 3, 7.5, ""); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("plan generation used %d requests, want 2", requests)
	}
}

func TestNormalizeListeningPlanCompletionMixRepairsOneCompletion(t *testing.T) {
	plan, source := listeningQuestionPlanFixture()
	plan.Items[6].Type = "sentence_completion"
	plan.Items[6].Level = "STANDARD"
	plan.Items[6].FactA = "stated budget"
	plan.Items[6].FactB = ""
	plan.Items[6].Mechanisms = []string{"paraphrase"}
	if !normalizeListeningPlanCompletionMix(&plan, source, 7.5) {
		t.Fatal("one valid completion was not repaired to the required pair")
	}
	completions := 0
	for _, item := range plan.Items {
		if item.Type == "sentence_completion" {
			completions++
		}
	}
	if completions != 2 {
		t.Fatalf("got %d completions, want 2", completions)
	}
}

func TestListeningQuestionPlanGuidanceIsBinding(t *testing.T) {
	plan, _ := listeningQuestionPlanFixture()
	prompt := listeningQuestionPlanGuidance(plan)
	for _, required := range []string{"BINDING HIGH-BAND QUESTION PLAN", "both factA and factB from separate sentences/turns necessary", "do not expose levels", "PLAN_JSON:"} {
		if !strings.Contains(prompt, required) {
			t.Errorf("plan guidance missing %q", required)
		}
	}
}

func TestListeningPlanEvidenceRebuildUsesUniqueContinuousFacts(t *testing.T) {
	source := "Maya: The first group receives the neutral leaflet after lunch. Daniel: However, the second group receives the revised leaflet after lunch. Maya: That group can proceed only after the supervisor confirms the room change. Both groups then complete the same questionnaire."
	item := listeningQuestionPlanItem{
		Level:    "ADVANCED",
		Evidence: "the second group receives the revised leaflet. Both groups then complete the same questionnaire.",
		FactA:    "second group receives the revised leaflet",
		FactB:    "supervisor confirms the room change",
	}
	got := listeningPlanEvidenceFromFacts(source, item)
	if !mockExamExactEvidence(source, got) || mockExamWordCount(got) < 20 || mockExamWordCount(got) > 60 || !listeningFactsSeparated(got, item.FactA, item.FactB) {
		t.Fatalf("facts did not rebuild a valid exact evidence span: %q", got)
	}
	item.FactB = "a fact that does not exist"
	if rebuilt := listeningPlanEvidenceFromFacts(source, item); rebuilt != item.Evidence {
		t.Fatalf("missing fact unexpectedly changed evidence: %q", rebuilt)
	}
}

func TestListeningFactsSeparatedRejectsReversedAndOverlappingFacts(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
		factA    string
		factB    string
		want     bool
	}{
		{"reversed", "Maya: Cedar meets the budget. Daniel: Oak survives the wet-weather test.", "Oak survives the wet-weather test", "Cedar meets the budget", true},
		{"overlapping", "The revised plan meets the budget constraint.", "meets the budget", "budget constraint", false},
		{"same occurrence", "The revised plan meets the budget constraint.", "meets the budget", "meets the budget", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := listeningFactsSeparated(tc.evidence, tc.factA, tc.factB); got != tc.want {
				t.Fatalf("listeningFactsSeparated()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyListeningPlanEvidenceOnlyRepairsInvalidQuotes(t *testing.T) {
	source := "Maya: The first option meets the budget. Daniel: The second option alone survives the wet-weather test."
	plan := listeningQuestionPlan{Items: []listeningQuestionPlanItem{{
		Number:   1,
		Type:     "multiple_choice",
		Evidence: "The first option meets the budget. Daniel: The second option alone survives the wet-weather test.",
	}}}
	section := domain.MockExamSection{AudioScript: source, Questions: []domain.QuizQuestion{{
		Type: "multiple_choice", Evidence: "The first option meets the budget ... the second option survives the test.",
	}}}
	if got := applyListeningPlanEvidence(&section, plan); got != 1 || section.Questions[0].Evidence != plan.Items[0].Evidence {
		t.Fatalf("validated plan evidence was not restored: count=%d evidence=%q", got, section.Questions[0].Evidence)
	}
	section.Questions[0].Evidence = "invalid"
	section.Questions[0].Type = "sentence_completion"
	section.Questions[0].AnswerKey = "missing answer"
	if got := applyListeningPlanEvidence(&section, plan); got != 0 {
		t.Fatal("mismatched question type unexpectedly received plan evidence")
	}
}

func TestListeningPlanConformanceRequiresAllTenItems(t *testing.T) {
	review := listeningPlanConformanceReview{Approved: true, Feedback: "Every authored item implements its planned operation and grounded evidence."}
	for i := 1; i <= 10; i++ {
		review.Items = append(review.Items, listeningPlanConformanceItem{Number: i, Follows: true, Reason: "Both planned facts are necessary and the distractors preserve the intended contrast."})
	}
	if err := validateListeningPlanConformance(review); err != nil {
		t.Fatal(err)
	}
	review.Items[4].Follows = false
	if err := validateListeningPlanConformance(review); err == nil {
		t.Fatal("nonconforming planned item was accepted")
	}
	feedback := listeningPlanConformanceFeedback(review)
	for _, required := range []string{"AUTHOR PLAN-CONFORMANCE REJECTION", `"number":5`, "distractors preserve the intended contrast"} {
		if !strings.Contains(feedback, required) {
			t.Errorf("conformance feedback missing %q", required)
		}
	}
	section := domain.MockExamSection{Questions: make([]domain.QuizQuestion, 10)}
	for i := range section.Questions {
		section.Questions[i].Question = "question " + string(rune('A'+i))
	}
	fixed := listeningPlanConformingItems(section, review)
	if len(fixed) != 9 {
		t.Fatalf("expected nine conforming questions to be frozen, got %d", len(fixed))
	}
	if _, exists := fixed[4]; exists {
		t.Fatal("failed question was frozen for the next repair attempt")
	}
}

func TestListeningPlanConformanceRetainsItemsButPreservesOverallRejection(t *testing.T) {
	review := listeningPlanConformanceReview{Feedback: "Only newly authored items are evaluated in this revision round."}
	for i := 1; i <= 10; i++ {
		review.Items = append(review.Items, listeningPlanConformanceItem{Number: i, Follows: i > 2, Reason: "The current reviewer supplied a sufficiently detailed conformance reason."})
	}
	previouslyAccepted := listeningSemanticallyAcceptedItems{0: {Question: "locked one"}, 1: {Question: "locked two"}}
	retainListeningPlanConformance(&review, previouslyAccepted)
	if review.Approved || !review.Items[0].Follows || !review.Items[1].Follows {
		t.Fatal("previously accepted unchanged items did not remain accepted")
	}
	if err := validateListeningPlanConformance(review); err == nil {
		t.Fatal("retaining accepted items must not override the current overall rejection")
	}
}

func TestListeningPlanReviewPromptsSupportAcademicMonologues(t *testing.T) {
	for _, band := range []float64{6.5, 7} {
		t.Run(fmt.Sprint(band), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Messages []struct{ Content string } `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Messages) != 2 {
					t.Error("invalid model request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				calls++
				for _, required := range []string{"academic interpretation", "separate sentences by the same speaker", "either planned fact can be ignored"} {
					if !strings.Contains(req.Messages[1].Content, required) {
						t.Errorf("academic review prompt missing %q", required)
					}
				}
				content, _ := json.Marshal(approvedListeningPlanReview())
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			}))
			defer server.Close()
			g := NewOpenAIGenerator("test-key", "test-model", server.URL)
			plan, script := listeningQuestionPlanFixture()
			source := mockExamSource{Title: "Academic lecture fixture", AudioScript: script}
			if _, err := g.reviewListeningQuestionPlanViability(t.Context(), source, plan, band); err != nil {
				t.Fatal(err)
			}
			if _, err := g.reviewListeningPlanConformance(t.Context(), source, domain.MockExamSection{}, plan, band, nil); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("got %d reviews, want two", calls)
			}
		})
	}
}

func TestListeningPlanConformanceDoesNotFreezeWholeSetAfterOverallRejection(t *testing.T) {
	section := mockExamFixtureSection(2, 7)
	review := approvedListeningPlanReview()
	review.Approved = false
	if fixed := listeningPlanPartialConformingItems(section, review); len(fixed) != 0 {
		t.Fatalf("overall rejection with all item flags true froze %d items", len(fixed))
	}
	review.Items[3].Follows = false
	if fixed := listeningPlanPartialConformingItems(section, review); len(fixed) != 9 {
		t.Fatalf("partial conformance retained %d items, want 9", len(fixed))
	}
}

func TestListeningRepairFeedbackPreservesOriginalObjective(t *testing.T) {
	got := listeningRepairFeedback("replace weak tasks at positions 2 and 4", "item 2 still uses direct retrieval")
	for _, required := range []string{"ORIGINAL_REVISION_OBJECTIVE", "replace weak tasks at positions 2 and 4", "LATEST_REPAIR_DIAGNOSTIC", "item 2 still uses direct retrieval"} {
		if !strings.Contains(got, required) {
			t.Errorf("repair feedback missing %q", required)
		}
	}
}
