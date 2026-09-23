package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linguaquest/server/internal/ai"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/questionbank"
	"github.com/linguaquest/server/internal/store"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type speakingTestEngine struct {
	mockExamTestEngine
	calls atomic.Int32
	block chan struct{}
}

type validSpeakingTestEngine struct{ speakingTestEngine }

type speakingReviewSequenceEngine struct {
	speakingTestEngine
	mu                  sync.Mutex
	reviewErrors        []error
	reviewRejectedIndex map[int]int
	fingerprints        []string
	promptIDs           [][]string
}

func (e *speakingReviewSequenceEngine) ReviewSpeakingPrompts(ctx context.Context, prompts []domain.SpeakingPrompt) ([]domain.ProductionApproval, error) {
	e.mu.Lock()
	call := len(e.fingerprints)
	e.fingerprints = append(e.fingerprints, questionbank.SpeakingSelectionFingerprint(prompts))
	ids := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		ids = append(ids, prompt.QuestionID)
	}
	e.promptIDs = append(e.promptIDs, ids)
	var reviewErr error
	if call < len(e.reviewErrors) {
		reviewErr = e.reviewErrors[call]
	}
	e.mu.Unlock()
	if reviewErr != nil {
		if rejectedIndex, ok := e.reviewRejectedIndex[call]; ok {
			approvals, err := e.speakingTestEngine.ReviewSpeakingPrompts(ctx, prompts)
			if err != nil {
				return nil, err
			}
			approvals[rejectedIndex].Status = contentquality.ApprovalRejected
			approvals[rejectedIndex].Checks[1].Status = contentquality.CheckFailed
			return approvals, reviewErr
		}
		return nil, reviewErr
	}
	return e.speakingTestEngine.ReviewSpeakingPrompts(ctx, prompts)
}

func (e *speakingReviewSequenceEngine) reviewFingerprints() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.fingerprints...)
}

func (e *speakingReviewSequenceEngine) reviewedPromptIDs() [][]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([][]string, len(e.promptIDs))
	for i := range e.promptIDs {
		result[i] = append([]string(nil), e.promptIDs[i]...)
	}
	return result
}

func approveSpeakingPromptsForTest(t *testing.T, prompts []domain.SpeakingPrompt) {
	t.Helper()
	for i := range prompts {
		if prompts[i].AudioURL == "" {
			prompts[i].AudioURL = "/media/test-speaking-prompt.mp3"
		}
	}
	approvals, err := (&speakingTestEngine{}).ReviewSpeakingPrompts(context.Background(), prompts)
	if err != nil {
		t.Fatal(err)
	}
	for i := range prompts {
		prompts[i].ProductionApproval = &approvals[i]
	}
}

func (e *speakingTestEngine) ReviewSpeakingPrompts(_ context.Context, prompts []domain.SpeakingPrompt) ([]domain.ProductionApproval, error) {
	checks := []domain.QualityCheck{{Key: "source", Status: contentquality.CheckPassed}, {Key: "semantic_quality", Status: contentquality.CheckPassed}, {Key: "part_alignment", Status: contentquality.CheckPassed}, {Key: "audio", Status: contentquality.CheckPassed}}
	approvals := make([]domain.ProductionApproval, len(prompts))
	for i := range prompts {
		hash, err := contentquality.SpeakingPromptContentHash(prompts[i])
		if err != nil {
			return nil, err
		}
		approvals[i] = contentquality.ApprovedApproval(contentquality.SpeakingPromptRubricVersion, "controlled-test-reviewer", hash, time.Now().UTC(), checks)
	}
	return approvals, nil
}

func (e *speakingTestEngine) ReviewSpeakingEvaluation(_ context.Context, session domain.SpeakingSession) (domain.ProductionApproval, error) {
	hash, err := contentquality.SpeakingEvaluationContentHash(session)
	if err != nil {
		return domain.ProductionApproval{}, err
	}
	checks := []domain.QualityCheck{{Key: "completeness", Status: contentquality.CheckPassed}, {Key: "evidence", Status: contentquality.CheckPassed}, {Key: "band_consistency", Status: contentquality.CheckPassed}, {Key: "audio_basis", Status: contentquality.CheckPassed}}
	return contentquality.ApprovedApproval(contentquality.SpeakingEvaluationRubricVersion, "controlled-test-reviewer", hash, time.Now().UTC(), checks), nil
}

