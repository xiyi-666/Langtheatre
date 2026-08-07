package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/analytics"
	"github.com/linguaquest/server/internal/auth"
	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
	"github.com/linguaquest/server/internal/ielts"
	"golang.org/x/crypto/bcrypt"
)

type Store interface {
	CreateUser(username string, email string, passwordHash string, emailVerified bool) (domain.User, error)
	ListUsersByEmail(email string) ([]domain.User, error)
	GetUserByUsername(username string) (domain.User, error)
	GetUserByID(id string) (domain.User, error)
	UpdateUserPassword(userID string, passwordHash string) error
	SetUserEmailVerified(userID string, verified bool) error
	CreateAuthToken(userID string, purpose string, tokenHash string, expiresAt time.Time) error
	ConsumeAuthToken(tokenHash string, purpose string, now time.Time) (string, error)
	UpdateUserProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error)
	GetModelConfig() (domain.ModelConfig, error)
	SaveModelConfig(config domain.ModelConfig) (domain.ModelConfig, error)
	GetTTSConfig() (domain.TTSConfig, error)
	SaveTTSConfig(config domain.TTSConfig) (domain.TTSConfig, error)
	GetASRConfig() (domain.ASRConfig, error)
	SaveASRConfig(config domain.ASRConfig) (domain.ASRConfig, error)
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
	UpdateReadingMaterialExisting(material domain.ReadingMaterial) (domain.ReadingMaterial, error)
	GetReadingMaterial(id string, userID string) (domain.ReadingMaterial, error)
	ListReadingMaterialsByUser(userID string, exam string) ([]domain.ReadingMaterial, error)
	DeleteReadingMaterial(userID string, materialID string) error
	SaveVoiceProfile(profile domain.VoiceProfile) (domain.VoiceProfile, error)
	ListVoiceProfiles(userID string) ([]domain.VoiceProfile, error)
	DeleteVoiceProfile(userID string, profileID string) error
	CreateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error)
	GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error)
	UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error)
	SaveWritingSession(session domain.WritingSession) (domain.WritingSession, error)
	UpdateWritingSessionExisting(session domain.WritingSession) (domain.WritingSession, error)
	GetWritingSession(sessionID string, userID string) (domain.WritingSession, error)
	ListWritingSessions(userID string) ([]domain.WritingSession, error)
	DeleteWritingSession(userID string, sessionID string) error
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

type ModelConfigManager interface {
	GetModelConfig() domain.ModelConfig
	UpdateModelConfig(config domain.ModelConfig)
}

type TTSConfigManager interface {
	GetTTSConfig() domain.TTSConfig
	UpdateTTSConfig(config domain.TTSConfig)
}

type ASRConfigManager interface {
	GetASRConfig() domain.ASRConfig
	UpdateASRConfig(config domain.ASRConfig)
}

type SpeechSynthesizer interface {
	Synthesize(ctx context.Context, text string, language string, voice string) (string, error)
}

type SpeechRecognizer interface {
	Transcribe(ctx context.Context, audioDataURL string, language string) (domain.TranscriptResult, error)
}

type VoiceDesigner interface {
	DesignVoice(ctx context.Context, prompt string, language string) (string, error)
}

type AuthMailer interface {
	SendEmailVerification(ctx context.Context, to string, username string, verifyURL string) error
	SendPasswordReset(ctx context.Context, to string, username string, resetURL string) error
	SendUsernameRecovery(ctx context.Context, to string, usernames []string) error
}

type Options struct {
	Mailer                   AuthMailer
	PublicAppURL             string
	MediaDir                 string
	RequireEmailVerification bool
	TaskConcurrency          int
	TaskTimeout              time.Duration
	Recognizer               SpeechRecognizer
	Billing                  BillingOptions
	UsageProtection          UsageProtectionOptions
	Analytics                *analytics.Reporter
}

type Service struct {
	store                    Store
	session                  SessionStore
	generator                TheaterGenerator
	modelConfig              ModelConfigManager
	tts                      SpeechSynthesizer
	ttsConfig                TTSConfigManager
	asr                      SpeechRecognizer
	asrConfig                ASRConfigManager
	jwtSecret                string
	tokenExpiry              time.Duration
	mailer                   AuthMailer
	publicAppURL             string
	requireEmailVerification bool
	tasks                    *taskQueue
	readingMu                sync.RWMutex
	readingMaterials         map[string]domain.ReadingMaterial
	readingAudioJobs         map[string]bool
	readingAudioKick         map[string]time.Time
	readingListKick          map[string]time.Time
	mediaDir                 string
	billing                  *billingService
	usageGuard               *aiRequestGuard
	analytics                *analytics.Reporter
}

const (
	maxReadingAudioListRetries = 2
	readingAudioRetryCooldown  = 15 * time.Minute
	readingListRetryCooldown   = 2 * time.Minute
	defaultTTSMaxConcurrency   = 2
)

type roleplayEngine interface {
	RoleplayTurn(ctx context.Context, theater domain.Theater, userRole string, transcript []domain.Dialogue, userReply string) (domain.RoleplayTurnEval, error)
	RoleplaySummary(ctx context.Context, theater domain.Theater, transcript []domain.Dialogue, currentScore int) (string, error)
}

func New(store Store, session SessionStore, generator TheaterGenerator, tts SpeechSynthesizer, jwtSecret string) *Service {
	return NewWithOptions(store, session, generator, tts, jwtSecret, Options{})
}

func NewWithOptions(store Store, session SessionStore, generator TheaterGenerator, tts SpeechSynthesizer, jwtSecret string, options Options) *Service {
	var modelConfigManager ModelConfigManager
	if manager, ok := any(generator).(ModelConfigManager); ok {
		modelConfigManager = manager
	}
	var ttsConfigManager TTSConfigManager
	if manager, ok := any(tts).(TTSConfigManager); ok {
		ttsConfigManager = manager
	}
	var asrConfigManager ASRConfigManager
	if manager, ok := any(options.Recognizer).(ASRConfigManager); ok {
		asrConfigManager = manager
	}
	publicURL := strings.TrimRight(strings.TrimSpace(options.PublicAppURL), "/")
	if publicURL == "" {
		publicURL = "http://localhost:5174"
	}
	mediaDir := strings.TrimSpace(options.MediaDir)
	if mediaDir == "" {
		mediaDir = "media"
	}
	service := &Service{
		store:                    store,
		session:                  session,
		generator:                generator,
		modelConfig:              modelConfigManager,
		tts:                      tts,
		ttsConfig:                ttsConfigManager,
		asr:                      options.Recognizer,
		asrConfig:                asrConfigManager,
		jwtSecret:                jwtSecret,
		tokenExpiry:              2 * time.Hour,
		mailer:                   options.Mailer,
		publicAppURL:             publicURL,
		requireEmailVerification: options.RequireEmailVerification,
		tasks:                    newTaskQueue(options.TaskConcurrency, options.TaskTimeout),
		readingMaterials:         map[string]domain.ReadingMaterial{},
		readingAudioJobs:         map[string]bool{},
		readingAudioKick:         map[string]time.Time{},
		readingListKick:          map[string]time.Time{},
		mediaDir:                 mediaDir,
		usageGuard:               newAIRequestGuard(options.UsageProtection),
		analytics:                options.Analytics,
	}
	if billingStore, ok := store.(BillingStore); ok && !options.Billing.OpenSourceEdition {
		service.billing = newBillingService(billingStore, options.Billing, publicURL)
	}
	return service
}

func (s *Service) trackFeature(name string) {
	if s != nil && s.analytics != nil {
		s.analytics.RecordProductMetric(analytics.MetricCategoryFeature, name)
	}
}

var usernamePattern = regexp.MustCompile(`^[\p{L}\p{N}_-]{3,24}$`)

func validatePassword(password string) error {
	length := len([]rune(password))
	if length < 8 || length > 15 {
		return errors.New("password must be 8-15 characters and include uppercase, lowercase, and number")
	}
	var upper, lower, digit bool
	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			upper = true
		case char >= 'a' && char <= 'z':
			lower = true
		case char >= '0' && char <= '9':
			digit = true
		}
	}
	if !upper || !lower || !digit {
		return errors.New("password must be 8-15 characters and include uppercase, lowercase, and number")
	}
	return nil
}

func normalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return "", errors.New("username must be 3-24 characters and contain only letters, numbers, underscores, or hyphens")
	}
	return username, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func (s *Service) Register(username string, email string, password string) (domain.AuthResult, error) {
	var err error
	if username, err = normalizeUsername(username); err != nil {
		return domain.AuthResult{}, err
	}
	if email, err = normalizeEmail(email); err != nil {
		return domain.AuthResult{}, err
	}
	if err = validatePassword(password); err != nil {
		return domain.AuthResult{}, err
	}
	if s.requireEmailVerification && s.mailer == nil {
		return domain.AuthResult{}, errors.New("邮箱验证暂不可用：未配置 SMTP。")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.AuthResult{}, err
	}
	user, err := s.store.CreateUser(username, email, string(hash), !s.requireEmailVerification)
	if err != nil {
		return domain.AuthResult{}, err
	}
	result := domain.AuthResult{UserID: user.ID, EmailVerificationRequired: s.requireEmailVerification}
	if s.requireEmailVerification {
		sent, sendErr := s.sendVerification(user)
		result.EmailSent = sent
		if sendErr != nil {
			result.Message = "账号已创建，但验证邮件发送失败。"
		} else {
			result.Message = "验证邮件已发送。"
		}
		return result, nil
	}
	authResult, err := s.issueAuthResult(user)
	if err != nil {
		return domain.AuthResult{}, err
	}
	result.AccessToken = authResult.AccessToken
	result.RefreshToken = authResult.RefreshToken
	return result, nil
}

func (s *Service) LoginCandidates(identifier string) ([]domain.LoginCandidate, error) {
	users, err := s.findUsers(identifier)
	if err != nil {
		return nil, err
	}
	result := make([]domain.LoginCandidate, 0, len(users))
	for _, user := range users {
		result = append(result, domain.LoginCandidate{ID: user.ID, Username: user.Username, Email: user.Email})
	}
	return result, nil
}

func (s *Service) Login(identifier string, password string, userID string) (domain.AuthResult, error) {
	user, err := s.selectUser(identifier, userID)
	if err != nil {
		return domain.AuthResult{}, err
	}
	if s.requireEmailVerification && !user.EmailVerified {
		return domain.AuthResult{}, errors.New("邮箱尚未验证。")
	}
	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.AuthResult{}, errors.New("invalid credentials")
	}
	return s.issueAuthResult(user)
}

func (s *Service) RequestEmailVerification(identifier string, userID string) (domain.EmailActionResult, error) {
	user, result, err := s.selectUserForEmailAction(identifier, userID)
	if err != nil || result.RequiresSelection {
		return result, err
	}
	if user.EmailVerified {
		return domain.EmailActionResult{Message: "邮箱验证成功"}, nil
	}
	sent, sendErr := s.sendVerification(user)
	if sendErr != nil {
		return domain.EmailActionResult{}, sendErr
	}
	if sent {
		return domain.EmailActionResult{Message: "验证邮件已发送。"}, nil
	}
	return domain.EmailActionResult{Message: "验证邮件发送失败。"}, nil
}

func (s *Service) VerifyEmail(token string) (domain.AuthResult, error) {
	userID, err := s.consumeToken(token, "verify_email")
	if err != nil {
		return domain.AuthResult{}, err
	}
	if err = s.store.SetUserEmailVerified(userID, true); err != nil {
		return domain.AuthResult{}, err
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return domain.AuthResult{}, err
	}
	return s.issueAuthResult(user)
}

func (s *Service) RequestPasswordReset(identifier string, userID string) (domain.EmailActionResult, error) {
	user, result, err := s.selectUserForEmailAction(identifier, userID)
	if err != nil || result.RequiresSelection {
		return result, err
	}
	if s.mailer == nil {
		return domain.EmailActionResult{}, errors.New("password reset is unavailable: SMTP is not configured")
	}
	raw, err := s.createToken(user.ID, "reset_password")
	if err != nil {
		return domain.EmailActionResult{}, err
	}
	resetURL := s.tokenURL("/login", "reset", raw)
	if err = s.mailer.SendPasswordReset(context.Background(), user.Email, user.Username, resetURL); err != nil {
		return domain.EmailActionResult{}, err
	}
	return domain.EmailActionResult{Message: "password reset email sent"}, nil
}

func (s *Service) ResetPassword(token string, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	userID, err := s.consumeToken(token, "reset_password")
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(userID, string(hash))
}

func (s *Service) RequestUsernameRecovery(email string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return errors.New("username recovery is unavailable: SMTP is not configured")
	}
	users, err := s.store.ListUsersByEmail(email)
	if err != nil || len(users) == 0 {
		return nil // Keep the result neutral to avoid account enumeration.
	}
	usernames := make([]string, 0, len(users))
	for _, user := range users {
		usernames = append(usernames, user.Username)
	}
	return s.mailer.SendUsernameRecovery(context.Background(), email, usernames)
}

func (s *Service) findUsers(identifier string) ([]domain.User, error) {
	identifier = strings.TrimSpace(identifier)
	if strings.Contains(identifier, "@") {
		email, err := normalizeEmail(identifier)
		if err != nil {
			return nil, err
		}
		return s.store.ListUsersByEmail(email)
	}
	user, err := s.store.GetUserByUsername(identifier)
	if err != nil {
		return nil, err
	}
	return []domain.User{user}, nil
}

