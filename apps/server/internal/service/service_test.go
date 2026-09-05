package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/ielts"
	"github.com/linguaquest/server/internal/store"
)

type sequenceTTS struct {
	urls  []string
	errs  []error
	calls int
}

type contextualRecordingTTS struct {
	context string
	voice   string
}

type voiceDesignTTS struct{}

type recordingAuthMailer struct {
	verificationCalls     int
	passwordResetCalls    int
	usernameRecoveryCalls int
	err                   error
}

func (m *recordingAuthMailer) SendEmailVerification(context.Context, string, string, string) error {
	m.verificationCalls++
	return m.err
}

func (m *recordingAuthMailer) SendPasswordReset(context.Context, string, string, string) error {
	m.passwordResetCalls++
	return m.err
}

func (m *recordingAuthMailer) SendUsernameRecovery(context.Context, string, []string) error {
	m.usernameRecoveryCalls++
	return m.err
}

func (voiceDesignTTS) Synthesize(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "fallback", nil
}

func (voiceDesignTTS) DesignVoice(_ context.Context, _ string, _ string) (string, error) {
	return "data:audio/mpeg;base64,AAAA", nil
}

func (t *contextualRecordingTTS) Synthesize(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "fallback", nil
}

func (t *contextualRecordingTTS) SynthesizeWithContext(_ context.Context, _ string, _ string, voice string, dialogueContext string) (string, error) {
	t.context = dialogueContext
	t.voice = voice
	return "contextual", nil
}

func (s *sequenceTTS) Synthesize(_ context.Context, _ string, _ string, _ string) (string, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return "", s.errs[i]
	}
	if i < len(s.urls) {
		return s.urls[i], nil
	}
	return "", nil
}

type failingGenerator struct {
	err error
}

type blockingTTS struct {
	mu      sync.Mutex
	active  int
	maximum int
	started chan struct{}
	release <-chan struct{}
}

func (t *blockingTTS) Synthesize(_ context.Context, _ string, _ string, _ string) (string, error) {
	t.mu.Lock()
	t.active++
	if t.active > t.maximum {
		t.maximum = t.active
	}
	t.mu.Unlock()
	t.started <- struct{}{}
	<-t.release
	t.mu.Lock()
	t.active--
	t.mu.Unlock()
	return "audio", nil
}

func TestTTSConcurrencyIsGloballyLimited(t *testing.T) {
	release := make(chan struct{})
	tts := &blockingTTS{started: make(chan struct{}, 3), release: release}
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, tts, "secret", Options{TTSMaxConcurrency: 2})
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.synthesizeWithTTSLimit(context.Background(), "hello", "ENGLISH", "")
		}()
	}
	for range 2 {
		select {
		case <-tts.started:
		case <-time.After(time.Second):
			t.Fatal("expected two TTS calls to start")
		}
	}
	select {
	case <-tts.started:
		t.Fatal("third TTS call bypassed the configured concurrency limit")
	default:
	}
	close(release)
	wg.Wait()
	if tts.maximum != 2 {
		t.Fatalf("maximum concurrent TTS calls = %d, want 2", tts.maximum)
	}
}

func TestSynthesizeWithTTSContextLimitUsesOptionalContextInterface(t *testing.T) {
	tts := &contextualRecordingTTS{}
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, tts, "secret", Options{TTSMaxConcurrency: 1})
	audio, err := svc.synthesizeWithTTSContextLimit(context.Background(), "早晨", "CANTONESE", "活力少年", "上一句係阿欣問路")
	if err != nil {
		t.Fatal(err)
	}
	if audio != "contextual" || tts.voice != "活力少年" || !strings.Contains(tts.context, "上一句係阿欣") {
		t.Fatalf("contextual synthesis was not used: audio=%q voice=%q context=%q", audio, tts.voice, tts.context)
	}
}

func TestBuildDialogueTTSContextIncludesSceneAndPreviousTurn(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "阿欣（女顧客）", Text: "唔該，而家仲有冇位呀？"},
		{Speaker: "阿明（男店員）", Text: "有呀，呢邊請。"},
	}
	got := buildDialogueTTSContext("茶餐厅点餐", dialogues, 1)
	for _, want := range []string{"茶餐廳點餐", "阿明（男店員）", "上一句係阿欣（女顧客）", "而家仲有冇位"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildDialogueTTSContext() = %q, want %q", got, want)
		}
	}
}

