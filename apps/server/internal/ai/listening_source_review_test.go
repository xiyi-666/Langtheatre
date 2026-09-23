package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func academicReviewFixture(approved bool, failedCheck string) map[string]any {
	checks := map[string]any{}
	for _, name := range []string{"chronology", "quantities", "comparisons", "inference"} {
		status, reason := "PASS", "Whole-script facts and their stated limitations agree."
		if name == failedCheck {
			status, reason = "FAIL", "The opening introduces two stakes but the conclusion refers to three; reconcile every mention."
		}
		checks[name] = map[string]string{"status": status, "reason": reason}
	}
	return map[string]any{"approved": approved, "feedback": "Checked the complete source and its conclusion.", "checks": checks}
}

func listeningRevisionFixture(source mockExamSource, repaired ...string) map[string]any {
	repairedSet := make(map[string]bool, len(repaired))
	for _, name := range repaired {
		repairedSet[name] = true
	}
	evidence := strings.TrimSpace(strings.Split(source.AudioScript, "\n")[0])
	checks := make(map[string]any, len(listeningSourceReviewDimensions))
	for _, name := range listeningSourceReviewDimensions {
		status := "UNCHANGED"
		if repairedSet[name] {
			status = "REPAIRED"
		}
		checks[name] = map[string]string{
			"status":        status,
			"evidenceQuote": evidence,
			"resolution":    "This exact revised passage now provides consistent supporting evidence.",
		}
	}
	return map[string]any{"title": source.Title, "audioScript": source.AudioScript, "repairs": checks}
}

func TestAcademicListeningReviewCannotBypassFailedOrMissingChecks(t *testing.T) {
	for _, mode := range []string{"pass", "failed_check", "rejected", "missing_check", "extra_check", "empty_reason", "invalid_status", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			review := academicReviewFixture(mode != "rejected", "")
			checks := review["checks"].(map[string]any)
			switch mode {
			case "failed_check":
				review = academicReviewFixture(true, "quantities")
			case "missing_check":
				delete(checks, "quantities")
			case "extra_check":
				checks["language"] = map[string]string{"status": "PASS", "reason": "Not part of the required four checks."}
			case "empty_reason":
				checks["chronology"] = map[string]string{"status": "PASS", "reason": ""}
			case "invalid_status":
				checks["chronology"] = map[string]string{"status": "UNCERTAIN", "reason": "Cannot establish sequence."}
			case "legacy":
				delete(review, "checks")
			}
			data, _ := json.Marshal(review)
			approved, feedback, err := decodeListeningSourceReview(string(data), true)
			invalid := mode == "missing_check" || mode == "extra_check" || mode == "empty_reason" || mode == "invalid_status" || mode == "legacy"
			if errors.Is(err, ErrListeningReviewInvalid) != invalid || approved != (mode == "pass") {
				t.Fatalf("approved=%t err=%v", approved, err)
			}
			if mode == "failed_check" && !strings.Contains(feedback, "reconcile every mention") {
				t.Fatal("structured rejection evidence was lost")
			}
		})
	}
}

func TestListeningSourceReviewParserAllowsOnlyOneWholeJSONObject(t *testing.T) {
	data, _ := json.Marshal(academicReviewFixture(true, ""))
	for _, tc := range []struct {
		name    string
		content string
		valid   bool
	}{
		{name: "plain", content: string(data), valid: true},
		{name: "fenced", content: "```json\n" + string(data) + "\n```", valid: true},
		{name: "prose_prefix", content: "Review result:\n" + string(data), valid: false},
		{name: "prose_suffix", content: string(data) + "\nApproved after review.", valid: false},
		{name: "second_object", content: string(data) + "\n{}", valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			approved, _, err := decodeListeningSourceReview(tc.content, true)
			if (err == nil) != tc.valid || approved != tc.valid {
				t.Fatalf("approved=%t err=%v", approved, err)
			}
		})
	}
}