func (s *Service) selectUser(identifier string, userID string) (domain.User, error) {
	users, err := s.findUsers(identifier)
	if err != nil || len(users) == 0 {
		return domain.User{}, errors.New("invalid credentials")
	}
	if len(users) > 1 && strings.TrimSpace(userID) == "" {
		return domain.User{}, errors.New("multiple accounts found: select a user")
	}
	for _, user := range users {
		if len(users) == 1 || user.ID == userID {
			return user, nil
		}
	}
	return domain.User{}, errors.New("invalid account selection")
}

func (s *Service) selectUserForEmailAction(identifier string, userID string) (domain.User, domain.EmailActionResult, error) {
	users, err := s.findUsers(identifier)
	if err != nil || len(users) == 0 {
		return domain.User{}, domain.EmailActionResult{Message: "if the account exists, an email will be sent"}, nil
	}
	candidates := make([]domain.LoginCandidate, 0, len(users))
	for _, user := range users {
		candidates = append(candidates, domain.LoginCandidate{ID: user.ID, Username: user.Username, Email: user.Email})
	}
	if len(users) > 1 && strings.TrimSpace(userID) == "" {
		return domain.User{}, domain.EmailActionResult{RequiresSelection: true, Candidates: candidates}, nil
	}
	for _, user := range users {
		if len(users) == 1 || user.ID == userID {
			return user, domain.EmailActionResult{}, nil
		}
	}
	return domain.User{}, domain.EmailActionResult{}, errors.New("invalid account selection")
}

func (s *Service) sendVerification(user domain.User) (bool, error) {
	if s.mailer == nil {
		return false, errors.New("邮箱验证暂不可用：未配置 SMTP。")
	}
	raw, err := s.createToken(user.ID, "verify_email")
	if err != nil {
		return false, err
	}
	if err = s.mailer.SendEmailVerification(context.Background(), user.Email, user.Username, s.tokenURL("/login", "verify", raw)); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) createToken(userID string, purpose string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	raw := hex.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(raw))
	if err := s.store.CreateAuthToken(userID, purpose, hex.EncodeToString(hash[:]), time.Now().Add(30*time.Minute)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) consumeToken(raw string, purpose string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("验证链接无效或已过期。")
	}
	hash := sha256.Sum256([]byte(raw))
	userID, err := s.store.ConsumeAuthToken(hex.EncodeToString(hash[:]), purpose, time.Now())
	if err != nil {
		return "", errors.New("验证链接无效或已过期。")
	}
	return userID, nil
}

func (s *Service) tokenURL(path string, key string, token string) string {
	return s.publicAppURL + path + "?" + url.Values{key: []string{token}}.Encode()
}

func (s *Service) issueAuthResult(user domain.User) (domain.AuthResult, error) {
	accessToken, err := auth.CreateAccessToken(s.jwtSecret, user.ID, user.Email)
	if err != nil {
		return domain.AuthResult{}, err
	}
	result := domain.AuthResult{AccessToken: accessToken, UserID: user.ID}
	if s.session == nil {
		return result, nil
	}
	refreshToken, err := newRefreshToken(user.ID)
	if err != nil {
		return domain.AuthResult{}, err
	}
	if err = s.session.SetRefreshToken(context.Background(), user.ID, hashRefreshToken(refreshToken)); err != nil {
		return domain.AuthResult{}, errors.New("session is unavailable")
	}
	result.RefreshToken = refreshToken
	return result, nil
}

func newRefreshToken(userID string) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return userID + "." + base64.RawURLEncoding.EncodeToString(secret), nil
}

func refreshTokenUserID(token string) (string, bool) {
	separator := strings.LastIndex(token, ".")
	if separator <= 0 || separator == len(token)-1 {
		return "", false
	}
	return token[:separator], true
}

func hashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Refresh rotates a valid independent refresh token without parsing the access token.
func (s *Service) Refresh(refreshToken string) (domain.AuthResult, error) {
	if s.session == nil {
		return domain.AuthResult{}, errors.New("refresh is unavailable")
	}
	userID, ok := refreshTokenUserID(refreshToken)
	if !ok {
		return domain.AuthResult{}, errors.New("refresh token invalid")
	}
	stored, err := s.session.GetRefreshToken(context.Background(), userID)
	if err != nil || stored == "" || stored != hashRefreshToken(refreshToken) {
		return domain.AuthResult{}, errors.New("refresh token invalid")
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return domain.AuthResult{}, errors.New("refresh token invalid")
	}
	return s.issueAuthResult(user)
}

// RefreshLegacyAccessToken allows one release-cycle migration from sessions that
// stored the access token itself. It cannot recover expired or rotated JWTs.
func (s *Service) RefreshLegacyAccessToken(accessToken string) (domain.AuthResult, error) {
	if s.session == nil {
		return domain.AuthResult{}, errors.New("refresh is unavailable")
	}
	claims, err := auth.ParseAccessToken(s.jwtSecret, accessToken)
	if err != nil {
		return domain.AuthResult{}, err
	}
	stored, err := s.session.GetRefreshToken(context.Background(), claims.UserID)
	if err != nil || stored == "" || stored != accessToken {
		return domain.AuthResult{}, errors.New("refresh token invalid")
	}
	user, err := s.store.GetUserByID(claims.UserID)
	if err != nil {
		return domain.AuthResult{}, errors.New("refresh token invalid")
	}
	return s.issueAuthResult(user)
}

func (s *Service) Logout(userID string) error {
	if s.session == nil {
		return nil
	}
	return s.session.SetRefreshToken(context.Background(), userID, "")
}

func buildModelConfigView(config domain.ModelConfig) domain.ModelConfigView {
	apiKey := strings.TrimSpace(config.APIKey)
	return domain.ModelConfigView{
		Provider:      strings.TrimSpace(config.Provider),
		Model:         strings.TrimSpace(config.Model),
		BaseURL:       strings.TrimSpace(config.BaseURL),
		HasAPIKey:     apiKey != "",
		APIKeyPreview: previewAPIKey(apiKey),
		UpdatedAt:     config.UpdatedAt,
	}
}

func buildTTSConfigView(config domain.TTSConfig) domain.TTSConfigView {
	apiKey := strings.TrimSpace(config.APIKey)
	return domain.TTSConfigView{
		Provider:      strings.TrimSpace(config.Provider),
		Model:         strings.TrimSpace(config.Model),
		BaseURL:       strings.TrimSpace(config.BaseURL),
		Voice:         strings.TrimSpace(config.Voice),
		AudioFormat:   strings.TrimSpace(config.AudioFormat),
		HasAPIKey:     apiKey != "",
		APIKeyPreview: previewAPIKey(apiKey),
		UpdatedAt:     config.UpdatedAt,
	}
}

func buildASRConfigView(config domain.ASRConfig) domain.ASRConfigView {
	apiKey := strings.TrimSpace(config.APIKey)
	return domain.ASRConfigView{
		Provider: strings.TrimSpace(config.Provider), Model: strings.TrimSpace(config.Model), BaseURL: strings.TrimSpace(config.BaseURL),
		HasAPIKey: apiKey != "", APIKeyPreview: previewAPIKey(apiKey), AppID: strings.TrimSpace(config.AppID), UpdatedAt: config.UpdatedAt,
	}
}

func previewAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return apiKey[:2] + "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

func (s *Service) Me(userID string) (domain.User, error) {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return domain.User{}, err
	}
	return decorateLearningProgress(user), nil
}

func (s *Service) GetModelConfig() (domain.ModelConfigView, error) {
	if s.modelConfig == nil {
		return domain.ModelConfigView{}, errors.New("model management unavailable")
	}
	return buildModelConfigView(s.modelConfig.GetModelConfig()), nil
}

func (s *Service) UpdateModelConfig(input domain.ModelConfigUpdate) (domain.ModelConfigView, error) {
	if s.modelConfig == nil {
		return domain.ModelConfigView{}, errors.New("model management unavailable")
	}

	current := s.modelConfig.GetModelConfig()
	next := current

	provider := normalizeModelProvider(input.Provider, current.Provider)
	providerChanged := !strings.EqualFold(strings.TrimSpace(current.Provider), provider)
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		if providerChanged {
			baseURL = defaultModelBaseURL(provider)
		} else {
			baseURL = strings.TrimSpace(current.BaseURL)
		}
	}
	if baseURL == "" {
		baseURL = defaultModelBaseURL(provider)
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		if providerChanged {
			model = defaultModelName(provider)
		} else {
			model = strings.TrimSpace(current.Model)
		}
	}
	if model == "" {
		model = defaultModelName(provider)
	}

	next.Provider = provider
	next.Model = model
	next.BaseURL = strings.TrimRight(baseURL, "/")
	switch {
	case strings.TrimSpace(input.APIKey) != "":
		next.APIKey = strings.TrimSpace(input.APIKey)
	case input.ClearAPIKey:
		next.APIKey = ""
	}
	next.UpdatedAt = time.Now().UTC()

	saved, err := s.store.SaveModelConfig(next)
	if err != nil {
		return domain.ModelConfigView{}, err
	}
	s.modelConfig.UpdateModelConfig(saved)
	return buildModelConfigView(saved), nil
}

func normalizeModelProvider(input string, fallback string) string {
	provider := strings.ToUpper(strings.TrimSpace(input))
	if provider == "" {
		provider = strings.ToUpper(strings.TrimSpace(fallback))
	}
	switch provider {
	case "OPENAI_COMPATIBLE":
		return "OPENAI_COMPATIBLE"
	case "OPENAI":
		return "OPENAI"
	case "CLAUDE":
		return "CLAUDE"
	case "GEMINI":
		return "GEMINI"
	case "GLM":
		return "GLM"
	case "MINIMAX":
		return "MINIMAX"
	case "DEEPSEEK":
		return "DEEPSEEK"
	case "DOUBAO":
		return "DOUBAO"
	case "QWEN":
		return "QWEN"
	default:
		return "OPENAI_COMPATIBLE"
	}
}

func defaultModelBaseURL(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "OPENAI_COMPATIBLE":
		return "http://43.172.5.210:3000/v1"
	case "OPENAI":
		return "http://43.172.5.210:3000/v1"
	case "CLAUDE":
		return "https://api.anthropic.com/v1"
	case "GEMINI":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	case "GLM":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "MINIMAX":
		return "https://api.minimax.io/v1"
	case "DEEPSEEK":
		return "https://api.deepseek.com"
	case "DOUBAO":
		return "https://ark.cn-beijing.volces.com/api/v3"
	case "QWEN":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	default:
		return "http://43.172.5.210:3000/v1"
	}
}

func defaultModelName(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "OPENAI_COMPATIBLE":
		return "gpt-5.4"
	case "OPENAI":
		return "gpt-5.4"
	case "CLAUDE":
		return "claude-sonnet-4-6"
	case "GEMINI":
		return "gemini-2.5-flash"
	case "GLM":
		return "glm-5.1"
	case "MINIMAX":
		return "MiniMax-M2.7"
	case "DEEPSEEK":
		return "deepseek-v4-flash"
	case "DOUBAO":
		return "doubao-seed-2-0-lite-260428"
	case "QWEN":
		return "qwen3.6-plus"
	default:
		return "gpt-5.4"
	}
}

func (s *Service) GetTTSConfig() (domain.TTSConfigView, error) {
	if s.ttsConfig == nil {
		return domain.TTSConfigView{}, errors.New("tts management unavailable")
	}
	return buildTTSConfigView(s.ttsConfig.GetTTSConfig()), nil
}

func (s *Service) UpdateTTSConfig(input domain.TTSConfigUpdate) (domain.TTSConfigView, error) {
	if s.ttsConfig == nil {
		return domain.TTSConfigView{}, errors.New("tts management unavailable")
	}

	current := s.ttsConfig.GetTTSConfig()
	next := current

	provider := normalizeTTSProvider(input.Provider, current.Provider)
	providerChanged := !strings.EqualFold(strings.TrimSpace(current.Provider), provider)
	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		if providerChanged {
			baseURL = defaultTTSBaseURL(provider)
		} else {
			baseURL = strings.TrimSpace(current.BaseURL)
		}
	}
	if baseURL == "" {
		return domain.TTSConfigView{}, errors.New("tts base url is required")
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		if providerChanged {
			model = defaultTTSModel(provider)
		} else {
			model = strings.TrimSpace(current.Model)
		}
	}

	voice := strings.TrimSpace(input.Voice)
	if voice == "" {
		if providerChanged {
			voice = defaultTTSVoice(provider, model)
		} else {
			voice = strings.TrimSpace(current.Voice)
		}
	}
	if provider != "CUSTOM" && strings.EqualFold(voice, "female-1") {
		voice = defaultTTSVoice(provider, model)
	}
	voice = normalizeTTSVoiceValue(provider, model, voice)
	if voice == "" {
		voice = defaultTTSVoice(provider, model)
	}

	next.Provider = provider
	next.Model = model
	next.BaseURL = strings.TrimRight(baseURL, "/")
	next.Voice = voice
	next.AudioFormat = strings.TrimSpace(current.AudioFormat)
	if next.AudioFormat == "" || next.AudioFormat == "wav" {
		next.AudioFormat = "mp3"
	}
	switch {
	case strings.TrimSpace(input.APIKey) != "":
		next.APIKey = strings.TrimSpace(input.APIKey)
	case input.ClearAPIKey:
		next.APIKey = ""
	}
	next.UpdatedAt = time.Now().UTC()

	saved, err := s.store.SaveTTSConfig(next)
	if err != nil {
		return domain.TTSConfigView{}, err
	}
	s.ttsConfig.UpdateTTSConfig(saved)
	return buildTTSConfigView(saved), nil
}

func (s *Service) GetASRConfig() (domain.ASRConfigView, error) {
	if s.asrConfig == nil {
		return domain.ASRConfigView{}, errors.New("asr management unavailable")
	}
	return buildASRConfigView(s.asrConfig.GetASRConfig()), nil
}

