package graph

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
)

func TestListeningTrainingProjectionAndOwnership(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := service.New(mem, nil, nil, nil, "test")
	schema, err := NewSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	paper := domain.MockExam{ID: "listen", UserID: "owner", Exam: "IELTS_LISTENING_PART_1", Sections: []domain.MockExamSection{{Title: "Training title", Key: "LISTENING_1", AudioScript: "PRIVATE_SCRIPT", AudioURLs: []string{"/media/PRIVATE_AUDIO"}, Questions: []domain.QuizQuestion{{Question: "Visible question", Type: "sentence_completion", AnswerKey: "PRIVATE_KEY", Evidence: "PRIVATE_EVIDENCE"}}}}}
	_, _ = mem.SaveMockExam(paper)
	query := `{ listeningTraining(id:"listen") { id transcript audioUrls questions { question answerKey evidence correct } } listeningTrainings { id transcript audioUrls questions { answerKey } } }`
	for _, status := range []string{"GENERATING", "READY", "IN_PROGRESS", "FAILED", "COMPLETED"} {
		paper.Status = status
		_, _ = mem.UpdateMockExam(paper)
		r := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, "owner")})
		if len(r.Errors) > 0 {
			t.Fatal(r.Errors)
		}
		raw, _ := json.Marshal(r.Data)
		text := string(raw)
		for _, private := range []string{"PRIVATE_SCRIPT", "PRIVATE_KEY", "PRIVATE_EVIDENCE"} {
			if strings.Contains(text, private) {
				t.Fatalf("%s projection leak/missing data: %s", status, text)
			}
		}
		// 这些旧记录没有难度审核，所有读接口均不应开放其材料。
		if status == "READY" || status == "IN_PROGRESS" || status == "COMPLETED" {
			if strings.Contains(text, "PRIVATE_AUDIO") {
				t.Fatal("unreviewed legacy content exposed")
			}
		}
		if (status == "GENERATING" || status == "READY" || status == "FAILED") && strings.Contains(text, "PRIVATE_AUDIO") {
			t.Fatal("prestart audio exposed")
		}
		if list, ok := r.Data.(map[string]interface{})["listeningTrainings"].([]interface{}); ok {
			raw, _ = json.Marshal(list)
			if strings.Contains(string(raw), "PRIVATE_") {
				t.Fatal("list exposed detailed content")
			}
		}
	}
	paper.Status = "COMPLETED"
	projection, _ := json.Marshal(listeningTrainingView(paper, false))
	if !strings.Contains(string(projection), "PRIVATE_KEY") {
		t.Fatal("completed projection missing answers")
	}
	for _, uid := range []string{"foreign", ""} {
		r := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.WithValue(context.Background(), UserIDKey, uid)})
		if len(r.Errors) == 0 {
			t.Fatal("unauthorized read allowed")
		}
	}
	for _, mutation := range []string{`startListeningTraining(part:1)`, `beginListeningTraining(id:"listen")`, `saveListeningAnswers(id:"listen",answers:[])`, `finishListeningTraining(id:"listen",answers:[])`} {
		r := graphql.Do(graphql.Params{Schema: schema, RequestString: "mutation { " + mutation + " { id } }", Context: context.Background()})
		if len(r.Errors) == 0 || !strings.Contains(r.Errors[0].Message, "登录") {
			t.Fatal("missing auth contract", r.Errors)
		}
	}
}
