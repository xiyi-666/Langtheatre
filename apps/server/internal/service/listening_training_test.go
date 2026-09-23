package service

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/store"
)

func loadListeningContractFixture(t *testing.T) domain.MockExam {
	fixture := loadMockContractFixture(t)
	for i := 0; i < 4; i++ {
		s := &fixture.Sections[i]
		review := domain.ListeningDifficultyReview{Approved: true, Confidence: "HIGH", Feedback: "Controlled test audit for verifying the persistence and lifecycle contracts."}
		for j, q := range s.Questions {
			item := domain.ListeningDifficultyItem{Number: j + 1, Level: "STANDARD", Answer: q.AnswerKey, Evidence: q.Evidence, Reason: "Controlled test reasoning used only for storage and lifecycle regression.", Mechanisms: []string{"paraphrase"}}
			if j < 3 {
				item.Level, item.Mechanisms = "ADVANCED", []string{"correction", "constraint"}
			}
			review.Items = append(review.Items, item)
		}
		s.ListeningDifficulty = &domain.ListeningDifficultyAudit{RubricVersion: ai.ListeningDifficultyRubricVersion, ReviewerModel: "controlled-test-reviewer", ContentHash: ai.ListeningContentHash(*s), ReviewedAt: time.Now().UTC(), Review: review}
	}
	return fixture
}

type listeningTestEngine struct {
	mockExamTestEngine
	fail bool
}

func (e *listeningTestEngine) GenerateListeningTraining(ctx context.Context, part int, band float64) (domain.MockExamSection, error) {
	if e.fail {
		return domain.MockExamSection{}, errors.New("test provider failed")
	}
	paper, err := e.GenerateMockExam(ctx, band)
	if err != nil {
		return domain.MockExamSection{}, err
	}
	return paper.Sections[part-1], nil
}

