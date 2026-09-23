package store

import (
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteMockExamPersistsActualStartAndPrivateScript(t *testing.T) {
	db, err := NewSQLiteStore(filepath.Join(t.TempDir(), "mock.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Truncate(time.Second)
	exam := domain.MockExam{ID: "exam", UserID: "owner", Exam: "IELTS", Status: "GENERATING", TotalDurationSeconds: 9000, CreatedAt: now, UpdatedAt: now, Sections: []domain.MockExamSection{{Key: "LISTENING_1", AudioScript: "Mia: Actual original source.", AudioURLs: []string{"/media/one.mp3", "/media/two.mp3"}, PaperVersion: "IELTS-Academic-v2", TargetBand: 8}}}
	if _, err = db.SaveMockExam(exam); err != nil {
		t.Fatal(err)
	}
	exam.Status = "IN_PROGRESS"
	exam.StartedAt = now.Add(5 * time.Minute)
	exam.Sections[0].Answers = []string{"A"}
	if _, err = db.UpdateMockExam(exam); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetMockExam("exam", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if !got.StartedAt.Equal(exam.StartedAt) || got.Sections[0].AudioScript != exam.Sections[0].AudioScript || len(got.Sections[0].AudioURLs) != 2 || got.Sections[0].Answers[0] != "A" || got.Sections[0].TargetBand != 8 {
		t.Fatalf("lost persisted state: %+v", got)
	}
	if _, err = db.GetMockExam("exam", "other"); err == nil {
		t.Fatal("foreign exam visible")
	}
	exam.Status = "COMPLETED"
	exam.Result = &domain.MockExamResult{WritingEvaluations: []domain.WritingEvaluation{{BandEstimate: 7, Evidence: []string{"verified essay quote"}, GrammarScore: 700.0 / 9}}}
	exam.ProductionApproval = domain.ProductionApproval{Status: contentquality.ApprovalApproved, Reviewer: "paper-reviewer"}
	exam.EvaluationApproval = domain.ProductionApproval{Status: contentquality.ApprovalApproved, Reviewer: "score-reviewer"}
	if _, err = db.UpdateMockExam(exam); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetMockExam("exam", "owner")
	if err != nil || got.Result == nil || len(got.Result.WritingEvaluations) != 1 || got.Result.WritingEvaluations[0].Evidence[0] != "verified essay quote" || got.ProductionApproval.Reviewer != "paper-reviewer" || got.EvaluationApproval.Reviewer != "score-reviewer" {
		t.Fatal("writing criterion report was not persisted")
	}
}