func TestAssignDialogueVoiceStylesKeepsCharactersConsistentAndDistinct(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "顾客"},
		{Speaker: "顾客"},
		{Speaker: "店员"},
		{Speaker: "顾客"},
		{Speaker: "店员"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"温柔女生", "播音男生"}, nil)
	if len(styles) != len(dialogues) {
		t.Fatalf("style count = %d, want %d", len(styles), len(dialogues))
	}
	if styles[0] != styles[1] || styles[0] != styles[3] {
		t.Fatalf("customer styles are not stable: %#v", styles)
	}
	if styles[2] != styles[4] {
		t.Fatalf("staff styles are not stable: %#v", styles)
	}
	if styles[0] == styles[2] {
		t.Fatalf("distinct speakers received the same automatic style: %#v", styles)
	}
}

func TestAssignDialogueVoiceStylesMatchesExplicitSpeakerGender(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "陳小姐（女顧客）"},
		{Speaker: "王先生（男店員）"},
		{Speaker: "陳小姐（女顧客）"},
		{Speaker: "王先生（男店員）"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"播音男生", "温柔女生"}, nil)
	if automaticVoiceStyleGender(styles[0]) != dialogueVoiceGenderFemale || automaticVoiceStyleGender(styles[2]) != dialogueVoiceGenderFemale {
		t.Fatalf("female speaker received non-female styles: %#v", styles)
	}
	if automaticVoiceStyleGender(styles[1]) != dialogueVoiceGenderMale || automaticVoiceStyleGender(styles[3]) != dialogueVoiceGenderMale {
		t.Fatalf("male speaker received non-male styles: %#v", styles)
	}
}

func TestAssignDialogueVoiceStylesPrefersGeneratedGenderAndNormalizesSpeaker(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "**梁姐（女侍應）**", Gender: "FEMALE"},
		{Speaker: "陳先生（男顧客）", Gender: "MALE"},
		{Speaker: "梁姐（女侍應）", Gender: "FEMALE"},
		{Speaker: "陳先生（男顧客）", Gender: "MALE"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"播音男生", "温柔女生"}, nil)
	if automaticVoiceStyleGender(styles[0]) != dialogueVoiceGenderFemale || styles[0] != styles[2] {
		t.Fatalf("梁姐 should keep one female voice: %#v", styles)
	}
	if automaticVoiceStyleGender(styles[1]) != dialogueVoiceGenderMale || styles[1] != styles[3] {
		t.Fatalf("陳先生 should keep one male voice: %#v", styles)
	}
	if styles[0] == styles[1] {
		t.Fatalf("two speakers should not share one automatic voice: %#v", styles)
	}
}

func TestAssignDialogueVoiceStylesInfersGenderFromVocatives(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "店員", Text: "小姐，呢邊可以落單。"},
		{Speaker: "顧客", Text: "唔該晒，先生，我想要熱奶茶。"},
		{Speaker: "店員", Text: "好呀，陣間送過嚟。"},
		{Speaker: "顧客", Text: "冇問題。"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"温柔女生", "播音男生"}, nil)
	if automaticVoiceStyleGender(styles[0]) != dialogueVoiceGenderMale {
		t.Fatalf("male addressee inference failed: %#v", styles)
	}
	if automaticVoiceStyleGender(styles[1]) != dialogueVoiceGenderFemale {
		t.Fatalf("female addressee inference failed: %#v", styles)
	}
}

func TestAssignDialogueVoiceProfilesMatchesPromptGender(t *testing.T) {
	dialogues := []domain.Dialogue{{Speaker: "李女士"}, {Speaker: "張先生"}}
	profiles := []domain.VoiceProfile{
		{ID: "male", Name: "成熟男聲", Prompt: "香港男聲", PreviewAudioURL: "male-audio"},
		{ID: "female", Name: "溫柔女聲", Prompt: "香港女聲", PreviewAudioURL: "female-audio"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"温柔女生", "播音男生"}, profiles)
	if styles[0] != "female-audio" || styles[1] != "male-audio" {
		t.Fatalf("voice profiles did not match speaker gender: %#v", styles)
	}
}

func TestAssignDialogueVoiceStylesMatchesObservedCantoneseNames(t *testing.T) {
	dialogues := []domain.Dialogue{
		{Speaker: "阿明（侍應）", Text: "小姐，幾位呀？而家午市好趕。"},
		{Speaker: "阿欣（顧客）", Text: "一位，唔該，我想快啲落單。"},
		{Speaker: "阿明（侍應）", Text: "今日快餐大概八分鐘。"},
		{Speaker: "阿欣（顧客）", Text: "咁我要黑椒牛柳絲飯。"},
	}
	styles := assignDialogueVoiceStyles(dialogues, [2]string{"温柔女生", "播音男生"}, nil)
	if automaticVoiceStyleGender(styles[0]) != dialogueVoiceGenderMale || automaticVoiceStyleGender(styles[2]) != dialogueVoiceGenderMale {
		t.Fatalf("阿明 should use a male voice: %#v", styles)
	}
	if automaticVoiceStyleGender(styles[1]) != dialogueVoiceGenderFemale || automaticVoiceStyleGender(styles[3]) != dialogueVoiceGenderFemale {
		t.Fatalf("阿欣 should use a female voice: %#v", styles)
	}
}

