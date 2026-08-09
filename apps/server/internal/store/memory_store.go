package store

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/analytics"
	"github.com/linguaquest/server/internal/domain"
)

type MemoryStore struct {
	mu                  sync.RWMutex
	users               map[string]domain.User
	byEmail             map[string][]string
	byUsername          map[string]string
	authTokens          map[string]memoryAuthToken
	theater             map[string]domain.Theater
	readings            map[string]domain.ReadingMaterial
	voices              map[string]domain.VoiceProfile
	sessions            map[string]domain.RoleplaySession
	oauth               map[string]domain.OAuthAccount
	writings            map[string]domain.WritingSession
	orders              map[string]domain.PaymentOrder
	billing             map[string]domain.BillingStatus
	creditUses          map[string]memoryCreditUse
	xpEvents            map[string]domain.XPEvent
	modelUsageDaily     map[string]analytics.ModelUsage
	productMetricsDaily map[string]analytics.ProductMetric
	model               domain.ModelConfig
	tts                 domain.TTSConfig
	asr                 domain.ASRConfig
}

type memoryAuthToken struct {
	UserID    string
	Purpose   string
	ExpiresAt time.Time
}

type memoryCreditUse struct {
	UserID    string
	Activity  string
	SourceID  string
	Amount    int
	IsFree    bool
	CreatedAt time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:               map[string]domain.User{},
		byEmail:             map[string][]string{},
		byUsername:          map[string]string{},
		authTokens:          map[string]memoryAuthToken{},
		theater:             map[string]domain.Theater{},
		readings:            map[string]domain.ReadingMaterial{},
		voices:              map[string]domain.VoiceProfile{},
		sessions:            map[string]domain.RoleplaySession{},
		oauth:               map[string]domain.OAuthAccount{},
		writings:            map[string]domain.WritingSession{},
		orders:              map[string]domain.PaymentOrder{},
		billing:             map[string]domain.BillingStatus{},
		creditUses:          map[string]memoryCreditUse{},
		xpEvents:            map[string]domain.XPEvent{},
		modelUsageDaily:     map[string]analytics.ModelUsage{},
		productMetricsDaily: map[string]analytics.ProductMetric{},
	}
}

func (s *MemoryStore) SaveOAuthAccount(account domain.OAuthAccount) domain.OAuthAccount {
	s.mu.Lock()
	defer s.mu.Unlock()
	if account.ID == "" {
		account.ID = uuid.NewString()
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	account.UpdatedAt = time.Now()
	s.oauth[account.ID] = account
	return account
}

func (s *MemoryStore) ListOAuthAccounts(provider string) ([]domain.OAuthAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	result := make([]domain.OAuthAccount, 0, len(s.oauth))
	for _, account := range s.oauth {
		if normalizedProvider != "" && strings.ToLower(strings.TrimSpace(account.Provider)) != normalizedProvider {
			continue
		}
		result = append(result, account)
	}
	return result, nil
}

func (s *MemoryStore) CreateUser(username string, email string, passwordHash string, emailVerified bool) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	email = strings.ToLower(strings.TrimSpace(email))
	usernameKey := strings.ToLower(strings.TrimSpace(username))
	if _, exists := s.byUsername[usernameKey]; exists {
		return domain.User{}, errors.New("username already exists")
	}
	if len(s.byEmail[email]) >= 3 {
		return domain.User{}, errors.New("email account limit reached")
	}
	id := uuid.NewString()
	user := domain.User{
		ID:            id,
		Username:      username,
		Email:         email,
		EmailVerified: emailVerified,
		PasswordHash:  passwordHash,
		TotalXP:       0,
		CreatedAt:     time.Now(),
	}
	s.users[id] = user
	s.byEmail[email] = append(s.byEmail[email], id)
	s.byUsername[usernameKey] = id
	return user, nil
}

func (s *MemoryStore) UpdateUserProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	if nickname != "" {
		user.Nickname = nickname
	}
	user.AvatarURL = avatarURL
	user.Bio = bio
	s.users[userID] = user
	return user, nil
}

func (s *MemoryStore) GetModelConfig() (domain.ModelConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.model.Model == "" && s.model.BaseURL == "" && s.model.APIKey == "" {
		return domain.ModelConfig{}, errors.New("model config not found")
	}
	return s.model, nil
}

func (s *MemoryStore) SaveModelConfig(config domain.ModelConfig) (domain.ModelConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = config
	return s.model, nil
}

func (s *MemoryStore) GetTTSConfig() (domain.TTSConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.tts.Provider == "" && s.tts.BaseURL == "" && s.tts.APIKey == "" {
		return domain.TTSConfig{}, errors.New("tts config not found")
	}
	return s.tts, nil
}

