package store

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

func (s *MemoryStore) AwardXP(event domain.XPEvent, dailyCap int, dayStart time.Time) (domain.XPAward, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := xpEventKey(event.UserID, event.Activity, event.SourceID)
	if prior, exists := s.xpEvents[key]; exists {
		return domain.XPAward{Event: prior, Duplicate: true}, nil
	}
	user, exists := s.users[event.UserID]
	if !exists {
		return domain.XPAward{}, errors.New("user not found")
	}
	awardedToday := 0
	for _, prior := range s.xpEvents {
		if prior.UserID == event.UserID && !prior.CreatedAt.Before(dayStart) {
			awardedToday += prior.XPEarned
		}
	}
	granted := event.XPEarned
	if remaining := dailyCap - awardedToday; remaining < granted {
		granted = max(0, remaining)
	}
	event.XPEarned = granted
	s.xpEvents[key] = event
	user.TotalXP += granted
	s.users[event.UserID] = user
	return domain.XPAward{Event: event, GrantedXP: granted}, nil
}

func (s *MemoryStore) ListXPEvents(userID string, limit int) ([]domain.XPEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.XPEvent, 0)
	for _, event := range s.xpEvents {
		if event.UserID == userID {
			items = append(items, event)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func xpEventKey(userID string, activity string, sourceID string) string {
	return strings.Join([]string{userID, strings.TrimSpace(activity), strings.TrimSpace(sourceID)}, "\x00")
}
