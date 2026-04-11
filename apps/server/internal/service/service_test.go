package service

import (
	"context"
	"testing"

	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/store"
)

type fakeGenerator struct{}

func (fakeGenerator) Generate(_ context.Context, _ string, _ string, _ float64, _ string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	return []domain.Dialogue{
		{Speaker: "阿明", Text: "欢迎光临茶餐厅", ZhSubtitle: "欢迎光临茶餐厅"},
		{Speaker: "小美", Text: "我要一杯奶茶", ZhSubtitle: "我要一杯奶茶"},
	}, []domain.QuizQuestion{
		{Question: "小美点了什么？", Options: []string{"奶茶", "咖啡", "柠檬茶", "热水"}, AnswerKey: "奶茶"},
		{Question: "场景在哪里？", Options: []string{"图书馆", "茶餐厅", "地铁站", "学校"}, AnswerKey: "茶餐厅"},
	}, nil
}

func TestRegisterLoginAndGenerateFlow(t *testing.T) {
	svc := New(store.NewMemoryStore(), nil, fakeGenerator{}, nil, "unit-test-secret")
	token, err := svc.Register("qa@linguaquest.app", "password123")
	if err != nil || token == "" {
		t.Fatalf("register failed: %v", err)
	}
	loginToken, err := svc.Login("qa@linguaquest.app", "password123")
	if err != nil || loginToken == "" {
		t.Fatalf("login failed: %v", err)
	}
	user, err := svc.MeFromToken(loginToken)
	if err != nil {
		t.Fatalf("parse token failed: %v", err)
	}
	theater, err := svc.GenerateTheater(user.ID, "CANTONESE", "茶餐厅对话", 5.5, "ROLEPLAY")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	result, err := svc.SubmitAnswers(user.ID, theater.ID, []string{"奶茶", "茶餐厅"})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if result.Score <= 0 {
		t.Fatalf("expected positive score, got %d", result.Score)
	}
	if err = svc.ToggleFavorite(user.ID, theater.ID, true); err != nil {
		t.Fatalf("toggle favorite failed: %v", err)
	}
	list, err := svc.MyTheaters(user.ID, "CANTONESE", "READY", nil)
	if err != nil || len(list) == 0 {
		t.Fatalf("my theaters failed: %v", err)
	}
	session, err := svc.StartRoleplay(user.ID, theater.ID, "Learner")
	if err != nil {
		t.Fatalf("start roleplay failed: %v", err)
	}
	session, err = svc.SubmitRoleplayReply(user.ID, session.ID, "我想先点一杯奶茶。")
	if err != nil || session.TurnIndex == 0 {
		t.Fatalf("submit roleplay failed: %v", err)
	}
	session, err = svc.EndRoleplay(user.ID, session.ID)
	if err != nil || session.Status != "completed" {
		t.Fatalf("end roleplay failed: %v", err)
	}
	courses, err := svc.ListCourses("CANTONESE")
	if err != nil || len(courses) == 0 {
		t.Fatalf("list courses failed: %v", err)
	}
}
