package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/auth"
	"github.com/linguaquest/server/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type Store interface {
	CreateUser(email string, passwordHash string) (domain.User, error)
	GetUserByEmail(email string) (domain.User, error)
	GetUserByID(id string) (domain.User, error)
	UpdateUserProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error)
	SaveTheater(theater domain.Theater) (domain.Theater, error)
	GetTheater(id string) (domain.Theater, error)
	GetTheaterByShareCode(shareCode string) (domain.Theater, error)
	ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error)
	SetTheaterFavorite(userID string, theaterID string, favorite bool) error
	SetTheaterShareCode(userID string, theaterID string, shareCode string) error
	DeleteTheater(userID string, theaterID string) error
	AddUserXP(userID string, xp int) error
	SavePracticeRecord(userID string, theaterID string, score int, answers []string, xpEarned int) error
	SaveReadingPracticeRecord(userID string, materialID string, score int, answers []string, xpEarned int) error
	ListCourses(language string) ([]domain.Course, error)
	SaveReadingMaterial(material domain.ReadingMaterial) (domain.ReadingMaterial, error)
	GetReadingMaterial(id string, userID string) (domain.ReadingMaterial, error)
	ListReadingMaterialsByUser(userID string, exam string) ([]domain.ReadingMaterial, error)
	CreateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error)
	GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error)
	UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error)
}

type SessionStore interface {
	SetRefreshToken(ctx context.Context, userID string, token string) error
	GetRefreshToken(ctx context.Context, userID string) (string, error)
}

type TheaterGenerator interface {
	Generate(ctx context.Context, language string, topic string, difficulty float64, mode string) ([]domain.Dialogue, []domain.QuizQuestion, error)
}

type ReadingAnalyzer interface {
	AnalyzeReading(ctx context.Context, exam string, topic string, passage string, vocabulary []string) (domain.ReadingAnalysis, error)
}

type SpeechSynthesizer interface {
	Synthesize(ctx context.Context, text string, language string, voice string) (string, error)
}

type Service struct {
	store            Store
	session          SessionStore
	generator        TheaterGenerator
	tts              SpeechSynthesizer
	jwtSecret        string
	tokenExpiry      time.Duration
	readingMu        sync.RWMutex
	readingMaterials map[string]domain.ReadingMaterial
}

type roleplayEngine interface {
	RoleplayTurn(ctx context.Context, theater domain.Theater, userRole string, transcript []domain.Dialogue, userReply string) (domain.RoleplayTurnEval, error)
	RoleplaySummary(ctx context.Context, theater domain.Theater, transcript []domain.Dialogue, currentScore int) (string, error)
}

func New(store Store, session SessionStore, generator TheaterGenerator, tts SpeechSynthesizer, jwtSecret string) *Service {
	return &Service{
		store:            store,
		session:          session,
		generator:        generator,
		tts:              tts,
		jwtSecret:        jwtSecret,
		tokenExpiry:      2 * time.Hour,
		readingMaterials: map[string]domain.ReadingMaterial{},
	}
}

func (s *Service) Register(email string, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	user, err := s.store.CreateUser(email, string(hash))
	if err != nil {
		return "", err
	}
	accessToken, err := auth.CreateAccessToken(s.jwtSecret, user.ID, user.Email)
	if err == nil && s.session != nil {
		_ = s.session.SetRefreshToken(context.Background(), user.ID, accessToken)
	}
	return accessToken, err
}

func (s *Service) Login(email string, password string) (string, error) {
	user, err := s.store.GetUserByEmail(email)
	if err != nil {
		return "", err
	}
	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	accessToken, err := auth.CreateAccessToken(s.jwtSecret, user.ID, user.Email)
	if err == nil && s.session != nil {
		_ = s.session.SetRefreshToken(context.Background(), user.ID, accessToken)
	}
	return accessToken, err
}

func (s *Service) Refresh(accessToken string) (string, error) {
	claims, err := auth.ParseAccessToken(s.jwtSecret, accessToken)
	if err != nil {
		return "", err
	}
	if s.session != nil {
		stored, getErr := s.session.GetRefreshToken(context.Background(), claims.UserID)
		if getErr != nil || stored == "" || stored != accessToken {
			return "", errors.New("refresh token invalid")
		}
	}
	return auth.CreateAccessToken(s.jwtSecret, claims.UserID, claims.Email)
}

func (s *Service) Logout(userID string) error {
	if s.session == nil {
		return nil
	}
	return s.session.SetRefreshToken(context.Background(), userID, "")
}

func (s *Service) Me(userID string) (domain.User, error) {
	return s.store.GetUserByID(userID)
}

func (s *Service) UpdateProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error) {
	nickname = strings.TrimSpace(nickname)
	avatarURL = strings.TrimSpace(avatarURL)
	bio = strings.TrimSpace(bio)
	return s.store.UpdateUserProfile(userID, nickname, avatarURL, bio)
}

func (s *Service) MeFromToken(token string) (domain.User, error) {
	claims, err := auth.ParseAccessToken(s.jwtSecret, token)
	if err != nil {
		return domain.User{}, err
	}
	return s.Me(claims.UserID)
}

func (s *Service) GenerateTheater(userID string, language string, topic string, difficulty float64, mode string) (domain.Theater, error) {
	requiredQuiz := 2
	if difficulty >= 7.0 {
		requiredQuiz = 3
	}
	preparedTopic := prepareTheaterTopic(language, topic)

	var dialogues []domain.Dialogue
	var quiz []domain.QuizQuestion

	if s.generator == nil {
		log.Printf("generator is nil, use fallback content language=%s topic=%s", language, topic)
		dialogues, quiz = fallbackGeneratedContent(language, topic, requiredQuiz)
	} else {
		generated, q, err := s.generator.Generate(context.Background(), language, preparedTopic, difficulty, mode)
		if err != nil {
			log.Printf("model generate failed err=%v", err)
			return domain.Theater{}, fmt.Errorf("ai generation failed: %w", err)
		} else {
			if len(generated) == 0 || dialogueLooksTemplated(generated) {
				log.Printf("model returned empty or templated content, use fallback content dialogues=%d quiz=%d", len(generated), len(q))
				dialogues, quiz = fallbackGeneratedContent(language, topic, requiredQuiz)
			} else {
				dialogues = generated
				quiz = completeQuizSet(language, topic, q, requiredQuiz)
			}
		}
	}
	if s.tts != nil {
		voicePair := selectDialogueVoicePair(topic)
		for i := range dialogues {
			voiceStyle := voicePair[i%2]
			audioURL, err := s.tts.Synthesize(context.Background(), dialogues[i].Text, language, voiceStyle)
			if err != nil {
				log.Printf("tts failed index=%d err=%v", i, err)
				continue
			}
			if strings.TrimSpace(audioURL) == "" {
				log.Printf("tts returned empty audio url index=%d", i)
				continue
			}
			dialogues[i].AudioURL = audioURL
		}
	} else {
		log.Printf("tts disabled: synthesizer is nil")
	}
	theater := domain.Theater{
		ID:            uuid.NewString(),
		UserID:        userID,
		Language:      language,
		Topic:         topic,
		Difficulty:    difficulty,
		Mode:          mode,
		Status:        "READY",
		Dialogues:     dialogues,
		QuizQuestions: quiz,
		CreatedAt:     time.Now(),
	}
	return s.store.SaveTheater(theater)
}