func (s *Service) UpdateASRConfig(input domain.ASRConfigUpdate) (domain.ASRConfigView, error) {
	if s.asrConfig == nil {
		return domain.ASRConfigView{}, errors.New("asr management unavailable")
	}
	current := s.asrConfig.GetASRConfig()
	provider := normalizeASRProvider(input.Provider, current.Provider)
	changed := !strings.EqualFold(provider, current.Provider)
	next := current
	next.Provider = provider
	next.Model = strings.TrimSpace(input.Model)
	if next.Model == "" {
		if changed {
			next.Model = defaultASRModel(provider)
		} else {
			next.Model = current.Model
		}
	}
	next.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if next.BaseURL == "" {
		if changed {
			next.BaseURL = defaultASRBaseURL(provider)
		} else {
			next.BaseURL = current.BaseURL
		}
	}
	if next.BaseURL == "" {
		return domain.ASRConfigView{}, errors.New("asr base url is required")
	}
	if strings.TrimSpace(input.APIKey) != "" {
		next.APIKey = strings.TrimSpace(input.APIKey)
	} else if input.ClearAPIKey || changed {
		next.APIKey = ""
	}
	if provider == "DOUBAO" {
		if strings.TrimSpace(input.AppID) != "" || changed {
			next.AppID = strings.TrimSpace(input.AppID)
		}
	} else {
		next.AppID = ""
	}
	next.UpdatedAt = time.Now().UTC()
	saved, err := s.store.SaveASRConfig(next)
	if err != nil {
		return domain.ASRConfigView{}, err
	}
	s.asrConfig.UpdateASRConfig(saved)
	return buildASRConfigView(saved), nil
}

func normalizeASRProvider(input, fallback string) string {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		value = strings.ToUpper(strings.TrimSpace(fallback))
	}
	switch value {
	case "XIAOMI", "ALIYUN", "OPENAI", "DOUBAO", "GEMINI", "MINIMAX":
		return value
	}
	return "OPENAI_COMPATIBLE"
}

func defaultASRBaseURL(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "https://api.xiaomimimo.com/v1"
	case "ALIYUN":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "DOUBAO":
		return "https://openspeech.bytedance.com/api/v3/auc/bigmodel/recognize/flash"
	case "GEMINI":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "MINIMAX":
		return "https://api.minimax.io/v1"
	}
	return "https://api.openai.com/v1"
}
func defaultASRModel(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "mimo-v2.5-asr"
	case "ALIYUN":
		return "qwen3-asr-flash"
	case "DOUBAO":
		return "bigmodel"
	case "GEMINI":
		return "gemini-2.5-flash"
	case "MINIMAX":
		return "speech-2.8-hd"
	}
	return "gpt-4o-mini-transcribe"
}

func (s *Service) VoiceProfiles(userID string) ([]domain.VoiceProfile, error) {
	return s.store.ListVoiceProfiles(userID)
}

func (s *Service) CreateVoiceProfile(userID string, name string, prompt string, language string) (domain.VoiceProfile, error) {
	name = strings.TrimSpace(name)
	prompt = strings.TrimSpace(prompt)
	language = strings.ToUpper(strings.TrimSpace(language))
	if len([]rune(name)) < 2 || len([]rune(name)) > 40 {
		return domain.VoiceProfile{}, errors.New("voice profile name must be 2-40 characters")
	}
	if len([]rune(prompt)) < 8 || len([]rune(prompt)) > 500 {
		return domain.VoiceProfile{}, errors.New("voice design prompt must be 8-500 characters")
	}
	if language != "CANTONESE" && language != "ENGLISH" {
		return domain.VoiceProfile{}, errors.New("voice language must be CANTONESE or ENGLISH")
	}
	if _, ok := s.tts.(VoiceDesigner); !ok {
		return domain.VoiceProfile{}, errors.New("voice design is unavailable: configure Xiaomi TTS first")
	}
	profile := domain.VoiceProfile{
		ID:                uuid.NewString(),
		UserID:            userID,
		Name:              name,
		Prompt:            prompt,
		Language:          language,
		Provider:          "XIAOMI",
		Model:             "mimo-v2.5-tts-voicedesign",
		Status:            "GENERATING",
		GenerationMessage: "音色设计任务已排队",
		CreatedAt:         time.Now(),
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.VoiceProfile{}, err
	}
	if err := s.ConsumeAIConfidence(userID, AICreditActionVoiceDesign, profile.ID, aiCreditAmount(AICreditActionVoiceDesign)); err != nil {
		release()
		return domain.VoiceProfile{}, err
	}
	saved, err := s.store.SaveVoiceProfile(profile)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionVoiceDesign, profile.ID, aiCreditAmount(AICreditActionVoiceDesign))
		return domain.VoiceProfile{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.designVoiceProfileAsync(ctx, saved)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionVoiceDesign, saved.ID, aiCreditAmount(AICreditActionVoiceDesign))
		saved.Status = "FAILED"
		saved.GenerationMessage = "后台任务队列繁忙，请稍后重试"
		return s.store.SaveVoiceProfile(saved)
	}
	s.trackFeature("VOICE_DESIGN_REQUESTED")
	return saved, nil
}

func (s *Service) designVoiceProfileAsync(ctx context.Context, profile domain.VoiceProfile) {
	designer, ok := s.tts.(VoiceDesigner)
	if !ok {
		s.RefundAIConfidence(profile.UserID, AICreditActionVoiceDesign, profile.ID, aiCreditAmount(AICreditActionVoiceDesign))
		profile.Status = "FAILED"
		profile.GenerationMessage = "小米 VoiceDesign 未配置"
		_, _ = s.store.SaveVoiceProfile(profile)
		return
	}
	profile.GenerationMessage = "正在根据提示词设计音色"
	_, _ = s.store.SaveVoiceProfile(profile)
	previewAudioURL, err := designer.DesignVoice(ctx, profile.Prompt, profile.Language)
	if err != nil {
		log.Printf("voice design failed profile_id=%s err=%v", profile.ID, err)
		s.RefundAIConfidence(profile.UserID, AICreditActionVoiceDesign, profile.ID, aiCreditAmount(AICreditActionVoiceDesign))
		profile.Status = "FAILED"
		profile.GenerationMessage = "音色生成失败，请检查小米 TTS 配置后重试"
		_, _ = s.store.SaveVoiceProfile(profile)
		return
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(previewAudioURL)), "data:audio/") {
		s.RefundAIConfidence(profile.UserID, AICreditActionVoiceDesign, profile.ID, aiCreditAmount(AICreditActionVoiceDesign))
		profile.Status = "FAILED"
		profile.GenerationMessage = "音色服务未返回可保存的音频样本"
		_, _ = s.store.SaveVoiceProfile(profile)
		return
	}
	profile.PreviewAudioURL = previewAudioURL
	profile.Status = "READY"
	profile.GenerationMessage = "音色已保存到角色音色库"
	if _, err = s.store.SaveVoiceProfile(profile); err != nil {
		log.Printf("save completed voice profile failed profile_id=%s err=%v", profile.ID, err)
	}
}

func (s *Service) DeleteVoiceProfile(userID string, profileID string) error {
	return s.store.DeleteVoiceProfile(userID, strings.TrimSpace(profileID))
}

func (s *Service) selectedVoiceProfiles(userID string, profileIDs []string) ([]domain.VoiceProfile, error) {
	if len(profileIDs) == 0 {
		return nil, errors.New("select at least one voice profile")
	}
	profiles, err := s.store.ListVoiceProfiles(userID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.VoiceProfile, len(profiles))
	for _, profile := range profiles {
		byID[profile.ID] = profile
	}
	selected := make([]domain.VoiceProfile, 0, len(profileIDs))
	seen := make(map[string]struct{}, len(profileIDs))
	for _, profileID := range profileIDs {
		profileID = strings.TrimSpace(profileID)
		if profileID == "" {
			continue
		}
		if _, exists := seen[profileID]; exists {
			continue
		}
		profile, exists := byID[profileID]
		if !exists || profile.Status != "READY" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(profile.PreviewAudioURL)), "data:audio/") {
			return nil, errors.New("selected voice profile is unavailable")
		}
		seen[profileID] = struct{}{}
		selected = append(selected, profile)
	}
	if len(selected) == 0 {
		return nil, errors.New("select at least one ready voice profile")
	}
	return selected, nil
}

func normalizeTTSProvider(input string, fallback string) string {
	provider := strings.ToUpper(strings.TrimSpace(input))
	if provider == "" {
		provider = strings.ToUpper(strings.TrimSpace(fallback))
	}
	switch provider {
	case "XIAOMI":
		return "XIAOMI"
	case "MINIMAX":
		return "MINIMAX"
	case "ALIYUN":
		return "ALIYUN"
	case "CUSTOM", "API":
		return "CUSTOM"
	default:
		return "CUSTOM"
	}
}

func defaultTTSBaseURL(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "https://api.xiaomimimo.com/v1"
	case "MINIMAX":
		return "https://api.minimax.io/v1/t2a_v2"
	case "ALIYUN":
		return "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer"
	default:
		return ""
	}
}

func defaultTTSModel(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "mimo-v2.5-tts"
	case "MINIMAX":
		return "speech-2.8-hd"
	case "ALIYUN":
		return "cosyvoice-v3-flash"
	default:
		return ""
	}
}

func defaultTTSVoice(provider string, model string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		switch strings.ToLower(strings.TrimSpace(model)) {
		case "mimo-v2.5-tts-voicedesign":
			return "20 多岁女性，声音亲和自然，吐字清晰，适合粤语和英文学习内容。"
		case "mimo-v2.5-tts-voiceclone":
			return ""
		default:
			return "mimo_default"
		}
	case "MINIMAX":
		return "Cantonese_GentleLady"
	case "ALIYUN":
		return "longjiaxin_v3"
	default:
		return "female-1"
	}
}

func normalizeTTSVoiceValue(provider string, model string, voice string) string {
	cleaned := strings.TrimSpace(voice)
	if !strings.EqualFold(provider, "XIAOMI") {
		return cleaned
	}
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "mimo-v2.5-tts-voicedesign":
		if cleaned == "" || strings.HasPrefix(strings.ToLower(cleaned), "data:audio/") {
			return defaultTTSVoice(provider, model)
		}
		return cleaned
	case "mimo-v2.5-tts-voiceclone":
		if strings.HasPrefix(strings.ToLower(cleaned), "data:audio/") {
			return cleaned
		}
		return ""
	default:
		switch cleaned {
		case "mimo_default", "冰糖", "茉莉", "苏打", "白桦", "Mia", "Chloe", "Milo", "Dean":
			return cleaned
		default:
			return defaultTTSVoice(provider, model)
		}
	}
}

func (s *Service) UpdateProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error) {
	nickname = strings.TrimSpace(nickname)
	avatarURL = strings.TrimSpace(avatarURL)
	bio = strings.TrimSpace(bio)
	user, err := s.store.UpdateUserProfile(userID, nickname, avatarURL, bio)
	if err != nil {
		return domain.User{}, err
	}
	return decorateLearningProgress(user), nil
}

func (s *Service) MeFromToken(token string) (domain.User, error) {
	claims, err := auth.ParseAccessToken(s.jwtSecret, token)
	if err != nil {
		return domain.User{}, err
	}
	return s.Me(claims.UserID)
}

func (s *Service) GenerateTheater(userID string, language string, topic string, difficulty float64, mode string) (domain.Theater, error) {
	return s.GenerateTheaterWithVoices(userID, language, topic, difficulty, mode, "AUTO", nil)
}

func (s *Service) GenerateTheaterWithVoices(userID string, language string, topic string, difficulty float64, mode string, voiceMode string, voiceProfileIDs []string) (domain.Theater, error) {
	voiceMode = strings.ToUpper(strings.TrimSpace(voiceMode))
	if voiceMode == "" {
		voiceMode = "AUTO"
	}
	if voiceMode != "AUTO" && voiceMode != "LIBRARY" {
		return domain.Theater{}, errors.New("voice mode must be AUTO or LIBRARY")
	}
	var voiceProfiles []domain.VoiceProfile
	var err error
	if voiceMode == "LIBRARY" {
		voiceProfiles, err = s.selectedVoiceProfiles(userID, voiceProfileIDs)
		if err != nil {
			return domain.Theater{}, err
		}
	}
	requiredQuiz := 2
	if difficulty >= 7.0 {
		requiredQuiz = 3
	}
	preparedTopic := prepareTheaterTopic(language, topic)
	placeholder := domain.Theater{
		ID:                 uuid.NewString(),
		UserID:             userID,
		Language:           language,
		Topic:              topic,
		Difficulty:         difficulty,
		Mode:               mode,
		Status:             "GENERATING",
		GenerationProgress: 5,
		GenerationMessage:  "任务已排队",
		Dialogues:          []domain.Dialogue{},
		QuizQuestions:      []domain.QuizQuestion{},
		CreatedAt:          time.Now(),
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.Theater{}, err
	}
	if err = s.ConsumeAIConfidence(userID, AICreditActionTheaterGeneration, placeholder.ID, aiCreditAmount(AICreditActionTheaterGeneration)); err != nil {
		release()
		return domain.Theater{}, err
	}
	saved, err := s.store.SaveTheater(placeholder)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionTheaterGeneration, placeholder.ID, aiCreditAmount(AICreditActionTheaterGeneration))
		return domain.Theater{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.generateTheaterAsync(ctx, saved, preparedTopic, requiredQuiz, voiceProfiles)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionTheaterGeneration, saved.ID, aiCreditAmount(AICreditActionTheaterGeneration))
		_ = s.updateTheaterProgress(saved.ID, "FAILED", 0, "后台任务队列繁忙，请稍后重试")
		return s.store.GetTheater(saved.ID)
	}
	s.trackFeature("THEATER_GENERATION_REQUESTED")
	return saved, nil
}