func TestAcademicListeningReviewPromptRequiresExhaustiveMethodAndComparisonChecks(t *testing.T) {
	prompt := listeningSourceReviewPrompt(mockExamSpecs[2], 7.5, `{"title":"fixture","audioScript":"fixture"}`, true)
	for _, safeguard := range []string{
		"Complete all four checks even after finding a failure",
		"do not stop at the first defect",
		"every comparative and superlative",
		"explicit valid baseline or comparison set",
		"can genuinely vary along the stated time, location or group dimension",
		"unsupported seasonal or temporal comparison of an effectively static attribute",
		"instrument and procedure can identify the claimed endpoint",
		"operational definition",
		"permanent deformation",
		"not blinding or bias control unless the relevant identity is genuinely hidden",
		"every material defect found in that check",
	} {
		if !strings.Contains(prompt, safeguard) {
			t.Errorf("review prompt missing safeguard %q", safeguard)
		}
	}
}

func TestListeningSourceRevisionPromptRepairsAllDefectsAndReauditsWholeScript(t *testing.T) {
	diagnostic := `{"previousSource":"fixture","feedback":"two independent defects"}`
	prompt := listeningSourceRevisionPrompt(diagnostic)
	for _, safeguard := range []string{
		"Repair EVERY defect listed",
		"independently re-audit the COMPLETE revised script",
		"chronology, quantities, comparisons, inference and method validity",
		"Preserve the original topic, genre, setting, named speakers, target difficulty",
		"immediately preceding review MUST be REPAIRED",
		"later full independent review decides approval",
		"unsupported comparative or superlative",
		"unmeasurable endpoint",
		"false claim that coding/random labels provide blinding or bias control",
		"680-740 spoken English words",
		"790 as an absolute drafting ceiling",
		"ten distinct questions",
	} {
		if !strings.Contains(prompt, safeguard) {
			t.Errorf("revision prompt missing safeguard %q", safeguard)
		}
	}
	if !strings.Contains(prompt, diagnostic) {
		t.Fatal("revision prompt lost the untrusted diagnostic")
	}
}

