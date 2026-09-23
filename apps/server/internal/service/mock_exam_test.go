package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/store"
)

type mockExamTestEngine struct {
	calls      atomic.Int32
	block      chan struct{}
	scoreError error
	bands      [2]float64
	paper      *domain.MockExam
}

func (e *mockExamTestEngine) Generate(context.Context, string, string, float64, string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	return nil, nil, errors.New("unused")
}
func (e *mockExamTestEngine) GenerateMockExam(ctx context.Context, _ float64) (domain.MockExam, error) {
	e.calls.Add(1)
	if e.block != nil {
		select {
		case <-e.block:
		case <-ctx.Done():
			return domain.MockExam{}, ctx.Err()
		}
	}
	if e.paper != nil {
		return cloneMockExam(*e.paper), nil
	}
	return domain.MockExam{}, errors.New("provider unavailable")
}
func (e *mockExamTestEngine) GenerateWritingPrompt(context.Context, string) (domain.WritingPrompt, error) {
	return domain.WritingPrompt{}, nil
}
func (e *mockExamTestEngine) EvaluateWriting(_ context.Context, _ string, p domain.WritingPrompt, _ string, _ int, _ int) (domain.WritingEvaluation, error) {
	if e.scoreError != nil {
		return domain.WritingEvaluation{}, e.scoreError
	}
	i := 0
	if strings.HasPrefix(p.Title, "Task 2") {
		i = 1
	}
	b := e.bands[i]
	return domain.WritingEvaluation{BandEstimate: b, OverallScore: b * 100 / 9}, nil
}

func (e *mockExamTestEngine) ReviewMockExamEvaluation(_ context.Context, exam domain.MockExam) (domain.ProductionApproval, error) {
	hash, err := contentquality.MockExamEvaluationContentHash(exam)
	if err != nil {
		return domain.ProductionApproval{}, err
	}
	checks := []domain.QualityCheck{{Key: "input_integrity", Status: contentquality.CheckPassed}, {Key: "objective_scoring", Status: contentquality.CheckPassed}, {Key: "subjective_review", Status: contentquality.CheckPassed}, {Key: "score_consistency", Status: contentquality.CheckPassed}}
	return contentquality.ApprovedApproval(contentquality.MockEvaluationRubricVersion, "controlled-test-reviewer", hash, time.Now().UTC(), checks), nil
}

func TestMockReferenceBandsSixThroughEight(t *testing.T) {
	for _, fn := range []func(int, int) float64{ieltsAcademicReadingBand, ieltsListeningBand} {
		for _, tc := range []struct {
			raw  int
			want float64
		}{{23, 6}, {30, 7}, {35, 8}, {39, 9}, {40, 9}, {0, 0}} {
			if got := fn(tc.raw, 40); got != tc.want {
				t.Fatalf("raw %d got %v want %v", tc.raw, got, tc.want)
			}
		}
		previous := 0.0
		for raw := 0; raw <= 40; raw++ {
			got := fn(raw, 40)
			if got < previous || got > 9 {
				t.Fatalf("non-monotonic table at %d", raw)
			}
			previous = got
		}
		if fn(30, 30) != 0 || fn(-1, 40) != 0 || fn(41, 40) != 0 {
			t.Fatal("invalid totals must not be converted")
		}
	}
}