func (s *Service) generateTheaterAsync(ctx context.Context, theater domain.Theater, preparedTopic string, requiredQuiz int, voiceProfiles []domain.VoiceProfile) {
	var dialogues []domain.Dialogue
	var quiz []domain.QuizQuestion
	_ = s.updateTheaterProgress(theater.ID, "GENERATING", 15, "正在生成剧场文本")

	if s.generator == nil {
		err := errors.New("content generator is not configured")
		log.Printf("model generate failed theater_id=%s err=%v", theater.ID, err)
		s.markTheaterGenerationFailed(theater, err)
		return
	} else {
		generated, q, err := s.generator.Generate(ctx, theater.Language, preparedTopic, theater.Difficulty, theater.Mode)
		if err != nil {
			log.Printf("model generate failed theater_id=%s err=%v", theater.ID, err)
			s.RefundAIConfidence(theater.UserID, AICreditActionTheaterGeneration, theater.ID, aiCreditAmount(AICreditActionTheaterGeneration))
			_ = s.updateTheaterProgress(theater.ID, "FAILED", 0, "文本生成失败，请稍后重试")
			return
		} else {
			if len(generated) == 0 || dialogueLooksTemplated(generated) {
				err := fmt.Errorf("model returned empty or templated content: dialogues=%d quiz=%d", len(generated), len(q))
				log.Printf("model generate failed theater_id=%s err=%v", theater.ID, err)
				s.markTheaterGenerationFailed(theater, err)
				return
			}
			if len(q) < requiredQuiz {
				err := fmt.Errorf("model returned too few quiz questions: got %d want %d", len(q), requiredQuiz)
				log.Printf("model generate failed theater_id=%s err=%v", theater.ID, err)
				s.markTheaterGenerationFailed(theater, err)
				return
			} else {
				dialogues = generated
				quiz = q[:requiredQuiz]
			}
		}
	}
	_ = s.updateTheaterProgress(theater.ID, "GENERATING", 55, "文本已生成，正在合成语音")
	if s.tts != nil {
		voicePair := selectDialogueVoicePair(theater.Topic)
		profileBySpeaker := make(map[string]domain.VoiceProfile)
		nextProfile := 0
		for i := range dialogues {
			progress := 55 + (i * 35 / max(1, len(dialogues)))
			_ = s.updateTheaterProgress(theater.ID, "GENERATING", progress, fmt.Sprintf("正在合成语音 %d/%d", i+1, len(dialogues)))
			voiceStyle := voicePair[i%2]
			if len(voiceProfiles) > 0 {
				speaker := strings.TrimSpace(dialogues[i].Speaker)
				profile, assigned := profileBySpeaker[speaker]
				if !assigned {
					profile = voiceProfiles[nextProfile%len(voiceProfiles)]
					profileBySpeaker[speaker] = profile
					nextProfile++
				}
				voiceStyle = profile.PreviewAudioURL
			}
			audioURL, err := s.tts.Synthesize(ctx, dialogues[i].Text, theater.Language, voiceStyle)
			if err != nil {
				log.Printf("tts failed theater_id=%s index=%d err=%v", theater.ID, i, err)
				continue
			}
			if strings.TrimSpace(audioURL) == "" {
				log.Printf("tts returned empty audio url theater_id=%s index=%d", theater.ID, i)
				continue
			}
			dialogues[i].AudioURL = audioURL
		}
	} else {
		log.Printf("tts disabled: synthesizer is nil theater_id=%s", theater.ID)
	}
	current, err := s.store.GetTheater(theater.ID)
	if err != nil {
		log.Printf("skip ready theater persist theater_id=%s err=%v", theater.ID, err)
		s.RefundAIConfidence(theater.UserID, AICreditActionTheaterGeneration, theater.ID, aiCreditAmount(AICreditActionTheaterGeneration))
		return
	}
	theater.Status = "READY"
	theater.GenerationProgress = 100
	theater.GenerationMessage = "生成完成"
	theater.Dialogues = dialogues
	theater.QuizQuestions = quiz
	theater.IsFavorite = current.IsFavorite
	theater.ShareCode = current.ShareCode
	theater.CreatedAt = current.CreatedAt
	if _, err := s.store.SaveTheater(theater); err != nil {
		log.Printf("persist ready theater failed theater_id=%s err=%v", theater.ID, err)
		s.RefundAIConfidence(theater.UserID, AICreditActionTheaterGeneration, theater.ID, aiCreditAmount(AICreditActionTheaterGeneration))
	}
}

func (s *Service) updateTheaterProgress(theaterID string, status string, progress int, message string) error {
	theater, err := s.store.GetTheater(theaterID)
	if err != nil {
		return err
	}
	theater.Status = status
	theater.GenerationProgress = max(0, min(100, progress))
	theater.GenerationMessage = message
	_, err = s.store.SaveTheater(theater)
	return err
}

func (s *Service) markTheaterGenerationFailed(theater domain.Theater, reason error) {
	theater.Status = "FAILED"
	theater.GenerationProgress = 0
	theater.GenerationMessage = "生成失败，请稍后重试"
	current, err := s.store.GetTheater(theater.ID)
	if err != nil {
		log.Printf("skip failed theater persist theater_id=%s err=%v", theater.ID, err)
		return
	}
	theater.IsFavorite = current.IsFavorite
	theater.ShareCode = current.ShareCode
	theater.CreatedAt = current.CreatedAt
	if _, err := s.store.SaveTheater(theater); err != nil {
		log.Printf("persist failed theater status failed theater_id=%s reason=%v err=%v", theater.ID, reason, err)
	}
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
	theater, err := s.store.GetTheater(id)
	if err != nil {
		return domain.Theater{}, err
	}
	return s.migrateTheaterAudioDataURLs(theater)
}

func (s *Service) materializeAudioURL(rawURL string, scope string, ownerID string) (string, error) {
	clean := strings.TrimSpace(rawURL)
	if clean == "" || !strings.HasPrefix(strings.ToLower(clean), "data:audio/") {
		return clean, nil
	}
	payload, mime, err := decodeAudioDataURL(clean)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	ext := audioExtensionForMIME(mime)
	cleanScope := safeMediaPathPart(scope, "tts")
	cleanOwner := safeMediaPathPart(ownerID, "general")
	dir := filepath.Join(s.mediaDir, "tts", cleanScope, cleanOwner)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := hash + ext
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return "/media/tts/" + cleanScope + "/" + cleanOwner + "/" + name, nil
}

func decodeAudioDataURL(value string) ([]byte, string, error) {
	meta, encoded, ok := strings.Cut(strings.TrimSpace(value), ",")
	if !ok {
		return nil, "", errors.New("invalid audio data url")
	}
	meta = strings.TrimSpace(meta)
	if !strings.HasPrefix(strings.ToLower(meta), "data:audio/") || !strings.Contains(strings.ToLower(meta), ";base64") {
		return nil, "", errors.New("unsupported audio data url")
	}
	mime := strings.TrimPrefix(strings.Split(meta, ";")[0], "data:")
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, "", err
	}
	if len(payload) == 0 {
		return nil, "", errors.New("empty audio data payload")
	}
	return payload, mime, nil
}

func audioExtensionForMIME(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	default:
		return ".bin"
	}
}

func safeMediaPathPart(value string, fallback string) string {
	clean := strings.ToLower(strings.TrimSpace(value))
	if clean == "" {
		clean = fallback
	}
	var b strings.Builder
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return fallback
	}
	if len(result) > 80 {
		return result[:80]
	}
	return result
}

func (s *Service) migrateTheaterAudioDataURLs(theater domain.Theater) (domain.Theater, error) {
	changed := false
	for i := range theater.Dialogues {
		materialized, err := s.materializeAudioURL(theater.Dialogues[i].AudioURL, "theater", theater.ID)
		if err != nil {
			return theater, err
		}
		if materialized != theater.Dialogues[i].AudioURL {
			theater.Dialogues[i].AudioURL = materialized
			changed = true
		}
	}
	if !changed {
		return theater, nil
	}
	saved, err := s.store.SaveTheater(theater)
	if err != nil {
		return theater, err
	}
	return saved, nil
}

func (s *Service) SharedTheater(shareCode string) (domain.Theater, error) {
	code := strings.ToUpper(strings.TrimSpace(shareCode))
	if code == "" {
		return domain.Theater{}, errors.New("share code is required")
	}
	theater, err := s.store.GetTheaterByShareCode(code)
	if err != nil {
		return domain.Theater{}, err
	}
	return s.migrateTheaterAudioDataURLs(theater)
}

func (s *Service) MyTheaters(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	items, err := s.store.ListTheatersByUser(userID, language, status, favorite)
	if err != nil {
		return nil, err
	}
	for i := range items {
		migrated, migrateErr := s.migrateTheaterAudioDataURLs(items[i])
		if migrateErr != nil {
			log.Printf("theater audio data migration failed theater_id=%s err=%v", items[i].ID, migrateErr)
			continue
		}
		items[i] = migrated
	}
	return items, nil
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
	if err = ensureTheaterReady(theater); err != nil {
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
	xp, err := s.awardLearningXP(userID, "THEATER_PRACTICE", theaterID, score)
	if err != nil {
		return domain.PracticeResult{}, err
	}
	feedback := buildPracticeFeedback(correct, total, score)
	if err = s.store.SavePracticeRecord(userID, theaterID, score, answers, xp); err != nil {
		return domain.PracticeResult{}, err
	}
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
	xp, err := s.awardLearningXP(userID, "READING_PRACTICE", materialID, score)
	if err != nil {
		return domain.PracticeResult{}, err
	}
	if err = s.store.SaveReadingPracticeRecord(userID, materialID, score, answers, xp); err != nil {
		return domain.PracticeResult{}, err
	}

	return domain.PracticeResult{
		Score:        score,
		XPEarned:     xp,
		Feedback:     buildPracticeFeedback(correct, total, score),
		CorrectCount: correct,
		TotalCount:   total,
	}, nil
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

	material := domain.ReadingMaterial{
		ID:                 uuid.NewString(),
		UserID:             userID,
		Exam:               exam,
		Language:           "ENGLISH",
		Level:              level,
		Topic:              topic,
		Title:              fmt.Sprintf("%s Reading Drill: %s", exam, topic),
		SourceIDs:          sourceIDs,
		GenerationNote:     "任务已创建，正在后台生成阅读材料。",
		AudioStatus:        "PENDING",
		Status:             "GENERATING",
		GenerationProgress: 5,
		GenerationMessage:  "任务已排队",
		CreatedAt:          time.Now(),
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	if err := s.ConsumeAIConfidence(userID, AICreditActionReadingGeneration, material.ID, aiCreditAmount(AICreditActionReadingGeneration)); err != nil {
		release()
		return domain.ReadingMaterial{}, err
	}

	saved, err := s.store.SaveReadingMaterial(material)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionReadingGeneration, material.ID, aiCreditAmount(AICreditActionReadingGeneration))
		return domain.ReadingMaterial{}, err
	}
	s.cacheReadingMaterial(saved)
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.generateReadingAsync(ctx, saved)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionReadingGeneration, saved.ID, aiCreditAmount(AICreditActionReadingGeneration))
		_ = s.updateReadingProgress(saved.ID, userID, "FAILED", 0, "后台任务队列繁忙，请稍后重试")
		return s.store.GetReadingMaterial(saved.ID, userID)
	}

	s.trackFeature("READING_GENERATION_REQUESTED")
	return saved, nil
}

func (s *Service) generateReadingAsync(ctx context.Context, material domain.ReadingMaterial) {
	_ = s.updateReadingProgress(material.ID, material.UserID, "GENERATING", 15, "正在生成阅读文本")
	difficulty := 6.5
	if material.Exam == "CET" {
		difficulty = 5.5
	}
	generated, questions, err := s.generator.Generate(ctx, material.Language, fmt.Sprintf("[%s Reading] %s", material.Exam, material.Topic), difficulty, "APPRECIATION")
	if err != nil || len(generated) == 0 || len(questions) < 5 {
		if err == nil {
			err = errors.New("reading generation returned insufficient content")
		}
		log.Printf("reading generation failed material_id=%s err=%v", material.ID, err)
		s.RefundAIConfidence(material.UserID, AICreditActionReadingGeneration, material.ID, aiCreditAmount(AICreditActionReadingGeneration))
		_ = s.updateReadingProgress(material.ID, material.UserID, "FAILED", 0, "阅读文本生成失败，请稍后重试")
		return
	}
	if len(questions) > 5 {
		questions = questions[:5]
	}
	passageParts := make([]string, 0, len(generated))
	for _, dialogue := range generated {
		if line := strings.TrimSpace(dialogue.Text); line != "" {
			passageParts = append(passageParts, line)
		}
	}
	passage := strings.Join(passageParts, "\n")
	if strings.TrimSpace(passage) == "" {
		s.RefundAIConfidence(material.UserID, AICreditActionReadingGeneration, material.ID, aiCreditAmount(AICreditActionReadingGeneration))
		_ = s.updateReadingProgress(material.ID, material.UserID, "FAILED", 0, "阅读文本生成失败，请稍后重试")
		return
	}
	vocabulary := extractReadingVocabulary(passage)
	_ = s.updateReadingProgress(material.ID, material.UserID, "GENERATING", 55, "正在分析词汇与语法")
	analysis := domain.ReadingAnalysis{}
	if analyzer, ok := s.generator.(ReadingAnalyzer); ok {
		if aiResult, analysisErr := analyzer.AnalyzeReading(ctx, material.Exam, material.Topic, passage, vocabulary); analysisErr == nil {
			analysis = normalizeReadingAnalysis(aiResult, vocabulary, material.Topic)
		} else {
			log.Printf("reading semantic analysis fallback material_id=%s err=%v", material.ID, analysisErr)
		}
	}
	if len(analysis.VocabularyItems) == 0 {
		analysis = normalizeReadingAnalysis(domain.ReadingAnalysis{}, vocabulary, material.Topic)
	}
	current, err := s.store.GetReadingMaterial(material.ID, material.UserID)
	if err != nil {
		return
	}
	current.Passage = passage
	current.Vocabulary = vocabulary
	current.Questions = questions
	current.VocabularyItems = analysis.VocabularyItems
	current.AssociationSentences = analysis.AssociationSentences
	current.GrammarInsights = analysis.GrammarInsights
	current.GenerationNote = "文本已生成，正在合成音频。"
	current.Status = "GENERATING"
	current.GenerationProgress = 65
	current.GenerationMessage = "正在合成音频"
	if _, err = s.store.UpdateReadingMaterialExisting(current); err != nil {
		return
	}
	s.cacheReadingMaterial(current)
	s.generateReadingAudio(ctx, current)
}

