package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

func TestAcademic65SectionUsesRelaxedEditorialProfile(t *testing.T) {
	for _, key := range []string{"LISTENING_3", "LISTENING_4"} {
		section := domain.MockExamSection{Key: key, TargetBand: 6.5}
		if got := listeningProfileForSection(section); got != (listeningDifficultyProfile{7, 0, 5}) {
			t.Fatalf("%s profile=%v, want relaxed academic 6.5 profile", key, got)
		}
	}
	if got := listeningProfileForSection(domain.MockExamSection{Key: "LISTENING_1", TargetBand: 6.5}); got != listeningProfile(6.5) {
		t.Fatalf("everyday 6.5 profile changed: %v", got)
	}
}

func TestOrderListeningQuestionsByEvidenceOnlyReordersSafeFreshSet(t *testing.T) {
	section := mockExamFixtureSection(1, 6.5)
	section.Questions[0], section.Questions[1] = section.Questions[1], section.Questions[0]
	if !orderListeningQuestionsByEvidence(&section) {
		t.Fatal("safe out-of-order evidence was not repaired")
	}
	if !strings.Contains(section.Questions[0].Evidence, mockExamFixtureAnswers[0]) {
		t.Fatal("question order was not restored")
	}
	if !strings.HasPrefix(section.Questions[0].Question, "1. ") || !strings.HasPrefix(section.Questions[1].Question, "2. ") {
		t.Fatal("question numbers were not normalized")
	}

	section = mockExamFixtureSection(1, 6.5)
	section.Questions[0].Question = "Which fact is tested in question 2?"
	section.Questions[0], section.Questions[1] = section.Questions[1], section.Questions[0]
	if orderListeningQuestionsByEvidence(&section) {
		t.Fatal("question references must prevent automatic reorder")
	}
}

func TestListeningEvidenceReorderIsAtomicAndPreservesAnswerContent(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "ambiguous", "duplicate", "reference"} {
		t.Run(mode, func(t *testing.T) {
			section := mockExamFixtureSection(1, 6.5)
			section.Questions[0], section.Questions[8] = section.Questions[8], section.Questions[0]
			switch mode {
			case "missing":
				section.Questions[8].Evidence = "This quotation does not occur in the recording."
			case "ambiguous":
				section.AudioScript += "\n" + section.Questions[8].Evidence
			case "duplicate":
				section.Questions[9].Evidence = section.Questions[8].Evidence
			case "reference":
				section.Questions[8].Options[0] = "A. The answer to question 2"
			}
			before := append([]domain.QuizQuestion(nil), section.Questions...)
			if changed := orderListeningQuestionsByEvidence(&section); changed != (mode == "valid") {
				t.Fatalf("unexpected reorder result: %t", changed)
			}
			if mode != "valid" {
				if !reflect.DeepEqual(section.Questions, before) {
					t.Fatal("unsafe set was partially mutated")
				}
				return
			}
			for _, q := range section.Questions {
				found := false
				for _, prior := range before {
					if prior.Evidence != q.Evidence {
						continue
					}
					found = true
					q.Question = candidateQuestionNumberPrefix.ReplaceAllString(q.Question, "")
					prior.Question = candidateQuestionNumberPrefix.ReplaceAllString(prior.Question, "")
					if !reflect.DeepEqual(q, prior) {
						t.Fatal("reorder changed content, options or answer")
					}
				}
				if !found {
					t.Fatal("reorder introduced an unknown question")
				}
			}
		})
	}
}

func TestAcademic65SectionAcceptsRichStandardMixWithoutAdvancedItems(t *testing.T) {
	validSection := func() domain.MockExamSection {
		section := mockExamFixtureSection(2, 6.5)
		section.ListeningDifficulty = listeningAuditFixture(section, 0)
		mechanisms := []string{"paraphrase", "correction", "stance"}
		for i := range section.ListeningDifficulty.Review.Items {
			section.ListeningDifficulty.Review.Items[i].Mechanisms = []string{mechanisms[i%len(mechanisms)]}
		}
		return section
	}

	if err := ValidateListeningDifficulty(validSection()); err != nil {
		t.Fatalf("rich academic 6.5 standard mix rejected: %v", err)
	}

	tests := map[string]func(*domain.MockExamSection){
		"insufficient mechanism variety": func(section *domain.MockExamSection) {
			for i := range section.ListeningDifficulty.Review.Items {
				section.ListeningDifficulty.Review.Items[i].Mechanisms = []string{"paraphrase"}
			}
		},
		"low confidence": func(section *domain.MockExamSection) {
			section.ListeningDifficulty.Review.Confidence = "LOW"
		},
		"wrong answer": func(section *domain.MockExamSection) {
			section.ListeningDifficulty.Review.Items[0].Answer = "not the answer"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			section := validSection()
			mutate(&section)
			if err := ValidateListeningDifficulty(section); !errors.Is(err, ErrListeningDifficulty) {
				t.Fatalf("invalid academic 6.5 content accepted: %v", err)
			}
		})
	}
}

