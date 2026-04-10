package store

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]domain.User
	byEmail  map[string]string
	theater  map[string]domain.Theater
	sessions map[string]domain.RoleplaySession
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:   map[string]domain.User{},
		byEmail: map[string]string{},
		theater: map[string]domain.Theater{},
		sessions: map[string]domain.RoleplaySession{},
	}
}

func (s *MemoryStore) CreateUser(email string, passwordHash string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byEmail[email]; exists {
		return domain.User{}, errors.New("email already exists")
	}
	id := uuid.NewString()
	user := domain.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		TotalXP:      0,
		CreatedAt:    time.Now(),
	}
	s.users[id] = user
	s.byEmail[email] = id
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

func (s *MemoryStore) GetUserByEmail(email string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byEmail[email]
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
