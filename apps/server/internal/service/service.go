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
	ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error)
	SetTheaterFavorite(userID string, theaterID string, favorite bool) error
	SetTheaterShareCode(userID string, theaterID string, shareCode string) error
	DeleteTheater(userID string, theaterID string) error
	AddUserXP(userID string, xp int) error
	SavePracticeRecord(userID string, theaterID string, score int, answers []string, xpEarned int) error
	ListCourses(language string) ([]domain.Course, error)
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

	var dialogues []domain.Dialogue
	var quiz []domain.QuizQuestion

	if s.generator == nil {
		log.Printf("generator is nil, use fallback content language=%s topic=%s", language, topic)
		dialogues, quiz = fallbackGeneratedContent(language, topic, requiredQuiz)
	} else {
		generated, q, err := s.generator.Generate(context.Background(), language, topic, difficulty, mode)
		if err != nil {
			log.Printf("model generate failed, use fallback content err=%v", err)
			dialogues, quiz = fallbackGeneratedContent(language, topic, requiredQuiz)
		} else {
			if len(generated) == 0 || len(q) < requiredQuiz {
				log.Printf("model returned incomplete content, use fallback content dialogues=%d quiz=%d requiredQuiz=%d", len(generated), len(q), requiredQuiz)
				dialogues, quiz = fallbackGeneratedContent(language, topic, requiredQuiz)
			} else {
				dialogues = generated
				quiz = q[:requiredQuiz]
			}
		}
	}
	if s.tts != nil {
		for i := range dialogues {
			audioURL, err := s.tts.Synthesize(context.Background(), dialogues[i].Text, language, "")
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

func (s *Service) Theater(id string) (domain.Theater, error) {
	return s.store.GetTheater(id)
}

func (s *Service) MyTheaters(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	return s.store.ListTheatersByUser(userID, language, status, favorite)
}

func (s *Service) ToggleFavorite(userID string, theaterID string, favorite bool) error {
	return s.store.SetTheaterFavorite(userID, theaterID, favorite)
}

func (s *Service) ShareTheater(userID string, theaterID string) (string, error) {
	shareCode := uuid.NewString()[:8]
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
	xp := score / 2
	if xp < 1 && score > 0 {
		xp = 1
	}
	_ = s.store.AddUserXP(userID, xp)
	feedback := fmt.Sprintf("答对 %d / %d 题。", correct, total)
	if score >= 80 {
		feedback = fmt.Sprintf("答对 %d / %d 题，表现很棒，建议挑战更高难度。", correct, total)
	} else if score < 40 {
		feedback = fmt.Sprintf("答对 %d / %d 题，建议再听一遍对话后重试。", correct, total)
	}
	_ = s.store.SavePracticeRecord(userID, theaterID, score, answers, xp)
	return domain.PracticeResult{
		Score:        score,
		XPEarned:     xp,
		Feedback:     feedback,
		CorrectCount: correct,
		TotalCount:   total,
	}, nil
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
	if err != nil || len(generated) == 0 || len(q) < quizCount {
		log.Printf("reading generation degraded to fallback err=%v dialogues=%d quiz=%d", err, len(generated), len(q))
		generated, q = fallbackGeneratedContent(language, fmt.Sprintf("[%s Reading] %s", exam, topic), quizCount)
	}
	if len(generated) == 0 {
		return domain.ReadingMaterial{}, errors.New("reading generation failed: empty passage")
	}
	if len(q) > quizCount {
		q = q[:quizCount]
	}
	if len(q) < quizCount {
		return domain.ReadingMaterial{}, errors.New("reading generation failed: insufficient questions")
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

	material := domain.ReadingMaterial{
		ID:             uuid.NewString(),
		UserID:         userID,
		Exam:           exam,
		Language:       language,
		Level:          level,
		Topic:          topic,
		Title:          fmt.Sprintf("%s Reading Drill: %s", exam, topic),
		Passage:        passage,
		Vocabulary:     vocabulary,
		Questions:      q,
		SourceIDs:      sourceIDs,
		GenerationNote: "Generated via AI chain with source-category constraints.",
		AudioStatus:    "PENDING",
		CreatedAt:      time.Now(),
	}

	s.readingMu.Lock()
	s.readingMaterials[material.ID] = material
	s.readingMu.Unlock()

	go s.generateReadingAudio(material.ID, material.Passage, material.Language)

	return material, nil
}

func (s *Service) generateReadingAudio(materialID string, text string, language string) {
	if s.tts == nil || strings.TrimSpace(text) == "" {
		s.readingMu.Lock()
		m := s.readingMaterials[materialID]
		m.AudioStatus = "FAILED"
		m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio generation unavailable")
		s.readingMaterials[materialID] = m
		s.readingMu.Unlock()
		return
	}

	chunks := splitTextChunks(text, 420)
	audioURLs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		audioURL, err := s.tts.Synthesize(context.Background(), chunk, language, "")
		if err != nil || strings.TrimSpace(audioURL) == "" {
			s.readingMu.Lock()
			m, ok := s.readingMaterials[materialID]
			if ok {
				m.AudioStatus = "FAILED"
				if err != nil {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio error: " + err.Error())
				}
				s.readingMaterials[materialID] = m
			}
			s.readingMu.Unlock()
			return
		}
		audioURLs = append(audioURLs, strings.TrimSpace(audioURL))
	}

	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	m, ok := s.readingMaterials[materialID]
	if !ok {
		return
	}
	m.AudioStatus = "READY"
	m.AudioURLs = audioURLs
	if len(audioURLs) > 0 {
		m.AudioURL = audioURLs[0]
	}
	s.readingMaterials[materialID] = m
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
	s.readingMu.RLock()
	defer s.readingMu.RUnlock()
	result := make([]domain.ReadingMaterial, 0)
	for _, item := range s.readingMaterials {
		if item.UserID != userID {
			continue
		}
		if exam != "" && item.Exam != exam {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *Service) ReadingMaterial(userID string, materialID string) (domain.ReadingMaterial, error) {
	s.readingMu.RLock()
	defer s.readingMu.RUnlock()
	item, ok := s.readingMaterials[materialID]
	if !ok || item.UserID != userID {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	return item, nil
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
		Speaker:   "AI-Role",
		Text:      opening,
		ZhSubtitle: openingZh,
		AudioURL:  "",
		Timestamp: 0,
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
		Speaker:   "AI-Role",
		Text:      coach,
		ZhSubtitle: eval.AssistantZhSub,
		AudioURL:  "",
		Timestamp: float64(session.TurnIndex) + 0.3,
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
		lines := []string{
			fmt.Sprintf("Today we are discussing: %s.", topic),
			"Could you share one concrete situation from your daily life?",
			"Sure, I usually face this when I commute in the morning.",
			"What challenge appears most frequently in that situation?",
			"Time pressure and unclear communication are the biggest issues.",
			"How do you usually solve them in a practical way?",
			"I prioritize key information first, then confirm details step by step.",
			"Great. Let's summarize the key lesson in one sentence.",
		}
		for i, text := range lines {
			dialogues = append(dialogues, domain.Dialogue{
				Speaker:    map[bool]string{i%2 == 0: "Coach", i%2 != 0: "Learner"}[true],
				Text:       text,
				ZhSubtitle: "本句用于训练阅读与听力理解。",
				Timestamp:  float64(i) * 2.0,
			})
		}
		quiz := []domain.QuizQuestion{
			{Question: "What is the main topic of this dialogue?", Options: []string{"Morning exercise", topic, "Job interview", "Online shopping"}, AnswerKey: topic},
			{Question: "What challenge is mentioned most frequently?", Options: []string{"Budget limits", "Time pressure and unclear communication", "Weather changes", "Technical failure"}, AnswerKey: "Time pressure and unclear communication"},
			{Question: "What strategy does the learner use?", Options: []string{"Ignore details", "Ask someone else", "Prioritize key information then confirm details", "Change the topic"}, AnswerKey: "Prioritize key information then confirm details"},
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

	lines := []string{
		fmt.Sprintf("今日主题系：%s。", topic),
		"你可唔可以讲一个生活入面常见嘅具体情境？",
		"可以，我朝早通勤嗰阵最容易遇到呢个问题。",
		"你最常遇到嘅困难系咩？",
		"时间紧迫，同埋信息沟通唔够清晰。",
		"你通常点样处理先最有效？",
		"我会先讲重点，再逐步确认细节。",
		"好，总结一句你今日学到嘅关键策略。",
	}
	for i, text := range lines {
		speaker := "教练"
		if i%2 == 1 {
			speaker = "学员"
		}
		dialogues = append(dialogues, domain.Dialogue{Speaker: speaker, Text: text, ZhSubtitle: text, Timestamp: float64(i) * 2.0})
	}
	quiz := []domain.QuizQuestion{
		{Question: "本段对话的核心主题是什么？", Options: []string{"旅行安排", topic, "面试技巧", "运动计划"}, AnswerKey: topic},
		{Question: "对话中提到的主要困难是什么？", Options: []string{"预算不足", "时间紧迫和沟通不清", "天气变化", "设备故障"}, AnswerKey: "时间紧迫和沟通不清"},
		{Question: "学员采用了哪种处理策略？", Options: []string{"先回避问题", "先讲重点再确认细节", "让别人决定", "直接结束对话"}, AnswerKey: "先讲重点再确认细节"},
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