// 仅用于状态/校验规则测试，不表示这些人工测试题通过了真实难度评审。
func listeningAuditFixture(section domain.MockExamSection, advanced int) *domain.ListeningDifficultyAudit {
	review := domain.ListeningDifficultyReview{Approved: true, Confidence: "HIGH", Feedback: "Controlled test review for exercising the internal difficulty gate only."}
	for i, q := range section.Questions {
		item := domain.ListeningDifficultyItem{Number: i + 1, Level: "STANDARD", Answer: q.AnswerKey, Evidence: q.Evidence, Reason: "Controlled fixture reasoning for verifying source and audit storage contracts.", Mechanisms: []string{"paraphrase"}}
		if i < advanced {
			item.Level, item.Mechanisms = "ADVANCED", []string{"correction", "constraint"}
		}
		review.Items = append(review.Items, item)
	}
	return &domain.ListeningDifficultyAudit{RubricVersion: ListeningDifficultyRubricVersion, ContentHash: ListeningContentHash(section), ReviewedAt: time.Now().UTC(), Review: review}
}

func TestListeningDifficultyProfilesAndTampering(t *testing.T) {
	wantProfiles := map[float64]listeningDifficultyProfile{
		6:   {4, 1, 3},
		6.5: {3, 2, 4},
		7:   {2, 3, 5},
		7.5: {2, 4, 6},
		8:   {1, 5, 7},
	}
	for band, want := range wantProfiles {
		if got := listeningProfile(band); got != want {
			t.Fatalf("target %.1f profile=%v, want %v", band, got, want)
		}
	}
	for _, band := range []float64{6, 6.5, 7, 7.5, 8} {
		for advanced := 0; advanced <= 10; advanced++ {
			section := mockExamFixtureSection(0, band)
			section.ListeningDifficulty = listeningAuditFixture(section, advanced)
			profile := listeningProfile(band)
			want := advanced >= profile.minAdvanced && advanced <= profile.maxAdvanced
			if got := ValidateListeningDifficulty(section) == nil; got != want {
				t.Fatalf("target %.1f advanced=%d allowed=%v want=%v", band, advanced, got, want)
			}
		}
	}
	mutations := map[string]func(*domain.MockExamSection){
		"missing":          func(s *domain.MockExamSection) { s.ListeningDifficulty = nil },
		"old rubric":       func(s *domain.MockExamSection) { s.ListeningDifficulty.RubricVersion = "old" },
		"changed content":  func(s *domain.MockExamSection) { s.AudioScript += " Changed." },
		"changed question": func(s *domain.MockExamSection) { s.Questions[0].Question += " Changed?" },
		"changed target":   func(s *domain.MockExamSection) { s.TargetBand = 8 },
		"rejected":         func(s *domain.MockExamSection) { s.ListeningDifficulty.Review.Approved = false },
		"uncertain":        func(s *domain.MockExamSection) { s.ListeningDifficulty.Review.Confidence = "LOW" },
		"partial": func(s *domain.MockExamSection) {
			s.ListeningDifficulty.Review.Items = s.ListeningDifficulty.Review.Items[:9]
		},
		"wrong answer": func(s *domain.MockExamSection) { s.ListeningDifficulty.Review.Items[0].Answer = "not the answer" },
		"no evidence": func(s *domain.MockExamSection) {
			s.ListeningDifficulty.Review.Items[0].Evidence = "fabricated quotation unrelated to this original source"
		},
		"unsupported advanced": func(s *domain.MockExamSection) { s.ListeningDifficulty.Review.Items[0].Mechanisms = []string{"detail"} },
		"overloaded standard": func(s *domain.MockExamSection) {
			s.ListeningDifficulty.Review.Items[3].Mechanisms = []string{"paraphrase", "constraint"}
		},
		"duplicate mechanisms": func(s *domain.MockExamSection) {
			s.ListeningDifficulty.Review.Items[0].Mechanisms = []string{"paraphrase", "paraphrase"}
		},
		"future audit": func(s *domain.MockExamSection) { s.ListeningDifficulty.ReviewedAt = time.Now().Add(time.Hour) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			section := mockExamFixtureSection(0, 7)
			section.ListeningDifficulty = listeningAuditFixture(section, 3)
			mutate(&section)
			if err := ValidateListeningDifficulty(section); !errors.Is(err, ErrListeningDifficulty) {
				t.Fatalf("invalid content accepted: %v", err)
			}
		})
	}
}