func TestListeningSourceRevisionValidationRequiresCompleteVerbatimChecklist(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{name: "valid", valid: true},
		{name: "missing_check", mutate: func(v map[string]any) {
			delete(v["repairs"].(map[string]any), "quantities")
		}},
		{name: "extra_check", mutate: func(v map[string]any) {
			v["repairs"].(map[string]any)["language"] = map[string]string{"status": "UNCHANGED", "evidenceQuote": "fixture", "resolution": "This extra item must be rejected locally."}
		}},
		{name: "failed_dimension_unchanged", mutate: func(v map[string]any) {
			v["repairs"].(map[string]any)["quantities"].(map[string]string)["status"] = "UNCHANGED"
		}},
		{name: "quote_not_verbatim", mutate: func(v map[string]any) {
			v["repairs"].(map[string]any)["chronology"].(map[string]string)["evidenceQuote"] = "This sentence is absent from the revised source."
		}},
		{name: "empty_resolution", mutate: func(v map[string]any) {
			v["repairs"].(map[string]any)["chronology"].(map[string]string)["resolution"] = ""
		}},
		{name: "empty_title", mutate: func(v map[string]any) {
			v["title"] = ""
		}},
		{name: "empty_audio_script", mutate: func(v map[string]any) {
			v["audioScript"] = ""
		}},
		{name: "speaker_changed", mutate: func(v map[string]any) {
			audioScript := strings.ReplaceAll(v["audioScript"].(string), "Maya:", "Replacement Speaker:")
			v["audioScript"] = audioScript
			evidence := strings.TrimSpace(strings.Split(audioScript, "\n\n")[1])
			for _, name := range listeningSourceReviewDimensions {
				v["repairs"].(map[string]any)[name].(map[string]string)["evidenceQuote"] = evidence
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := listeningRevisionFixture(source, "quantities")
			if tc.mutate != nil {
				tc.mutate(value)
			}
			data, _ := json.Marshal(value)
			got, err := decodeAndValidateListeningSourceRevision(string(data), source, mockExamSpecs[2], map[string]bool{"quantities": true})
			if (err == nil) != tc.valid || (err == nil && got.AudioScript != source.AudioScript) {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
			if !tc.valid && !errors.Is(err, ErrListeningReviewInvalid) {
				t.Fatalf("invalid revision error = %v", err)
			}
		})
	}

	valid, _ := json.Marshal(listeningRevisionFixture(source, "quantities"))
	duplicate := strings.Replace(string(valid), `"quantities":`, `"quantities":{"status":"REPAIRED","evidenceQuote":"`+strings.TrimSpace(strings.Split(source.AudioScript, "\n")[0])+`","resolution":"This duplicate checklist entry must be rejected locally."},"quantities":`, 1)
	if _, err := decodeAndValidateListeningSourceRevision(duplicate, source, mockExamSpecs[2], map[string]bool{"quantities": true}); !errors.Is(err, ErrListeningReviewInvalid) {
		t.Fatalf("duplicate checklist key error = %v", err)
	}
}

func TestListeningSourceRevisionParserRecoversHarmlessEnvelopeAndAliases(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	value := listeningRevisionFixture(source, "quantities")
	value["audio_script"] = value["audioScript"]
	delete(value, "audioScript")
	value["checklist"] = value["repairs"]
	delete(value, "repairs")
	for _, name := range listeningSourceReviewDimensions {
		item := value["checklist"].(map[string]any)[name].(map[string]string)
		item["evidence_quote"] = item["evidenceQuote"]
		delete(item, "evidenceQuote")
	}
	data, _ := json.Marshal(value)
	content := "Here is the complete repair object:\n```json\n" + string(data) + "\n```\nNo fields were omitted."
	got, err := decodeAndValidateListeningSourceRevision(content, source, mockExamSpecs[2], map[string]bool{"quantities": true})
	if err != nil || got != source {
		t.Fatalf("wrapped aliased revision was not recovered: got=%q err=%v", got.Title, err)
	}
}

func TestListeningSourceRevisionParserStillRejectsAmbiguousOrIncompleteRecovery(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	valid, _ := json.Marshal(listeningRevisionFixture(source, "quantities"))
	missingEvidence := listeningRevisionFixture(source, "quantities")
	delete(missingEvidence["repairs"].(map[string]any)["chronology"].(map[string]string), "evidenceQuote")
	missingEvidenceJSON, _ := json.Marshal(missingEvidence)
	for _, content := range []string{
		string(valid) + "\n{}",
		string(missingEvidenceJSON),
		strings.Replace(string(valid), `"audioScript":`, `"audio_script":"duplicate","audioScript":`, 1),
		strings.Replace(string(valid), `"repairs":`, `"checklist":{},"repairs":`, 1),
	} {
		if _, err := decodeAndValidateListeningSourceRevision(content, source, mockExamSpecs[2], map[string]bool{"quantities": true}); !errors.Is(err, ErrListeningReviewInvalid) {
			t.Fatalf("unsafe recovery unexpectedly passed: %v", err)
		}
	}
}

func TestListeningSourceRevisionAcceptsEquivalentQuoteFormattingOnly(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	value := listeningRevisionFixture(source, "quantities")
	checks := value["repairs"].(map[string]any)
	original := checks["chronology"].(map[string]string)["evidenceQuote"]
	words := strings.Fields(original)
	if len(words) < 4 {
		t.Fatal("fixture evidence is too short")
	}
	// 模拟供应商调整引文中的空白。
	checks["chronology"].(map[string]string)["evidenceQuote"] = strings.Join(words, "  ")
	data, _ := json.Marshal(value)
	if _, err := decodeAndValidateListeningSourceRevision(string(data), source, mockExamSpecs[2], map[string]bool{"quantities": true}); err != nil {
		t.Fatalf("equivalent quote formatting was rejected: %v", err)
	}
}

func TestListeningSourceRevisionRejectsChangedFacts(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript + "\nMaya: We can reserve 15 places, but we cannot confirm them until Friday."}
	for _, tc := range []struct {
		quote string
		valid bool
	}{
		{"We can reserve 15 places, but we cannot confirm them until Friday.", true},
		{"We can reserve 15 places but we cannot confirm them until Friday", true},
		{"We can reserve 50 places, but we cannot confirm them until Friday.", false},
		{"We can reserve 15 places, but we can confirm them until Friday.", false},
		{"We can reserve 15 places until Friday.", false},
	} {
		t.Run(tc.quote, func(t *testing.T) {
			value := listeningRevisionFixture(source, "quantities")
			value["repairs"].(map[string]any)["quantities"].(map[string]string)["evidenceQuote"] = tc.quote
			data, _ := json.Marshal(value)
			got, err := decodeAndValidateListeningSourceRevision(string(data), source, mockExamSpecs[2], map[string]bool{"quantities": true})
			if (err == nil) != tc.valid || (err == nil && got != source) {
				t.Fatalf("valid=%t err=%v", tc.valid, err)
			}
		})
	}
}