func TestMockAnswersRequireExactSupportedAnswer(t *testing.T) {
	for _, tc := range []struct {
		answer, key, kind string
		want              bool
	}{
		{"A", "A", "multiple_choice", true}, {"a", "A", "multiple_choice", true}, {"A or B", "A", "multiple_choice", false},
		{"public", "public transport", "sentence_completion", false}, {"public transport system", "public transport", "sentence_completion", false},
		{" PUBLIC  transport ", "public transport", "sentence_completion", true}, {"NOT", "NOT GIVEN", "true_false_not_given", false}, {"not given", "NOT GIVEN", "true_false_not_given", true},
		{"", "", "sentence_completion", false},
	} {
		if got := mockAnswerMatches(tc.answer, domain.QuizQuestion{AnswerKey: tc.key, Type: tc.kind}); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
}

func mockScoringPaper() domain.MockExam {
	sections := []domain.MockExamSection{}
	for _, skill := range []string{"READING", "LISTENING"} {
		s := domain.MockExamSection{Skill: skill}
		for i := 0; i < 40; i++ {
			s.Questions = append(s.Questions, domain.QuizQuestion{Type: "multiple_choice", AnswerKey: "B"})
			s.Answers = append(s.Answers, "B")
		}
		sections = append(sections, s)
	}
	sections = append(sections, domain.MockExamSection{Skill: "WRITING", WritingPrompts: []domain.WritingPrompt{{Title: "Task 1"}, {Title: "Task 2"}}, Responses: []string{"submitted essay one", "submitted essay two"}})
	return domain.MockExam{Exam: "IELTS", Sections: sections}
}

func TestMockWritingWeightsAndFailure(t *testing.T) {
	engine := &mockExamTestEngine{}
	svc := New(store.NewMemoryStore(), nil, engine, nil, "test")
	for _, tc := range []struct {
		bands [2]float64
		want  float64
	}{{[2]float64{6, 9}, 8}, {[2]float64{9, 6}, 7}, {[2]float64{6, 6}, 6}, {[2]float64{7, 7}, 7}, {[2]float64{8, 8}, 8}, {[2]float64{0, 0}, 0}} {
		engine.bands = tc.bands
		paper, err := svc.evaluateMockExam(context.Background(), mockScoringPaper())
		if err != nil || paper.Result.WritingBand != tc.want {
			t.Fatalf("bands %v: paper=%+v err=%v", tc.bands, paper.Result, err)
		}
		if paper.Result.QualityStatus != "TRAINING_ESTIMATE_UNCALIBRATED" {
			t.Fatal("must not claim official calibrated band")
		}
		if len(paper.Result.WritingEvaluations) != 2 || paper.Result.WritingEvaluations[0].BandEstimate != tc.bands[0] || paper.Result.WritingEvaluations[1].BandEstimate != tc.bands[1] {
			t.Fatal("per-task evaluation lost from report")
		}
	}
	engine.scoreError = errors.New("model failed")
	if paper, err := svc.evaluateMockExam(context.Background(), mockScoringPaper()); err == nil || paper.Result != nil {
		t.Fatal("failed model must not produce a score")
	}
	engine.scoreError = nil
	engine.bands = [2]float64{math.NaN(), 8}
	if _, err := svc.evaluateMockExam(context.Background(), mockScoringPaper()); err == nil {
		t.Fatal("NaN band accepted")
	}
	paper := mockScoringPaper()
	paper.Sections[2].Responses = nil
	result, err := svc.evaluateMockExam(context.Background(), paper)
	if err != nil || result.Result.WritingBand != 0 || len(result.Result.Weaknesses) != 2 {
		t.Fatal("no attempt must be explicitly recorded as zero")
	}
}

func TestMockGenerationDeduplicatesAndFailsClosed(t *testing.T) {
	engine := &mockExamTestEngine{block: make(chan struct{})}
	svc := New(store.NewMemoryStore(), nil, engine, &sequenceTTS{}, "test")
	first, err := svc.StartMockExam("user", "IELTS", 7)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.StartMockExam("user", "IELTS", 7)
	if err != nil || first.ID != second.ID || first.Status != "GENERATING" || !first.StartedAt.IsZero() {
		t.Fatalf("duplicate or prematurely started: %v %+v", err, second)
	}
	close(engine.block)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		item, err := svc.MockExam("user", first.ID)
		if err != nil {
			t.Fatal(err)
		}
		if item.Status == "FAILED" {
			if item.Result != nil || engine.calls.Load() != 1 {
				t.Fatal("fake result or duplicate generation")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("failed generation never recorded")
}

func TestRetryCompletedMockExamCreatesIndependentAttempts(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, nil, nil, "test")
	source := loadMockContractFixture(t)
	source.ID = "completed-exam"
	source.UserID = "owner"
	source.Status = "COMPLETED"
	source.CurrentSection = "RESULT"
	source.StartedAt = time.Now().UTC().Add(-2 * time.Hour)
	source.SubmittedAt = time.Now().UTC().Add(-time.Hour)
	source.Result = &domain.MockExamResult{EstimatedBand: 7, Feedback: "original result"}
	for index := range source.Sections {
		source.Sections[index].Answers = []string{"original answer"}
		source.Sections[index].Responses = []string{"original response"}
	}
	if _, err := mem.SaveMockExam(source); err != nil {
		t.Fatal(err)
	}

	first, err := svc.RetryMockExam("owner", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.RetryMockExam("owner", source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == source.ID || second.ID == source.ID || first.ID == second.ID {
		t.Fatal("retry attempts did not receive independent ids")
	}
	for _, attempt := range []domain.MockExam{first, second} {
		if attempt.Status != "READY" || attempt.CurrentSection != attempt.Sections[0].Key || attempt.Result != nil || !attempt.StartedAt.IsZero() || !attempt.SubmittedAt.IsZero() {
			t.Fatalf("retry retained completion state: %+v", attempt)
		}
		if err = ensureMockExamProductionApproved(attempt); err != nil {
			t.Fatalf("retry lost paper approval: %v", err)
		}
		for _, section := range attempt.Sections {
			if len(section.Answers) != 0 || len(section.Responses) != 0 || section.GenerationDurationSeconds != 0 {
				t.Fatal("retry retained answers or generation timing")
			}
		}
	}
	stored, err := mem.GetMockExam(source.ID, "owner")
	if err != nil || stored.Status != "COMPLETED" || stored.Result == nil || stored.Result.Feedback != "original result" {
		t.Fatalf("source result was modified: %v %+v", err, stored)
	}
	if _, err = svc.RetryMockExam("other", source.ID); err == nil {
		t.Fatal("foreign user retried completed exam")
	}
	if _, err = svc.RetryMockExam("owner", first.ID); err == nil {
		t.Fatal("non-completed exam was retried")
	}
}

func TestMockSaveOwnershipDeadlineAndProgression(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, nil, nil, "test")
	paper := loadMockContractFixture(t)
	paper.ID, paper.UserID, paper.Status = "exam", "owner", "IN_PROGRESS"
	paper.CurrentSection, paper.StartedAt = paper.Sections[0].Key, time.Now()
	approveMockContractFixture(t, &paper)
	if _, err := mem.SaveMockExam(paper); err != nil {
		t.Fatal(err)
	}
	first, second := paper.Sections[0], paper.Sections[1]
	firstAnswers := make([]string, len(first.Questions))
	secondAnswers := make([]string, len(second.Questions))
	if _, err := svc.SaveMockExamAnswers("other", "exam", first.Key, firstAnswers, nil); err == nil {
		t.Fatal("foreign user allowed")
	}
	if _, err := svc.SaveMockExamAnswers("owner", "exam", second.Key, secondAnswers, nil); err == nil {
		t.Fatal("future section allowed")
	}
	saved, err := svc.SaveMockExamAnswers("owner", "exam", first.Key, firstAnswers, nil)
	if err != nil || saved.CurrentSection != first.Key {
		t.Fatal("autosave advanced section")
	}
	saved, err = svc.SubmitMockExamSection("owner", "exam", first.Key, firstAnswers, nil)
	if err != nil || saved.CurrentSection != second.Key {
		t.Fatal("submit did not advance")
	}
	fetched, _ := svc.MockExam("owner", "exam")
	fetched.Sections[0].Answers[0] = "tampered"
	stored, _ := svc.MockExam("owner", "exam")
	if stored.Sections[0].Answers[0] != firstAnswers[0] {
		t.Fatal("reader mutated persisted answer")
	}
	saved.StartedAt = time.Now().Add(-3 * time.Hour)
	_, _ = mem.UpdateMockExam(saved)
	if _, err = svc.SaveMockExamAnswers("owner", "exam", second.Key, secondAnswers, nil); err == nil {
		t.Fatal("expired exam accepted new answers")
	}
}

func TestMockRejectsUnreviewedPaperAndInvalidTarget(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, &mockExamTestEngine{}, &sequenceTTS{}, "test")
	for _, target := range []float64{5.5, 8.5, 6.1, math.NaN(), math.Inf(1)} {
		if _, err := svc.StartMockExam("user", "IELTS", target); err == nil {
			t.Fatalf("accepted %v", target)
		}
	}
	_, _ = mem.SaveMockExam(domain.MockExam{ID: "old", UserID: "user", Status: "READY"})
	if _, err := svc.BeginMockExam("user", "old"); err == nil {
		t.Fatal("unreviewed paper started")
	}
	if _, err := svc.FinishMockExam("user", "old"); err == nil {
		t.Fatal("unstarted paper scored")
	}
}

func TestMockListeningTurnsStripNamesPreserveSpeech(t *testing.T) {
	turns, err := mockListeningTurns("Maya: Welcome to the new centre.\nThe afternoon tour starts at three.\nOwen: Thanks. Is booking required?\nMaya: Please book at reception.")
	if err != nil || len(turns) != 3 {
		t.Fatalf("invalid turns: %v %+v", err, turns)
	}
	if turns[0].Speaker != "Maya" || turns[0].Text != "Welcome to the new centre.\nThe afternoon tour starts at three." {
		t.Fatal("speech was lost or label not stripped")
	}
	styles := assignDialogueVoiceStyles(turns, [2]string{"御姐音色", "沉稳大叔"}, nil)
	if styles[0] == styles[1] || styles[0] != styles[2] {
		t.Fatal("distinct/stable speaker voices lost")
	}
	for _, script := range []string{"", "Unlabelled opening\nMaya: Hello", "Maya:"} {
		if _, err = mockListeningTurns(script); err == nil {
			t.Fatalf("invalid script accepted: %q", script)
		}
	}
}

func TestAssignListeningVoiceStylesCapsRecordingAtTwoVoices(t *testing.T) {
	turns := []domain.Dialogue{
		{Speaker: "Narrator"},
		{Speaker: "Maya"},
		{Speaker: "Owen"},
		{Speaker: "Maya"},
		{Speaker: "Narrator"},
	}
	styles := assignListeningVoiceStyles(turns, [2]string{"female", "male"})
	if len(styles) != len(turns) || styles[0] != styles[4] || styles[1] != styles[3] {
		t.Fatalf("listening speaker voices are not stable: %#v", styles)
	}
	used := map[string]bool{}
	for _, style := range styles {
		used[style] = true
	}
	if len(used) != 2 {
		t.Fatalf("listening used %d voices, want exactly two: %#v", len(used), styles)
	}
}

func loadMockContractFixture(t *testing.T) domain.MockExam {
	t.Helper()
	data, err := os.ReadFile("../ai/testdata/mock_exam_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	var paper domain.MockExam
	if err = json.Unmarshal(data, &paper); err != nil {
		t.Fatal(err)
	}
	approveMockContractFixture(t, &paper)
	return paper
}

func approveMockContractFixture(t *testing.T, paper *domain.MockExam) {
	t.Helper()
	now := time.Now().UTC()
	checks := []domain.QualityCheck{
		{Key: "completeness", Status: contentquality.CheckPassed},
		{Key: "answer_integrity", Status: contentquality.CheckPassed},
		{Key: "difficulty", Status: contentquality.CheckPassed},
		{Key: "media", Status: contentquality.CheckPassed},
	}
	for i := range paper.Sections {
		hash, err := contentquality.MockExamSectionContentHash(paper.Sections[i])
		if err != nil {
			t.Fatal(err)
		}
		paper.Sections[i].ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, "controlled-test-reviewer", hash, now, checks)
	}
	hash, err := contentquality.MockExamContentHash(*paper)
	if err != nil {
		t.Fatal(err)
	}
	paper.ProductionApproval = contentquality.ApprovedApproval(contentquality.MockExamRubricVersion, "controlled-test-reviewer", hash, now, checks)
}

func awaitMockState(t *testing.T, svc *Service, id, status string) domain.MockExam {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		paper, err := svc.MockExam("owner", id)
		if err != nil {
			t.Fatal(err)
		}
		if paper.Status == status {
			return paper
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, _ := svc.MockExam("owner", id)
	t.Fatalf("want %s got %s (%s)", status, got.Status, got.CurrentSection)
	return domain.MockExam{}
}

type mockPlayableTTS struct{ fail bool }

func (s *mockPlayableTTS) Synthesize(context.Context, string, string, string) (string, error) {
	if s.fail {
		return "", errors.New("audio service failed")
	}
	return "/media/test-only-fixture.mp3", nil
}

type orderedParallelMockTTS struct {
	mu      sync.Mutex
	active  int
	maximum int
}

func (s *orderedParallelMockTTS) Synthesize(ctx context.Context, text string, _ string, _ string) (string, error) {
	s.mu.Lock()
	s.active++
	if s.active > s.maximum {
		s.maximum = s.active
	}
	s.mu.Unlock()
	delay := 5 * time.Millisecond
	if strings.HasPrefix(text, "First") {
		delay = 35 * time.Millisecond
	} else if strings.HasPrefix(text, "Second") {
		delay = 15 * time.Millisecond
	}
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	return "/media/" + strings.ToLower(strings.Fields(text)[0]) + ".mp3", nil
}

func TestMockListeningAudioRunsConcurrentlyAndPreservesOrder(t *testing.T) {
	tts := &orderedParallelMockTTS{}
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, tts, "test", Options{TTSMaxConcurrency: 3})
	section := domain.MockExamSection{
		Key:         "LISTENING_1",
		Title:       "Parallel audio",
		AudioScript: "Maya: First response is deliberately slow.\nOwen: Second response is medium.\nMaya: Third response is fast.",
	}
	urls, err := svc.mockListeningAudio(context.Background(), "exam", section)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/media/first.mp3", "/media/second.mp3", "/media/third.mp3"}
	if len(urls) != len(want) {
		t.Fatalf("got %d urls want %d", len(urls), len(want))
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Fatalf("audio order changed at %d: got %q want %q", i, urls[i], want[i])
		}
	}
	tts.mu.Lock()
	maximum := tts.maximum
	tts.mu.Unlock()
	if maximum < 2 {
		t.Fatalf("speaker turns remained serial; max concurrency=%d", maximum)
	}
}

func TestMockExamFailureMessagesIdentifyStage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  string
		cause   error
		kind    string
		message string
	}{
		{"paper review", "FAILED", errors.New("READING_2 failed after 3 generation attempts: independent review rejected"), "PAPER_REVIEW", "试卷内容生成或独立质量审核未通过"},
		{"audio generation", "FAILED", errors.New("LISTENING_1 audio: speaker turn 2 TTS: upstream failed"), "AUDIO_GENERATION", "听力音频生成或保存失败"},
		{"audio review", "FAILED", errors.New("production review gate: listening media missing"), "AUDIO_REVIEW", "听力音频生产审核未通过"},
		{"timeout", "FAILED", context.DeadlineExceeded, "TIMEOUT", "处理超时"},
		{"evaluation", "EVALUATION_FAILED", errors.New("assessment failed"), "EVALUATION", "评分服务或评分校验失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kind := mockExamFailureKind(tc.status, tc.cause)
			if kind != tc.kind {
				t.Fatalf("got kind %q want %q", kind, tc.kind)
			}
			message := mockExamFailureMessage("record-id", tc.status, kind)
			if !strings.Contains(message, tc.message) || !strings.Contains(message, "record-id") {
				t.Fatalf("unexpected message %q", message)
			}
		})
	}
}

