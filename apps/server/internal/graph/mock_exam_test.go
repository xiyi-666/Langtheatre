package graph

import (
	"context"
	"encoding/json"
	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
	"os"
	"strings"
	"testing"
	"time"
)

func approveGraphMockExam(t *testing.T, exam *domain.MockExam) {
	t.Helper()
	checks := []domain.QualityCheck{
		{Key: "completeness", Status: contentquality.CheckPassed},
		{Key: "answer_integrity", Status: contentquality.CheckPassed},
		{Key: "difficulty", Status: contentquality.CheckPassed},
		{Key: "media", Status: contentquality.CheckPassed},
	}
	for i := range exam.Sections {
		hash, err := contentquality.MockExamSectionContentHash(exam.Sections[i])
		if err != nil {
			t.Fatal(err)
		}
		exam.Sections[i].ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, "controlled-graph-test-reviewer", hash, time.Now().UTC(), checks)
	}
	hash, err := contentquality.MockExamContentHash(*exam)
	if err != nil {
		t.Fatal(err)
	}
	exam.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, "controlled-graph-test-reviewer", hash, time.Now().UTC(), checks)
	if exam.Result != nil {
		evaluationHash, err := contentquality.MockExamEvaluationContentHash(*exam)
		if err != nil {
			t.Fatal(err)
		}
		evaluationChecks := []domain.QualityCheck{{Key: "input_integrity", Status: contentquality.CheckPassed}, {Key: "objective_scoring", Status: contentquality.CheckPassed}, {Key: "subjective_review", Status: contentquality.CheckPassed}, {Key: "score_consistency", Status: contentquality.CheckPassed}}
		exam.EvaluationApproval = contentquality.ApprovedApproval(contentquality.MockEvaluationRubricVersion, "controlled-graph-test-reviewer", evaluationHash, time.Now().UTC(), evaluationChecks)
	}
}

func TestMockExamGraphHidesSolutionsAndPreStartPaper(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := service.New(mem, nil, nil, nil, "test")
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	exam := domain.MockExam{ID: "exam", UserID: "owner", Status: "READY", Sections: []domain.MockExamSection{{Key: "READING_1", TargetBand: 7, PaperVersion: "IELTS-Academic-v2", Passage: "PRIVATE_SOURCE", AudioScript: "PRIVATE_SCRIPT", Questions: []domain.QuizQuestion{{Question: "Actual question", AnswerKey: "PRIVATE_KEY", Evidence: "PRIVATE_EVIDENCE"}}}}}
	approveGraphMockExam(t, &exam)
	_, _ = mem.SaveMockExam(exam)
	query := `query { mockExam(id:"exam") { id status targetBand paperVersion sections { passage questions { question options type } } } mockExamGenerationCost }`
	ctx := context.WithValue(context.Background(), UserIDKey, "owner")
	r := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctx})
	if len(r.Errors) > 0 {
		t.Fatal(r.Errors)
	}
	data, _ := json.Marshal(r.Data)
	if strings.Contains(string(data), "PRIVATE_") {
		t.Fatalf("pre-start source leaked: %s", data)
	}
	exam.Status = "IN_PROGRESS"
	_, _ = mem.UpdateMockExam(exam)
	r = graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: ctx})
	if len(r.Errors) > 0 {
		t.Fatal(r.Errors)
	}
	data, _ = json.Marshal(r.Data)
	if !strings.Contains(string(data), "PRIVATE_SOURCE") || strings.Contains(string(data), "PRIVATE_KEY") || strings.Contains(string(data), "PRIVATE_EVIDENCE") || strings.Contains(string(data), "PRIVATE_SCRIPT") {
		t.Fatalf("invalid active projection: %s", data)
	}
	r = graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "foreign")})
	if len(r.Errors) == 0 {
		t.Fatal("foreign user accessed paper")
	}
	r = graphql.Do(graphql.Params{Schema: schema, RequestString: `{ __type(name:"MockExamQuestion") { fields {name} } }`})
	data, _ = json.Marshal(r.Data)
	if strings.Contains(string(data), "answerKey") || strings.Contains(string(data), "evidence") {
		t.Fatal("solution fields exposed in active question schema")
	}
}