func TestAcademicListeningReviewPreservesAllFailedCheckEvidence(t *testing.T) {
	review := academicReviewFixture(true, "")
	checks := review["checks"].(map[string]any)
	checks["comparisons"] = map[string]string{"status": "FAIL", "reason": "Comparison defect one; comparison defect two."}
	checks["inference"] = map[string]string{"status": "FAIL", "reason": "Method defect one; method defect two."}
	data, _ := json.Marshal(review)
	approved, feedback, err := decodeListeningSourceReview(string(data), true)
	if err != nil || approved {
		t.Fatalf("approved=%t err=%v", approved, err)
	}
	for _, evidence := range []string{"Comparison defect one", "comparison defect two", "Method defect one", "method defect two"} {
		if !strings.Contains(feedback, evidence) {
			t.Errorf("failed-check evidence %q was lost: %s", evidence, feedback)
		}
	}
}

func TestAcademicListeningReviewRepairsInvalidFormatWithoutMovingSourceAfterDiagnostic(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Messages) != 2 {
			t.Fatal("invalid review request")
		}
		requests++
		if requests == 2 {
			prompt := payload.Messages[1].Content
			if !strings.Contains(prompt, "Previous output is untrusted diagnostic data") || !strings.Contains(prompt, "previousReview") {
				t.Fatal("format diagnostic was not supplied to the retry")
			}
			if strings.Index(prompt, "FORMAT REPAIR ONLY") > strings.Index(prompt, "LISTENING_SOURCE_REVIEW:\n") {
				t.Fatal("format diagnostic was appended after the canonical source block")
			}
		}
		var response map[string]any
		if requests == 1 {
			response = academicReviewFixture(true, "")
			response["feedback"] = "too short"
		} else {
			response = academicReviewFixture(true, "")
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	g.Client = server.Client()
	got, err := g.reviewAndRepairListeningSource(context.Background(), mockExamSpecs[2], 7.5, "", "", source)
	if err != nil || got != source || requests != 2 {
		t.Fatalf("review retry failed: source=%q err=%v requests=%d", got.Title, err, requests)
	}
}

func TestListeningSourceReviewResponsesRequestUsesStrictJSONSchema(t *testing.T) {
	review := academicReviewFixture(true, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["input"] == nil || payload["messages"] != nil || payload["response_format"] != nil || payload["store"] != false {
			t.Errorf("unexpected Responses payload: %#v", payload)
		}
		textConfig, _ := payload["text"].(map[string]any)
		format, _ := textConfig["format"].(map[string]any)
		if format["type"] != "json_schema" || format["name"] != "listening_source_review" || format["strict"] != true || format["json_schema"] != nil {
			t.Errorf("unexpected Responses format: %#v", format)
		}
		schema, _ := format["schema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		checks, _ := properties["checks"].(map[string]any)
		checkProperties, _ := checks["properties"].(map[string]any)
		for _, name := range []string{"chronology", "quantities", "comparisons", "inference"} {
			if checkProperties[name] == nil {
				t.Errorf("schema missing %s check", name)
			}
		}
		content, _ := json.Marshal(review)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []any{map[string]any{
				"type":    "message",
				"content": []any{map[string]any{"type": "output_text", "text": string(content)}},
			}},
		})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL+"/v1/responses")
	g.Client = server.Client()
	approved, _, err := func() (bool, string, error) {
		content, callErr := g.listeningSourceReviewCompletion(context.Background(), "JSON only", "Review this harmless fixture.", true)
		if callErr != nil {
			return false, "", callErr
		}
		return decodeListeningSourceReview(content, true)
	}()
	if err != nil || !approved {
		t.Fatalf("approved=%t err=%v", approved, err)
	}
}