func prepareTheaterTopic(language string, topic string) string {
	clean := strings.TrimSpace(topic)
	if clean == "" {
		return clean
	}
	if !strings.EqualFold(language, "CANTONESE") {
		return clean
	}
	converted := simplifiedToTraditionalHK(clean)
	return converted + "；请先把这个主题落成一个香港生活中的具体情境，再生成真实对话。"
}

func selectDialogueVoicePair(topic string) [2]string {
	pairs := [][2]string{
		{"甜美女生", "播音男生"},
		{"御姐音色", "沉稳大叔"},
		{"温柔女生", "播音男生"},
	}
	clean := strings.TrimSpace(topic)
	if clean == "" {
		return pairs[0]
	}
	sum := 0
	for _, r := range clean {
		sum += int(r)
	}
	return pairs[sum%len(pairs)]
}

func completeQuizSet(language string, topic string, generated []domain.QuizQuestion, requiredQuiz int) []domain.QuizQuestion {
	if len(generated) >= requiredQuiz {
		return generated[:requiredQuiz]
	}
	result := make([]domain.QuizQuestion, 0, requiredQuiz)
	result = append(result, generated...)
	for _, extra := range fallbackQuizOnly(language, topic) {
		if len(result) >= requiredQuiz {
			break
		}
		duplicate := false
		for _, existing := range result {
			if strings.TrimSpace(existing.Question) == strings.TrimSpace(extra.Question) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, extra)
		}
	}
	if len(result) > requiredQuiz {
		return result[:requiredQuiz]
	}
	return result
}

func dialogueLooksTemplated(dialogues []domain.Dialogue) bool {
	if len(dialogues) == 0 {
		return true
	}
	hits := 0
	for _, dialogue := range dialogues {
		text := strings.ToLower(strings.TrimSpace(dialogue.Text))
		if text == "" {
			continue
		}
		if strings.Contains(text, "today we are discussing") ||
			strings.Contains(text, "welcome to today's mini-theater") ||
			strings.Contains(text, "今日主题") ||
			strings.Contains(text, "欢迎来到今天") ||
			strings.Contains(text, "歡迎來到今天") ||
			strings.Contains(text, "我哋会倾") {
			hits++
		}
	}
	return hits >= 1
}

func simplifiedToTraditionalHK(input string) string {
	replacer := strings.NewReplacer(
		"这", "這",
		"个", "個",
		"们", "們",
		"说", "說",
		"话", "話",
		"点", "點",
		"车", "車",
		"门", "門",
		"后", "後",
		"台", "檯",
		"里", "裡",
		"听", "聽",
		"习", "習",
		"学", "學",
		"场", "場",
		"气", "氣",
		"时", "時",
		"为", "為",
		"来", "來",
		"电", "電",
		"问", "問",
		"应", "應",
		"对", "對",
	)
	return replacer.Replace(strings.TrimSpace(input))
}

func (s *Service) Theater(id string) (domain.Theater, error) {
	return s.store.GetTheater(id)
}

func (s *Service) SharedTheater(shareCode string) (domain.Theater, error) {
	code := strings.ToUpper(strings.TrimSpace(shareCode))
	if code == "" {
		return domain.Theater{}, errors.New("share code is required")
	}
	return s.store.GetTheaterByShareCode(code)
}

func (s *Service) MyTheaters(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	return s.store.ListTheatersByUser(userID, language, status, favorite)
}

func (s *Service) ToggleFavorite(userID string, theaterID string, favorite bool) error {
	return s.store.SetTheaterFavorite(userID, theaterID, favorite)
}

func (s *Service) ShareTheater(userID string, theaterID string) (string, error) {
	theater, err := s.store.GetTheater(theaterID)
	if err != nil {
		return "", err
	}
	if theater.UserID != userID {
		return "", errors.New("theater not found")
	}
	existing := strings.TrimSpace(theater.ShareCode)
	if existing != "" {
		return existing, nil
	}
	shareCode := strings.ToUpper(uuid.NewString()[:8])
	if err := s.store.SetTheaterShareCode(userID, theaterID, shareCode); err != nil {
		return "", err
	}
	return shareCode, nil
}

func (s *Service) DeleteTheater(userID string, theaterID string) error {
	return s.store.DeleteTheater(userID, theaterID)
}

func (s *Service) SubmitAnswers(userID string, theaterID string, answers []string) (domain.PracticeResult, error) {
	theater, err := s.store.GetTheater(theaterID)
	if err != nil {
		return domain.PracticeResult{}, err
	}
	quiz := theater.QuizQuestions
	total := len(quiz)
	if total == 0 {
		return domain.PracticeResult{}, errors.New("该剧场没有听力题，请重新生成小剧场")
	}
	correct := 0
	for i := range quiz {
		userAns := ""
		if i < len(answers) {
			userAns = answers[i]
		}
		if answerMatches(userAns, quiz[i].AnswerKey, theater.Language) {
			correct++
		}
	}
	score := (correct * 100) / total
	xp := calculatePracticeXP(score)
	_ = s.store.AddUserXP(userID, xp)
	feedback := buildPracticeFeedback(correct, total, score)
	_ = s.store.SavePracticeRecord(userID, theaterID, score, answers, xp)
	return domain.PracticeResult{
		Score:        score,
		XPEarned:     xp,
		Feedback:     feedback,
		CorrectCount: correct,
		TotalCount:   total,
	}, nil
}

func (s *Service) SubmitReadingAnswers(userID string, materialID string, answers []string) (domain.PracticeResult, error) {
	material, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		return domain.PracticeResult{}, err
	}
	questions := material.Questions
	total := len(questions)
	if total == 0 {
		return domain.PracticeResult{}, errors.New("该阅读材料没有题目，请重新生成")
	}

	correct := 0
	for i := range questions {
		userAns := ""
		if i < len(answers) {
			userAns = answers[i]
		}
		if answerMatches(userAns, questions[i].AnswerKey, material.Language) {
			correct++
		}
	}

	score := (correct * 100) / total
	xp := calculatePracticeXP(score)
	_ = s.store.AddUserXP(userID, xp)
	_ = s.store.SaveReadingPracticeRecord(userID, materialID, score, answers, xp)

	return domain.PracticeResult{
		Score:        score,
		XPEarned:     xp,
		Feedback:     buildPracticeFeedback(correct, total, score),
		CorrectCount: correct,
		TotalCount:   total,
	}, nil
}

func calculatePracticeXP(score int) int {
	xp := score / 2
	if xp < 1 && score > 0 {
		return 1
	}
	return xp
}

func buildPracticeFeedback(correct int, total int, score int) string {
	feedback := fmt.Sprintf("答对 %d / %d 题。", correct, total)
	if score >= 80 {
		return fmt.Sprintf("答对 %d / %d 题，表现很棒，建议挑战更高难度。", correct, total)
	}
	if score < 40 {
		return fmt.Sprintf("答对 %d / %d 题，建议再听一遍对话后重试。", correct, total)
	}
	return feedback
}

func (s *Service) ListCourses(language string) ([]domain.Course, error) {
	return s.store.ListCourses(language)
}