func TestMockFullLifecycleAndAssessmentRetry(t *testing.T) {
	fixture := loadMockContractFixture(t)
	engine := &mockExamTestEngine{paper: &fixture, bands: [2]float64{6, 9}}
	mem := store.NewMemoryStore()
	svc := New(mem, nil, engine, &mockPlayableTTS{}, "test")
	created, err := svc.StartMockExam("owner", "IELTS", 7)
	if err != nil {
		t.Fatal(err)
	}
	ready := awaitMockState(t, svc, created.ID, "READY")
	if !ready.StartedAt.IsZero() {
		t.Fatal("clock ran while preparing")
	}
	for _, s := range ready.Sections {
		if s.Skill == "LISTENING" && (s.AudioURL == "" || len(s.AudioURLs) == 0) {
			t.Fatal("audio not prepared")
		}
	}
	started, err := svc.BeginMockExam("owner", created.ID)
	if err != nil || started.StartedAt.IsZero() {
		t.Fatalf("begin failed %v", err)
	}
	for _, part := range started.Sections {
		answers := []string{}
		for _, q := range part.Questions {
			answers = append(answers, q.AnswerKey)
		}
		responses := []string{}
		if part.Skill == "WRITING" {
			responses = []string{"actual essay one", "actual essay two"}
		}
		if _, err = svc.SubmitMockExamSection("owner", created.ID, part.Key, answers, responses); err != nil {
			t.Fatal(err)
		}
	}
	engine.scoreError = errors.New("upstream failed")
	if _, err = svc.FinishMockExam("owner", created.ID); err != nil {
		t.Fatal(err)
	}
	failed := awaitMockState(t, svc, created.ID, "EVALUATION_FAILED")
	if failed.Result != nil {
		t.Fatal("failed assessment fabricated a report")
	}
	engine.scoreError = nil
	if _, err = svc.FinishMockExam("owner", created.ID); err != nil {
		t.Fatal(err)
	}
	result := awaitMockState(t, svc, created.ID, "COMPLETED")
	if result.Result.ReadingCorrect != 40 || result.Result.ListeningCorrect != 40 || result.Result.WritingBand != 8 {
		t.Fatalf("wrong result %+v", result.Result)
	}
	if _, err = svc.SaveMockExamAnswers("owner", created.ID, "READING_1", []string{"A"}, nil); err == nil {
		t.Fatal("completed attempt mutable")
	}
	if _, err = svc.FinishMockExam("owner", created.ID); err != nil {
		t.Fatal("idempotent finish failed", err)
	}
}

func TestMockAudioFailureNeverBecomesReady(t *testing.T) {
	fixture := loadMockContractFixture(t)
	engine := &mockExamTestEngine{paper: &fixture}
	svc := New(store.NewMemoryStore(), nil, engine, &mockPlayableTTS{fail: true}, "test")
	exam, err := svc.StartMockExam("owner", "IELTS", 7)
	if err != nil {
		t.Fatal(err)
	}
	failed := awaitMockState(t, svc, exam.ID, "FAILED")
	if !failed.StartedAt.IsZero() || failed.Result != nil {
		t.Fatal("failed audio started or scored")
	}
}