func (e *validSpeakingTestEngine) EvaluateSpeaking(context.Context, []domain.SpeakingPrompt, []domain.SpeakingTurn) (domain.SpeakingEvaluation, error) {
	score := 6.5
	return domain.SpeakingEvaluation{TextCoherence: &score, LexicalResource: &score, GrammarAccuracy: &score, IsPartial: true, AssessmentMode: "TEXT_ONLY", Summary: "表达清楚，需要具体事例支持。", Strengths: []string{"回答表达清楚。"}, Improvements: []string{"补充学习语言的具体经历。"}, Evidence: []string{"I enjoy learning languages", "because they connect people", "I enjoy learning languages because they connect people"}}, nil
}

func (e *speakingTestEngine) NextSpeakingPrompt(ctx context.Context, session domain.SpeakingSession, _ domain.SpeakingTurn) (domain.SpeakingExaminerReply, error) {
	e.calls.Add(1)
	if e.block != nil {
		select {
		case <-e.block:
		case <-ctx.Done():
			return domain.SpeakingExaminerReply{}, ctx.Err()
		}
	}
	text := "Why do you enjoy learning new languages?"
	return domain.SpeakingExaminerReply{Text: text, Question: text}, nil
}
func (e *speakingTestEngine) EvaluateSpeaking(context.Context, []domain.SpeakingPrompt, []domain.SpeakingTurn) (domain.SpeakingEvaluation, error) {
	score := 6.5
	return domain.SpeakingEvaluation{TextCoherence: &score, LexicalResource: &score, GrammarAccuracy: &score, Pronunciation: &score, OverallBand: &score, IsPartial: true, AssessmentMode: "TEXT_ONLY", Summary: "表达清楚，需要具体事例支持。", Strengths: []string{"回答表达清楚。"}, Improvements: []string{"补充学习语言的具体经历。"}, Evidence: []string{"I enjoy learning languages", "because they connect people", "I enjoy learning languages because they connect people"}}, nil
}

type speakingTestTTS struct{ fail atomic.Bool }

func (s *speakingTestTTS) Synthesize(context.Context, string, string, string) (string, error) {
	if s.fail.Load() {
		return "", errors.New("PRIVATE_PROVIDER_ERROR")
	}
	return "https://example.invalid/test-fixture.mp3", nil
}

type speakingTestASR struct{ fail atomic.Bool }

func (s *speakingTestASR) Transcribe(context.Context, string, string) (domain.TranscriptResult, error) {
	if s.fail.Load() {
		return domain.TranscriptResult{}, errors.New("PRIVATE_ASR_ERROR")
	}
	return domain.TranscriptResult{Text: "I enjoy learning languages because they connect people."}, nil
}

func waitSpeaking(t *testing.T, s *Service, id, status string) domain.SpeakingSession {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		session, err := s.GetSpeakingSession("owner", id)
		if err != nil {
			t.Fatal(err)
		}
		if session.Status == status {
			return session
		}
		time.Sleep(5 * time.Millisecond)
	}
	session, _ := s.GetSpeakingSession("owner", id)
	t.Fatalf("expected %s got %+v", status, session)
	return domain.SpeakingSession{}
}