func TestListeningDifficultyReviewerFailsClosed(t *testing.T) {
	for _, mode := range []string{"valid", "inconsistent_once", "too easy", "uncertain", "uncertain_rejection", "malformed", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			section := mockExamFixtureSection(0, 7)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				for _, forbidden := range []string{"targetBand", "TargetBand", "answerKey", "AnswerKey", "RubricVersion"} {
					if strings.Contains(string(body), forbidden) {
						t.Errorf("review was given author/target metadata: %s", forbidden)
					}
				}
				if mode == "unavailable" {
					w.WriteHeader(401)
					return
				}
				review := listeningAuditFixture(section, 3).Review
				if mode == "inconsistent_once" && calls == 1 {
					review.Items[0].Level = "BASIC"
				}
				if mode == "too easy" {
					review = listeningAuditFixture(section, 0).Review
				}
				if mode == "uncertain" {
					review.Confidence = "LOW"
				}
				if mode == "uncertain_rejection" {
					review.Approved = false
					review.Confidence = "LOW"
					review.Feedback = "Question six contains an unsupported premise and must be replaced before this set can be used."
				}
				content, _ := json.Marshal(review)
				if mode == "malformed" {
					content = []byte(`{"approved":true}`)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			}))
			defer server.Close()
			g := NewOpenAIGenerator("test-key", "test-model", server.URL)
			err := g.reviewListeningDifficulty(context.Background(), &section)
			if (err == nil) != (mode == "valid" || mode == "inconsistent_once") {
				t.Fatalf("mode=%s err=%v", mode, err)
			}
			wantCalls := 1
			if mode == "inconsistent_once" || mode == "malformed" {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatalf("difficulty reviewer was retried until approval: %d", calls)
			}
			if mode == "too easy" && section.ListeningDifficulty == nil {
				t.Fatal("rejection evidence lost")
			}
			if mode == "uncertain_rejection" {
				if errors.Is(err, ErrListeningReviewInvalid) || section.ListeningDifficulty == nil {
					t.Fatalf("complete low-confidence rejection was not preserved as content feedback: %v", err)
				}
			}
		})
	}
}

func TestListeningTrainingQualityGate(t *testing.T) {
	for part := 1; part <= 4; part++ {
		section := mockExamFixtureSection(part-1, 7)
		section.ListeningDifficulty = listeningAuditFixture(section, 3)
		if err := ValidateListeningTraining(section, part); err != nil {
			t.Fatalf("part %d: %v", part, err)
		}
		section.QualityApproved = false
		if err := ValidateListeningTraining(section, part); err == nil {
			t.Fatal("unreviewed training accepted")
		}
		section.QualityApproved = true
		section.AudioScript = "Mia: Short placeholder"
		if err := ValidateListeningTraining(section, part); err == nil {
			t.Fatal("invalid source accepted")
		}
	}
	if err := ValidateListeningTraining(mockExamFixtureSection(0, 7), 2); err == nil {
		t.Fatal("wrong Part accepted")
	}
}

func TestUseBindingAcademicBlueprintOnlyForHighBands(t *testing.T) {
	for _, tc := range []struct {
		part int
		band float64
		want bool
	}{
		{3, 6.5, false},
		{4, 7, true},
		{3, 7.5, true},
		{4, 8, true},
		{1, 6.5, false},
		{2, 7, false},
		{1, 8, false},
		{2, 8, false},
	} {
		if got := useBindingAcademicBlueprint(tc.part, tc.band); got != tc.want {
			t.Fatalf("part=%d band=%.1f useBlueprint=%v want %v", tc.part, tc.band, got, tc.want)
		}
	}
}

func TestListeningAnswerPositionBias(t *testing.T) {
	for _, tc := range []struct {
		answers string
		valid   bool
	}{{"AAA", true}, {"AAAA", false}, {"AABB", false}, {"AABC", true}, {"AAAAABCD", false}, {"ABDACBCD", true}} {
		section := domain.MockExamSection{}
		for _, answer := range tc.answers {
			section.Questions = append(section.Questions, domain.QuizQuestion{Type: "multiple_choice", Options: []string{"A. first", "B. second", "C. third", "D. fourth"}, AnswerKey: string(answer)})
		}
		if valid := validateListeningAnswerPositions(section) == nil; valid != tc.valid {
			t.Errorf("answers=%s valid=%t, want=%t", tc.answers, valid, tc.valid)
		}
	}
}

func TestListeningAnswerPositionRebalancePreservesCorrectContent(t *testing.T) {
	section := domain.MockExamSection{AudioScript: "A source with a stable length for deterministic option rotation."}
	for i := 0; i < 8; i++ {
		section.Questions = append(section.Questions, domain.QuizQuestion{
			Type:      "multiple_choice",
			Options:   []string{"A. alpha", "B. beta", "C. correct " + string(rune('a'+i)), "D. delta"},
			AnswerKey: "C",
			Question:  "Which complete option remains correct after deterministic reordering?",
			Evidence:  "A source with a stable length for deterministic option rotation.",
		})
	}
	if !rebalanceListeningAnswerPositions(&section) {
		t.Fatal("concentrated answers were not rebalanced")
	}
	if err := validateListeningAnswerPositions(section); err != nil {
		t.Fatal(err)
	}
	for i, q := range section.Questions {
		answer := mockExamCanonicalAnswer(q, q.AnswerKey)
		option := mockExamOption.FindStringSubmatch(q.Options[int(answer[0]-'A')])[2]
		want := "correct " + string(rune('a'+i))
		if option != want {
			t.Fatalf("question %d correct content changed: got %q want %q", i+1, option, want)
		}
	}
}