func (s *Service) ListContentSources(exam string, category string) ([]domain.ContentSource, error) {
	sources := []domain.ContentSource{
		{ID: "s1", Name: "IELTS", Domain: "ielts.org", Category: "IELTS_OFFICIAL", Exam: "IELTS", UseCases: []string{"题型规范", "评分标准"}, ContentMode: "official_spec", Enabled: true, Priority: 1},
		{ID: "s2", Name: "British Council IELTS", Domain: "takeielts.britishcouncil.org", Category: "IELTS_OFFICIAL", Exam: "IELTS", UseCases: []string{"sample questions", "assessment criteria"}, ContentMode: "official_spec", Enabled: true, Priority: 2},
		{ID: "s3", Name: "IDP IELTS", Domain: "ielts.idp.com", Category: "IELTS_OFFICIAL", Exam: "IELTS", UseCases: []string{"speaking format", "practice directions"}, ContentMode: "official_spec", Enabled: true, Priority: 3},
		{ID: "s4", Name: "BBC Learning English", Domain: "bbc.co.uk/learningenglish", Category: "IELTS_READING_LISTENING", Exam: "IELTS", UseCases: []string{"阅读题材", "听力脚本题材"}, ContentMode: "topic_source", Enabled: true, Priority: 4},
		{ID: "s5", Name: "VOA Learning English", Domain: "learningenglish.voanews.com", Category: "IELTS_READING_LISTENING", Exam: "BOTH", UseCases: []string{"新闻题材", "词汇点提取"}, ContentMode: "topic_source", Enabled: true, Priority: 5},
		{ID: "s6", Name: "National Geographic", Domain: "nationalgeographic.com", Category: "IELTS_READING_LISTENING", Exam: "IELTS", UseCases: []string{"科普阅读", "主题延展"}, ContentMode: "topic_source", Enabled: true, Priority: 6},
		{ID: "s7", Name: "CET 官方", Domain: "cet.neea.edu.cn", Category: "CET_OFFICIAL", Exam: "CET", UseCases: []string{"题型分值", "考试说明"}, ContentMode: "official_spec", Enabled: true, Priority: 7},
		{ID: "s8", Name: "NEEA", Domain: "neea.edu.cn", Category: "CET_OFFICIAL", Exam: "CET", UseCases: []string{"政策与成绩说明"}, ContentMode: "official_spec", Enabled: true, Priority: 8},
		{ID: "s9", Name: "China Daily English", Domain: "chinadaily.com.cn", Category: "CET_READING_LISTENING", Exam: "CET", UseCases: []string{"短新闻改写", "长篇阅读题源"}, ContentMode: "topic_source", Enabled: true, Priority: 9},
		{ID: "s10", Name: "Xinhua English", Domain: "english.news.cn", Category: "CET_READING_LISTENING", Exam: "CET", UseCases: []string{"时政题材", "听力素材"}, ContentMode: "topic_source", Enabled: true, Priority: 10},
		{ID: "s11", Name: "Our World in Data", Domain: "ourworldindata.org", Category: "CET_READING_LISTENING", Exam: "CET", UseCases: []string{"数据型阅读"}, ContentMode: "topic_source", Enabled: true, Priority: 11},
		{ID: "s12", Name: "Magoosh IELTS", Domain: "magoosh.com", Category: "METHOD_REFERENCE", Exam: "IELTS", UseCases: []string{"训练流程借鉴"}, ContentMode: "method_reference", Enabled: true, Priority: 12},
		{ID: "s13", Name: "E2 IELTS", Domain: "e2language.com", Category: "METHOD_REFERENCE", Exam: "IELTS", UseCases: []string{"教学结构借鉴"}, ContentMode: "method_reference", Enabled: true, Priority: 13},
		{ID: "s14", Name: "新东方 CET", Domain: "xdf.cn", Category: "METHOD_REFERENCE", Exam: "CET", UseCases: []string{"复习路径借鉴"}, ContentMode: "method_reference", Enabled: true, Priority: 14},
	}

	exam = strings.TrimSpace(strings.ToUpper(exam))
	category = strings.TrimSpace(category)
	filtered := make([]domain.ContentSource, 0, len(sources))
	for _, item := range sources {
		if exam != "" && item.Exam != "BOTH" && item.Exam != exam {
			continue
		}
		if category != "" && item.Category != category {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func (s *Service) GenerateReadingMaterial(userID string, exam string, topic string, level string, sourceIDs []string) (domain.ReadingMaterial, error) {
	exam = strings.TrimSpace(strings.ToUpper(exam))
	level = strings.TrimSpace(level)
	if exam == "" {
		exam = "IELTS"
	}
	if strings.TrimSpace(topic) == "" {
		return domain.ReadingMaterial{}, errors.New("topic is required")
	}
	if level == "" {
		if exam == "CET" {
			level = "intermediate"
		} else {
			level = "upper-intermediate"
		}
	}

	language := "ENGLISH"
	difficulty := 6.5
	if exam == "CET" {
		difficulty = 5.5
	}

	// Reading generation should not pollute theater library.
	quizCount := 5
	generated, q, err := s.generator.Generate(context.Background(), language, fmt.Sprintf("[%s Reading] %s", exam, topic), difficulty, "APPRECIATION")
	if err != nil {
		log.Printf("reading ai generation failed, use fallback passage err=%v", err)
		generated, q = fallbackReadingGeneratedContent(exam, topic, quizCount)
	}
	if len(generated) == 0 {
		log.Printf("reading generation returned empty passage, use fallback passage")
		generated, q = fallbackReadingGeneratedContent(exam, topic, quizCount)
	}
	if len(q) < quizCount {
		log.Printf("reading generation returned insufficient questions, use fallback quiz got=%d required=%d", len(q), quizCount)
		_, q = fallbackReadingGeneratedContent(exam, topic, quizCount)
	}
	if len(q) > quizCount {
		q = q[:quizCount]
	}

	passageParts := make([]string, 0, len(generated))
	for _, d := range generated {
		line := strings.TrimSpace(d.Text)
		if line != "" {
			passageParts = append(passageParts, line)
		}
	}
	passage := strings.Join(passageParts, "\n")
	vocabSet := map[string]struct{}{}
	vocabulary := make([]string, 0, 8)
	for _, word := range strings.Fields(strings.ToLower(passage)) {
		w := strings.Trim(word, ",.!?;:\"'()[]{}")
		if len(w) < 6 {
			continue
		}
		if _, exists := vocabSet[w]; exists {
			continue
		}
		vocabSet[w] = struct{}{}
		vocabulary = append(vocabulary, w)
		if len(vocabulary) >= 8 {
			break
		}
	}

	analysis := domain.ReadingAnalysis{}
	if analyzer, ok := s.generator.(ReadingAnalyzer); ok {
		aiResult, analysisErr := analyzer.AnalyzeReading(context.Background(), exam, topic, passage, vocabulary)
		if analysisErr != nil {
			log.Printf("reading semantic analysis failed, fallback to lightweight defaults err=%v", analysisErr)
		} else {
			analysis = normalizeReadingAnalysis(aiResult, vocabulary, topic)
		}
	}
	if len(analysis.VocabularyItems) == 0 {
		analysis = normalizeReadingAnalysis(domain.ReadingAnalysis{}, vocabulary, topic)
	}

	material := domain.ReadingMaterial{
		ID:                   uuid.NewString(),
		UserID:               userID,
		Exam:                 exam,
		Language:             language,
		Level:                level,
		Topic:                topic,
		Title:                fmt.Sprintf("%s Reading Drill: %s", exam, topic),
		Passage:              passage,
		Vocabulary:           vocabulary,
		Questions:            q,
		SourceIDs:            sourceIDs,
		GenerationNote:       "Generated via AI chain with source-category constraints.",
		AudioStatus:          "PENDING",
		VocabularyItems:      analysis.VocabularyItems,
		AssociationSentences: analysis.AssociationSentences,
		GrammarInsights:      analysis.GrammarInsights,
		CreatedAt:            time.Now(),
	}

	saved, err := s.store.SaveReadingMaterial(material)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	s.cacheReadingMaterial(saved)

	go s.generateReadingAudio(saved.ID, saved.Passage, saved.Language)

	return saved, nil
}

func fallbackReadingGeneratedContent(exam string, topic string, quizCount int) ([]domain.Dialogue, []domain.QuizQuestion) {
	paragraphs := []string{
		fmt.Sprintf("In recent years, educators have paid closer attention to how %s influences student learning outcomes, because reading tasks are no longer judged only by speed, but also by depth of understanding and evidence-based reasoning.", topic),
		"A strong reading routine usually combines previewing, question prediction, and focused scanning, so learners can quickly identify key details while still keeping the main argument in mind.",
		"Researchers also note that vocabulary growth is most effective when words are repeatedly encountered in meaningful contexts, rather than memorized in isolation, which explains why thematic reading units often outperform random drills.",
		"At the same time, digital tools can support progress tracking, yet they can become distracting when learners switch tasks too frequently, reducing sustained attention and weakening long-term retention.",
		"For exam preparation, high-performing students tend to annotate paragraph functions, such as background, evidence, contrast, and conclusion, enabling them to locate answers with greater precision under time pressure.",
		"Teachers therefore recommend a balanced plan that includes timed practice, error analysis, and periodic review, because each stage targets a different cognitive skill needed for accurate comprehension.",
		"Another practical strategy is to compare similar passages from different sources, which helps readers detect shifts in tone, purpose, and author stance, all of which are commonly tested in advanced reading sections.",
		"Ultimately, consistent reflection after each exercise turns reading from a passive activity into an active learning cycle, where students identify weaknesses, adjust methods, and steadily improve performance.",
	}

	dialogues := make([]domain.Dialogue, 0, len(paragraphs))
	for idx, p := range paragraphs {
		dialogues = append(dialogues, domain.Dialogue{
			Speaker:    "Passage",
			Text:       p,
			ZhSubtitle: "段落主旨：围绕阅读能力提升策略与考试表现改进展开。",
			Timestamp:  float64(idx) * 2.1,
		})
	}

	questions := []domain.QuizQuestion{
		{Question: "What is the main focus of the passage?", Options: []string{"Improving reading performance through structured strategies", "Replacing reading with digital media", "Eliminating vocabulary learning", "Reducing exam standards"}, AnswerKey: "Improving reading performance through structured strategies"},
		{Question: "Why are thematic reading units considered effective?", Options: []string{"They avoid repeated exposure", "They provide context for vocabulary use", "They remove the need for review", "They only test grammar"}, AnswerKey: "They provide context for vocabulary use"},
		{Question: "What risk of digital tools is mentioned?", Options: []string{"They always increase retention", "They reduce teacher workload to zero", "They may distract learners from sustained attention", "They prevent learners from taking notes"}, AnswerKey: "They may distract learners from sustained attention"},
		{Question: "What do high-performing students do during exam reading?", Options: []string{"Memorize entire passages", "Ignore paragraph roles", "Annotate functions of paragraphs", "Skip difficult sections"}, AnswerKey: "Annotate functions of paragraphs"},
		{Question: "What is the long-term benefit of post-reading reflection?", Options: []string{"It turns reading into an active improvement cycle", "It removes the need for practice", "It guarantees full marks immediately", "It shortens all passages"}, AnswerKey: "It turns reading into an active improvement cycle"},
	}

	if len(questions) > quizCount {
		questions = questions[:quizCount]
	}
	return dialogues, questions
}

func normalizeReadingAnalysis(in domain.ReadingAnalysis, baseVocabulary []string, topic string) domain.ReadingAnalysis {
	vocab := make([]domain.VocabularyItem, 0, 15)
	seen := map[string]struct{}{}
	for _, item := range in.VocabularyItems {
		word := strings.TrimSpace(item.Word)
		if word == "" {
			continue
		}
		key := strings.ToLower(word)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		meanings := make([]string, 0, len(item.Meanings))
		for _, meaning := range item.Meanings {
			m := strings.TrimSpace(meaning)
			if m != "" && !containsLowQualityTemplate(m) {
				meanings = append(meanings, m)
			}
		}
		if len(meanings) == 0 {
			meanings = fallbackMeaningsByWord(word, topic)
		}
		pos := strings.TrimSpace(item.POS)
		if pos == "" {
			pos = fallbackPOSByWord(word)
		}
		vocab = append(vocab, domain.VocabularyItem{Word: word, POS: pos, Meanings: meanings})
		if len(vocab) >= 15 {
			break
		}
	}

	for _, word := range baseVocabulary {
		w := strings.TrimSpace(word)
		if w == "" {
			continue
		}
		key := strings.ToLower(w)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		vocab = append(vocab, domain.VocabularyItem{
			Word:     w,
			POS:      fallbackPOSByWord(w),
			Meanings: fallbackMeaningsByWord(w, topic),
		})
		if len(vocab) >= 15 {
			break
		}
	}

	association := make([]string, 0, 3)
	associationSeen := map[string]struct{}{}
	for _, sentence := range in.AssociationSentences {
		s := strings.TrimSpace(sentence)
		if s == "" || containsLowQualityTemplate(s) {
			continue
		}
		key := strings.ToLower(s)
		if _, exists := associationSeen[key]; exists {
			continue
		}
		associationSeen[key] = struct{}{}
		association = append(association, s)
		if len(association) >= 3 {
			break
		}
	}
	if len(association) < 3 {
		for _, candidate := range buildAssociationFallbackCandidates(topic, baseVocabulary) {
			key := strings.ToLower(candidate)
			if _, exists := associationSeen[key]; exists {
				continue
			}
			associationSeen[key] = struct{}{}
			association = append(association, candidate)
			if len(association) >= 3 {
				break
			}
		}
	}

	grammar := make([]domain.GrammarInsight, 0, len(in.GrammarInsights))
	for _, gi := range in.GrammarInsights {
		s := strings.TrimSpace(gi.Sentence)
		if s == "" {
			continue
		}
		diff := make([]string, 0, len(gi.DifficultyPoints))
		for _, d := range gi.DifficultyPoints {
			d = strings.TrimSpace(d)
			if d != "" {
				diff = append(diff, d)
			}
		}
		suggestion := make([]string, 0, len(gi.StudySuggestions))
		for _, t := range gi.StudySuggestions {
			t = strings.TrimSpace(t)
			if t != "" {
				suggestion = append(suggestion, t)
			}
		}
		if len(diff) == 0 {
			diff = []string{"句子层级较复杂，建议按主句与从句拆分。"}
		}
		if len(suggestion) == 0 {
			suggestion = []string{"先定位主语和谓语，再补充修饰信息。"}
		}
		grammar = append(grammar, domain.GrammarInsight{Sentence: s, DifficultyPoints: diff, StudySuggestions: suggestion})
	}

	return domain.ReadingAnalysis{
		VocabularyItems:      vocab,
		AssociationSentences: association,
		GrammarInsights:      grammar,
	}
}

func fallbackPOSByWord(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "reading" {
		return "n. 名词"
	}
	switch {
	case strings.HasSuffix(w, "ly"):
		return "adv. 副词"
	case strings.HasSuffix(w, "tion") || strings.HasSuffix(w, "sion") || strings.HasSuffix(w, "ment") || strings.HasSuffix(w, "ity"):
		return "n. 名词"
	case strings.HasSuffix(w, "ous") || strings.HasSuffix(w, "ive") || strings.HasSuffix(w, "able") || strings.HasSuffix(w, "al"):
		return "adj. 形容词"
	case strings.HasSuffix(w, "ing") || strings.HasSuffix(w, "ed") || strings.HasSuffix(w, "ize") || strings.HasSuffix(w, "ate"):
		return "v. 动词"
	default:
		return "n./v. 常见词"
	}
}

func buildAssociationFallbackCandidates(topic string, vocabulary []string) []string {
	topicText := strings.TrimSpace(topic)
	if topicText == "" {
		topicText = "the passage topic"
	}
	first := "key vocabulary"
	second := "context clues"
	third := "main claim"
	if len(vocabulary) > 0 && strings.TrimSpace(vocabulary[0]) != "" {
		first = strings.TrimSpace(vocabulary[0])
	}
	if len(vocabulary) > 1 && strings.TrimSpace(vocabulary[1]) != "" {
		second = strings.TrimSpace(vocabulary[1])
	}
	if len(vocabulary) > 2 && strings.TrimSpace(vocabulary[2]) != "" {
		third = strings.TrimSpace(vocabulary[2])
	}
	return []string{
		"When reading about " + topicText + ", connect " + first + " with " + second + " to infer the author's focus.",
		"Use " + third + " as a signal word, then verify the supporting detail in the next clause.",
		"After each paragraph, summarize one cause-effect link in your own words to reinforce retention.",
	}
}

func fallbackMeaningsByWord(word string, topic string) []string {
	w := strings.ToLower(strings.TrimSpace(word))
	if meanings, ok := readingMeaningDict[w]; ok {
		return meanings
	}
	displayWord := strings.TrimSpace(word)
	if displayWord == "" {
		displayWord = "the term"
	}
	topicHint := ""
	if strings.TrimSpace(topic) != "" {
		topicHint = "（结合“" + strings.TrimSpace(topic) + "”语境）"
	}
	pos := fallbackPOSByWord(w)
	if strings.HasPrefix(pos, "adj") {
		return []string{
			"adj. " + displayWord + " 常用于描述性质或状态" + topicHint,
			"adj. 用法提示：关注 " + displayWord + " 在句中修饰的是对象、过程还是结果。",
		}
	}
	if strings.HasPrefix(pos, "adv") {
		return []string{
			"adv. " + displayWord + " 常表示方式、程度或频率" + topicHint,
			"adv. 用法提示：观察 " + displayWord + " 修饰的动词或整句逻辑。",
		}
	}
	if strings.HasPrefix(pos, "v") {
		return []string{
			"v. " + displayWord + " 在文中多表示动作、过程或变化" + topicHint,
			"v. 用法提示：结合主语与宾语判断 " + displayWord + " 的具体语义。",
		}
	}
	return []string{
		"n. " + displayWord + " 在文中通常指代某个具体概念或对象" + topicHint,
		"n. 用法提示：根据上下文判断 " + displayWord + " 更偏向现象、方法还是结果。",
	}
}

var readingMeaningDict = map[string][]string{
	"context":        {"n. 语境；上下文", "n. 背景；来龙去脉"},
	"analysis":       {"n. 分析；解析", "n. 分解说明；研究结果"},
	"strategy":       {"n. 策略；行动方案", "n.（长期）布局思路"},
	"evidence":       {"n. 证据；依据", "n. 迹象；证明材料"},
	"principle":      {"n. 原则；准则", "n. 原理；基本规律"},
	"approach":       {"n. 方法；路径", "v. 接近；着手处理"},
	"outcome":        {"n. 结果；结局", "n. 产出；成效"},
	"impact":         {"n. 影响；冲击", "v. 对…产生作用"},
	"policy":         {"n. 政策；方针", "n. 保险单（特定语境）"},
	"resource":       {"n. 资源；物力财力", "n. 对策；应对手段"},
	"community":      {"n. 社区；社群", "n. 共同体；群体认同"},
	"sustainable":    {"adj. 可持续的", "adj. 可长期维持的"},
	"innovation":     {"n. 创新；革新", "n. 新方法；新制度"},
	"efficiency":     {"n. 效率；效能", "n. 功效（设备/流程）"},
	"collaboration":  {"n. 协作；合作", "n. 联合创作；协同"},
	"interpretation": {"n. 解释；阐释", "n. 演绎；表演诠释"},
	"practice":       {"n. 实践；练习", "v. 练习；实行"},
	"framework":      {"n. 框架；结构", "n. 体系；基本思路"},
	"pattern":        {"n. 模式；规律", "n. 图案；样板"},
	"insight":        {"n. 洞察；深刻理解", "n. 见解；领悟"},
	"issue":          {"n. 问题；议题", "n.（报刊）期号；发行"},
	"factor":         {"n. 因素；要素", "n. 因子（数学/科学）"},
	"challenge":      {"n. 挑战；难题", "v. 质疑；向…挑战"},
	"solution":       {"n. 解决方案", "n. 溶液（化学）"},
	"reflect":        {"v. 反映；体现", "v. 反思；认真思考"},
	"address":        {"v. 处理；应对", "n. 地址", "v. 向…讲话"},
	"learning":       {"n. 学习过程；学问", "adj. 学习相关的"},
	"reading":        {"n. 阅读；阅读能力", "n. 阅读材料；读物（语境）", "n.（考试）阅读题型"},
	"classroom":      {"n. 教室", "n. 课堂教学场景"},
	"technology":     {"n. 技术；工艺", "n. 科技手段"},
	"attention":      {"n. 注意力", "n. 关注；重视"},
	"comprehension":  {"n. 理解；领会", "n. 阅读理解能力"},
	"recent":         {"adj. 最近的；新近的", "adj. 近代的；近期发生的"},
	"educator":       {"n. 教育工作者", "n. 教育家；教师（语境）"},
	"educators":      {"n. 教育工作者（复数）", "n. 教育者群体"},
	"closer":         {"adj. 更近的；更紧密的", "adv. 更接近地（比较级）"},
	"transportation": {"n. 交通运输", "n. 运输系统；交通方式"},
	"climate":        {"n. 气候", "n. 氛围；环境趋势（引申）"},
	"influence":      {"n. 影响；作用", "v. 影响；对…产生作用"},
	"influences":     {"v. 影响（第三人称单数）", "n. 影响力（复数语境）"},
	"years":          {"n. 年（复数）", "n. 年代；时期（引申）"},
	"urban":          {"adj. 城市的", "adj. 都市化相关的"},
	"students":       {"n. 学生（复数）", "n. 学习者群体"},
	"student":        {"n. 学生", "n. 学习者；研修者"},
	"paid":           {"v. 支付（pay 的过去式/过去分词）", "adj. 有偿的；已付费的"},
	"outcomes":       {"n. 结果（复数）", "n. 学习产出（教育语境）"},
}

func (s *Service) generateReadingAudio(materialID string, text string, language string) {
	if s.tts == nil || strings.TrimSpace(text) == "" {
		if err := s.updateReadingMaterial(materialID, "", func(m *domain.ReadingMaterial) {
			m.AudioStatus = "FAILED"
			m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio generation unavailable")
		}); err != nil {
			log.Printf("reading audio fallback update failed material_id=%s err=%v", materialID, err)
		}
		return
	}

	chunks := splitTextChunks(text, 420)
	audioURLs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		audioURL, err := s.tts.Synthesize(context.Background(), chunk, language, "")
		if err != nil || strings.TrimSpace(audioURL) == "" {
			updateErr := s.updateReadingMaterial(materialID, "", func(m *domain.ReadingMaterial) {
				m.AudioStatus = "FAILED"
				if err != nil {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio error: " + err.Error())
				} else {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio error: empty audio url")
				}
			})
			if updateErr != nil {
				log.Printf("reading audio failure state persist failed material_id=%s err=%v", materialID, updateErr)
			}
			return
		}
		audioURLs = append(audioURLs, strings.TrimSpace(audioURL))
	}

	if err := s.updateReadingMaterial(materialID, "", func(m *domain.ReadingMaterial) {
		m.AudioStatus = "READY"
		m.AudioURLs = audioURLs
		if len(audioURLs) > 0 {
			m.AudioURL = audioURLs[0]
		}
	}); err != nil {
		log.Printf("reading audio ready state persist failed material_id=%s err=%v", materialID, err)
	}
}

func splitTextChunks(text string, maxLen int) []string {
	clean := strings.TrimSpace(text)
	if clean == "" || maxLen <= 0 {
		return []string{}
	}
	if len([]rune(clean)) <= maxLen {
		return []string{clean}
	}

	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '\n' || r == '。' || r == '.' || r == '!' || r == '?' || r == '；' || r == ';'
	})
	chunks := make([]string, 0)
	current := ""
	for _, p := range parts {
		piece := strings.TrimSpace(p)
		if piece == "" {
			continue
		}
		candidate := piece
		if current != "" {
			candidate = current + "。" + piece
		}
		if len([]rune(candidate)) > maxLen {
			if current != "" {
				chunks = append(chunks, current)
				current = piece
			} else {
				runes := []rune(piece)
				for len(runes) > maxLen {
					chunks = append(chunks, string(runes[:maxLen]))
					runes = runes[maxLen:]
				}
				current = string(runes)
			}
		} else {
			current = candidate
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	if len(chunks) == 0 {
		return []string{clean}
	}
	return chunks
}

func (s *Service) ReadingMaterials(userID string, exam string) ([]domain.ReadingMaterial, error) {
	exam = strings.TrimSpace(strings.ToUpper(exam))
	result, err := s.store.ListReadingMaterialsByUser(userID, exam)
	if err != nil {
		return nil, err
	}
	for _, item := range result {
		s.cacheReadingMaterial(item)
	}
	return result, nil
}

func (s *Service) ReadingMaterial(userID string, materialID string) (domain.ReadingMaterial, error) {
	item, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	s.cacheReadingMaterial(item)

	if needsReadingAnalysis(item) {
		analysis := domain.ReadingAnalysis{}
		if analyzer, supports := s.generator.(ReadingAnalyzer); supports {
			aiResult, err := analyzer.AnalyzeReading(context.Background(), item.Exam, item.Topic, item.Passage, item.Vocabulary)
			if err != nil {
				log.Printf("reading detail semantic backfill failed, fallback to dictionary mode err=%v", err)
			} else {
				analysis = aiResult
			}
		}
		normalized := normalizeReadingAnalysis(analysis, item.Vocabulary, item.Topic)
		item.VocabularyItems = normalized.VocabularyItems
		item.AssociationSentences = normalized.AssociationSentences
		item.GrammarInsights = normalized.GrammarInsights
		saved, saveErr := s.store.SaveReadingMaterial(item)
		if saveErr != nil {
			return domain.ReadingMaterial{}, saveErr
		}
		s.cacheReadingMaterial(saved)
		item = saved
	}
	return item, nil
}

func (s *Service) cacheReadingMaterial(material domain.ReadingMaterial) {
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	s.readingMaterials[material.ID] = material
}

func (s *Service) updateReadingMaterial(materialID string, userID string, mutate func(*domain.ReadingMaterial)) error {
	material, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		return err
	}
	mutate(&material)
	saved, err := s.store.SaveReadingMaterial(material)
	if err != nil {
		return err
	}
	s.cacheReadingMaterial(saved)
	return nil
}

func needsReadingAnalysis(item domain.ReadingMaterial) bool {
	if len(item.VocabularyItems) < 15 {
		return true
	}
	if len(item.AssociationSentences) < 3 {
		return true
	}
	if len(item.GrammarInsights) == 0 {
		return true
	}
	for _, v := range item.VocabularyItems {
		for _, m := range v.Meanings {
			if containsLowQualityTemplate(m) {
				return true
			}
		}
	}
	for _, s := range item.AssociationSentences {
		if containsLowQualityTemplate(s) {
			return true
		}
	}
	return false
}

func containsLowQualityTemplate(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	low := strings.ToLower(trimmed)
	templates := []string{
		"常见义：该词通常表示对象、概念或现象",
		"常见义：该词在阅读语境中表示核心概念或关键对象",
		"常见义：该词在阅读中通常表示核心概念或关键对象",
		"引申义：可表示与主题相关的抽象意义",
		"引申义：可表示相关方法、影响或结果",
		"引申义：可进一步表示相关的方法、影响或结果",
		"readers can connect key vocabulary to",
		"and retell one complete idea accurately",
	}
	for _, t := range templates {
		if strings.Contains(low, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func (s *Service) StartRoleplay(userID string, theaterID string, userRole string) (domain.RoleplaySession, error) {
	theater, err := s.store.GetTheater(theaterID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(theater.Mode), "ROLEPLAY") {
		return domain.RoleplaySession{}, errors.New("当前剧场不是角色扮演模式")
	}
	session := domain.RoleplaySession{
		ID:         uuid.NewString(),
		UserID:     userID,
		TheaterID:  theaterID,
		UserRole:   userRole,
		TurnIndex:  0,
		Status:     "active",
		Transcript: []domain.Dialogue{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	opening := "你好，我们开始角色扮演。请先用一句话介绍你的立场。"
	openingZh := "你好，我们开始角色扮演。请先用一句话介绍你的立场。"
	if strings.EqualFold(strings.TrimSpace(theater.Language), "ENGLISH") {
		opening = "Hi, let's start the roleplay. Please introduce your position in one sentence."
		openingZh = "你好，我们开始角色扮演。请先用一句话介绍你的立场。"
	}
	if engine, ok := any(s.generator).(roleplayEngine); ok {
		if eval, e := engine.RoleplayTurn(context.Background(), theater, userRole, session.Transcript, ""); e == nil && strings.TrimSpace(eval.AssistantReply) != "" {
			opening = eval.AssistantReply
			if strings.TrimSpace(eval.AssistantZhSub) != "" {
				openingZh = eval.AssistantZhSub
			}
		}
	}
	session.Transcript = append(session.Transcript, domain.Dialogue{
		Speaker:    "AI-Role",
		Text:       opening,
		ZhSubtitle: openingZh,
		AudioURL:   "",
		Timestamp:  0,
	})
	return s.store.CreateRoleplaySession(session)
}

func (s *Service) GetRoleplaySession(userID string, sessionID string) (domain.RoleplaySession, error) {
	return s.store.GetRoleplaySession(sessionID, userID)
}

func (s *Service) SubmitRoleplayReply(userID string, sessionID string, text string) (domain.RoleplaySession, error) {
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	theater, terr := s.store.GetTheater(session.TheaterID)
	if terr != nil {
		return domain.RoleplaySession{}, terr
	}
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return domain.RoleplaySession{}, errors.New("回复内容不能为空")
	}

	session.TurnIndex++
	session.Transcript = append(session.Transcript, domain.Dialogue{
		Speaker:   "USER",
		Text:      cleanText,
		AudioURL:  "",
		Timestamp: float64(session.TurnIndex),
	})

	eval := fallbackRoleplayTurn(theater.Language, cleanText)
	if engine, ok := any(s.generator).(roleplayEngine); ok {
		if generated, e := engine.RoleplayTurn(context.Background(), theater, session.UserRole, session.Transcript, cleanText); e == nil {
			eval = generated
		}
	}
	if session.TurnIndex > 0 {
		session.CurrentScore = ((session.CurrentScore * (session.TurnIndex - 1)) + eval.Total) / session.TurnIndex
	}
	coach := buildTurnFeedbackText(theater.Language, eval)
	session.Transcript = append(session.Transcript, domain.Dialogue{
		Speaker:    "AI-Role",
		Text:       coach,
		ZhSubtitle: eval.AssistantZhSub,
		AudioURL:   "",
		Timestamp:  float64(session.TurnIndex) + 0.3,
	})
	session.UpdatedAt = time.Now()
	return s.store.UpdateRoleplaySession(session)
}

func (s *Service) EndRoleplay(userID string, sessionID string) (domain.RoleplaySession, error) {
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	session.Status = "completed"
	th, terr := s.store.GetTheater(session.TheaterID)
	if terr != nil {
		return domain.RoleplaySession{}, terr
	}
	if engine, ok := any(s.generator).(roleplayEngine); ok {
		if summary, e := engine.RoleplaySummary(context.Background(), th, session.Transcript, session.CurrentScore); e == nil && strings.TrimSpace(summary) != "" {
			session.FinalFeedback = summary
		}
	}
	if strings.TrimSpace(session.FinalFeedback) == "" {
		session.FinalFeedback = fallbackRoleplaySummary(th.Language, session.CurrentScore)
	}
	session.UpdatedAt = time.Now()
	xp := 20 + session.CurrentScore/5
	_ = s.store.AddUserXP(userID, xp)
	return s.store.UpdateRoleplaySession(session)
}

func buildTurnFeedbackText(language string, eval domain.RoleplayTurnEval) string {
	_ = language
	return fmt.Sprintf("%s\n\n本轮评分：相关性 %d/40，准确性 %d/30，自然度 %d/30，总分 %d/100。\n改进建议：%s", eval.AssistantReply, eval.Relevance, eval.Accuracy, eval.Naturalness, eval.Total, eval.Feedback)
}

func fallbackRoleplayTurn(language string, userReply string) domain.RoleplayTurnEval {
	wordCount := len(strings.Fields(userReply))
	if wordCount == 0 {
		wordCount = len([]rune(strings.TrimSpace(userReply))) / 2
	}
	relevance := min(40, 18+wordCount*2)
	accuracy := min(30, 14+wordCount)
	naturalness := min(30, 12+wordCount)
	total := relevance + accuracy + naturalness
	if strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		return domain.RoleplayTurnEval{
			AssistantReply: "Thanks. Could you give one concrete example from your own experience?",
			AssistantZhSub: "收到。你可以结合自己的经历给一个具体例子吗？",
			Relevance:      relevance,
			Accuracy:       accuracy,
			Naturalness:    naturalness,
			Total:          total,
			Feedback:       "建议引用一个场景关键词，并把回答控制在一到两句。",
		}
	}
	return domain.RoleplayTurnEval{
		AssistantReply: "收到。可唔可以补充一个更具体嘅情境例子？",
		AssistantZhSub: "收到。你可以补充一个更具体的情境例子吗？",
		Relevance:      relevance,
		Accuracy:       accuracy,
		Naturalness:    naturalness,
		Total:          total,
		Feedback:       "建议加入场景关键词，并把句子控制在一到两句。",
	}
}

func fallbackRoleplaySummary(language string, score int) string {
	if strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		return fmt.Sprintf("Overall score: %d/100. You stayed engaged in multi-turn conversation. Next: 1) improve grammar precision in long sentences; 2) add scenario-specific vocabulary. Sample upgrade: I would prioritize customer clarity before offering alternatives.", score)
	}
	return fmt.Sprintf("总评：%d/100。你完成了多轮互动并保持了上下文连贯。下一步建议：1）提升长句语法准确性；2）增加场景关键词密度。示例优化句：我会先确认对方需求，再给出两个可行选项。", score)
}

func fallbackGeneratedContent(language string, topic string, requiredQuiz int) ([]domain.Dialogue, []domain.QuizQuestion) {
	lang := strings.ToUpper(strings.TrimSpace(language))
	if requiredQuiz >= 5 && strings.Contains(strings.ToLower(topic), "reading") {
		return fallbackReadingContent(topic, requiredQuiz)
	}
	dialogues := make([]domain.Dialogue, 0, 8)
	if lang == "ENGLISH" {
		scenario := strings.TrimSpace(topic)
		if scenario == "" {
			scenario = "a delayed morning commute"
		}
		lines := []string{
			fmt.Sprintf("Morning, I just got a message that our usual route is delayed, and it affects %s.", scenario),
			"Got it. What time do you need to arrive, and what's your backup option right now?",
			"I need to be there by 8:40, and the fastest backup seems to be bus 23 plus a short walk.",
			"Can you estimate the transfer time, so we can decide whether to call ahead?",
			"If traffic is normal, the transfer takes about 12 minutes; otherwise it could be 20.",
			"Then let's send a quick update first and confirm whether a five-minute delay is acceptable.",
			"Done. They said a short delay is okay if we share the revised arrival time now.",
			"Perfect. Keep this pattern: clarify constraints, compare options, then confirm next action.",
		}
		for i, text := range lines {
			dialogues = append(dialogues, domain.Dialogue{
				Speaker:    map[bool]string{i%2 == 0: "Coordinator", i%2 != 0: "Learner"}[true],
				Text:       text,
				ZhSubtitle: "场景化沟通练习句，强调真实决策流程。",
				Timestamp:  float64(i) * 2.0,
			})
		}
		quiz := []domain.QuizQuestion{
			{Question: "What is the learner's target arrival time?", Options: []string{"8:10", "8:25", "8:40", "9:00"}, AnswerKey: "8:40"},
			{Question: "Which backup route is mentioned?", Options: []string{"Train line A only", "Bus 23 plus a short walk", "Taxi with no transfer", "Bike sharing only"}, AnswerKey: "Bus 23 plus a short walk"},
			{Question: "What action do they take before finalizing the route?", Options: []string{"Cancel the appointment", "Wait without notifying anyone", "Send an update and confirm delay acceptance", "Switch to a completely different destination"}, AnswerKey: "Send an update and confirm delay acceptance"},
		}
		for len(quiz) < requiredQuiz {
			quiz = append(quiz, domain.QuizQuestion{
				Question: fmt.Sprintf("According to the dialogue, what is the best summary #%d?", len(quiz)+1),
				Options: []string{
					"The speakers avoid the issue.",
					"They focus on practical communication steps.",
					"They mainly discuss travel planning.",
					"They decide to cancel the conversation.",
				},
				AnswerKey: "They focus on practical communication steps.",
			})
		}
		return dialogues, quiz[:min(requiredQuiz, len(quiz))]
	}

	scene := strings.TrimSpace(topic)
	if scene == "" {
		scene = "朝早通勤安排"
	}
	lines := []string{
		fmt.Sprintf("喂，我啱啱收到通知，原本条线延误，会影响到%s。", scene),
		"明白，你最迟几点要到？而家有冇后备路线？",
		"我要八点四十前到，后备可以转 23 号巴士再行一段路。",
		"转车大概要几耐？我哋要唔要先同对方报备？",
		"顺利就十二分钟，塞车可能去到二十分钟。",
		"咁我建议先发讯息说明，再确认对方可唔可以接受五分钟内延迟。",
		"我已经发咗，对方话只要即时报预计到达时间就得。",
		"好，这个流程记住：先问限制，再比方案，最后确认下一步。",
	}
	for i, text := range lines {
		speaker := "教练"
		if i%2 == 1 {
			speaker = "学员"
		}
		dialogues = append(dialogues, domain.Dialogue{Speaker: speaker, Text: text, ZhSubtitle: text, Timestamp: float64(i) * 2.0})
	}
	quiz := []domain.QuizQuestion{
		{Question: "说话人最迟要几点前到达？", Options: []string{"八点二十", "八点四十", "九点整", "九点十五"}, AnswerKey: "八点四十"},
		{Question: "后备路线是什么？", Options: []string{"直接坐地铁到底", "转 23 号巴士再步行", "改坐的士不转车", "取消行程"}, AnswerKey: "转 23 号巴士再步行"},
		{Question: "他们在确定路线前先做了什么？", Options: []string{"先取消约会", "先和对方报备并确认延迟可接受", "先等十分钟不处理", "先换去其他地点"}, AnswerKey: "先和对方报备并确认延迟可接受"},
	}
	for len(quiz) < requiredQuiz {
		quiz = append(quiz, domain.QuizQuestion{
			Question: fmt.Sprintf("根据对话内容，最恰当的总结是第%d项？", len(quiz)+1),
			Options: []string{
				"回避沟通细节更有效",
				"聚焦重点并逐步确认细节",
				"只讨论天气变化",
				"立即终止对话",
			},
			AnswerKey: "聚焦重点并逐步确认细节",
		})
	}
	return dialogues, quiz[:min(requiredQuiz, len(quiz))]
}

func fallbackReadingContent(topic string, requiredQuiz int) ([]domain.Dialogue, []domain.QuizQuestion) {
	cleanTopic := strings.TrimSpace(topic)
	segments := []struct {
		text string
		zh   string
	}{
		{
			text: fmt.Sprintf("In recent years, the public debate around %s has moved from specialist circles into mainstream policy design. Researchers argue that the issue can no longer be described as a single technical problem, because it combines economic incentives, behavioral habits, and institutional constraints. This shift matters for exam candidates because modern reading passages often test whether readers can identify multi-causal explanations rather than linear cause-and-effect narratives.", cleanTopic),
			zh:   "该段说明该主题已从单一技术问题转向多因素综合议题。",
		},
		{
			text: "One influential school of thought emphasizes that individual choices are highly sensitive to context. When citizens face uncertain information, they often rely on simple heuristics, such as following peers or repeating familiar routines. These heuristics are efficient in daily life but can produce systematic bias in long-term decisions. Policymakers therefore design interventions that reduce cognitive burden, for example by presenting default options and clearer comparison frameworks.",
			zh:   "该段强调个体决策受情境影响，并解释启发式决策的利弊。",
		},
		{
			text: "A competing perspective warns that context-based interventions may produce short-term compliance without durable understanding. According to this view, people adapt quickly to new interfaces while leaving deeper assumptions unchanged. As a result, the initial gains may fade when incentives weaken or social norms evolve. Proponents of this perspective recommend combining immediate nudges with long-term education, so that procedural changes are reinforced by conceptual learning.",
			zh:   "该段提出反方观点：仅靠情境干预可能难以形成长期效果。",
		},
		{
			text: "Historical comparisons provide useful evidence for both positions. In several countries, pilot programs delivered rapid improvements during the first year, particularly where implementation teams monitored feedback weekly. Yet longitudinal data revealed uneven outcomes across regions, suggesting that local governance capacity and trust in institutions played decisive roles. This pattern illustrates a common exam theme: identical policy instruments can yield divergent results when background conditions differ.",
			zh:   "该段通过跨国历史比较说明同一政策在不同地区效果不一。",
		},
		{
			text: "Methodologically, scholars caution against overinterpreting headline statistics. Aggregate indicators can conceal distributional effects, meaning that average progress may coexist with widening inequality among subgroups. To address this limitation, recent studies increasingly integrate qualitative interviews with quantitative tracking. By triangulating multiple data sources, researchers can detect hidden trade-offs and provide more actionable recommendations for practitioners and administrators.",
			zh:   "该段强调研究方法需避免仅看平均值，并倡导多源证据。",
		},
		{
			text: "For test takers, a practical reading strategy is to map each paragraph to a function before answering questions: background framing, mechanism explanation, counterargument, evidence, methodological caution, and implication. This functional mapping prevents confusion when options include partially true statements. In high-level exams, distractors often recycle vocabulary from the passage while subtly changing logical relationships, so structural understanding is usually more reliable than keyword matching.",
			zh:   "该段给出应试策略：先识别段落功能，再处理选项干扰。",
		},
		{
			text: fmt.Sprintf("Looking ahead, analysts expect the next phase of work on %s to focus on adaptive governance. Instead of fixed annual plans, institutions may adopt iterative cycles with rapid experimentation, transparent reporting, and stakeholder negotiation. Such frameworks require stronger interdisciplinary communication, because legal feasibility, economic efficiency, and social acceptance must be evaluated simultaneously. This final point reinforces the passage's core message: durable progress depends on coordinated systems rather than isolated actions.", cleanTopic),
			zh:   "该段展望未来治理趋势，强调协同系统比单点行动更关键。",
		},
	}
	dialogues := make([]domain.Dialogue, 0, len(segments))
	for i, seg := range segments {
		dialogues = append(dialogues, domain.Dialogue{
			Speaker:    "Passage",
			Text:       seg.text,
			ZhSubtitle: seg.zh,
			Timestamp:  float64(i) * 2,
		})
	}

	quiz := []domain.QuizQuestion{
		{Question: "What is the central argument of the passage?", Options: []string{"The topic is purely technical and easy to solve", "Durable progress requires coordinated systems and context-aware design", "Short-term incentives always guarantee long-term outcomes", "Quantitative data should replace interviews in all studies"}, AnswerKey: "Durable progress requires coordinated systems and context-aware design"},
		{Question: "Why do the authors discuss heuristics in decision-making?", Options: []string{"To show that people never make rational choices", "To explain why context can shape choices but also create bias", "To reject all behavior-based interventions", "To argue that defaults are ineffective"}, AnswerKey: "To explain why context can shape choices but also create bias"},
		{Question: "What limitation of aggregate indicators is highlighted?", Options: []string{"They are too expensive to collect", "They cannot be compared across countries", "They may hide unequal outcomes within subgroups", "They always underestimate policy success"}, AnswerKey: "They may hide unequal outcomes within subgroups"},
		{Question: "Which reading strategy is recommended for exam candidates?", Options: []string{"Memorize every number in the passage", "Translate each sentence literally before answering", "Map paragraph functions before evaluating options", "Answer quickly based on repeated keywords"}, AnswerKey: "Map paragraph functions before evaluating options"},
		{Question: "How does the passage characterize future governance?", Options: []string{"More rigid annual plans with less feedback", "Adaptive cycles with experimentation and transparent reporting", "A return to single-discipline decision making", "Complete replacement of institutions by individuals"}, AnswerKey: "Adaptive cycles with experimentation and transparent reporting"},
	}
	for len(quiz) < requiredQuiz {
		quiz = append(quiz, domain.QuizQuestion{
			Question: fmt.Sprintf("According to the passage, which statement is best supported? (#%d)", len(quiz)+1),
			Options: []string{
				"Policy outcomes are identical across all regions.",
				"Structural understanding is often more reliable than keyword matching.",
				"Long-term education has no role in behavior change.",
				"Exam passages avoid counterarguments.",
			},
			AnswerKey: "Structural understanding is often more reliable than keyword matching.",
		})
	}
	return dialogues, quiz[:min(requiredQuiz, len(quiz))]
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