func writeSpeakingSelectionBank(t *testing.T) string {
	t.Helper()
	bank := questionbank.Bank{Version: 1}
	for _, topic := range []string{"home", "study", "travel"} {
		for question := 1; question <= 3; question++ {
			id := fmt.Sprintf("%s-%d", topic, question)
			bank.Items = append(bank.Items, questionbank.Item{ID: id, Part: 1, Topic: topic, Question: fmt.Sprintf("What do you think about %s question %d?", topic, question), Source: "fixture.pdf", SourcePage: 1, AudioURL: "/media/" + id + ".mp3"})
		}
	}
	for _, group := range []string{"work", "technology", "environment"} {
		bank.Items = append(bank.Items, questionbank.Item{ID: group + "-card", Part: 2, Group: group, CueCard: "Describe an experience. You should say: what happened, where it happened, and why it mattered.", PreparationSec: 60, AnswerSec: 120, Source: "fixture.pdf", SourcePage: 2, AudioURL: "/media/" + group + "-card.mp3"})
		for question := 1; question <= 3; question++ {
			id := fmt.Sprintf("%s-follow-%d", group, question)
			bank.Items = append(bank.Items, questionbank.Item{ID: id, Part: 3, Group: group, Question: fmt.Sprintf("How could %s change in the future, from perspective %d?", group, question), Source: "fixture.pdf", SourcePage: 3, AudioURL: "/media/" + id + ".mp3"})
		}
	}
	return writeSpeakingBank(t, bank)
}