func TestListeningQuestionEvidenceChronology(t *testing.T) {
	section := domain.MockExamSection{AudioScript: "Mia: The morning course starts at ten. Leo: The afternoon course requires prior experience. Mia: The evening course includes a workshop."}
	quotes := []string{"The morning course starts at ten.", "The afternoon course requires prior experience.", "The evening course includes a workshop."}
	for _, quote := range quotes {
		section.Questions = append(section.Questions, domain.QuizQuestion{Evidence: quote})
	}
	if err := validateListeningEditorialStructure(section); err != nil {
		t.Fatal(err)
	}
	section.Questions[1], section.Questions[2] = section.Questions[2], section.Questions[1]
	if validateListeningEditorialStructure(section) == nil {
		t.Fatal("reverse listening order accepted")
	}
	section.Questions = []domain.QuizQuestion{{Evidence: quotes[1]}, {Evidence: quotes[0] + " Leo: " + quotes[1]}}
	if err := validateListeningEditorialStructure(section); err != nil {
		t.Fatalf("overlapping evidence for linked questions rejected: %v", err)
	}
}

func TestListeningQuestionPromptRetainsSchemaContract(t *testing.T) {
	for part := 1; part <= 4; part++ {
		prompt := listeningTrainingQuestionPrompt(part, 7)
		for _, required := range []string{"multiple_choice", "sentence_completion", "matching_information", "NO MORE THAN ONE WORD", "NO MORE THAN TWO WORDS", "NO MORE THAN THREE WORDS", "____", "answerKey", "evidence", "at least TWO items", "STANDALONE TRAINING:", "at least three different correct letters"} {
			if !strings.Contains(prompt, required) {
				t.Errorf("part %d prompt missing schema/quality contract %q", part, required)
			}
		}
		if strings.Contains(prompt, "retain two accessible detail completions") {
			t.Fatal("standalone difficulty contradicted by whole-paper prompt")
		}
	}
}

func TestListeningDifficultyGuidanceBuildsHighBandReviewMargin(t *testing.T) {
	tests := []struct {
		band float64
		want string
	}{
		{band: 7.5, want: "Privately plan exactly 1 BASIC, 5 STANDARD and 4 ADVANCED"},
		{band: 8, want: "Privately plan exactly 0 BASIC, 5 STANDARD and 5 ADVANCED"},
	}
	for _, tt := range tests {
		guidance := listeningDifficultyGuidance(tt.band)
		for _, required := range []string{
			tt.want,
			"at most ONE may be BASIC",
			"genuine correction or constraint",
			"Never use a definition question",
			"only the correct option satisfies both",
		} {
			if !strings.Contains(guidance, required) {
				t.Errorf("band %.1f guidance missing %q", tt.band, required)
			}
		}
	}
}

func TestListeningQuestionPromptRejectsEasyHighBandCompletions(t *testing.T) {
	for _, tc := range []struct {
		band float64
		mix  string
	}{{7.5, "exactly 4 ADVANCED, 5 STANDARD and 1 BASIC"}, {8, "exactly 5 ADVANCED, 5 STANDARD and 0 BASIC"}} {
		prompt := listeningTrainingQuestionPrompt(3, tc.band)
		for _, required := range []string{
			"at most ONE may be a direct-detail BASIC item",
			"resolve a correction or constraint",
			"Definitions, directly stated reasons and single-sentence word spotting are BASIC",
			"only the correct option satisfies both",
			"HIGH-BAND AUTHORING CHECK:",
			tc.mix,
		} {
			if !strings.Contains(prompt, required) {
				t.Errorf("band %.1f high-band question prompt missing %q", tc.band, required)
			}
		}
	}
}

func TestListeningQuestionPromptRequiresExactMixFromBand65(t *testing.T) {
	for _, tc := range []struct {
		band float64
		mix  string
	}{
		{6.5, "exactly 1 BASIC, 6 STANDARD and 3 ADVANCED items"},
		{7, "exactly 1 BASIC, 5 STANDARD and 4 ADVANCED items"},
	} {
		prompt := listeningTrainingQuestionPrompt(1, tc.band)
		if !strings.Contains(prompt, "MANDATORY DIFFICULTY MIX") || !strings.Contains(prompt, tc.mix) {
			t.Errorf("band %.1f prompt missing exact mix %q", tc.band, tc.mix)
		}
	}
}

func TestListeningRevisionFeedbackIdentifiesWeakItems(t *testing.T) {
	section := domain.MockExamSection{TargetBand: 6.5, ListeningDifficulty: &domain.ListeningDifficultyAudit{Review: domain.ListeningDifficultyReview{Items: []domain.ListeningDifficultyItem{
		{Number: 1, Level: "BASIC"}, {Number: 2, Level: "STANDARD"}, {Number: 3, Level: "BASIC"}, {Number: 4, Level: "ADVANCED"},
	}, Feedback: "Items 2 and 4 repeat the same timetable."}}}
	feedback := listeningRevisionFeedback(section, ErrListeningDifficulty)
	for _, required := range []string{"2 BASIC", "[1 3]", "1 ADVANCED", "at most 3 BASIC", "2-4 ADVANCED", "Replace weak tasks", "do not invent facts", "REVIEWER SEMANTIC REJECTION", "Items 2 and 4 repeat", "incidental schedule step"} {
		if !strings.Contains(feedback, required) {
			t.Errorf("revision omitted actionable diagnostic %q", required)
		}
	}
}