func TestVoiceProfileGenerationRequiresApprovalBeforeTheaterUse(t *testing.T) {
	mem := store.NewMemoryStore()
	profile := domain.VoiceProfile{
		ID:       "voice-preview",
		UserID:   "user-1",
		Name:     "測試女聲",
		Prompt:   "自然香港女聲",
		Language: "CANTONESE",
		Provider: "XIAOMI",
		Model:    "mimo-v2.5-tts-voicedesign",
		Status:   "GENERATING",
	}
	if _, err := mem.SaveVoiceProfile(profile); err != nil {
		t.Fatal(err)
	}

	svc := New(mem, nil, nil, voiceDesignTTS{}, "secret")
	svc.designVoiceProfileAsync(context.Background(), profile)

	profiles, err := mem.ListVoiceProfiles("user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Status != "PREVIEW" {
		t.Fatalf("generated voice profile = %+v, want PREVIEW", profiles)
	}
	if _, err := svc.selectedVoiceProfiles("user-1", []string{profile.ID}); err == nil {
		t.Fatal("preview voice profile should not be selectable before approval")
	}

	approved, err := svc.ApproveVoiceProfile("user-1", profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "READY" || !strings.Contains(approved.GenerationMessage, "确认") {
		t.Fatalf("approved voice profile = %+v", approved)
	}
	selected, err := svc.selectedVoiceProfiles("user-1", []string{profile.ID})
	if err != nil || len(selected) != 1 {
		t.Fatalf("approved voice profile should be selectable: profiles=%+v err=%v", selected, err)
	}
	approvedAgain, err := svc.ApproveVoiceProfile("user-1", profile.ID)
	if err != nil || approvedAgain.Status != "READY" {
		t.Fatalf("approval should be idempotent: profile=%+v err=%v", approvedAgain, err)
	}
}

func TestApproveVoiceProfileRejectsNonPreviewAndOtherUsers(t *testing.T) {
	mem := store.NewMemoryStore()
	failed := domain.VoiceProfile{ID: "voice-failed", UserID: "user-1", Status: "FAILED", PreviewAudioURL: "data:audio/mpeg;base64,AAAA"}
	preview := domain.VoiceProfile{ID: "voice-other", UserID: "user-2", Status: "PREVIEW", PreviewAudioURL: "data:audio/mpeg;base64,AAAA"}
	if _, err := mem.SaveVoiceProfile(failed); err != nil {
		t.Fatal(err)
	}
	if _, err := mem.SaveVoiceProfile(preview); err != nil {
		t.Fatal(err)
	}
	svc := New(mem, nil, nil, nil, "secret")
	if _, err := svc.ApproveVoiceProfile("user-1", failed.ID); err == nil {
		t.Fatal("FAILED voice profile should not be approvable")
	}
	if _, err := svc.ApproveVoiceProfile("user-1", preview.ID); err == nil {
		t.Fatal("another user's voice profile should not be approvable")
	}
}

func (f failingGenerator) Generate(context.Context, string, string, float64, string) ([]domain.Dialogue, []domain.QuizQuestion, error) {
	return nil, nil, f.err
}

func TestGenerateTheaterMarksFailedWhenAIGenerationFails(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, failingGenerator{err: errors.New("upstream unavailable")}, nil, "secret")

	theater, err := svc.GenerateTheater("user-1", "ENGLISH", "[IELTS Listening][Band 7.0][Section 3] seminar scheduling dispute", 7.0, "DIALOGUE")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, err := mem.GetTheater(theater.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == "FAILED" {
			if len(current.Dialogues) != 0 || len(current.QuizQuestions) != 0 {
				t.Fatalf("failed theater should not contain fallback content: dialogues=%d quiz=%d", len(current.Dialogues), len(current.QuizQuestions))
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("theater did not reach FAILED after AI generation error")
}

func TestTheaterGenerationCompletionMessageReflectsAudioResult(t *testing.T) {
	tests := []struct {
		name         string
		audioSuccess int
		dialogues    int
		want         string
	}{
		{name: "all audio ready", audioSuccess: 8, dialogues: 8, want: "生成完成"},
		{name: "all audio failed", audioSuccess: 0, dialogues: 8, want: "文本生成完成，语音生成失败，请检查 TTS 配置"},
		{name: "partial audio", audioSuccess: 3, dialogues: 8, want: "文本生成完成，部分语音生成失败（3/8）"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theaterGenerationCompletionMessage(tt.audioSuccess, tt.dialogues); got != tt.want {
				t.Fatalf("theaterGenerationCompletionMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateReadingMaterialMarksFailedWhenAIGenerationFails(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, failingGenerator{err: errors.New("upstream unavailable")}, nil, "secret")

	created, err := svc.GenerateReadingMaterial("user-1", "IELTS", "[Band 7.0][Matching Headings] urban transport resilience", "advanced", nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "GENERATING" {
		t.Fatalf("initial status = %q, want GENERATING", created.Status)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, getErr := mem.GetReadingMaterial(created.ID, "user-1")
		if getErr != nil {
			t.Fatal(getErr)
		}
		if current.Status == "FAILED" {
			if current.Passage != "" || len(current.Questions) != 0 {
				t.Fatalf("failed reading material should not contain generated content: passage=%d questions=%d", len(current.Passage), len(current.Questions))
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("reading material did not reach FAILED after AI generation error")
}

func TestGenerateReadingMaterialPersistsNormalizedMetadataBeforeQueueing(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, failingGenerator{err: errors.New("upstream unavailable")}, nil, "secret")
	created, err := svc.GenerateReadingMaterialWithInput("user-1", domain.ReadingGenerationInput{
		Exam:           "IELTS",
		Topic:          "[Band 6.0][Matching Headings] urban transport resilience",
		Level:          "advanced",
		Band:           7.26,
		Stage:          "Stage 12",
		Section:        "Section 3",
		SkillFocus:     "author stance",
		QuestionType:   "TFNG",
		ScenarioFamily: "urban policy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Band != 7.3 || created.Stage != "Stage 12" || created.Section != "Section 3" {
		t.Fatalf("created metadata = %+v", created)
	}
	if created.SkillFocus != "author stance" || created.QuestionType != "TFNG" || created.ScenarioFamily != "urban policy" {
		t.Fatalf("explicit metadata missing from created material: %+v", created)
	}
	if strings.Contains(created.Title, "[Band") || strings.Contains(created.Title, "Matching Headings") {
		t.Fatalf("title leaked control metadata: %q", created.Title)
	}
	persisted, err := mem.GetReadingMaterial(created.ID, created.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Band != created.Band || persisted.QuestionType != created.QuestionType || persisted.ScenarioFamily != created.ScenarioFamily {
		t.Fatalf("persisted metadata = %+v, created = %+v", persisted, created)
	}
}

func TestMaterializeAudioDataURLStoresMediaFile(t *testing.T) {
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, nil, "secret", ServiceOptions{MediaDir: t.TempDir()})
	url, err := svc.materializeAudioURL("data:audio/mpeg;base64,SGVsbG8=", "reading", "material-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(url, "data:audio/") {
		t.Fatalf("audio URL still contains inline data: %q", url)
	}
	if !strings.HasPrefix(url, "/media/tts/reading/material-1/") || !strings.HasSuffix(url, ".mp3") {
		t.Fatalf("audio URL = %q, want /media/tts/reading/material-1/*.mp3", url)
	}
}

func TestReadingMaterialCacheDropsAudioURLs(t *testing.T) {
	svc := New(store.NewMemoryStore(), nil, nil, nil, "secret")
	material := domain.ReadingMaterial{
		ID:        "reading-cache",
		UserID:    "user-1",
		AudioURL:  "data:audio/wav;base64,AAAA",
		AudioURLs: []string{"data:audio/wav;base64,BBBB"},
	}
	svc.cacheReadingMaterial(material)

	cached := svc.readingMaterials[material.ID]
	if cached.AudioURL != "" || len(cached.AudioURLs) != 0 {
		t.Fatalf("cached reading material kept audio data: audioURL=%q audioURLs=%d", cached.AudioURL, len(cached.AudioURLs))
	}
}

func TestReadingMaterialFallsBackToIDWhenUserTokenChanges(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:          "reading-cross-user",
		UserID:      "original-user",
		Exam:        "IELTS",
		Language:    "ENGLISH",
		Level:       "advanced",
		Topic:       "urban transport resilience",
		Title:       "Urban transport resilience",
		Passage:     longAudioPassage(),
		Questions:   []domain.QuizQuestion{},
		AudioStatus: "PENDING",
		CreatedAt:   time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	svc := New(mem, nil, nil, nil, "secret")

	got, err := svc.ReadingMaterial("new-guest-user", material.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != material.ID {
		t.Fatalf("ReadingMaterial ID = %q, want %q", got.ID, material.ID)
	}
}

func TestExportOAuthAccountsUsesPlainTextImportFormat(t *testing.T) {
	mem := store.NewMemoryStore()
	mem.SaveOAuthAccount(domain.OAuthAccount{
		Email:        "first@example.com",
		Provider:     "microsoft",
		ClientID:     "client-a",
		RefreshToken: "refresh-token-a",
	})
	mem.SaveOAuthAccount(domain.OAuthAccount{
		Email:        "second@example.com",
		Provider:     "google",
		ClientID:     "client-b",
		RefreshToken: "refresh-token-b",
	})
	mem.SaveOAuthAccount(domain.OAuthAccount{
		Email:        "skip@example.com",
		Provider:     "microsoft",
		ClientID:     "client-c",
		RefreshToken: "",
	})
	svc := New(mem, nil, nil, nil, "secret")

	text, err := svc.ExportOAuthAccounts("microsoft")
	if err != nil {
		t.Fatal(err)
	}
	if text != "first@example.com----microsoft----client-a----refresh-token-a" {
		t.Fatalf("export text = %q", text)
	}
}

func TestMiniProgramRejectsUserServiceConfigurationUpdates(t *testing.T) {
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, nil, "secret", Options{
		Billing: BillingOptions{MiniProgramEdition: true},
	})

	if _, err := svc.UpdateModelConfig(domain.ModelConfigUpdate{}); err == nil || !strings.Contains(err.Error(), "managed by the online service") {
		t.Fatalf("UpdateModelConfig error = %v, want mini program configuration restriction", err)
	}
	if _, err := svc.UpdateTTSConfig(domain.TTSConfigUpdate{}); err == nil || !strings.Contains(err.Error(), "managed by the online service") {
		t.Fatalf("UpdateTTSConfig error = %v, want mini program configuration restriction", err)
	}
	if _, err := svc.UpdateASRConfig(domain.ASRConfigUpdate{}); err == nil || !strings.Contains(err.Error(), "managed by the online service") {
		t.Fatalf("UpdateASRConfig error = %v, want mini program configuration restriction", err)
	}
}

func TestFallbackReadingContentMatchesQuestionType(t *testing.T) {
	meta := ielts.ReadingMetadataFromTopic("IELTS", "[Band 7.0][Matching Headings] urban transport", "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata("urban transport", meta, 5)
	limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", "[Band 7.0][Matching Headings] urban transport", meta)
	passage := joinDialogueText(dialogues)
	if got := contentquality.WordCount(passage); got < limits.MinWords {
		t.Fatalf("fallback word count = %d, want at least %d", got, limits.MinWords)
	}
	if len(dialogues) < limits.MinSegments {
		t.Fatalf("dialogues len = %d, want at least %d", len(dialogues), limits.MinSegments)
	}
	if len(quiz) != 5 {
		t.Fatalf("quiz len = %d, want 5", len(quiz))
	}
	for _, q := range quiz {
		if q.Type != "Matching Headings" {
			t.Fatalf("question type = %q, want Matching Headings", q.Type)
		}
		if len(q.Headings) < 4 || q.ParagraphRef == "" {
			t.Fatalf("question missing heading structure: %+v", q)
		}
		if contentquality.IsGenericReadingQuestion(q.Question) {
			t.Fatalf("fallback produced generic question: %q", q.Question)
		}
	}
}

func TestFallbackMixedReadingContentHasExpectedQuestionMix(t *testing.T) {
	meta := ielts.ReadingMetadataFromTopic("IELTS", "[Band 7.0][Mixed Question Set] public health data", "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata("[Band 7.0][Mixed Question Set] public health data", meta, 5)
	passage := joinDialogueText(dialogues)
	limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", "[Band 7.0][Mixed Question Set] public health data", meta)
	if err := validateReadingMaterialText(passage, quiz, limits.MinWords, limits.MinSegments); err != nil {
		t.Fatalf("mixed fallback failed material quality: %v", err)
	}
	wantTypes := []string{"Multiple Choice", "Matching Information", "TFNG", "Summary Completion", "Multiple Choice"}
	if len(quiz) != len(wantTypes) {
		t.Fatalf("quiz len = %d, want %d", len(quiz), len(wantTypes))
	}
	seenQuestions := map[string]bool{}
	for i, wantType := range wantTypes {
		if quiz[i].Type != wantType {
			t.Fatalf("quiz[%d].Type = %q, want %q", i, quiz[i].Type, wantType)
		}
		if seenQuestions[quiz[i].Question] {
			t.Fatalf("mixed fallback repeated question %q", quiz[i].Question)
		}
		seenQuestions[quiz[i].Question] = true
	}
	assertReadingQuestionShapes(t, meta.QuestionType, quiz)
	assertReadingEvidenceAnchored(t, passage, quiz)
}

func TestFallbackReadingPassageUsesTopicSpecificFrame(t *testing.T) {
	topic := "[IELTS Reading][Stage 11][Band 6.5][Section 2][Matching Headings] why certain migratory birds alter their routes after changes in agricultural lighting"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "upper-intermediate")
	dialogues, _ := fallbackReadingContentWithMetadata(topic, meta, 5)
	passage := joinDialogueText(dialogues)
	if strings.Contains(passage, "Public discussion of why certain migratory birds") {
		t.Fatalf("fallback passage still uses generic public discussion frame: %q", passage[:min(180, len(passage))])
	}
	for _, want := range []string{"Ecologists", "agricultural landscapes", "artificial lighting"} {
		if !strings.Contains(passage, want) {
			t.Fatalf("fallback passage missing topic-specific marker %q in %q", want, passage[:min(240, len(passage))])
		}
	}
}

func TestFallbackAdvancedMixedReadingAvoidsLongTopicRepetition(t *testing.T) {
	topic := "[IELTS Reading][Stage 18][Band 7.5][Section 3][Mixed Question Set][Focus: inference and paragraph evidence] the limits of algorithmic decision-making in climate adaptation, public health triage, and urban infrastructure planning"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "advanced")
	dialogues, quiz := fallbackReadingContentWithMetadata(topic, meta, 5)
	passage := joinDialogueText(dialogues)
	cleanTopic := ielts.CleanTopic(topic)

	if strings.Contains(passage, "Public discussion of") {
		t.Fatalf("advanced fallback still uses generic public discussion frame: %q", passage[:min(180, len(passage))])
	}
	if strings.Count(strings.ToLower(passage), strings.ToLower(cleanTopic)) > 0 {
		t.Fatalf("advanced fallback repeats full raw topic instead of concise subject: %q", cleanTopic)
	}
	lowerPassage := strings.ToLower(passage)
	for _, want := range []string{"algorithmic decision systems", "policy analysts", "datasets, scoring rules and appeal procedures"} {
		if !strings.Contains(lowerPassage, want) {
			t.Fatalf("advanced fallback missing high-level topic marker %q in %q", want, passage[:min(260, len(passage))])
		}
	}
	assertReadingQuestionShapes(t, meta.QuestionType, quiz)
}

func TestReadingMaterialTitleRemovesMetadataTags(t *testing.T) {
	topic := "[IELTS Reading][Stage 18][Band 7.5][Section 3][Mixed Question Set][Focus: inference and paragraph evidence] the limits of algorithmic decision-making in climate adaptation"
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "advanced")
	title := readingMaterialTitle("IELTS", topic, meta)

	for _, blocked := range []string{"[IELTS", "[Stage", "[Band", "[Focus", "Mixed Question Set"} {
		if strings.Contains(title, blocked) {
			t.Fatalf("title leaked metadata marker %q: %q", blocked, title)
		}
	}
	if !strings.Contains(title, "The limits of algorithmic decision-making") {
		t.Fatalf("title = %q, want cleaned human-readable topic", title)
	}
}

func TestGenerateReadingAudioPreservesAndResumesChunks(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:          "reading-1",
		UserID:      "user-1",
		Exam:        "IELTS",
		Language:    "ENGLISH",
		Level:       "advanced",
		Topic:       "urban transport",
		Title:       "Urban transport",
		Passage:     longAudioPassage(),
		AudioStatus: "PENDING",
		CreatedAt:   time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	firstTTS := &sequenceTTS{urls: []string{"audio-1"}, errs: []error{nil, errors.New("temporary timeout")}}
	svc := New(mem, nil, nil, firstTTS, "secret")
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)
	partial, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if partial.AudioStatus != "PENDING" {
		t.Fatalf("AudioStatus after partial failure = %q, want PENDING", partial.AudioStatus)
	}
	if len(partial.AudioURLs) != 1 {
		t.Fatalf("AudioURLs len after partial failure = %d, want 1", len(partial.AudioURLs))
	}

	secondTTS := &sequenceTTS{urls: []string{"audio-2"}}
	svc.tts = secondTTS
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)
	ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.AudioStatus != "READY" {
		t.Fatalf("AudioStatus after resume = %q, want READY", ready.AudioStatus)
	}
	if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
		t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
	}
	if secondTTS.calls != 1 {
		t.Fatalf("resume TTS calls = %d, want 1", secondTTS.calls)
	}
}

func TestGenerateReadingAudioClearsProgressNoteAfterReady(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:          "reading-2",
		UserID:      "user-1",
		Exam:        "IELTS",
		Language:    "ENGLISH",
		Level:       "advanced",
		Topic:       "public health",
		Title:       "Public health",
		Passage:     longAudioPassage(),
		AudioStatus: "PENDING",
		CreatedAt:   time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2"}}
	svc := New(mem, nil, nil, tts, "secret")
	svc.generateReadingAudio(material.ID, material.Passage, material.Language)

	ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.AudioStatus != "READY" {
		t.Fatalf("AudioStatus = %q, want READY", ready.AudioStatus)
	}
	if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
		t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
	}
	if strings.Contains(ready.GenerationNote, "audio chunk") {
		t.Fatalf("GenerationNote = %q, want progress note cleared after ready", ready.GenerationNote)
	}
}

func TestReadingMaterialRetriesFallbackAudioAndDeduplicatesJobs(t *testing.T) {
	mem := store.NewMemoryStore()
	material := domain.ReadingMaterial{
		ID:             "reading-fallback",
		UserID:         "user-1",
		Exam:           "IELTS",
		Language:       "ENGLISH",
		Level:          "advanced",
		Topic:          "[IELTS Reading][Band 6.5][Matching Headings] public health systems",
		Title:          "Public health systems",
		Passage:        longAudioPassage(),
		AudioStatus:    "PENDING",
		GenerationNote: "Generated via structured fallback after AI generation was unavailable or failed quality validation.",
		CreatedAt:      time.Now(),
	}
	if _, err := mem.SaveReadingMaterial(material); err != nil {
		t.Fatal(err)
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2"}}
	svc := New(mem, nil, nil, tts, "secret")

	first, err := svc.ReadingMaterial(material.UserID, material.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ReadingMaterial(material.UserID, material.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != material.ID || second.ID != material.ID {
		t.Fatalf("unexpected material ids: %q %q", first.ID, second.ID)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ready, err := mem.GetReadingMaterial(material.ID, material.UserID)
		if err != nil {
			t.Fatal(err)
		}
		if ready.AudioStatus == "READY" {
			if got := strings.Join(ready.AudioURLs, ","); got != "audio-1,audio-2" {
				t.Fatalf("AudioURLs = %q, want audio-1,audio-2", got)
			}
			if tts.calls != 2 {
				t.Fatalf("TTS calls = %d, want 2", tts.calls)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("fallback reading audio did not reach READY in time")
}

func TestReadingMaterialsQueuesLimitedFallbackAudioRetriesWithCooldown(t *testing.T) {
	mem := store.NewMemoryStore()
	now := time.Now()
	for i := 0; i < 3; i++ {
		material := domain.ReadingMaterial{
			ID:             fmt.Sprintf("reading-list-%d", i+1),
			UserID:         "user-1",
			Exam:           "IELTS",
			Language:       "ENGLISH",
			Level:          "advanced",
			Topic:          fmt.Sprintf("[IELTS Reading][Band 6.0][Matching Information] sample %d", i+1),
			Title:          "Sample",
			Passage:        "Short fallback passage for retry.",
			AudioStatus:    "PENDING",
			GenerationNote: "Generated via structured fallback after AI generation was unavailable or failed quality validation.",
			CreatedAt:      now.Add(time.Duration(i) * time.Second),
		}
		if _, err := mem.SaveReadingMaterial(material); err != nil {
			t.Fatal(err)
		}
	}
	tts := &sequenceTTS{urls: []string{"audio-1", "audio-2", "audio-3"}}
	svc := New(mem, nil, nil, tts, "secret")

	items, err := svc.ReadingMaterials("user-1", "IELTS")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("ReadingMaterials len = %d, want 3", len(items))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if tts.calls >= maxReadingAudioListRetries {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tts.calls != maxReadingAudioListRetries {
		t.Fatalf("initial TTS calls = %d, want %d", tts.calls, maxReadingAudioListRetries)
	}

	if _, err := svc.ReadingMaterials("user-1", "IELTS"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if tts.calls != maxReadingAudioListRetries {
		t.Fatalf("TTS calls after second list = %d, want still %d", tts.calls, maxReadingAudioListRetries)
	}
}

func longAudioPassage() string {
	return strings.Repeat("governance ", 35) + ". " + strings.Repeat("evidence ", 35) + "."
}

func TestLoginLocksAfterFiveFailuresAndClearsAfterSuccess(t *testing.T) {
	mem := store.NewMemoryStore()
	svc := New(mem, nil, nil, nil, "secret")
	if _, err := svc.Register("lock_user", "lock@example.com", "GoodPass1"); err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= maxLoginFailures; attempt++ {
		_, err := svc.Login("lock_user", "WrongPass1", "")
		if err == nil {
			t.Fatal("wrong password should fail")
		}
		if attempt < maxLoginFailures && !strings.Contains(err.Error(), "还可尝试") {
			t.Fatalf("attempt %d error = %q, want remaining-attempt message", attempt, err)
		}
	}
	if _, err := svc.Login("lock_user", "GoodPass1", ""); err == nil || !strings.Contains(err.Error(), "60 秒") {
		t.Fatalf("locked login error = %v, want 60-second lock", err)
	}

	user, err := mem.GetUserByUsername("lock_user")
	if err != nil {
		t.Fatal(err)
	}
	key := loginFailureKey("lock_user", user.ID)
	svc.authMu.Lock()
	state := svc.loginFailures[key]
	state.LockedUntil = time.Now().Add(-time.Second)
	svc.loginFailures[key] = state
	svc.authMu.Unlock()
	if _, err := svc.Login("lock_user", "GoodPass1", ""); err != nil {
		t.Fatalf("login after lock expiry failed: %v", err)
	}
	if _, err := svc.Login("lock_user", "WrongPass1", ""); err == nil || !strings.Contains(err.Error(), "还可尝试 4 次") {
		t.Fatalf("failure counter was not cleared after success: %v", err)
	}
}

func TestAuthenticationEmailsAreRateLimitedForOneMinute(t *testing.T) {
	mailer := &recordingAuthMailer{}
	mem := store.NewMemoryStore()
	svc := NewWithOptions(mem, nil, nil, nil, "secret", Options{Mailer: mailer, RequireEmailVerification: true})
	registered, err := svc.Register("mail_user", "mail@example.com", "GoodPass1")
	if err != nil {
		t.Fatal(err)
	}
	if mailer.verificationCalls != 1 {
		t.Fatalf("verification calls = %d, want 1", mailer.verificationCalls)
	}
	if _, err = svc.Register("mail_user_two", "mail@example.com", "GoodPass1"); err == nil || !strings.Contains(err.Error(), "邮件发送过于频繁") {
		t.Fatalf("registration cooldown error = %v", err)
	}
	users, err := mem.ListUsersByEmail("mail@example.com")
	if err != nil || len(users) != 1 {
		t.Fatalf("registration cooldown should not create another account: users=%d err=%v", len(users), err)
	}
	if _, err = svc.RequestEmailVerification("mail_user", registered.UserID); err == nil || !strings.Contains(err.Error(), "邮件发送过于频繁") {
		t.Fatalf("verification cooldown error = %v", err)
	}
	if mailer.verificationCalls != 1 {
		t.Fatalf("verification calls after cooldown = %d, want 1", mailer.verificationCalls)
	}

	resetSvc := NewWithOptions(mem, nil, nil, nil, "secret", Options{Mailer: mailer})
	if _, err = resetSvc.RequestPasswordReset("mail_user", registered.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err = resetSvc.RequestPasswordReset("mail_user", registered.UserID); err == nil || !strings.Contains(err.Error(), "邮件发送过于频繁") {
		t.Fatalf("password reset cooldown error = %v", err)
	}
	if mailer.passwordResetCalls != 1 {
		t.Fatalf("password reset calls = %d, want 1", mailer.passwordResetCalls)
	}
}

func TestAuthenticationErrorsDoNotExposeMailerDetails(t *testing.T) {
	mailer := &recordingAuthMailer{err: errors.New("dial tcp smtp.example: timeout")}
	svc := NewWithOptions(store.NewMemoryStore(), nil, nil, nil, "secret", Options{Mailer: mailer, RequireEmailVerification: true})
	result, err := svc.Register("smtp_user", "smtp@example.com", "GoodPass1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "验证邮件发送失败") || strings.Contains(result.Message, "smtp.example") {
		t.Fatalf("user-facing SMTP result = %q", result.Message)
	}
}