func TestListeningSourceRevisionResponsesRequestUsesStrictJSONSchema(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		textConfig, _ := payload["text"].(map[string]any)
		format, _ := textConfig["format"].(map[string]any)
		if format["type"] != "json_schema" || format["name"] != "listening_source_revision" || format["strict"] != true {
			t.Fatalf("unexpected revision format: %#v", format)
		}
		schema, _ := format["schema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		repairs, _ := properties["repairs"].(map[string]any)
		repairProperties, _ := repairs["properties"].(map[string]any)
		if properties["title"] == nil || properties["audioScript"] == nil || len(repairProperties) != 4 {
			t.Fatalf("incomplete revision schema: %#v", schema)
		}
		content, _ := json.Marshal(listeningRevisionFixture(source, "quantities"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []any{map[string]any{
				"type":    "message",
				"content": []any{map[string]any{"type": "output_text", "text": string(content)}},
			}},
		})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL+"/v1/responses")
	g.Client = server.Client()
	content, err := g.listeningSourceRevisionCompletion(context.Background(), "Repair this harmless fixture.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decodeAndValidateListeningSourceRevision(content, source, mockExamSpecs[2], map[string]bool{"quantities": true}); err != nil {
		t.Fatal(err)
	}
}

func TestListeningSourceReviewChatRequestRemainsBackwardCompatible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["response_format"] != nil || payload["messages"] == nil {
			t.Errorf("legacy Chat payload changed unexpectedly: %#v", payload)
		}
		content, _ := json.Marshal(map[string]any{"approved": false, "feedback": "The harmless fixture is insufficient."})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL+"/v1")
	g.Client = server.Client()
	content, err := g.listeningSourceReviewCompletion(context.Background(), "JSON only", "Review this harmless fixture.", false)
	if err != nil {
		t.Fatal(err)
	}
	approved, _, err := decodeListeningSourceReview(content, false)
	if err != nil || approved {
		t.Fatalf("approved=%t err=%v", approved, err)
	}
}

func TestAcademicListeningRevisionLoopIsBoundedAndRequiresFinalFullReview(t *testing.T) {
	for _, tc := range []struct {
		name          string
		approvalRound int
		wantReviews   int
		wantRevisions int
		wantSuccess   bool
	}{
		{name: "first_revision_succeeds", approvalRound: 2, wantReviews: 2, wantRevisions: 1, wantSuccess: true},
		{name: "second_revision_succeeds", approvalRound: 3, wantReviews: 3, wantRevisions: 2, wantSuccess: true},
		{name: "third_revision_succeeds", approvalRound: 4, wantReviews: 4, wantRevisions: 3, wantSuccess: true},
		{name: "third_revision_exhausted", approvalRound: 0, wantReviews: 4, wantRevisions: 3 + maxListeningSourceRevisionFormats, wantSuccess: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := mockExamFixtureSection(2, 7.5)
			source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
			reviews, revisions := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var response any
				if strings.Contains(string(body), "LISTENING_SOURCE_REVIEW:") {
					reviews++
					failed := map[int]string{1: "quantities", 2: "inference", 3: "comparisons", 4: "chronology"}[reviews]
					if reviews == tc.approvalRound {
						failed = ""
					}
					// 顶层错误地同意时，程序仍须尊重细项拒绝。
					response = academicReviewFixture(true, failed)
				} else {
					revisions++
					if !strings.Contains(string(body), "TASK-MIX BOUNDARY") || !strings.Contains(string(body), "Only 4 opportunities") {
						t.Error("revision lost the source difficulty allocation")
					}
					if !strings.Contains(string(body), "previousSource") || !strings.Contains(string(body), "review") {
						t.Error("revision lost the actual source or failed-check evidence")
					}
					failed := map[int]string{1: "quantities", 2: "inference", 3: "comparisons"}[revisions]
					response = listeningRevisionFixture(source, failed)
				}
				content, _ := json.Marshal(response)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			}))
			defer server.Close()
			g := NewOpenAIGenerator("test", "test", server.URL)
			_, err := g.reviewAndRepairListeningSource(context.Background(), mockExamSpecs[2], 7.5, mockExamSourcePrompt(mockExamSpecs[2], 7.5), "", source)
			if (err == nil) != tc.wantSuccess || reviews != tc.wantReviews || revisions != tc.wantRevisions {
				t.Fatalf("reviews=%d revisions=%d err=%v", reviews, revisions, err)
			}
		})
	}
}