func TestListeningRevisionPreservesVerifiedItems(t *testing.T) {
	section := mockExamFixtureSection(2, 7)
	section.ListeningDifficulty = listeningAuditFixture(section, 3)
	section.ListeningDifficulty.Review.Items[7].Level = "BASIC"
	fixed := listeningRevisionFixedItems(section)
	if len(fixed) != 8 {
		t.Fatalf("expected weak and review-margin items unlocked, got %d fixed", len(fixed))
	}
	if _, ok := fixed[7]; ok {
		t.Fatal("weak item was frozen")
	}
	section.ListeningDifficulty.Review.Approved = false
	if len(listeningRevisionFixedItems(section)) != 0 {
		t.Fatal("semantically rejected items frozen")
	}
	section.ListeningDifficulty.Review.Approved = true
	section.AudioScript += " Changed."
	if len(listeningRevisionFixedItems(section)) != 0 {
		t.Fatal("stale audit reused to freeze items")
	}
	for _, advanced := range []int{1, 7} {
		section = mockExamFixtureSection(2, 7)
		section.ListeningDifficulty = listeningAuditFixture(section, advanced)
		want := 7
		if advanced == 7 {
			want = 8
		}
		if len(listeningRevisionFixedItems(section)) != want {
			t.Fatal("wrong number of items unlocked to fix advanced quota")
		}
	}
}

func TestListeningFixedItemsStillReceiveBlindReview(t *testing.T) {
	for _, reject := range []bool{false, true} {
		section := mockExamFixtureSection(2, 7)
		reviews := 0
		g := NewOpenAIGenerator("test-key", "test", "https://test.invalid")
		g.Client.Transport = speakingQualityTransport(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			var payload any
			if strings.Contains(string(raw), "SECTION_JSON:") {
				reviews++
				if strings.Contains(string(raw), "damaged fixed question") {
					t.Error("independent reviewer saw a silently changed fixed item")
				}
				review := mockExamFixtureReview(section)
				review.Approved = !reject
				payload = review
			} else {
				questions := append([]domain.QuizQuestion(nil), section.Questions...)
				questions[0].Question = "damaged fixed question"
				payload = mockExamDraft{Instructions: section.Instructions, Questions: questions}
			}
			content, _ := json.Marshal(payload)
			body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
		})
		got, err := g.generateMockExamQuestionsWithFixedItems(context.Background(), mockExamSpecs[2], 7, mockExamSource{Title: section.Title, AudioScript: section.AudioScript}, listeningTrainingQuestionPrompt(3, 7), "", "", map[int]domain.QuizQuestion{0: section.Questions[0]})
		if (err != nil) != reject || reviews == 0 || !reflect.DeepEqual(got.Questions[0], section.Questions[0]) {
			t.Fatalf("fixed items must be preserved AND independently approved: err=%v reviews=%d", err, reviews)
		}
	}
}

func listeningTrainingPlanFixture(section domain.MockExamSection) listeningQuestionPlan {
	plan := listeningQuestionPlan{Items: make([]listeningQuestionPlanItem, len(section.Questions))}
	advancedSlots := map[int]bool{0: true, 2: true, 4: true, 6: true}
	for i, q := range section.Questions {
		item := listeningQuestionPlanItem{
			Number: i + 1, Type: "multiple_choice", Level: "STANDARD",
			Evidence: q.Evidence, FactA: "selected " + mockExamFixtureAnswers[i],
			Mechanisms: []string{"paraphrase"},
			Design:     "Require a substantive paraphrase of the source decision while keeping every distractor grounded.",
		}
		if advancedSlots[i] {
			item.Level = "ADVANCED"
			item.Evidence = q.Evidence + " " + section.Questions[i+1].Evidence
			item.FactB = "selected " + mockExamFixtureAnswers[i+1]
			item.Mechanisms = []string{"integration", "constraint"}
			item.Design = "Combine two separately stated material decisions so each partial alternative violates one necessary fact."
		}
		if i == 7 {
			item.Type = "sentence_completion"
			item.FactA = mockExamFixtureAnswers[i]
		}
		if i == 9 {
			item.Type, item.Level = "sentence_completion", "BASIC"
			item.FactA = mockExamFixtureAnswers[i]
			item.Mechanisms = []string{"detail"}
		}
		plan.Items[i] = item
	}
	return plan
}

func approvedListeningPlanReview() listeningPlanConformanceReview {
	review := listeningPlanConformanceReview{Approved: true, Feedback: "Every planned item is grounded, distinct and feasible from the frozen source."}
	for i := 1; i <= 10; i++ {
		review.Items = append(review.Items, listeningPlanConformanceItem{Number: i, Follows: true, Reason: "The source supports the declared operation and grounded alternatives without invented information."})
	}
	return review
}

