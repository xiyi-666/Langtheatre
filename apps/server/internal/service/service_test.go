package service

import (
	"testing"

	"github.com/linguaquest/server/internal/store"
)

func TestRegisterLoginAndGenerateFlow(t *testing.T) {
	svc := New(store.NewMemoryStore(), nil, nil, nil, nil, "unit-test-secret")
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
	theater, err := svc.GenerateTheater(user.ID, "CANTONESE", "茶餐厅对话", 5.5, "LISTENING")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	result, err := svc.SubmitAnswers(user.ID, theater.ID, []string{"茶餐厅对话", "緊張"})
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