func TestMockExamGraphMutationContracts(t *testing.T) {
	schema, err := NewSchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ __type(name:"Mutation") {fields {name args {name}}} }`})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	data, _ := json.Marshal(result.Data)
	for _, name := range []string{"beginMockExam", "retryMockExam", "saveMockExamAnswers", "submitMockExamSection", "finishMockExam", "deleteMockExam", "targetBand"} {
		if !strings.Contains(string(data), name) {
			t.Errorf("missing %s", name)
		}
	}
}

func TestRetryMockExamGraphCreatesRedactedReadyAttempt(t *testing.T) {
	raw, err := os.ReadFile("../ai/testdata/mock_exam_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var source domain.MockExam
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	source.ID, source.UserID, source.Status, source.CurrentSection = "completed", "owner", "COMPLETED", "RESULT"
	source.Result = &domain.MockExamResult{EstimatedBand: 7}
	approveGraphMockExam(t, &source)
	mem := store.NewMemoryStore()
	if _, err = mem.SaveMockExam(source); err != nil {
		t.Fatal(err)
	}
	schema, err := NewSchema(service.New(mem, nil, nil, nil, "test"))
	if err != nil {
		t.Fatal(err)
	}
	request := `mutation { retryMockExam(examId:"completed") { id status sections { passage questions { question } answers responses } result { estimatedBand } } }`
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: request, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	data, _ := json.Marshal(result.Data)
	text := string(data)
	if !strings.Contains(text, `"status":"READY"`) || strings.Contains(text, `"id":"completed"`) || strings.Contains(text, `"estimatedBand":7`) {
		t.Fatalf("invalid retry response: %s", text)
	}
	if passage := source.Sections[0].Passage; passage != "" && strings.Contains(text, passage) {
		t.Fatalf("ready paper leaked passage: %s", text)
	}
}

func TestMockExamGraphDeleteOwnershipAndState(t *testing.T) {
	mem := store.NewMemoryStore()
	schema, err := NewSchema(service.New(mem, nil, nil, nil, "test"))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"GENERATING", "IN_PROGRESS", "EVALUATING", "READY", "COMPLETED", "FAILED", "EVALUATION_FAILED"} {
		t.Run(status, func(t *testing.T) {
			_, err := mem.SaveMockExam(domain.MockExam{ID: status, UserID: "owner", Exam: "CET4", Status: status})
			if err != nil {
				t.Fatal(err)
			}
			query := `mutation { deleteMockExam(examId:"` + status + `") }`
			for _, user := range []string{"", "other"} {
				r := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, user)})
				if len(r.Errors) == 0 {
					t.Fatal("unauthorized deletion succeeded")
				}
			}
			r := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
			active := status == "GENERATING" || status == "IN_PROGRESS" || status == "EVALUATING"
			if active && len(r.Errors) == 0 {
				t.Fatal("active job deleted")
			}
			if !active && len(r.Errors) != 0 {
				t.Fatal(r.Errors)
			}
			_, err = mem.GetMockExam(status, "owner")
			if active && err != nil || !active && err == nil {
				t.Fatal("unexpected persisted deletion state")
			}
		})
	}
}

func TestMockExamGraphReturnsWritingCriteria(t *testing.T) {
	mem := store.NewMemoryStore()
	exam := domain.MockExam{ID: "report", UserID: "owner", Status: "COMPLETED", Result: &domain.MockExamResult{WritingEvaluations: []domain.WritingEvaluation{{BandEstimate: 7, GrammarScore: 700.0 / 9, Evidence: []string{"essay evidence"}}}}}
	approveGraphMockExam(t, &exam)
	if _, err := mem.SaveMockExam(exam); err != nil {
		t.Fatal(err)
	}
	schema, err := NewSchema(service.New(mem, nil, nil, nil, "test"))
	if err != nil {
		t.Fatal(err)
	}
	r := graphql.Do(graphql.Params{Schema: schema, RequestString: `{mockExam(id:"report"){result{writingEvaluations{bandEstimate grammarScore evidence}}}}`, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	data, _ := json.Marshal(r.Data)
	if !strings.Contains(string(data), `"bandEstimate":7`) || !strings.Contains(string(data), "essay evidence") {
		t.Fatalf("writing report omitted: %s", data)
	}
}