func TestListeningTrainingGenerationFailuresAndReuse(t *testing.T) {
	fixture := loadListeningContractFixture(t)
	block := make(chan struct{})
	engine := &listeningTestEngine{mockExamTestEngine: mockExamTestEngine{paper: &fixture, block: block}}
	svc := New(store.NewMemoryStore(), nil, engine, &mockPlayableTTS{}, "test")
	created, err := svc.StartListeningTraining("owner", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := svc.StartListeningTraining("owner", 1, 7)
	if err != nil || repeated.ID != created.ID {
		t.Fatal("duplicate generation", err)
	}
	if _, err := svc.StartListeningTraining("owner", 2, 7); err == nil {
		t.Fatal("parallel task bypassed limit")
	}
	if err := svc.DeleteListeningTraining("owner", created.ID); err == nil {
		t.Fatal("deleted active generation")
	}
	close(block)
	ready := awaitMockState(t, svc, created.ID, "READY")
	if len(ready.Sections) != 1 || len(ready.Sections[0].AudioURLs) == 0 {
		t.Fatal("listening source/audio not ready")
	}
	if items, err := svc.MockExams("owner"); err != nil || len(items) != 0 {
		t.Fatal("training leaked into full mock list")
	}
	if items, err := svc.ListeningTrainings("owner"); err != nil || len(items) != 1 {
		t.Fatal("training missing")
	}
	for _, bad := range []float64{5.5, 8.5, 6.2, math.NaN(), math.Inf(1)} {
		if _, err := svc.StartListeningTraining("owner", 1, bad); err == nil {
			t.Fatal("invalid band accepted")
		}
	}
	for _, part := range []int{0, 5} {
		if _, err := svc.StartListeningTraining("owner", part, 7); err == nil {
			t.Fatal("invalid part accepted")
		}
	}
	for _, audioFail := range []bool{false, true} {
		failing := &listeningTestEngine{mockExamTestEngine: mockExamTestEngine{paper: &fixture}, fail: !audioFail}
		service := New(store.NewMemoryStore(), nil, failing, &mockPlayableTTS{fail: audioFail}, "test")
		failed, err := service.StartListeningTraining("owner", 1, 7)
		if err != nil {
			t.Fatal(err)
		}
		result := awaitMockState(t, service, failed.ID, "FAILED")
		if result.Result != nil {
			t.Fatal("failure produced score")
		}
		if _, err := service.BeginListeningTraining("owner", result.ID); err == nil {
			t.Fatal("failed training started")
		}
	}
	action, cost := mockGenerationCharge(created)
	if action != listeningGenerationAction || cost != aiCreditAmount(AICreditActionTheaterGeneration) {
		t.Fatal("wrong refund action or amount")
	}
}

func TestListeningTrainingLifecycleAndOwnership(t *testing.T) {
	for _, backend := range []string{"memory", "sqlite"} {
		t.Run(backend, func(t *testing.T) {
			var storage Store = store.NewMemoryStore()
			if backend == "sqlite" {
				db, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "listening.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = db.Close() })
				storage = db
			}
			user, err := storage.CreateUser("listener", "listener@example.test", "unused", true)
			if err != nil {
				t.Fatal(err)
			}
			svc := New(storage, nil, nil, nil, "test")
			fixture := loadListeningContractFixture(t)
			now := time.Now().UTC()
			paper := domain.MockExam{ID: "listening", UserID: user.ID, Exam: "IELTS_LISTENING_PART_1", Status: "READY", CreatedAt: now, UpdatedAt: now, TotalDurationSeconds: 450, Sections: []domain.MockExamSection{fixture.Sections[0]}}
			paper.Sections[0].AudioURL = "/media/test.mp3"
			paper.Sections[0].AudioURLs = []string{"/media/test.mp3"}
			if err := approveReviewedListeningTraining(&paper); err != nil {
				t.Fatal(err)
			}
			if _, err := storage.SaveMockExam(paper); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.BeginListeningTraining("foreign", paper.ID); err == nil {
				t.Fatal("foreign begin allowed")
			}
			if err := svc.DeleteListeningTraining("foreign", paper.ID); err == nil {
				t.Fatal("foreign deletion allowed")
			}
			started, err := svc.BeginListeningTraining(user.ID, paper.ID)
			if err != nil {
				t.Fatal(err)
			}
			started.StartedAt = now.Add(-24 * time.Hour)
			if _, err := storage.UpdateMockExam(started); err != nil {
				t.Fatal(err)
			}
			answers := make([]string, 10)
			for i, q := range started.Sections[0].Questions {
				answers[i] = strings.ToUpper(q.AnswerKey)
			}
			answers[0] = ""
			if _, err := svc.SaveListeningAnswers(user.ID, paper.ID, answers); err != nil {
				t.Fatal("practice wrongly expired", err)
			}
			if _, err := svc.FinishListeningTraining("foreign", paper.ID, answers); err == nil {
				t.Fatal("foreign submission")
			}
			if _, err := svc.FinishListeningTraining(user.ID, paper.ID, answers[:9]); err == nil {
				t.Fatal("truncated submission")
			}
			completed, err := svc.FinishListeningTraining(user.ID, paper.ID, answers)
			if err != nil {
				t.Fatal(err)
			}
			if completed.Result.ListeningCorrect != 9 || completed.Result.ListeningTotal != 10 || completed.Result.TotalScore != 90 || completed.Result.EstimatedBand != 0 {
				t.Fatalf("invalid result %+v", completed.Result)
			}
			if len(completed.Result.Recommendations) == 0 {
				t.Fatal("missing review guidance")
			}
			if _, err := svc.FinishListeningTraining(user.ID, paper.ID, make([]string, 10)); err != nil {
				t.Fatal(err)
			}
			profile, err := storage.GetUserByID(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if profile.TotalXP != learningXPAmount("LISTENING_PRACTICE", 90) {
				t.Fatalf("duplicate or missing XP: %d", profile.TotalXP)
			}
			if _, err := svc.SaveListeningAnswers(user.ID, paper.ID, answers); err == nil {
				t.Fatal("completed answers editable")
			}
			if _, err := svc.BeginListeningTraining(user.ID, paper.ID); err == nil {
				t.Fatal("completed restarted to view answers")
			}
			stored, err := storage.GetMockExam(paper.ID, user.ID)
			if err != nil || stored.Result.TotalScore != 90 || stored.Sections[0].AudioScript == "" {
				t.Fatal("result/source lost")
			}
			if err := svc.DeleteListeningTraining(user.ID, paper.ID); err != nil {
				t.Fatal(err)
			}
			// 被新版审核隔离的进行中记录也可以删除，普通整卷仍不能删除进行中考试。
			stored.ID, stored.Status, stored.Result = "legacy-active", "IN_PROGRESS", nil
			stored.Sections[0].ListeningDifficulty = nil
			if _, err := storage.SaveMockExam(stored); err != nil {
				t.Fatal(err)
			}
			if err := svc.DeleteListeningTraining(user.ID, stored.ID); err != nil {
				t.Fatal("cannot delete quarantined active training", err)
			}
			stored.ID, stored.Exam = "full-active", "IELTS"
			if _, err := storage.SaveMockExam(stored); err != nil {
				t.Fatal(err)
			}
			if err := storage.DeleteMockExam(stored.ID, user.ID); err == nil {
				t.Fatal("generic full exam deletion restriction lost")
			}
			if _, err := svc.ListeningTraining(user.ID, paper.ID); err == nil {
				t.Fatal("deleted training found")
			}
		})
	}
}

func TestListeningETADoesNotMixParts(t *testing.T) {
	now := time.Now().UTC()
	current := domain.MockExam{ID: "current", Exam: "IELTS_LISTENING_PART_2", Status: "GENERATING", CreatedAt: now}
	history := []domain.MockExam{{ID: "other", Exam: "IELTS_LISTENING_PART_1", Status: "READY", Sections: []domain.MockExamSection{{GenerationDurationSeconds: 120}}}}
	if _, n := estimateMockReadySeconds(history, current); n != 0 {
		t.Fatal("ETA mixed different Parts")
	}
	history[0].Exam = current.Exam
	if seconds, n := estimateMockReadySeconds(history, current); n != 1 || seconds <= 0 {
		t.Fatal("ETA ignored matching sample")
	}
}

type countingListeningTTS struct{ calls atomic.Int32 }

func (tts *countingListeningTTS) Synthesize(context.Context, string, string, string) (string, error) {
	tts.calls.Add(1)
	return "", errors.New("TTS must not run before difficulty approval")
}

func TestListeningDifficultyRejectsBeforeAudioAndRetainsAudit(t *testing.T) {
	fixture := loadListeningContractFixture(t)
	fixture.Sections[0].ListeningDifficulty.Review.Confidence = "LOW"
	storage := store.NewMemoryStore()
	tts := &countingListeningTTS{}
	svc := New(storage, nil, &listeningTestEngine{mockExamTestEngine: mockExamTestEngine{paper: &fixture}}, tts, "test")
	created, err := svc.StartListeningTraining("owner", 1, 7)
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitMockState(t, svc, created.ID, "FAILED")
	if tts.calls.Load() != 0 || failed.Result != nil || !strings.Contains(failed.CurrentSection, "难度审核") {
		t.Fatal("failed content was released or failure reason missing")
	}
	stored, err := storage.GetMockExam(created.ID, "owner")
	if err != nil || stored.Sections[0].ListeningDifficulty == nil || stored.Sections[0].ListeningDifficulty.Review.Confidence != "LOW" {
		t.Fatal("failed audit evidence lost")
	}
}

func TestListeningLegacyAndModifiedContentCannotBypassGate(t *testing.T) {
	for _, state := range []string{"READY", "IN_PROGRESS", "COMPLETED"} {
		for _, legacy := range []bool{true, false} {
			storage := store.NewMemoryStore()
			svc := New(storage, nil, nil, nil, "test")
			fixture := loadListeningContractFixture(t)
			section := fixture.Sections[0]
			if legacy {
				section.ListeningDifficulty = nil
			} else {
				section.Questions[0].Question += " Changed?"
			}
			paper := domain.MockExam{ID: "quarantine", UserID: "owner", Exam: "IELTS_LISTENING_PART_1", Status: state, Sections: []domain.MockExamSection{section}, StartedAt: time.Now(), Result: &domain.MockExamResult{TotalScore: 100}}
			if _, err := storage.SaveMockExam(paper); err != nil {
				t.Fatal(err)
			}
			view, err := svc.ListeningTraining("owner", paper.ID)
			if err != nil || view.Status != "QUALITY_REVIEW_PENDING" || view.Result != nil || len(view.Sections) != 0 {
				t.Fatal("unreviewed record visible as approved")
			}
			if _, err = svc.BeginListeningTraining("owner", paper.ID); err == nil {
				t.Fatal("begin bypass")
			}
			if _, err = svc.SaveListeningAnswers("owner", paper.ID, make([]string, 10)); err == nil {
				t.Fatal("save bypass")
			}
			if _, err = svc.FinishListeningTraining("owner", paper.ID, make([]string, 10)); err == nil {
				t.Fatal("finish bypass")
			}
			if _, err = svc.BeginMockExam("owner", paper.ID); err == nil {
				t.Fatal("generic API bypass")
			}
			stored, err := storage.GetMockExam(paper.ID, "owner")
			if err != nil || stored.Status != state {
				t.Fatal("quarantine unexpectedly erased historical record")
			}
		}
	}
}

func TestListeningFailureDistinguishesSystemFromDifficulty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cause   error
		message string
	}{
		{"difficulty", ai.ErrListeningDifficulty, "未通过目标难度审核"},
		{"timeout", context.DeadlineExceeded, "尚未完成难度检验"},
		{"unavailable", ai.ErrListeningReviewUnavailable, "尚未完成难度检验"},
		{"invalid", ai.ErrListeningReviewInvalid, "尚未完成难度检验"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := store.NewMemoryStore()
			svc := New(storage, nil, nil, nil, "test")
			paper := domain.MockExam{ID: "failure-kind", UserID: "owner", Exam: "IELTS_LISTENING_PART_1", Status: "GENERATING", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if _, err := storage.SaveMockExam(paper); err != nil {
				t.Fatal(err)
			}
			svc.failMockExam(paper, "FAILED", tc.cause)
			got, err := storage.GetMockExam(paper.ID, paper.UserID)
			if err != nil || got.Status != "FAILED" || got.Result != nil || !strings.Contains(got.CurrentSection, tc.message) || !strings.Contains(got.CurrentSection, paper.ID) {
				t.Fatalf("incorrect Chinese failure classification: %+v, %v", got, err)
			}
		})
	}
}