func TestListeningSourceReviewQuotesAreNotSourcePlaceholders(t *testing.T) {
	for _, quote := range []string{"...", "…"} {
		text := "The comparison 'warmer " + quote + " faster' has no baseline and must be revised."
		review := academicReviewFixture(false, "comparisons")
		review["feedback"] = text
		data, _ := json.Marshal(review)
		approved, feedback, err := decodeListeningSourceReview(string(data), true)
		if err != nil || approved || !strings.Contains(feedback, text) {
			t.Fatalf("valid rejection must survive abbreviated quotation: approved=%v err=%v", approved, err)
		}
		if !mockExamPlaceholders.MatchString(text) {
			t.Fatal("candidate source placeholder gate must remain strict")
		}
	}
	for _, invalid := range []string{"...", "N/A", "too short", "insert text in this reason", "placeholder explanation goes here"} {
		if listeningSourceReviewReasonPresent(invalid) {
			t.Fatalf("empty or placeholder review accepted: %q", invalid)
		}
	}
}

func TestListeningSourceReviewKeepsMixedTaskBoundary(t *testing.T) {
	for _, index := range []int{2, 3} {
		for _, band := range []float64{6, 6.5, 7, 7.5, 8} {
			prompt := listeningSourceReviewPrompt(mockExamSpecs[index], band, `{}`, true)
			for _, required := range []string{"not ten advanced discourse chains", fmt.Sprintf("Only %d opportunities", listeningAuthoringAdvanced(band)), "final questions still require independent difficulty"} {
				if !strings.Contains(prompt, required) {
					t.Fatalf("part=%d band=%.1f missing %q", index+1, band, required)
				}
			}
		}
	}
}

func TestAcademicListeningDiscourseGuidanceRelaxesOnly65(t *testing.T) {
	relaxed := academicListeningDiscourseGuidance(6.5)
	if !strings.Contains(relaxed, "at least one such independent chain") || !strings.Contains(relaxed, "fewer than several chains") {
		t.Fatalf("6.5 academic guidance is not relaxed: %s", relaxed)
	}
	strict := academicListeningDiscourseGuidance(7)
	if !strings.Contains(strict, "Require several independent chains") || strings.Contains(strict, "at least one such independent chain") {
		t.Fatalf("7.0 academic guidance should remain strict: %s", strict)
	}
}