func conformListeningTrainingFixtureToPlan(section *domain.MockExamSection) {
	if section == nil || len(section.Questions) != 10 {
		return
	}
	plan := listeningTrainingPlanFixture(*section)
	answerPositions := []int{0, 1, 2, 3, 1, 3, 0, 0, 2, 0}
	for i, item := range plan.Items {
		question := &section.Questions[i]
		question.Type = item.Type
		question.Evidence = item.Evidence
		if item.Type == "sentence_completion" {
			question.Question = fmt.Sprintf("For decision %d, the selected material was ____. Write NO MORE THAN THREE WORDS.", i+1)
			question.Options = nil
			question.AnswerKey = mockExamFixtureAnswers[i]
			continue
		}
		correct := answerPositions[i]
		choices := []string{"recycled paper", "woven fabric", "plastic panels", "rubber"}
		choices[correct] = mockExamFixtureAnswers[i]
		question.Question = fmt.Sprintf("Which material satisfies the stated requirements for decision %d?", i+1)
		question.Options = make([]string, 4)
		for option := range choices {
			question.Options[option] = fmt.Sprintf("%c. %s", 'A'+option, choices[option])
		}
		question.AnswerKey = string(rune('A' + correct))
	}
}

// 真实编排的离线回归：修题不能重写原文，系统错误不能当内容问题重试。
func TestListeningTrainingFrozenSourceRevision(t *testing.T) {
	for _, mode := range []string{"repair", "still_easy", "unavailable", "invalid", "quality_failure"} {
		t.Run(mode, func(t *testing.T) {
			section := mockExamFixtureSection(2, 7)
			conformListeningTrainingFixtureToPlan(&section)
			sourceCalls, planCalls, viabilityCalls, conformanceCalls, questionCalls, qualityCalls, difficultyCalls := 0, 0, 0, 0, 0, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Messages []struct{ Content string } `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Messages) != 2 {
					t.Error("invalid completion request")
					w.WriteHeader(400)
					return
				}
				prompt := req.Messages[1].Content
				var response any
				switch {
				case strings.Contains(prompt, "LISTENING_SOURCE_REVIEW:"):
					response = academicReviewFixture(true, "")
				case strings.Contains(prompt, "practice paper. SOURCE_ONLY stage."):
					sourceCalls++
					response = mockExamSource{Title: section.Title, AudioScript: section.AudioScript}
				case strings.Contains(prompt, "LISTENING_DIFFICULTY_SOURCE:"):
					difficultyCalls++
					if mode == "unavailable" {
						w.WriteHeader(401)
						return
					}
					if mode == "invalid" {
						response = map[string]any{"unexpected": true}
					} else {
						advanced := 0
						if mode == "repair" && difficultyCalls == 2 {
							advanced = 3
						}
						response = listeningAuditFixture(section, advanced).Review
					}
				case strings.Contains(prompt, "Design a binding construction plan"):
					planCalls++
					response = listeningTrainingPlanFixture(section)
				case strings.Contains(prompt, "LISTENING_PLAN_VIABILITY:"):
					viabilityCalls++
					response = approvedListeningPlanReview()
				case strings.Contains(prompt, "LISTENING_PLAN_CONFORMANCE:"):
					conformanceCalls++
					response = approvedListeningPlanReview()
				case strings.Contains(prompt, "SECTION_JSON:"):
					qualityCalls++
					if mode == "quality_failure" {
						w.WriteHeader(401)
						return
					}
					response = mockExamFixtureReview(section)
				default:
					questionCalls++
					if mode == "repair" && questionCalls == 2 && !strings.Contains(prompt, "LISTENING_FAILED_ITEM_REPAIR:") {
						t.Error("difficulty retry regenerated the full planned set instead of repairing only mutable items")
					}
					if strings.Contains(prompt, "retain two accessible detail completions") {
						t.Error("contradictory difficulty prompt remains")
					}
					encodedSource, _ := json.Marshal(mockExamSource{Title: section.Title, AudioScript: section.AudioScript})
					if !strings.Contains(prompt, string(encodedSource)) {
						t.Error("question revision lost frozen source")
					}
					if questionCalls == 2 && (!strings.Contains(prompt, "previousDraft") || !strings.Contains(prompt, "Independent item-level review")) {
						t.Error("revision lacks previous questions or concrete review")
					}
					response = mockExamDraft{Instructions: section.Instructions, Questions: section.Questions}
				}
				content, _ := json.Marshal(response)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			}))
			defer server.Close()
			g := NewOpenAIGenerator("test-key", "test-model", server.URL)
			got, err := g.GenerateListeningTraining(context.Background(), 3, 7)
			if (err == nil) != (mode == "repair") {
				t.Fatalf("mode=%s unexpected outcome: %v", mode, err)
			}
			if sourceCalls != 1 || got.AudioScript != section.AudioScript || len(got.Questions) != 10 {
				t.Fatalf("source regenerated or failed draft lost: sourceCalls=%d questions=%d", sourceCalls, len(got.Questions))
			}
			if planCalls < 1 {
				t.Fatal("structured question blueprint was skipped")
			}
			wantCalls := 1
			if mode == "repair" {
				wantCalls = 2
			} else if mode == "still_easy" {
				wantCalls = maxListeningQuestionAttempts
			}
			if conformanceCalls != wantCalls {
				t.Fatalf("7.0 generation used %d blueprint conformance reviews, want %d", conformanceCalls, wantCalls)
			}
			// 中高阶蓝图均须先确认原文能支撑计划；7.0 使用较宽松的自然场景准则。
			if viabilityCalls != 1 {
				t.Fatalf("7.0 generation used %d blueprint viability reviews, want 1", viabilityCalls)
			}
			if questionCalls != wantCalls || qualityCalls != wantCalls {
				t.Fatalf("unexpected generation/review retries: %d/%d", questionCalls, qualityCalls)
			}
			if mode == "quality_failure" {
				if difficultyCalls != 0 || got.QualityApproved {
					t.Fatal("quality failure reached difficulty approval")
				}
				if !errors.Is(err, ErrListeningReviewUnavailable) {
					t.Fatalf("quality-review provider failure was not classified as unavailable: %v", err)
				}
			} else if mode == "invalid" {
				if difficultyCalls != 2 {
					t.Fatalf("invalid reviewer response was not retried exactly once: %d", difficultyCalls)
				}
			} else if difficultyCalls != wantCalls {
				t.Fatalf("unexpected difficulty retries: %d", difficultyCalls)
			}
			if mode == "unavailable" && !errors.Is(err, ErrListeningReviewUnavailable) {
				t.Fatal("provider failure incorrectly classified as content rejection")
			}
			if mode == "invalid" && !errors.Is(err, ErrListeningReviewInvalid) {
				t.Fatal("invalid response incorrectly classified as content rejection")
			}
			if mode == "repair" {
				if err := ValidateListeningTraining(got, 3); err != nil {
					t.Fatalf("revised section not valid: %v", err)
				}
			}
		})
	}
}

func TestListeningDifficultyCancellationPreserved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := NewOpenAIGenerator("test-key", "test-model", "http://127.0.0.1:1")
	section := mockExamFixtureSection(0, 6)
	err := g.reviewListeningDifficulty(ctx, &section)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrListeningReviewUnavailable) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if section.ListeningDifficulty != nil {
		t.Fatal("cancelled review created a fake audit")
	}
}

func TestListeningQuoteRestorationNeverSkipsSpeech(t *testing.T) {
	source := "Maya: We can collect on Friday afternoon.\nLeo: Only after the replacement arrives at noon."
	quote := "collect on Friday afternoon. Only after the replacement arrives at noon."
	got := listeningRestoreSourceQuote(source, quote)
	if !strings.Contains(got, "Leo:") || !mockExamExactEvidence(source, got) {
		t.Fatalf("speaker metadata not restored: %q", got)
	}
	for _, bad := range []string{
		"collect on Thursday afternoon. Only after the replacement arrives at noon.",
		"collect on Friday afternoon. The replacement arrives at noon.",
		"collect on Friday afternoon ... arrives at noon",
	} {
		if repaired := listeningRestoreSourceQuote(source, bad); mockExamExactEvidence(source, repaired) {
			t.Fatalf("changed or discontinuous words accepted: %q", repaired)
		}
	}
	numbers := "Maya: The fee is 1,000 pounds today.\nLeo: Collection is after 12:30 on Friday."
	if got := listeningRestoreSourceQuote(numbers, "The fee is 1.000 pounds today. Collection is after 12:30 on Friday."); mockExamExactEvidence(numbers, got) {
		t.Fatal("numeric punctuation changed")
	}
}

func TestListeningSourceConsistencyRevisionIsBounded(t *testing.T) {
	for _, mode := range []string{"repair", "grammar_repair", "content_repair", "overlong_revision", "late_repair", "reject", "malformed_once", "malformed", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			source := mockExamFixtureSection(0, 6)
			sourceCalls, reviews := 0, 0
			firstSubject := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var req struct{ Messages []struct{ Content string } }
				if json.Unmarshal(body, &req) != nil || len(req.Messages) != 2 {
					t.Error("invalid source request")
					w.WriteHeader(400)
					return
				}
				var response any
				if strings.Contains(string(body), "LISTENING_SOURCE_REVIEW:") {
					reviews++
					if !strings.Contains(string(body), "English language quality") || !strings.Contains(string(body), "missing words") {
						t.Error("source was frozen without language-quality instructions")
					}
					if !strings.Contains(string(body), "TEN DISTINCT chronological comprehension tasks") || !strings.Contains(string(body), "two necessary operations") {
						t.Error("source suitability was not reviewed before freezing")
					}
					if mode == "unavailable" {
						w.WriteHeader(401)
						return
					}
					response = map[string]any{"approved": ((mode == "repair" || mode == "grammar_repair" || mode == "overlong_revision") && reviews == 2) || (mode == "late_repair" && reviews == 4) || mode == "malformed_once", "feedback": "The collection precedes delivery; correct Friday morning to afternoon."}
					if mode == "grammar_repair" {
						response = map[string]any{"approved": reviews == 2, "feedback": "The phrase 'retain an convention' is ungrammatical; use 'retain a convention'."}
					}
					if mode == "content_repair" {
						response = map[string]any{"approved": reviews == 2, "feedback": "The route paragraph announces the answer directly; develop the conflicting access conditions."}
					}
					if mode == "malformed" || (mode == "malformed_once" && reviews == 1) {
						response = map[string]any{"feedback": "Missing decision must not be treated as rejection."}
					}
				} else {
					sourceCalls++
					for _, line := range strings.Split(req.Messages[1].Content, "\n") {
						if strings.HasPrefix(line, "Assigned subject:") {
							if sourceCalls == 1 {
								firstSubject = line
							} else if firstSubject != line {
								t.Error("source repair randomly changed its assigned subject")
							}
						}
					}
					if mode == "grammar_repair" && sourceCalls == 2 && !strings.Contains(string(body), "retain an convention") {
						t.Error("language defects were not supplied for source repair")
					}
					if mode == "content_repair" && sourceCalls == 2 && (!strings.Contains(string(body), "conflicting access conditions") || !strings.Contains(string(body), "insufficient comprehension opportunities")) {
						t.Error("source revision omitted the content-depth defect")
					}
					if sourceCalls == 2 && !strings.Contains(string(body), "previousSource") {
						t.Error("source repair lacks previous source")
					}
					generated := mockExamSource{Title: source.Title, AudioScript: source.AudioScript}
					if mode == "unavailable" && sourceCalls > 1 {
						generated.AudioScript = strings.Replace(generated.AudioScript, "researchers", "field researchers", 1)
					}
					if mode == "overlong_revision" && sourceCalls == 2 {
						generated.AudioScript += strings.Repeat(" excess", 300)
					}
					response = generated
					if sourceCalls > 1 && mode != "unavailable" {
						response = listeningRevisionFixture(generated, listeningSourceReviewDimensions[:]...)
					}
				}
				content, _ := json.Marshal(response)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			}))
			defer server.Close()
			g := NewOpenAIGenerator("test-key", "test-model", server.URL)
			_, err := g.prepareListeningSource(context.Background(), mockExamSpecs[0], 6, "")
			wantSources, wantReviews := 2, 2
			if mode == "late_repair" {
				wantSources, wantReviews = 4, 4
			}
			if mode == "reject" {
				wantSources, wantReviews = 5, 5
			}
			if mode == "malformed_once" {
				wantSources, wantReviews = 1, 2
			}
			if mode == "overlong_revision" {
				wantSources, wantReviews = 3, 2
			}
			if mode == "malformed" {
				wantSources, wantReviews = 1, 3
			}
			if mode == "unavailable" {
				// A transient review-service failure receives one bounded fresh-source
				// retry; both candidates still fail closed when the provider stays unavailable.
				wantSources, wantReviews = 2, 2
			}
			if (err == nil) != (mode == "repair" || mode == "grammar_repair" || mode == "content_repair" || mode == "overlong_revision" || mode == "late_repair" || mode == "malformed_once") || sourceCalls != wantSources || reviews != wantReviews {
				t.Fatalf("mode=%s sources=%d reviews=%d err=%v", mode, sourceCalls, reviews, err)
			}
			if mode == "unavailable" && !errors.Is(err, ErrListeningReviewUnavailable) {
				t.Fatal("source review service failure must not be classified as difficulty failure")
			}
		})
	}
}

func TestListeningSourcePromptDoesNotImportRepairScene(t *testing.T) {
	for _, spec := range mockExamSpecs[:4] {
		for i := 0; i < 12; i++ {
			prompt := mockExamSourcePrompt(spec, 7)
			if !strings.Contains(prompt, "NEVER remove function words") {
				t.Fatal("word-budget guidance must preserve grammatical English")
			}
			repairSubject := strings.Contains(prompt, "Assigned subject: arranging a bicycle repair and collection.")
			if strings.Contains(prompt, "For this repair scenario") != repairSubject {
				t.Fatalf("repair-specific instructions leaked into another subject: %s", spec.key)
			}
		}
	}
}

func TestListeningRevisionObjectiveSurvivesFormatRepair(t *testing.T) {
	section := mockExamFixtureSection(2, 7)
	sources, reviews := 0, 0
	g := NewOpenAIGenerator("test-key", "test", "https://test.invalid")
	g.Client.Transport = speakingQualityTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var payload any
		if strings.Contains(string(raw), "SECTION_JSON:") {
			reviews++
			payload = mockExamFixtureReview(section)
		} else {
			sources++
			if !strings.Contains(string(raw), "replace literal tasks at positions 2 and 4") {
				t.Error("format repair lost the original difficulty objective")
			}
			if sources == 1 {
				payload = map[string]string{"title": "Incomplete test draft"}
			} else {
				payload = mockExamDraft{Instructions: section.Instructions, Questions: section.Questions}
			}
		}
		content, _ := json.Marshal(payload)
		body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	})
	_, err := g.generateMockExamQuestionsWithFixedItems(context.Background(), mockExamSpecs[2], 7, mockExamSource{Title: section.Title, AudioScript: section.AudioScript}, listeningTrainingQuestionPrompt(3, 7), "replace literal tasks at positions 2 and 4", "", nil)
	if err != nil || sources != 2 || reviews != 1 {
		t.Fatalf("bounded format repair failed: %v, sources=%d reviews=%d", err, sources, reviews)
	}
}