func writeSpeakingBank(t *testing.T, bank questionbank.Bank) string {
	t.Helper()
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(mediaDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, item := range bank.Items {
		if !strings.HasPrefix(item.AudioURL, "/media/") || strings.ContainsAny(item.AudioURL, `\?#`) {
			continue
		}
		relative := strings.TrimPrefix(item.AudioURL, "/media/")
		if relative == "" || strings.Contains(relative, "..") {
			continue
		}
		if err := os.WriteFile(filepath.Join(mediaDir, filepath.FromSlash(relative)), []byte("fixture audio"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(bank)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "speaking.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartSpeakingSessionRetriesOnlyRejectedSelections(t *testing.T) {
	rejected := fmt.Errorf("controlled rejection: %w", ai.ErrProductionReviewRejected)
	tests := []struct {
		name          string
		reviewErrors  []error
		wantCalls     int
		wantErrorText string
	}{
		{name: "second selection passes", reviewErrors: []error{rejected}, wantCalls: 2},
		{name: "three rejected selections stop", reviewErrors: []error{rejected, rejected, rejected}, wantCalls: 3, wantErrorText: "未通过质量审核"},
		{name: "inconclusive review does not retry", reviewErrors: []error{fmt.Errorf("controlled timeout: %w", ai.ErrProductionReviewInconclusive)}, wantCalls: 1, wantErrorText: "审核服务暂时不可用"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bankPath := writeSpeakingSelectionBank(t)
			t.Setenv("SPEAKING_BANK_PATH", bankPath)
			engine := &speakingReviewSequenceEngine{reviewErrors: test.reviewErrors}
			svc := NewWithOptions(store.NewMemoryStore(), nil, engine, &speakingTestTTS{}, "test", Options{MediaDir: filepath.Join(filepath.Dir(bankPath), "media")})
			session, err := svc.StartSpeakingSession("owner")
			if test.wantErrorText == "" {
				if err != nil {
					t.Fatal(err)
				}
				if session.Status != "ACTIVE" || len(session.Prompts) == 0 {
					t.Fatalf("unexpected session: %+v", session)
				}
				for index, prompt := range session.Prompts {
					if prompt.ProductionApproval == nil || prompt.ProductionApproval.Status != contentquality.ApprovalApproved {
						t.Fatalf("prompt %d was not approved", index+1)
					}
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantErrorText) {
				t.Fatalf("expected %q, got %v", test.wantErrorText, err)
			}
			fingerprints := engine.reviewFingerprints()
			if len(fingerprints) != test.wantCalls {
				t.Fatalf("expected %d reviews, got %d", test.wantCalls, len(fingerprints))
			}
			seen := map[string]bool{}
			for _, fingerprint := range fingerprints {
				if fingerprint == "" || seen[fingerprint] {
					t.Fatalf("selection was empty or repeated: %q", fingerprint)
				}
				seen[fingerprint] = true
			}
		})
	}
}

func TestStartSpeakingSessionExcludesItemsFailedByRejectedReview(t *testing.T) {
	bankPath := writeSpeakingSelectionBank(t)
	t.Setenv("SPEAKING_BANK_PATH", bankPath)
	rejected := fmt.Errorf("one prompt failed: %w", ai.ErrProductionReviewRejected)
	engine := &speakingReviewSequenceEngine{
		reviewErrors:        []error{rejected},
		reviewRejectedIndex: map[int]int{0: 0},
	}
	svc := NewWithOptions(store.NewMemoryStore(), nil, engine, &speakingTestTTS{}, "test", Options{MediaDir: filepath.Join(filepath.Dir(bankPath), "media")})
	if _, err := svc.StartSpeakingSession("owner"); err != nil {
		t.Fatal(err)
	}
	ids := engine.reviewedPromptIDs()
	if len(ids) != 2 {
		t.Fatalf("expected two full selections, got %d", len(ids))
	}
	rejectedID := ids[0][0]
	for _, id := range ids[1] {
		if id == rejectedID {
			t.Fatalf("rejected item %q was selected again", rejectedID)
		}
	}
}

func TestStartSpeakingSessionFiltersIncompleteAndUnsafeAudioGroups(t *testing.T) {
	bank := questionbank.Bank{Version: 1}
	for _, topic := range []string{"home", "study", "travel"} {
		for question := 1; question <= 3; question++ {
			id := fmt.Sprintf("%s-%d", topic, question)
			bank.Items = append(bank.Items, questionbank.Item{ID: id, Part: 1, Topic: topic, Question: fmt.Sprintf("What do you think about %s question %d?", topic, question), Source: "fixture.pdf", SourcePage: 1, AudioURL: "/media/" + id + ".mp3"})
		}
	}
	for _, group := range []string{"work", "technology"} {
		bank.Items = append(bank.Items, questionbank.Item{ID: group + "-card", Part: 2, Group: group, CueCard: "Describe an experience. You should say: what happened, where it happened, and why it mattered.", PreparationSec: 60, AnswerSec: 120, Source: "fixture.pdf", SourcePage: 2, AudioURL: "/media/" + group + "-card.mp3"})
		for question := 1; question <= 3; question++ {
			id := fmt.Sprintf("%s-follow-%d", group, question)
			bank.Items = append(bank.Items, questionbank.Item{ID: id, Part: 3, Group: group, Question: fmt.Sprintf("How could %s change in the future, from perspective %d?", group, question), Source: "fixture.pdf", SourcePage: 3, AudioURL: "/media/" + id + ".mp3"})
		}
	}
	bank.Items[0].AudioURL = "/media/../outside.mp3"
	bankPath := writeSpeakingBank(t, bank)
	mediaDir := filepath.Join(filepath.Dir(bankPath), "media")
	if err := os.Remove(filepath.Join(mediaDir, "home-2.mp3")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPEAKING_BANK_PATH", bankPath)
	svc := NewWithOptions(store.NewMemoryStore(), nil, &speakingReviewSequenceEngine{}, &speakingTestTTS{}, "test", Options{MediaDir: mediaDir})
	if _, err := svc.StartSpeakingSession("owner"); err == nil || !strings.Contains(err.Error(), "题库读取或校验失败") {
		t.Fatalf("expected unsafe bank rejection, got %v", err)
	}

	bank.Items[0].AudioURL = "/media/home-1.mp3"
	bankPath = writeSpeakingBank(t, bank)
	mediaDir = filepath.Join(filepath.Dir(bankPath), "media")
	if err := os.Remove(filepath.Join(mediaDir, "home-2.mp3")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPEAKING_BANK_PATH", bankPath)
	engine := &speakingReviewSequenceEngine{}
	svc = NewWithOptions(store.NewMemoryStore(), nil, engine, &speakingTestTTS{}, "test", Options{MediaDir: mediaDir})
	session, err := svc.StartSpeakingSession("owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, prompt := range session.Prompts {
		if strings.HasPrefix(prompt.QuestionID, "home-") {
			t.Fatalf("selected item from incomplete topic: %s", prompt.QuestionID)
		}
	}
}

func TestSpeakingAudioFileRequiresNonEmptyContainedRegularFile(t *testing.T) {
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "valid.mp3"), []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	if !speakingAudioFilePlayable(mediaDir, "/media/valid.mp3") {
		t.Fatal("valid audio was rejected")
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "empty.mp3"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if speakingAudioFilePlayable(mediaDir, "/media/empty.mp3") {
		t.Fatal("empty audio was accepted")
	}
	if err := os.Mkdir(filepath.Join(mediaDir, "directory.mp3"), 0700); err != nil {
		t.Fatal(err)
	}
	if speakingAudioFilePlayable(mediaDir, "/media/directory.mp3") {
		t.Fatal("directory was accepted as audio")
	}

	outside := filepath.Join(t.TempDir(), "outside.mp3")
	if err := os.WriteFile(outside, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(mediaDir, "outside.mp3")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if speakingAudioFilePlayable(mediaDir, "/media/outside.mp3") {
		t.Fatal("audio symlink escaped media directory")
	}
}

func TestSpeakingAtomicTurnRetryAndRealTextEvaluation(t *testing.T) {
	for _, kind := range []string{"memory", "sqlite"} {
		t.Run(kind, func(t *testing.T) {
			var mem Store = store.NewMemoryStore()
			if kind == "sqlite" {
				db, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "speaking.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				mem = db
			}
			engine := &speakingTestEngine{block: make(chan struct{})}
			tts := &speakingTestTTS{}
			asr := &speakingTestASR{}
			svc := New(mem, nil, engine, tts, "test")
			svc.asr = asr
			original := domain.SpeakingSession{ID: "test-speaking", UserID: "owner", Status: "ACTIVE", Prompts: []domain.SpeakingPrompt{{Part: 1, Question: "What do you enjoy learning?"}, {Part: 1, Question: "Which skills matter to you?"}}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			approveSpeakingPromptsForTest(t, original.Prompts)
			if _, err := mem.CreateSpeakingSession(original); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.SubmitSpeakingTurn("other", original.ID, 0, "", "hello", "ENGLISH"); err == nil {
				t.Fatal("foreign submission accepted")
			}
			tts.fail.Store(true)
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := svc.SubmitSpeakingTurn("owner", original.ID, 0, "test-audio", "", "ENGLISH"); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			inflight, err := svc.GetSpeakingSession("owner", original.ID)
			if err != nil {
				t.Fatal(err)
			}
			if inflight.PromptIndex != 0 || len(inflight.Turns) != 0 {
				t.Fatal("turn advanced before TTS success")
			}
			close(engine.block)
			failed := waitSpeaking(t, svc, original.ID, "TURN_FAILED")
			if failed.PromptIndex != 0 || len(failed.Turns) != 0 || strings.Contains(failed.LastError, "PRIVATE") {
				t.Fatal("failure lost atomicity or leaked provider error")
			}
			if engine.calls.Load() != 1 {
				t.Fatalf("duplicate examiner calls %d", engine.calls.Load())
			}
			tts.fail.Store(false)
			if _, err = svc.SubmitSpeakingTurn("owner", original.ID, 0, "test-audio", "", "ENGLISH"); err != nil {
				t.Fatal(err)
			}
			active := waitSpeaking(t, svc, original.ID, "ACTIVE")
			if active.PromptIndex != 1 || len(active.Turns) != 1 || active.Prompts[1].AudioURL == "" {
				t.Fatal("next question or audio missing")
			}
			if _, err = svc.SubmitSpeakingTurn("owner", original.ID, 0, "test-audio", "", "ENGLISH"); err != nil {
				t.Fatal("idempotent retry failed", err)
			}
			if _, err = svc.SubmitSpeakingTurn("owner", original.ID, 1, "", "I enjoy learning languages because they connect people.", "ENGLISH"); err != nil {
				t.Fatal(err)
			}
			waitSpeaking(t, svc, original.ID, "READY_TO_FINISH")
			if _, err = svc.FinishSpeakingSession("owner", original.ID); err != nil {
				t.Fatal(err)
			}
			failedEvaluation := waitSpeaking(t, svc, original.ID, "EVALUATION_FAILED")
			if failedEvaluation.Evaluation != nil || failedEvaluation.LastError == "" || strings.Contains(failedEvaluation.LastError, "PRIVATE") {
				t.Fatal("invalid model evaluation exposed or provider detail leaked")
			}
		})
	}
}

func TestSpeakingASRFailureAndStaleSession(t *testing.T) {
	mem := store.NewMemoryStore()
	engine := &speakingTestEngine{}
	svc := New(mem, nil, engine, &speakingTestTTS{}, "test")
	asr := &speakingTestASR{}
	asr.fail.Store(true)
	svc.asr = asr
	session := domain.SpeakingSession{ID: "asr-failure", UserID: "owner", Status: "ACTIVE", Prompts: []domain.SpeakingPrompt{{Part: 1, Question: "What do you enjoy?"}}, UpdatedAt: time.Now()}
	approveSpeakingPromptsForTest(t, session.Prompts)
	mem.CreateSpeakingSession(session)
	if _, err := svc.SubmitSpeakingTurn("owner", session.ID, 0, "test-audio", "do not use this fallback text", "ENGLISH"); err != nil {
		t.Fatal(err)
	}
	failed := waitSpeaking(t, svc, session.ID, "TURN_FAILED")
	if len(failed.Turns) != 0 || engine.calls.Load() != 0 {
		t.Fatal("ASR failure fell back to text")
	}
	failed.Status = "PROCESSING"
	failed.UpdatedAt = time.Now().Add(-time.Hour)
	mem.UpdateSpeakingSession(failed)
	recovered, err := svc.GetSpeakingSession("owner", session.ID)
	if err != nil || recovered.Status != "TURN_FAILED" {
		t.Fatal("stale task remains stuck", err)
	}
}

func TestSpeakingValidTextOnlyEvaluationCompletes(t *testing.T) {
	mem := store.NewMemoryStore()
	engine := &validSpeakingTestEngine{}
	svc := New(mem, nil, engine, &speakingTestTTS{}, "test")
	session := domain.SpeakingSession{ID: "valid-evaluation", UserID: "owner", Status: "READY_TO_FINISH", PromptIndex: 1, Prompts: []domain.SpeakingPrompt{{Part: 1, Question: "What do you enjoy?"}}, Turns: []domain.SpeakingTurn{{Part: 1, PromptIndex: 0, Transcript: "I enjoy learning languages because they connect people. Last summer I joined a local conversation club and met students from several countries. We practised together every weekend, which helped me explain my ideas more clearly."}}, UpdatedAt: time.Now()}
	approveSpeakingPromptsForTest(t, session.Prompts)
	if _, err := mem.CreateSpeakingSession(session); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinishSpeakingSession("owner", session.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitSpeaking(t, svc, session.ID, "COMPLETED")
	if completed.Evaluation == nil || completed.Evaluation.AssessmentMode != "TEXT_ONLY" || completed.Evaluation.Pronunciation != nil || completed.Evaluation.OverallBand != nil {
		t.Fatalf("valid text-only evaluation not stored: %+v", completed.Evaluation)
	}
}

func TestAbandonSpeakingSessionRemovesItFromRecovery(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, &speakingTestEngine{}, &speakingTestTTS{}, "test")
	session := domain.SpeakingSession{ID: "abandon-speaking", UserID: "owner", Status: "ACTIVE", UpdatedAt: time.Now().UTC()}
	if _, err := mem.CreateSpeakingSession(session); err != nil {
		t.Fatal(err)
	}
	if err := svc.AbandonSpeakingSession("other", session.ID); err == nil {
		t.Fatal("foreign user abandoned speaking session")
	}
	if err := svc.AbandonSpeakingSession("owner", session.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := mem.GetSpeakingSession(session.ID, "owner")
	if err != nil || stored.Status != "ABANDONED" {
		t.Fatalf("abandoned session not persisted: %+v err=%v", stored, err)
	}
	latest, err := svc.LatestSpeakingSession("owner")
	if err != nil || latest != nil {
		t.Fatalf("abandoned session remained recoverable: %+v err=%v", latest, err)
	}
}