func TestListeningSourceRevisionExhaustionRegeneratesFreshSource(t *testing.T) {
	initial := mockExamFixtureSection(0, 6)
	first := mockExamSource{Title: initial.Title, AudioScript: initial.AudioScript}
	second := first
	second.AudioScript = strings.Replace(first.AudioScript, "researchers", "specialists", 1)
	sourceCalls, reviews, revisions := 0, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		var response any
		switch {
		case strings.Contains(text, "LISTENING_SOURCE_REVIEW:"):
			reviews++
			approved := reviews == 2
			response = map[string]any{"approved": approved, "feedback": "The complete source needs a repair before it can be frozen."}
		case strings.Contains(text, "SOURCE_ONLY stage."):
			sourceCalls++
			if sourceCalls == 1 {
				response = first
			} else {
				if !strings.Contains(text, "previousSource") {
					t.Error("replacement source was not explicitly marked as independent")
				}
				response = second
			}
		default:
			revisions++
			response = map[string]any{"malformed": true}
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	got, err := g.prepareListeningSource(context.Background(), mockExamSpecs[0], 6, "")
	if err != nil || got != second {
		t.Fatalf("err=%v source=%q", err, got.Title)
	}
	if sourceCalls != 2 || reviews != 2 || revisions != maxListeningSourceRevisionFormats {
		t.Fatalf("source/review/revision calls = %d/%d/%d", sourceCalls, reviews, revisions)
	}
}

func TestListeningSourceRevisionExhaustionRemainsBounded(t *testing.T) {
	initial := mockExamFixtureSection(0, 6)
	first := mockExamSource{Title: initial.Title, AudioScript: initial.AudioScript}
	second := first
	second.AudioScript = strings.Replace(first.AudioScript, "researchers", "specialists", 1)
	sourceCalls, reviews, revisions := 0, 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		var response any
		switch {
		case strings.Contains(text, "LISTENING_SOURCE_REVIEW:"):
			reviews++
			response = map[string]any{"approved": false, "feedback": "The complete source needs a repair before it can be frozen."}
		case strings.Contains(text, "SOURCE_ONLY stage."):
			sourceCalls++
			if sourceCalls == 1 {
				response = first
			} else {
				response = second
			}
		default:
			revisions++
			response = map[string]any{"malformed": true}
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	_, err := g.prepareListeningSource(context.Background(), mockExamSpecs[0], 6, "")
	if err == nil || !errors.Is(err, ErrListeningReviewInvalid) || errors.Is(err, ErrListeningReviewUnavailable) {
		t.Fatalf("unexpected bounded failure: %v", err)
	}
	wantRevisions := maxListeningSourceRevisionFormats * (maxListeningSourceReplacements + 1)
	if sourceCalls != maxListeningSourceReplacements+1 || reviews != maxListeningSourceReplacements+1 || revisions != wantRevisions {
		t.Fatalf("source/review/revision calls = %d/%d/%d, want %d/%d/%d", sourceCalls, reviews, revisions, maxListeningSourceReplacements+1, maxListeningSourceReplacements+1, wantRevisions)
	}
}

func TestListeningSourceReviewServiceFailureRetriesWithFreshSource(t *testing.T) {
	fixture := mockExamFixtureSection(3, 6.5)
	first := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	second := first
	second.AudioScript = strings.Replace(first.AudioScript, "researchers", "field specialists", 1)
	sourceCalls, reviewCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		text := string(body)
		var response any
		switch {
		case strings.Contains(text, "LISTENING_SOURCE_REVIEW:"):
			reviewCalls++
			if reviewCalls <= modelAPIMaxRetries+1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			response = academicReviewFixture(true, "")
		case strings.Contains(text, "SOURCE_ONLY stage."):
			sourceCalls++
			if sourceCalls == 1 {
				response = first
			} else {
				if !strings.Contains(text, "previousSource") {
					t.Error("service-failure replacement was not marked as independent")
				}
				response = second
			}
		default:
			response = map[string]any{"malformed": true}
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test-key", "test-model", server.URL)
	got, err := g.prepareListeningSource(context.Background(), mockExamSpecs[3], 6.5, "")
	if err != nil || got != second {
		t.Fatalf("err=%v source=%q", err, got.Title)
	}
	if sourceCalls != 2 || reviewCalls != modelAPIMaxRetries+2 {
		t.Fatalf("source/review calls = %d/%d, want 2/%d", sourceCalls, reviewCalls, modelAPIMaxRetries+2)
	}
}

func TestAcademicListeningInvalidRevisionStopsBeforeRereview(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	reviews, revisions := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var response any
		if strings.Contains(string(body), "LISTENING_SOURCE_REVIEW:") {
			reviews++
			response = academicReviewFixture(false, "quantities")
		} else {
			revisions++
			response = listeningRevisionFixture(source)
		}
		content, _ := json.Marshal(response)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test", "test", server.URL)
	_, err := g.reviewAndRepairListeningSource(context.Background(), mockExamSpecs[2], 7.5, "", "", source)
	if !errors.Is(err, ErrListeningReviewInvalid) || reviews != 1 || revisions != maxListeningSourceRevisionFormats {
		t.Fatalf("reviews=%d revisions=%d err=%v", reviews, revisions, err)
	}
}

func TestAcademicListeningRevisionTransportFailureRemainsUnavailable(t *testing.T) {
	fixture := mockExamFixtureSection(2, 7.5)
	source := mockExamSource{Title: fixture.Title, AudioScript: fixture.AudioScript}
	reviews, revisions := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "LISTENING_SOURCE_REVIEW:") {
			reviews++
			content, _ := json.Marshal(academicReviewFixture(false, "quantities"))
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
			return
		}
		revisions++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	g := NewOpenAIGenerator("test", "test", server.URL)
	_, err := g.reviewAndRepairListeningSource(context.Background(), mockExamSpecs[2], 7.5, "", "", source)
	if !errors.Is(err, ErrListeningReviewUnavailable) || reviews != 1 || revisions != 1 {
		t.Fatalf("reviews=%d revisions=%d err=%v", reviews, revisions, err)
	}
}