func extractReadingVocabulary(passage string) []string {
	seen := map[string]struct{}{}
	vocabulary := make([]string, 0, 8)
	for _, word := range strings.Fields(strings.ToLower(passage)) {
		word = strings.Trim(word, ",.!?;:\"'()[]{}")
		if len(word) < 6 {
			continue
		}
		if _, exists := seen[word]; exists {
			continue
		}
		seen[word] = struct{}{}
		vocabulary = append(vocabulary, word)
		if len(vocabulary) >= 8 {
			break
		}
	}
	return vocabulary
}

func fallbackReadingGeneratedContent(exam string, topic string, quizCount int) ([]domain.Dialogue, []domain.QuizQuestion) {
	meta := ielts.ReadingMetadataFromTopic(exam, topic, "")
	return fallbackReadingContentWithMetadata(topic, meta, quizCount)
}

func readingGenerationNote(usedFallback bool) string {
	if usedFallback {
		return "Generated via structured fallback after AI generation was unavailable or failed quality validation."
	}
	return "Generated via AI chain with source-category constraints."
}

func readingMaterialTitle(exam string, topic string, metadata ielts.ReadingMetadata) string {
	cleanTopic := ielts.CleanTopic(topic)
	if cleanTopic == "" {
		cleanTopic = strings.TrimSpace(metadata.QuestionType)
	}
	if cleanTopic == "" {
		cleanTopic = "Reading Practice"
	}
	cleanTopic = sentenceSubject(cleanTopic)
	return fmt.Sprintf("%s Reading Drill: %s", strings.TrimSpace(exam), cleanTopic)
}

func readingGenerationTopic(exam string, topic string, metadata ielts.ReadingMetadata) string {
	parts := []string{fmt.Sprintf("[%s Reading]", strings.ToUpper(strings.TrimSpace(exam)))}
	if metadata.Stage != "" {
		parts = append(parts, "["+metadata.Stage+"]")
	}
	if metadata.Band > 0 {
		parts = append(parts, fmt.Sprintf("[Band %.1f]", metadata.Band))
	}
	if metadata.Section != "" {
		parts = append(parts, "["+metadata.Section+"]")
	}
	if metadata.QuestionType != "" {
		parts = append(parts, "["+metadata.QuestionType+"]")
	}
	if metadata.SkillFocus != "" {
		parts = append(parts, "Focus: "+metadata.SkillFocus)
	}
	cleanTopic := ielts.CleanTopic(topic)
	if cleanTopic == "" {
		cleanTopic = strings.TrimSpace(topic)
	}
	parts = append(parts, cleanTopic)
	return strings.Join(parts, " ")
}

func normalizeGeneratedDialoguesForDelivery(language string, dialogues []domain.Dialogue) []domain.Dialogue {
	if !strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		return dialogues
	}
	for i := range dialogues {
		dialogues[i].Text = contentquality.NormalizeEnglishSpacing(dialogues[i].Text)
	}
	return dialogues
}

func normalizeGeneratedQuizForDelivery(language string, quiz []domain.QuizQuestion) []domain.QuizQuestion {
	if !strings.EqualFold(strings.TrimSpace(language), "ENGLISH") {
		return quiz
	}
	for i := range quiz {
		quiz[i].Question = contentquality.NormalizeEnglishSpacing(quiz[i].Question)
		for j := range quiz[i].Options {
			quiz[i].Options[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Options[j])
		}
		quiz[i].AnswerKey = contentquality.NormalizeEnglishSpacing(quiz[i].AnswerKey)
		quiz[i].ParagraphRef = contentquality.NormalizeEnglishSpacing(quiz[i].ParagraphRef)
		quiz[i].Evidence = contentquality.NormalizeEnglishSpacing(quiz[i].Evidence)
		for j := range quiz[i].Headings {
			quiz[i].Headings[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Headings[j])
		}
		quiz[i].SummaryText = contentquality.NormalizeEnglishSpacing(quiz[i].SummaryText)
		for j := range quiz[i].WordBank {
			quiz[i].WordBank[j] = contentquality.NormalizeEnglishSpacing(quiz[i].WordBank[j])
		}
		for j := range quiz[i].Answers {
			quiz[i].Answers[j] = contentquality.NormalizeEnglishSpacing(quiz[i].Answers[j])
		}
		for j := range quiz[i].Statements {
			quiz[i].Statements[j].Text = contentquality.NormalizeEnglishSpacing(quiz[i].Statements[j].Text)
			quiz[i].Statements[j].Answer = contentquality.NormalizeEnglishSpacing(quiz[i].Statements[j].Answer)
		}
	}
	return quiz
}

func validateGeneratedPracticeForDelivery(language string, readingMode bool, dialogues []domain.Dialogue, quiz []domain.QuizQuestion) error {
	english := strings.EqualFold(strings.TrimSpace(language), "ENGLISH")
	for i, dialogue := range dialogues {
		if err := contentquality.ValidateReadableText(fmt.Sprintf("dialogue %d", i+1), dialogue.Text, english); err != nil {
			return err
		}
	}
	genericReadingQuestions := 0
	for i, question := range quiz {
		if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d", i+1), question.Question, english); err != nil {
			return err
		}
		for j, option := range question.Options {
			if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d option %d", i+1, j+1), option, english); err != nil {
				return err
			}
		}
		if err := contentquality.ValidateReadableText(fmt.Sprintf("question %d answer", i+1), question.AnswerKey, english); err != nil {
			return err
		}
		if readingMode && contentquality.IsGenericReadingQuestion(question.Question) {
			genericReadingQuestions++
		}
	}
	if readingMode && genericReadingQuestions >= 2 {
		return fmt.Errorf("reading content contains too many generic questions: %d", genericReadingQuestions)
	}
	return nil
}

func validateReadingMaterialText(passage string, quiz []domain.QuizQuestion, minWords int, minParagraphs int) error {
	if err := contentquality.ValidateReadableText("reading passage", passage, true); err != nil {
		return err
	}
	if words := contentquality.WordCount(passage); words < minWords {
		return fmt.Errorf("reading passage too short: got %d words, want at least %d", words, minWords)
	}
	if paragraphs := contentquality.ParagraphCount(passage); paragraphs < minParagraphs {
		return fmt.Errorf("reading passage has too few paragraphs: got %d, want at least %d", paragraphs, minParagraphs)
	}
	return validateGeneratedPracticeForDelivery("ENGLISH", true, nil, quiz)
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

func (s *Service) generateReadingAudio(ctx context.Context, material domain.ReadingMaterial) {
	if s.tts == nil || strings.TrimSpace(material.Passage) == "" {
		if err := s.updateReadingMaterial(material.ID, material.UserID, func(m *domain.ReadingMaterial) {
			m.AudioStatus = "FAILED"
			m.Status = "READY"
			m.GenerationProgress = 100
			m.GenerationMessage = "文本生成完成，音频不可用"
			m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio generation unavailable")
		}); err != nil {
			log.Printf("reading audio fallback update failed material_id=%s err=%v", material.ID, err)
		}
		return
	}

	chunks := splitTextChunks(material.Passage, 420)
	audioURLs := make([]string, 0, len(chunks))
	for index, chunk := range chunks {
		progress := 65 + ((index * 30) / max(1, len(chunks)))
		_ = s.updateReadingProgress(material.ID, material.UserID, "GENERATING", progress, fmt.Sprintf("正在合成音频 %d/%d", index+1, len(chunks)))
		audioURL, err := s.tts.Synthesize(ctx, chunk, material.Language, "")
		if err != nil || strings.TrimSpace(audioURL) == "" {
			updateErr := s.updateReadingMaterial(material.ID, material.UserID, func(m *domain.ReadingMaterial) {
				m.AudioStatus = "FAILED"
				m.Status = "READY"
				m.GenerationProgress = 100
				m.GenerationMessage = "文本已生成，音频合成失败"
				if err != nil {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + " | audio error: " + err.Error())
				} else {
					m.AudioStatus = "FAILED"
				}
				if err != nil {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + fmt.Sprintf(" | audio chunk %d/%d error: %s", index+1, len(chunks), err.Error()))
				} else {
					m.GenerationNote = strings.TrimSpace(m.GenerationNote + fmt.Sprintf(" | audio chunk %d/%d error: empty audio url", index+1, len(chunks)))
				}
			})
			if updateErr != nil {
				log.Printf("reading audio failure state persist failed material_id=%s err=%v", material.ID, updateErr)
			}
			return
		}
		audioURLs = append(audioURLs, strings.TrimSpace(audioURL))
		latestIndex := index
		if persistErr := s.updateReadingMaterial(material.ID, material.UserID, func(m *domain.ReadingMaterial) {
			m.AudioURLs = append([]string(nil), audioURLs...)
			m.AudioURL = audioURLs[0]
			if len(audioURLs) >= len(chunks) {
				m.AudioStatus = "READY"
			} else {
				m.AudioStatus = "PENDING"
			}
			m.GenerationNote = trimReadingAudioProgressNote(m.GenerationNote)
			if len(audioURLs) < len(chunks) {
				m.GenerationNote = strings.TrimSpace(m.GenerationNote + fmt.Sprintf(" | audio chunk %d/%d ready", latestIndex+1, len(chunks)))
			}
		}); persistErr != nil {
			log.Printf("reading audio progress persist failed material_id=%s chunk=%d/%d err=%v", material.ID, latestIndex+1, len(chunks), persistErr)
		}
	}

	if err := s.updateReadingMaterial(material.ID, material.UserID, func(m *domain.ReadingMaterial) {
		m.AudioStatus = "READY"
		m.Status = "READY"
		m.GenerationProgress = 100
		m.GenerationMessage = "生成完成"
		m.AudioURLs = audioURLs
		if len(audioURLs) > 0 {
			m.AudioURL = audioURLs[0]
		}
	}); err != nil {
		log.Printf("reading audio ready state persist failed material_id=%s err=%v", material.ID, err)
	}
}

func (s *Service) updateReadingProgress(materialID string, userID string, status string, progress int, message string) error {
	return s.updateReadingMaterial(materialID, userID, func(material *domain.ReadingMaterial) {
		material.Status = status
		material.GenerationProgress = max(0, min(100, progress))
		material.GenerationMessage = message
	})
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
	queued := 0
	allowListRetry := s.allowReadingListAudioRetry(userID, exam, readingListRetryCooldown)
	for i := range result {
		ensureReadingMetadata(&result[i])
		if migrated, migrateErr := s.migrateReadingAudioDataURLs(result[i]); migrateErr == nil {
			result[i] = migrated
		} else {
			log.Printf("reading list audio data migration failed material_id=%s err=%v", result[i].ID, migrateErr)
		}
		s.cacheReadingMaterial(result[i])
		if allowListRetry && queued < maxReadingAudioListRetries && shouldRetryFallbackReadingAudio(result[i]) && s.queueReadingAudioGenerationWithCooldown(result[i].ID, result[i].Passage, result[i].Language, readingAudioRetryCooldown) {
			queued++
		}
	}
	return result, nil
}

func (s *Service) migrateReadingAudioDataURLs(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	changed := false
	if material.AudioURL != "" {
		materialized, err := s.materializeAudioURL(material.AudioURL, "reading", material.ID)
		if err != nil {
			return material, err
		}
		if materialized != material.AudioURL {
			material.AudioURL = materialized
			changed = true
		}
	}
	for i := range material.AudioURLs {
		materialized, err := s.materializeAudioURL(material.AudioURLs[i], "reading", material.ID)
		if err != nil {
			return material, err
		}
		if materialized != material.AudioURLs[i] {
			material.AudioURLs[i] = materialized
			changed = true
		}
	}
	if !changed {
		return material, nil
	}
	saved, err := s.store.SaveReadingMaterial(material)
	if err != nil {
		return material, err
	}
	return saved, nil
}

func (s *Service) queueReadingAudioGenerationWithCooldown(materialID string, text string, language string, cooldown time.Duration) bool {
	if !s.startReadingAudioJob(materialID, cooldown) {
		return false
	}
	material, err := s.store.GetReadingMaterial(materialID, "")
	if err != nil {
		s.finishReadingAudioJob(materialID)
		return false
	}
	if strings.TrimSpace(material.Passage) == "" {
		material.Passage = text
	}
	if strings.TrimSpace(material.Language) == "" {
		material.Language = language
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer s.finishReadingAudioJob(materialID)
		s.generateReadingAudio(ctx, material)
	}) {
		s.finishReadingAudioJob(materialID)
		return false
	}
	return true
}