func (s *MemoryStore) SaveTTSConfig(config domain.TTSConfig) (domain.TTSConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tts = config
	return s.tts, nil
}

func (s *MemoryStore) GetASRConfig() (domain.ASRConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.asr.Provider == "" && s.asr.BaseURL == "" && s.asr.APIKey == "" {
		return domain.ASRConfig{}, errors.New("asr config not found")
	}
	return s.asr, nil
}

func (s *MemoryStore) SaveASRConfig(config domain.ASRConfig) (domain.ASRConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asr = config
	return s.asr, nil
}

func (s *MemoryStore) GetUserByEmail(email string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if len(ids) == 0 {
		return domain.User{}, errors.New("user not found")
	}
	return s.users[ids[0]], nil
}

func (s *MemoryStore) ListUsersByEmail(email string) ([]domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	users := make([]domain.User, 0, len(ids))
	for _, id := range ids {
		users = append(users, s.users[id])
	}
	return users, nil
}

func (s *MemoryStore) GetUserByUsername(username string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byUsername[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	return s.users[id], nil
}

func (s *MemoryStore) GetUserByID(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	return user, nil
}

func (s *MemoryStore) UpdateUserPassword(userID string, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	user.PasswordHash = passwordHash
	s.users[userID] = user
	return nil
}

func (s *MemoryStore) SetUserEmailVerified(userID string, verified bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	user.EmailVerified = verified
	s.users[userID] = user
	return nil
}

func (s *MemoryStore) CreateAuthToken(userID string, purpose string, tokenHash string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, token := range s.authTokens {
		if token.UserID == userID && token.Purpose == purpose {
			delete(s.authTokens, hash)
		}
	}
	s.authTokens[tokenHash] = memoryAuthToken{UserID: userID, Purpose: purpose, ExpiresAt: expiresAt}
	return nil
}

func (s *MemoryStore) ConsumeAuthToken(tokenHash string, purpose string, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.authTokens[tokenHash]
	if !ok || token.Purpose != purpose || !token.ExpiresAt.After(now) {
		return "", errors.New("token not found")
	}
	delete(s.authTokens, tokenHash)
	return token.UserID, nil
}

func (s *MemoryStore) SaveVoiceProfile(profile domain.VoiceProfile) (domain.VoiceProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(profile.ID) == "" || strings.TrimSpace(profile.UserID) == "" {
		return domain.VoiceProfile{}, errors.New("voice profile id and user id are required")
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now()
	}
	s.voices[profile.ID] = profile
	return profile, nil
}

func (s *MemoryStore) ListVoiceProfiles(userID string) ([]domain.VoiceProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profiles := make([]domain.VoiceProfile, 0)
	for _, profile := range s.voices {
		if profile.UserID == userID {
			profiles = append(profiles, profile)
		}
	}
	sort.Slice(profiles, func(i int, j int) bool { return profiles[i].CreatedAt.After(profiles[j].CreatedAt) })
	return profiles, nil
}

func (s *MemoryStore) DeleteVoiceProfile(userID string, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.voices[profileID]
	if !ok || profile.UserID != userID {
		return errors.New("voice profile not found")
	}
	delete(s.voices, profileID)
	return nil
}

func (s *MemoryStore) SaveTheater(theater domain.Theater) (domain.Theater, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.theater[theater.ID] = theater
	return theater, nil
}

func (s *MemoryStore) GetTheater(id string) (domain.Theater, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	theater, ok := s.theater[id]
	if !ok {
		return domain.Theater{}, errors.New("theater not found")
	}
	return theater, nil
}

func (s *MemoryStore) GetTheaterByShareCode(shareCode string) (domain.Theater, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	needle := strings.ToUpper(strings.TrimSpace(shareCode))
	for _, theater := range s.theater {
		if strings.ToUpper(strings.TrimSpace(theater.ShareCode)) == needle {
			return theater, nil
		}
	}
	return domain.Theater{}, errors.New("theater not found")
}

func (s *MemoryStore) ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Theater, 0)
	for _, theater := range s.theater {
		if theater.UserID != userID {
			continue
		}
		if language != "" && theater.Language != language {
			continue
		}
		if status != "" && theater.Status != status {
			continue
		}
		if favorite != nil && theater.IsFavorite != *favorite {
			continue
		}
		theater.Characters = nil
		theater.Dialogues = nil
		theater.QuizQuestions = nil
		result = append(result, theater)
	}
	return result, nil
}

