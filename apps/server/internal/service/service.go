package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
	store       Store
	session     SessionStore
	generator   TheaterGenerator
	tts         SpeechSynthesizer
	jwtSecret   string
	tokenExpiry time.Duration
}

type roleplayEngine interface {
	RoleplayTurn(ctx context.Context, theater domain.Theater, userRole string, transcript []domain.Dialogue, userReply string) (domain.RoleplayTurnEval, error)
	RoleplaySummary(ctx context.Context, theater domain.Theater, transcript []domain.Dialogue, currentScore int) (string, error)
}

func New(store Store, session SessionStore, generator TheaterGenerator, tts SpeechSynthesizer, jwtSecret string) *Service {
	return &Service{
		store:       store,
		session:     session,
		generator:   generator,
		tts:         tts,
		jwtSecret:   jwtSecret,
		tokenExpiry: 2 * time.Hour,
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
	if s.generator == nil {
		return domain.Theater{}, errors.New("剧情生成服务未配置，请联系管理员")
	}
	generated, q, err := s.generator.Generate(context.Background(), language, topic, difficulty, mode)
	if err != nil {
		return domain.Theater{}, fmt.Errorf("剧情生成失败: %w", err)
	}
	if len(generated) == 0 {
		return domain.Theater{}, errors.New("剧情生成失败：模型未返回有效对话")
	}
	requiredQuiz := 2
	if difficulty >= 7.0 {
		requiredQuiz = 3
	}
	if len(q) < requiredQuiz {
		return domain.Theater{}, errors.New("剧情生成失败：模型未返回完整题目")
	}
	dialogues := generated
	quiz := q[:requiredQuiz]
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

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