func (s *Service) startReadingAudioJob(materialID string, cooldown time.Duration) bool {
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	if s.readingAudioJobs[materialID] {
		return false
	}
	if cooldown > 0 {
		if lastKick, ok := s.readingAudioKick[materialID]; ok && time.Since(lastKick) < cooldown {
			return false
		}
	}
	s.readingAudioJobs[materialID] = true
	s.readingAudioKick[materialID] = time.Now()
	return true
}

func (s *Service) finishReadingAudioJob(materialID string) {
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	delete(s.readingAudioJobs, materialID)
}

func shouldRetryFallbackReadingAudio(item domain.ReadingMaterial) bool {
	if strings.TrimSpace(item.Passage) == "" {
		return false
	}
	if !strings.Contains(strings.ToLower(item.GenerationNote), "structured fallback") {
		return false
	}
	return item.AudioStatus != "READY"
}

func trimReadingAudioProgressNote(note string) string {
	parts := strings.Split(note, "|")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		if strings.HasPrefix(piece, "audio chunk ") && strings.HasSuffix(piece, " ready") {
			continue
		}
		kept = append(kept, piece)
	}
	return strings.Join(kept, " | ")
}

func (s *Service) allowReadingListAudioRetry(userID string, exam string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}
	key := strings.TrimSpace(userID) + "|" + strings.TrimSpace(strings.ToUpper(exam))
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	if lastKick, ok := s.readingListKick[key]; ok && time.Since(lastKick) < cooldown {
		return false
	}
	s.readingListKick[key] = time.Now()
	return true
}

func ensureReadingMetadata(material *domain.ReadingMaterial) {
	if material == nil {
		return
	}
	meta := ielts.ReadingMetadataFromTopic(material.Exam, material.Topic, material.Level)
	if material.Band <= 0 {
		material.Band = meta.Band
	}
	if strings.TrimSpace(material.Stage) == "" {
		material.Stage = meta.Stage
	}
	if strings.TrimSpace(material.Section) == "" {
		material.Section = meta.Section
	}
	if strings.TrimSpace(material.SkillFocus) == "" {
		material.SkillFocus = meta.SkillFocus
	}
	if strings.TrimSpace(material.QuestionType) == "" {
		material.QuestionType = meta.QuestionType
	}
	if strings.TrimSpace(material.ScenarioFamily) == "" {
		material.ScenarioFamily = meta.ScenarioFamily
	}
}

func (s *Service) ReadingMaterial(userID string, materialID string) (domain.ReadingMaterial, error) {
	item, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			return domain.ReadingMaterial{}, err
		}
		item, err = s.store.GetReadingMaterial(materialID, "")
		if err != nil {
			return domain.ReadingMaterial{}, err
		}
		log.Printf("reading material loaded by id fallback material_id=%s user_id=%s", materialID, userID)
	}
	ensureReadingMetadata(&item)
	if migrated, migrateErr := s.migrateReadingAudioDataURLs(item); migrateErr == nil {
		item = migrated
	} else {
		log.Printf("reading detail audio data migration failed material_id=%s err=%v", item.ID, migrateErr)
	}
	s.cacheReadingMaterial(item)
	if item.Status == "GENERATING" || item.Status == "FAILED" {
		return item, nil
	}

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
		saved, saveErr := s.store.UpdateReadingMaterialExisting(item)
		if saveErr != nil {
			return domain.ReadingMaterial{}, saveErr
		}
		s.cacheReadingMaterial(saved)
		item = saved
	}
	if shouldRetryFallbackReadingAudio(item) {
		s.queueReadingAudioGenerationWithCooldown(item.ID, item.Passage, item.Language, 0)
	}
	return item, nil
}

func (s *Service) DeleteReadingMaterial(userID string, materialID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("unauthorized")
	}
	material, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		return err
	}
	if isActiveGeneratedMaterialStatus(material.Status) {
		return errors.New("reading material is still being generated")
	}
	if err := s.store.DeleteReadingMaterial(userID, materialID); err != nil {
		return err
	}
	s.readingMu.Lock()
	delete(s.readingMaterials, materialID)
	s.readingMu.Unlock()
	return nil
}

func isActiveGeneratedMaterialStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "GENERATING", "PROCESSING", "EVALUATING":
		return true
	default:
		return false
	}
}

func (s *Service) cacheReadingMaterial(material domain.ReadingMaterial) {
	s.readingMu.Lock()
	defer s.readingMu.Unlock()
	s.readingMaterials[material.ID] = readingMaterialCacheCopy(material)
}

func readingMaterialCacheCopy(material domain.ReadingMaterial) domain.ReadingMaterial {
	cached := material
	cached.AudioURL = ""
	cached.AudioURLs = nil
	return cached
}

func (s *Service) updateReadingMaterial(materialID string, userID string, mutate func(*domain.ReadingMaterial)) error {
	material, err := s.store.GetReadingMaterial(materialID, userID)
	if err != nil {
		return err
	}
	mutate(&material)
	saved, err := s.store.UpdateReadingMaterialExisting(material)
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
	if err = ensureTheaterReady(theater); err != nil {
		return domain.RoleplaySession{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(theater.Mode), "ROLEPLAY") {
		return domain.RoleplaySession{}, errors.New("当前剧场不是角色扮演模式")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	defer release()
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
	creditSource := session.ID + ":opening"
	if err = s.ConsumeAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn)); err != nil {
		return domain.RoleplaySession{}, err
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
	saved, err := s.store.CreateRoleplaySession(session)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
	} else {
		s.trackFeature("ROLEPLAY_SESSION_STARTED")
	}
	return saved, err
}

func (s *Service) GetRoleplaySession(userID string, sessionID string) (domain.RoleplaySession, error) {
	return s.store.GetRoleplaySession(sessionID, userID)
}

func (s *Service) SubmitRoleplayReply(userID string, sessionID string, text string) (domain.RoleplaySession, error) {
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	defer release()
	return s.submitRoleplayReplyWithContext(context.Background(), userID, sessionID, text, false, true)
}

func (s *Service) SubmitRoleplayAudio(userID string, sessionID string, audioDataURL string, language string) (domain.RoleplaySession, error) {
	if s.asr == nil {
		return domain.RoleplaySession{}, errors.New("asr is unavailable: configure ASR first")
	}
	if len(audioDataURL) > 14*1024*1024 || !strings.HasPrefix(strings.TrimSpace(audioDataURL), "data:audio/") {
		return domain.RoleplaySession{}, errors.New("invalid audio input")
	}
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if !strings.EqualFold(session.Status, "active") {
		return domain.RoleplaySession{}, errors.New("roleplay is processing or has ended")
	}
	release, err := s.reserveAIRequest(userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	creditSource := fmt.Sprintf("%s:%d", sessionID, session.TurnIndex+1)
	if err = s.ConsumeAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn)); err != nil {
		release()
		return domain.RoleplaySession{}, err
	}
	session.Status = "PROCESSING"
	session.ProcessingMessage = "正在识别语音"
	saved, err := s.store.UpdateRoleplaySession(session)
	if err != nil {
		release()
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		return domain.RoleplaySession{}, err
	}
	if !s.tasks.enqueue(func(ctx context.Context) {
		defer release()
		s.processRoleplayAudio(ctx, userID, sessionID, audioDataURL, language, creditSource)
	}) {
		release()
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		session.Status = "active"
		session.ProcessingMessage = ""
		_, _ = s.store.UpdateRoleplaySession(session)
		return domain.RoleplaySession{}, errors.New("voice practice queue is full")
	}
	s.trackFeature("ROLEPLAY_AUDIO_TURN_REQUESTED")
	return saved, nil
}

func (s *Service) processRoleplayAudio(ctx context.Context, userID, sessionID, audioDataURL, language, creditSource string) {
	transcript, err := s.asr.Transcribe(ctx, audioDataURL, language)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		s.markRoleplayProcessingFailure(userID, sessionID, "语音识别失败，请检查音频格式或 ASR 配置")
		return
	}
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return
	}
	session.ProcessingMessage = "识别完成，AI 正在组织回复"
	_, _ = s.store.UpdateRoleplaySession(session)
	updated, err := s.submitRoleplayReplyWithContext(ctx, userID, sessionID, transcript.Text, true, false)
	if err != nil {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
		s.markRoleplayProcessingFailure(userID, sessionID, "生成回复失败，请稍后重试")
		return
	}
	updated.Status = "active"
	updated.ProcessingMessage = ""
	_, _ = s.store.UpdateRoleplaySession(updated)
}

func (s *Service) markRoleplayProcessingFailure(userID, sessionID, message string) {
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return
	}
	session.Status = "active"
	session.ProcessingMessage = message
	_, _ = s.store.UpdateRoleplaySession(session)
}

func (s *Service) submitRoleplayReplyWithContext(ctx context.Context, userID string, sessionID string, text string, synthesize bool, consumeCredit bool) (domain.RoleplaySession, error) {
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
	creditSource := fmt.Sprintf("%s:%d", sessionID, session.TurnIndex+1)
	if consumeCredit {
		if err = s.ConsumeAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn)); err != nil {
			return domain.RoleplaySession{}, err
		}
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
		if generated, e := engine.RoleplayTurn(ctx, theater, session.UserRole, session.Transcript, cleanText); e == nil {
			eval = generated
		}
	}
	if session.TurnIndex > 0 {
		session.CurrentScore = ((session.CurrentScore * (session.TurnIndex - 1)) + eval.Total) / session.TurnIndex
	}
	coach := buildTurnFeedbackText(theater.Language, eval)
	assistant := domain.Dialogue{
		Speaker:    "AI-Role",
		Text:       coach,
		ZhSubtitle: eval.AssistantZhSub,
		AudioURL:   "",
		Timestamp:  float64(session.TurnIndex) + 0.3,
	}
	if synthesize && s.tts != nil {
		if audioURL, synthErr := s.tts.Synthesize(ctx, eval.AssistantReply, theater.Language, ""); synthErr == nil {
			assistant.AudioURL = audioURL
		}
	}
	session.Transcript = append(session.Transcript, assistant)
	session.UpdatedAt = time.Now()
	saved, err := s.store.UpdateRoleplaySession(session)
	if err != nil && consumeCredit {
		s.RefundAIConfidence(userID, AICreditActionRoleplayTurn, creditSource, aiCreditAmount(AICreditActionRoleplayTurn))
	} else {
		_, _ = s.awardLearningXP(userID, "SPEAKING_TURN", creditSource, eval.Total)
		if err == nil {
			if synthesize {
				s.trackFeature("ROLEPLAY_AUDIO_TURN_COMPLETED")
			} else {
				s.trackFeature("ROLEPLAY_TEXT_TURN_COMPLETED")
			}
		}
	}
	return saved, err
}

func ensureTheaterReady(theater domain.Theater) error {
	switch strings.ToUpper(strings.TrimSpace(theater.Status)) {
	case "", "READY":
		return nil
	case "FAILED":
		return errors.New("该剧场生成失败，请重新生成")
	default:
		return errors.New("剧场仍在生成中，请稍后再试")
	}
}