func (s *MemoryStore) SetTheaterFavorite(userID string, theaterID string, favorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	theater, ok := s.theater[theaterID]
	if !ok || theater.UserID != userID {
		return errors.New("theater not found")
	}
	theater.IsFavorite = favorite
	s.theater[theaterID] = theater
	return nil
}

func (s *MemoryStore) SetTheaterShareCode(userID string, theaterID string, shareCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	theater, ok := s.theater[theaterID]
	if !ok || theater.UserID != userID {
		return errors.New("theater not found")
	}
	theater.ShareCode = shareCode
	s.theater[theaterID] = theater
	return nil
}

func (s *MemoryStore) DeleteTheater(userID string, theaterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	theater, ok := s.theater[theaterID]
	if !ok || theater.UserID != userID {
		return errors.New("theater not found")
	}
	delete(s.theater, theaterID)
	for id, session := range s.sessions {
		if session.UserID == userID && session.TheaterID == theaterID {
			delete(s.sessions, id)
		}
	}
	return nil
}

func (s *MemoryStore) AddUserXP(userID string, xp int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	user.TotalXP += xp
	s.users[userID] = user
	return nil
}

func (s *MemoryStore) SavePracticeRecord(_ string, _ string, _ int, answers []string, _ int) error {
	_, err := json.Marshal(answers)
	return err
}

func (s *MemoryStore) SaveReadingPracticeRecord(_ string, _ string, _ int, answers []string, _ int) error {
	_, err := json.Marshal(answers)
	return err
}

func (s *MemoryStore) ListCourses(language string) ([]domain.Course, error) {
	seed := []domain.Course{
		{ID: "c1", Language: "CANTONESE", Category: "daily", Title: "茶餐厅点单", Description: "日常场景对话", MinLevel: 4.0, MaxLevel: 6.0, IsActive: true},
		{ID: "c2", Language: "ENGLISH", Category: "ielts", Title: "Describe a memorable trip", Description: "IELTS 口语主题", MinLevel: 5.5, MaxLevel: 8.0, IsActive: true},
	}
	result := make([]domain.Course, 0)
	for _, item := range seed {
		if language == "" || item.Language == language {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *MemoryStore) SaveReadingMaterial(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readings[material.ID] = material
	return material, nil
}

func (s *MemoryStore) UpdateReadingMaterialExisting(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.readings[material.ID]
	if !ok || current.UserID != material.UserID {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	s.readings[material.ID] = material
	return material, nil
}

func (s *MemoryStore) GetReadingMaterial(id string, userID string) (domain.ReadingMaterial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	material, ok := s.readings[id]
	if !ok || (userID != "" && material.UserID != userID) {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	return material, nil
}

func (s *MemoryStore) ListReadingMaterialsByUser(userID string, exam string) ([]domain.ReadingMaterial, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.ReadingMaterial, 0)
	for _, material := range s.readings {
		if material.UserID != userID {
			continue
		}
		if exam != "" && material.Exam != exam {
			continue
		}
		result = append(result, material)
	}
	return result, nil
}

func (s *MemoryStore) DeleteReadingMaterial(userID string, materialID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	material, ok := s.readings[materialID]
	if !ok || material.UserID != userID {
		return errors.New("reading material not found")
	}
	delete(s.readings, materialID)
	return nil
}

func (s *MemoryStore) CreateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return session, nil
}

func (s *MemoryStore) GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.UserID != userID {
		return domain.RoleplaySession{}, errors.New("roleplay session not found")
	}
	return session, nil
}

func (s *MemoryStore) UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return session, nil
}

func (s *MemoryStore) SaveWritingSession(session domain.WritingSession) (domain.WritingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session.ID == "" || session.UserID == "" {
		return domain.WritingSession{}, errors.New("writing session id and user id are required")
	}
	s.writings[session.ID] = session
	return session, nil
}

func (s *MemoryStore) UpdateWritingSessionExisting(session domain.WritingSession) (domain.WritingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.writings[session.ID]
	if !ok || current.UserID != session.UserID {
		return domain.WritingSession{}, errors.New("writing session not found")
	}
	s.writings[session.ID] = session
	return session, nil
}

func (s *MemoryStore) GetWritingSession(sessionID string, userID string) (domain.WritingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.writings[sessionID]
	if !ok || item.UserID != userID {
		return domain.WritingSession{}, errors.New("writing session not found")
	}
	return item, nil
}

func (s *MemoryStore) ListWritingSessions(userID string) ([]domain.WritingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.WritingSession, 0)
	for _, item := range s.writings {
		if item.UserID == userID {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (s *MemoryStore) DeleteWritingSession(userID string, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.writings[sessionID]
	if !ok || session.UserID != userID {
		return errors.New("writing session not found")
	}
	delete(s.writings, sessionID)
	return nil
}