func (s *Service) EndRoleplay(userID string, sessionID string) (domain.RoleplaySession, error) {
	session, err := s.store.GetRoleplaySession(sessionID, userID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if strings.EqualFold(session.Status, "completed") {
		return session, nil
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
	if _, err = s.awardLearningXP(userID, "ROLEPLAY_COMPLETE", sessionID, session.CurrentScore); err != nil {
		return domain.RoleplaySession{}, err
	}
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
		profile := ielts.ListeningProfileFromTopic(topic, 6.5)
		lines := fallbackEnglishListeningLines(topic, profile)
		for i, text := range lines {
			dialogues = append(dialogues, domain.Dialogue{
				Speaker:    fallbackEnglishSpeaker(profile.Section, i),
				Text:       text,
				ZhSubtitle: "IELTS 听力场景句，强调信息定位与干扰项辨别。",
				Timestamp:  float64(i) * 2.0,
			})
		}
		quiz := fallbackEnglishListeningQuiz(profile)
		for len(quiz) < requiredQuiz {
			quiz = append(quiz, domain.QuizQuestion{
				Question: fmt.Sprintf("Which detail is corrected before the speakers agree on the next step? (#%d)", len(quiz)+1),
				Options: []string{
					"The original time or location is revised.",
					"The speakers cancel the task immediately.",
					"The final answer is unrelated to the situation.",
					"No detail is changed during the exchange.",
				},
				AnswerKey: "The original time or location is revised.",
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
	meta := ielts.ReadingMetadataFromTopic("IELTS", topic, "")
	return fallbackReadingContentWithMetadata(topic, meta, requiredQuiz)
}

func fallbackReadingContentWithMetadata(topic string, metadata ielts.ReadingMetadata, requiredQuiz int) ([]domain.Dialogue, []domain.QuizQuestion) {
	cleanTopic := ielts.CleanTopic(topic)
	if cleanTopic == "" {
		cleanTopic = strings.TrimSpace(topic)
	}
	frame := fallbackReadingFrameForTopic(cleanTopic)
	limits := ielts.ReadingLengthLimitsFromMetadata("IELTS", topic, metadata)
	segmentCount := fallbackReadingSegmentCount(limits)
	segments := []struct {
		text string
		zh   string
	}{
		{
			text: fmt.Sprintf("Debates about %s often begin with a straightforward promise: a visible problem can be solved by a better system, policy, or design. Yet the evidence behind that promise is more layered than the public slogan suggests. %s usually describe the issue through different expectations: some emphasise immediate gains, while others notice maintenance, access, and the groups who remain poorly served. This matters because the most persuasive early claims about %s tend to simplify trade-offs that only become clear after ordinary use begins.", frame.ShortSubject, frame.Actors, frame.ShortSubject),
			zh:   "该段引入主题，说明不同群体会从成本、维护和公平等角度看待同一方案。",
		},
		{
			text: fmt.Sprintf("The first practical question is whether the response fits existing routines in %s. A scheme that looks impressive in a report can fail if it asks people to change too many habits at once. Successful projects usually begin with familiar behaviour and then make a small part of that behaviour easier. In the case of %s, this may mean changing the timing, location, or presentation of support rather than expecting users to adopt an entirely new system. Convenience does not guarantee success, but it lowers the first barrier to participation.", frame.Setting, frame.ShortSubject),
			zh:   "该段说明方案是否贴近日常习惯会影响参与度。",
		},
		{
			text: fmt.Sprintf("A second issue is maintenance. Early descriptions of %s often emphasise launch dates, participation numbers, or dramatic early results, but long-term performance depends on quieter work. %s must be checked, explained, repaired, or revised when conditions change. If these tasks are not planned from the beginning, the project can appear successful for a few months and then gradually lose reliability. Maintenance is therefore not a minor technical detail; it is part of the social contract between organisers and users.", frame.ShortSubject, frame.OperationalDetail),
			zh:   "该段强调维护工作决定项目能否长期可靠运行。",
		},
		{
			text: fmt.Sprintf("Evidence from pilot projects is useful, but it needs careful interpretation. A small trial may attract motivated participants who already support the idea, especially when the trial concerns %s, so its results can look stronger than those of a later wider programme. Short trials also tend to measure visible outcomes, such as attendance, movement, cost, or compliance, while missing slower changes in trust and confidence. For this reason, researchers increasingly compare early results with follow-up interviews and administrative records collected after the initial publicity has faded.", frame.ShortSubject),
			zh:   "该段说明试点数据有价值，但需要结合后续证据谨慎解读。",
		},
		{
			text: fmt.Sprintf("Equity is another recurring concern. Average figures may suggest improvement even when benefits are unevenly distributed. In %s, a positive overall result can conceal weaker outcomes for people or places with fewer resources, less information, or more exposure to risk. It can also help confident users more than people who need guidance, translation, or flexible access. Stronger evaluations therefore separate results by location, income, age, habitat, or previous experience. This does not make the analysis unnecessarily complicated; it shows whether a public solution is genuinely public.", frame.Setting),
			zh:   "该段指出平均数据可能掩盖不同群体之间的受益差异。",
		},
		{
			text: fmt.Sprintf("There is also a communication problem. Organisers often describe %s using broad words such as innovation, access, resilience, or sustainability, but these labels do not tell affected groups what will change in practical terms. Clear communication links the general aim to specific actions: where to go, what to bring, how long it takes, what is being measured, who is responsible, and what support is available if something goes wrong. When instructions are vague, people may blame themselves for confusion and stop cooperating, even if the underlying design is sound.", frame.ShortSubject),
			zh:   "该段强调清晰说明具体操作比抽象口号更有用。",
		},
		{
			text: fmt.Sprintf("The future of %s is likely to depend on adaptive management rather than a single final design. Adaptive management means testing a limited version, collecting evidence, changing the weak parts and explaining those changes openly. This approach is slower than announcing a complete solution, but it is more honest about uncertainty. It also allows different forms of expertise to interact: technical knowledge can identify what is possible, while %s can reveal what is practical, trusted and worth repeating.", frame.ShortSubject, frame.LocalKnowledge),
			zh:   "该段说明未来更可能依赖持续调整和多方证据，而不是一次性方案。",
		},
		{
			text: "However, adaptation has limits. If every decision is treated as temporary, users may feel that rules are unstable and that promises cannot be relied on. Good governance therefore needs a balance between flexibility and commitment. The aims of a project should remain clear, while the methods used to reach them can be revised as evidence improves. This distinction is especially important when money, safety or access to essential services is involved, because users need both responsiveness and a sense of dependable obligation.",
			zh:   "该段提出适应性治理也需要稳定承诺，不能让规则显得随意。",
		},
		{
			text: "The broader lesson is that durable improvement comes from relationships among design, evidence and trust. A well-designed system makes participation easy; good evidence shows whether the benefits are real; and trust encourages people to keep using the system while it is refined. Weakness in any one part can reduce the value of the others. For that reason, serious assessments should look beyond whether a proposal sounds modern and ask whether it can be maintained, explained, measured and adjusted without losing public confidence.",
			zh:   "该段总结设计、证据和信任之间的关系，强调可维护和可解释的重要性。",
		},
	}
	dialogues := make([]domain.Dialogue, 0, len(segments))
	for i, seg := range segments[:segmentCount] {
		text := fallbackReadingSegmentText(seg.text, frame.ShortSubject, metadata.Band, i)
		dialogues = append(dialogues, domain.Dialogue{
			Speaker:    "Passage",
			Text:       text,
			ZhSubtitle: seg.zh,
			Timestamp:  float64(i) * 2,
		})
	}

	quiz := fallbackReadingQuiz(metadata, requiredQuiz, frame)
	for len(quiz) < requiredQuiz {
		quiz = append(quiz, fallbackReadingQuiz(ielts.ReadingMetadata{QuestionType: "Multiple Choice"}, 1, frame)[0])
	}
	return dialogues, quiz[:min(requiredQuiz, len(quiz))]
}

func fallbackReadingSegmentText(base string, topic string, band float64, index int) string {
	text := base
	if band >= 6.0 {
		text += " " + fallbackReadingEvidenceSentence(topic, index)
	}
	if band >= 7.0 {
		text += " " + fallbackReadingAdvancedSentence(topic, index)
	}
	return text
}

type fallbackReadingFrame struct {
	Subject           string
	ShortSubject      string
	Actors            string
	Setting           string
	OperationalDetail string
	LocalKnowledge    string
}

func fallbackReadingFrameForTopic(topic string) fallbackReadingFrame {
	clean := strings.TrimSpace(topic)
	if clean == "" {
		clean = "a public planning initiative"
	}
	lower := strings.ToLower(clean)
	frame := fallbackReadingFrame{
		Subject:           sentenceSubject(shortReadingSubject(clean)),
		ShortSubject:      shortReadingSubject(clean),
		Actors:            "Researchers, officials and local users",
		Setting:           "this field",
		OperationalDetail: "records, equipment and staff guidance",
		LocalKnowledge:    "local experience",
	}
	switch {
	case strings.Contains(lower, "bird") || strings.Contains(lower, "migratory"):
		frame.Subject = "Changes in migratory bird routes"
		frame.ShortSubject = "migratory bird route changes"
		frame.Actors = "Ecologists, farmers and conservation planners"
		frame.Setting = "agricultural landscapes affected by artificial lighting"
		frame.OperationalDetail = "lighting schedules, field observations and seasonal route data"
		frame.LocalKnowledge = "farmers' observations and long-term ecological monitoring"
	case strings.Contains(lower, "library") || strings.Contains(lower, "digital"):
		frame.Subject = "Digital change in neighbourhood libraries"
		frame.ShortSubject = "library service adaptation"
		frame.Actors = "Librarians, residents and local education teams"
		frame.Setting = "neighbourhood library services"
		frame.OperationalDetail = "devices, opening hours and one-to-one support sessions"
		frame.LocalKnowledge = "staff knowledge of residents' daily needs"
	case strings.Contains(lower, "wetland") || strings.Contains(lower, "suburb"):
		frame.Subject = "Urban wetland restoration"
		frame.ShortSubject = "wetland restoration"
		frame.Actors = "Ecologists, planners and residents near the restoration zone"
		frame.Setting = "urban wetland restoration near expanding suburbs"
		frame.OperationalDetail = "water levels, access paths and maintenance records"
		frame.LocalKnowledge = "residents' observations of flooding, access and wildlife"
	case strings.Contains(lower, "congestion") || strings.Contains(lower, "commuter"):
		frame.Subject = "Congestion pricing"
		frame.ShortSubject = "congestion pricing"
		frame.Actors = "Transport economists, commuters and local business owners"
		frame.Setting = "urban transport corridors affected by congestion pricing"
		frame.OperationalDetail = "pricing rules, travel data and business footfall records"
		frame.LocalKnowledge = "commuters' route choices and traders' daily sales experience"
	case strings.Contains(lower, "algorithm") || strings.Contains(lower, "decision"):
		frame.Subject = "Algorithmic decision systems"
		frame.ShortSubject = "algorithmic decision systems"
		frame.Actors = "Policy analysts, data scientists and affected communities"
		frame.Setting = "climate adaptation, public health triage and infrastructure planning"
		frame.OperationalDetail = "datasets, scoring rules and appeal procedures"
		frame.LocalKnowledge = "frontline judgement and lived experience"
	case strings.Contains(lower, "public health") || strings.Contains(lower, "triage"):
		frame.Subject = "Public health triage systems"
		frame.ShortSubject = "health triage systems"
		frame.Actors = "Clinicians, public health teams and community organisations"
		frame.Setting = "health services under resource pressure"
		frame.OperationalDetail = "patient records, referral rules and follow-up capacity"
		frame.LocalKnowledge = "clinical judgement and community health experience"
	}
	return frame
}

func shortReadingSubject(topic string) string {
	words := strings.Fields(strings.TrimSpace(topic))
	if len(words) == 0 {
		return "a public planning initiative"
	}
	if len(words) <= 8 {
		return strings.Join(words, " ")
	}
	if strings.EqualFold(words[0], "the") && len(words) > 1 {
		words = words[1:]
	}
	return strings.Join(words[:min(8, len(words))], " ")
}

func sentenceSubject(topic string) string {
	clean := strings.TrimSpace(topic)
	if clean == "" {
		return "A public planning initiative"
	}
	first, _ := utf8.DecodeRuneInString(clean)
	if first == utf8.RuneError {
		return clean
	}
	return string(unicode.ToUpper(first)) + clean[len(string(first)):]
}

func fallbackReadingEvidenceSentence(topic string, index int) string {
	sentences := []string{
		"Local reports on " + topic + " usually show this pattern most clearly when early enthusiasm is compared with ordinary use several months later.",
		"This distinction matters because small design details can decide whether a service becomes routine or remains an occasional novelty.",
		"In several evaluations, reliability was valued more highly than speed because users wanted confidence before changing established habits.",
		"Follow-up interviews are especially useful here because they reveal problems that headline figures often treat as minor exceptions.",
		"Such comparisons help explain why the same intervention can produce visible gains in one district and only limited change in another.",
		"Clear instructions also make later evaluation fairer, since users and organisers are then working with the same expectations.",
		"Without this cycle of evidence and revision, even a well-funded project can become a short-lived demonstration rather than a dependable service.",
		"Researchers therefore treat stability and flexibility as linked qualities, not as opposing choices.",
		"The most persuasive assessments combine measurable outcomes with evidence about trust, access and long-term administrative capacity.",
	}
	return sentences[index%len(sentences)]
}

func fallbackReadingAdvancedSentence(topic string, index int) string {
	sentences := []string{
		"For higher-level analysis, the important point is that " + topic + " should be read as a system of incentives and constraints rather than as a single isolated reform.",
		"This also creates a useful tension between measurable efficiency and less visible forms of social legitimacy.",
		"Consequently, a policy can be technically successful while remaining fragile if its benefits depend on conditions that cannot easily be reproduced.",
		"The methodological challenge is to separate genuine causal influence from the temporary effects of publicity, novelty and selective participation.",
		"That is why distributional evidence often changes the interpretation of results that initially appear straightforward.",
		"In this context, communication is not merely promotional; it becomes part of the mechanism through which cooperation is sustained.",
		"Adaptive systems are strongest when revision is transparent enough to preserve confidence rather than create uncertainty.",
		"The central issue is therefore not whether adjustment is necessary, but how institutions decide which adjustments are justified.",
		"This broader framing prevents the topic from being reduced to a simple question of approval or resistance.",
	}
	return sentences[index%len(sentences)]
}

func fallbackReadingSegmentCount(limits ielts.ReadingLengthLimits) int {
	switch {
	case limits.MinWords >= 820:
		return min(limits.MaxSegments, 9)
	case limits.MinWords >= 700:
		return min(limits.MaxSegments, 8)
	default:
		return min(limits.MaxSegments, 7)
	}
}

func fallbackEnglishListeningLines(topic string, profile ielts.ListeningProfile) []string {
	scenario := ielts.CleanTopic(topic)
	if scenario == "" {
		scenario = "a campus services enquiry"
	}
	switch profile.Section {
	case 1:
		return []string{
			fmt.Sprintf("Good morning, Brookdale Language Centre. Are you calling about the %s booking form?", scenario),
			"Yes. I need to confirm the afternoon course, but I may have written the start date incorrectly.",
			"The course starts on the fifteenth of July, not the fifth, and the fee is two hundred and forty pounds.",
			"Thanks. Could you spell the tutor's surname for the form?",
			"Certainly. It is H-A-R-G-R-E-A-V-E-S, and the room number is B twelve.",
			"I also had a note about a deposit. Is it forty pounds or fourteen pounds?",
			"It is forty pounds. The balance is due one week before the first class.",
			"Great, I will update the date, the spelling, and the payment details now.",
		}
	case 2:
		return []string{
			fmt.Sprintf("Welcome to the orientation for %s. We will begin beside the main gate, facing the library.", scenario),
			"The registration desk is not in the library this year; it has moved to the hall behind the cafe.",
			"If you need the quiet study room, walk past the cafe and turn left before the glass corridor.",
			"The sports centre is on the opposite side of the courtyard, but visitors should enter through the west door.",
			"Maps show an older route through the garden, yet that path is closed during resurfacing work.",
			"Workshop tickets can be collected after the safety briefing, not before it.",
			"The final stop is the advice office, where staff can explain bus passes and local bank letters.",
			"Please keep the printed map because the temporary signs will be removed tomorrow morning.",
		}
	case 3:
		return []string{
			fmt.Sprintf("I think our presentation on %s should compare the survey results with the interview notes.", scenario),
			"Maybe, but I am worried the survey sample is too narrow to support a strong conclusion.",
			"Dr Patel said the weakness is acceptable if we explain why the interviews add depth.",
			"I agree about the interviews, though I still want a chart showing how opinions changed after the trial.",
			"That works. I can introduce the chart, and you can discuss why two participants changed their views.",
			"Let's also mention the conflicting evidence instead of hiding it in the appendix.",
			"Good idea. The tutor usually rewards groups that acknowledge limitations clearly.",
			"So our final claim should be cautious: the trial suggests improvement, but the evidence is not yet decisive.",
		}
	default:
		return []string{
			fmt.Sprintf("Today we will examine %s as an example of how institutions respond to complex change.", scenario),
			"Early studies treated the issue as a technical problem, but later work emphasized social behaviour.",
			"A key turning point came when researchers compared short pilot schemes with longer regional data.",
			"The pilot schemes looked successful at first because participants received weekly guidance.",
			"However, the long-term data showed that gains faded where local trust and administrative capacity were weak.",
			"This contrast illustrates why a single policy tool may produce different results in different settings.",
			"For your notes, record three factors: incentives, institutional trust, and feedback speed.",
			"These factors will help explain why adaptive systems are often more durable than fixed annual plans.",
		}
	}
}

func fallbackEnglishSpeaker(section int, index int) string {
	switch section {
	case 1:
		if index%2 == 0 {
			return "Receptionist"
		}
		return "Caller"
	case 2:
		return "Guide"
	case 3:
		if index%2 == 0 {
			return "Student A"
		}
		return "Student B"
	default:
		return "Lecturer"
	}
}

func fallbackEnglishListeningQuiz(profile ielts.ListeningProfile) []domain.QuizQuestion {
	switch profile.Section {
	case 1:
		return []domain.QuizQuestion{
			{Question: "What is the corrected start date?", Options: []string{"5 July", "15 July", "14 July", "1 July"}, AnswerKey: "15 July", Type: "Form Completion"},
			{Question: "How is the tutor's surname spelled?", Options: []string{"H-A-R-G-R-E-A-V-E-S", "H-A-R-G-R-A-V-E-S", "H-A-R-G-R-E-E-V-S", "H-A-R-G-R-I-E-V-E-S"}, AnswerKey: "H-A-R-G-R-E-A-V-E-S", Type: "Spelling"},
			{Question: "What deposit must the caller pay?", Options: []string{"14 pounds", "40 pounds", "140 pounds", "240 pounds"}, AnswerKey: "40 pounds", Type: "Number Detail"},
		}
	case 2:
		return []domain.QuizQuestion{
			{Question: "Where is the registration desk this year?", Options: []string{"Inside the library", "Behind the cafe", "At the west door", "Beside the advice office"}, AnswerKey: "Behind the cafe", Type: "Map Detail"},
			{Question: "Which route is unavailable?", Options: []string{"The garden path", "The glass corridor", "The west entrance", "The cafe stairs"}, AnswerKey: "The garden path", Type: "Route Detail"},
			{Question: "When can workshop tickets be collected?", Options: []string{"Before the tour", "After the safety briefing", "Tomorrow morning", "During registration"}, AnswerKey: "After the safety briefing", Type: "Instruction Detail"},
		}
	case 3:
		return []domain.QuizQuestion{
			{Question: "What concern does Student B raise about the survey?", Options: []string{"The topic is outdated", "The sample is too narrow", "The chart is too detailed", "The tutor rejected the interviews"}, AnswerKey: "The sample is too narrow", Type: "Opinion Matching"},
			{Question: "Why do they decide to include conflicting evidence?", Options: []string{"It makes the claim more cautious", "It removes the need for interviews", "It proves the sample is large", "It belongs in the title"}, AnswerKey: "It makes the claim more cautious", Type: "Reason Matching"},
			{Question: "What final claim do the students prefer?", Options: []string{"The evidence is decisive", "The trial suggests improvement but remains limited", "The interviews should be ignored", "The survey should be repeated immediately"}, AnswerKey: "The trial suggests improvement but remains limited", Type: "Decision Detail"},
		}
	default:
		return []domain.QuizQuestion{
			{Question: "Which three factors should students record?", Options: []string{"Incentives, institutional trust, and feedback speed", "Costs, climate, and population size", "Technology, marketing, and tourism", "Legislation, exams, and transport"}, AnswerKey: "Incentives, institutional trust, and feedback speed", Type: "Note Completion"},
			{Question: "Why did the pilot schemes appear successful at first?", Options: []string{"Participants received weekly guidance", "Local trust was measured daily", "Annual plans were abandoned", "No regional data was collected"}, AnswerKey: "Participants received weekly guidance", Type: "Lecture Detail"},
			{Question: "What does the lecturer say about fixed annual plans?", Options: []string{"They are always more durable", "They may be less durable than adaptive systems", "They remove the need for feedback", "They explain every regional difference"}, AnswerKey: "They may be less durable than adaptive systems", Type: "Summary Detail"},
		}
	}
}

func fallbackReadingQuiz(metadata ielts.ReadingMetadata, requiredQuiz int, frame fallbackReadingFrame) []domain.QuizQuestion {
	switch ielts.QuestionTypeKey(metadata.QuestionType) {
	case "matching_headings":
		headings := []string{
			"Different expectations behind public proposals",
			"The importance of fitting existing routines",
			"Why maintenance determines long-term reliability",
			"Interpreting pilot evidence with caution",
			"How averages can hide unequal benefits",
			"Clear communication as practical support",
			"Adaptive management and open revision",
		}
		quiz := []domain.QuizQuestion{
			{Type: "Matching Headings", Question: "Choose the best heading for paragraph 1.", ParagraphRef: "Paragraph 1", Headings: headings, Options: headings, AnswerKey: headings[0], Evidence: "different expectations"},
			{Type: "Matching Headings", Question: "Choose the best heading for paragraph 2.", ParagraphRef: "Paragraph 2", Headings: headings, Options: headings, AnswerKey: headings[1], Evidence: "fits existing routines"},
			{Type: "Matching Headings", Question: "Choose the best heading for paragraph 3.", ParagraphRef: "Paragraph 3", Headings: headings, Options: headings, AnswerKey: headings[2], Evidence: "long-term performance depends on quieter work"},
			{Type: "Matching Headings", Question: "Choose the best heading for paragraph 4.", ParagraphRef: "Paragraph 4", Headings: headings, Options: headings, AnswerKey: headings[3], Evidence: "A small trial may attract motivated participants"},
			{Type: "Matching Headings", Question: "Choose the best heading for paragraph 5.", ParagraphRef: "Paragraph 5", Headings: headings, Options: headings, AnswerKey: headings[4], Evidence: "benefits are unevenly distributed"},
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	case "matching_information":
		options := []string{"Paragraph 3", "Paragraph 4", "Paragraph 5", "Paragraph 6", "Paragraph 7"}
		quiz := []domain.QuizQuestion{
			{Type: "Matching Information", Question: "Which paragraph explains that affected groups need concrete details before using or trusting a system?", Options: options, AnswerKey: "Paragraph 6", ParagraphRef: "Paragraph 6", Evidence: "where to go, what to bring, how long it takes"},
			{Type: "Matching Information", Question: "Which paragraph warns that small trials may overstate later performance?", Options: options, AnswerKey: "Paragraph 4", ParagraphRef: "Paragraph 4", Evidence: "A small trial may attract motivated participants"},
			{Type: "Matching Information", Question: "Which paragraph says average figures can hide uneven benefits?", Options: options, AnswerKey: "Paragraph 5", ParagraphRef: "Paragraph 5", Evidence: "benefits are unevenly distributed"},
			{Type: "Matching Information", Question: "Which paragraph describes adaptive management as testing and revising a limited version?", Options: options, AnswerKey: "Paragraph 7", ParagraphRef: "Paragraph 7", Evidence: "testing a limited version, collecting evidence, changing the weak parts"},
			{Type: "Matching Information", Question: "Which paragraph describes the quieter work behind long-term performance?", Options: options, AnswerKey: "Paragraph 3", ParagraphRef: "Paragraph 3", Evidence: "long-term performance depends on quieter work"},
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	case "tfng":
		options := []string{"TRUE", "FALSE", "NOT GIVEN"}
		quiz := []domain.QuizQuestion{
			{Type: "TFNG", Question: "Projects can fail if they require people to change too many habits at once.", Options: options, AnswerKey: "TRUE", Evidence: "fail if it asks people to change too many habits at once"},
			{Type: "TFNG", Question: "The passage says opening dates are more important than maintenance.", Options: options, AnswerKey: "FALSE", Evidence: "long-term performance depends on quieter work"},
			{Type: "TFNG", Question: "The passage gives the exact cost of running each pilot program.", Options: options, AnswerKey: "NOT GIVEN", Evidence: "not stated"},
			{Type: "TFNG", Question: "Average figures may hide unevenly distributed benefits.", Options: options, AnswerKey: "TRUE", Evidence: "benefits are unevenly distributed"},
			{Type: "TFNG", Question: "The passage argues that vague instructions can discourage continued use.", Options: options, AnswerKey: "TRUE", Evidence: "When instructions are vague"},
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	case "summary_completion":
		wordBank := []string{"routines", "maintenance", "interviews", "adaptive", "trust", "equity"}
		quiz := []domain.QuizQuestion{
			{Type: "Summary Completion", Question: "Complete the summary using the best word from the bank.", SummaryText: "A proposal is more likely to work when it fits existing _____.", WordBank: wordBank, Options: wordBank, AnswerKey: "routines", Answers: []string{"routines"}, Evidence: "fits existing routines"},
			{Type: "Summary Completion", Question: "Complete the summary using the best word from the bank.", SummaryText: "Long-term performance depends partly on planned _____.", WordBank: wordBank, Options: wordBank, AnswerKey: "maintenance", Answers: []string{"maintenance"}, Evidence: "A second issue is maintenance"},
			{Type: "Summary Completion", Question: "Complete the summary using the best word from the bank.", SummaryText: "Researchers compare early results with follow-up _____ and records.", WordBank: wordBank, Options: wordBank, AnswerKey: "interviews", Answers: []string{"interviews"}, Evidence: "follow-up interviews and administrative records"},
			{Type: "Summary Completion", Question: "Complete the summary using the best word from the bank.", SummaryText: "Future development may rely on _____ management.", WordBank: wordBank, Options: wordBank, AnswerKey: "adaptive", Answers: []string{"adaptive"}, Evidence: "adaptive management"},
			{Type: "Summary Completion", Question: "Complete the summary using the best word from the bank.", SummaryText: "Durable improvement depends on design, evidence and _____.", WordBank: wordBank, Options: wordBank, AnswerKey: "trust", Answers: []string{"trust"}, Evidence: "design, evidence and trust"},
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	case "mixed":
		multipleChoice := fallbackReadingQuiz(ielts.ReadingMetadata{QuestionType: "Multiple Choice"}, 5, frame)
		matchingInformation := fallbackReadingQuiz(ielts.ReadingMetadata{QuestionType: "Matching Information"}, 5, frame)
		tfng := fallbackReadingQuiz(ielts.ReadingMetadata{QuestionType: "TFNG"}, 5, frame)
		summaryCompletion := fallbackReadingQuiz(ielts.ReadingMetadata{QuestionType: "Summary Completion"}, 5, frame)
		quiz := []domain.QuizQuestion{
			multipleChoice[0],
			matchingInformation[0],
			tfng[0],
			summaryCompletion[0],
			multipleChoice[1],
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	default:
		quiz := []domain.QuizQuestion{
			{Type: "Multiple Choice", Question: "According to paragraph 4, why can a small trial look stronger than a later city-wide programme?", ParagraphRef: "Paragraph 4", Evidence: "A small trial may attract motivated participants", Options: []string{"It may attract participants who already support the idea", "It always measures every long-term outcome", "It excludes administrative records by design", "It is usually conducted after publicity has faded"}, AnswerKey: "It may attract participants who already support the idea"},
			{Type: "Multiple Choice", Question: "What problem with average figures is identified in paragraph 5?", ParagraphRef: "Paragraph 5", Evidence: "benefits are unevenly distributed", Options: []string{"They can hide unequal benefits across groups", "They make local evidence impossible to collect", "They always exaggerate the cost of services", "They prove that central districts receive no support"}, AnswerKey: "They can hide unequal benefits across groups"},
			{Type: "Multiple Choice", Question: "What may happen if maintenance is not planned from the beginning?", ParagraphRef: "Paragraph 3", Evidence: "gradually lose reliability", Options: []string{"The project may gradually lose reliability", "The opening date will become more popular", "User questions will disappear immediately", "Data checking will become unnecessary"}, AnswerKey: "The project may gradually lose reliability"},
			{Type: "Multiple Choice", Question: "Why are broad labels such as innovation or sustainability insufficient?", ParagraphRef: "Paragraph 6", Evidence: "do not tell affected groups what will change in practical terms", Options: []string{"They do not explain the specific actions users must take", "They provide too many exact operating instructions", "They remove the need for user support", "They make the underlying design unsound"}, AnswerKey: "They do not explain the specific actions users must take"},
			{Type: "Multiple Choice", Question: "What does paragraph 7 say adaptive management involves?", ParagraphRef: "Paragraph 7", Evidence: "testing a limited version, collecting evidence, changing the weak parts", Options: []string{"Testing a limited version and revising weak parts", "Announcing a complete solution without revision", "Replacing local experience with technical knowledge", "Avoiding evidence until public trust declines"}, AnswerKey: "Testing a limited version and revising weak parts"},
		}
		return quiz[:min(requiredQuiz, len(quiz))]
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
