package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

const dailyXPCap = 300

type XPStore interface {
	AwardXP(event domain.XPEvent, dailyCap int, dayStart time.Time) (domain.XPAward, error)
	ListXPEvents(userID string, limit int) ([]domain.XPEvent, error)
}

func (s *Service) awardLearningXP(userID string, activity string, sourceID string, quality int) (int, error) {
	if userID == "" || sourceID == "" {
		return 0, errors.New("missing XP event identity")
	}
	event := domain.XPEvent{ID: uuid.NewString(), UserID: userID, Activity: activity, SourceID: sourceID, XPEarned: learningXPAmount(activity, quality), CreatedAt: time.Now().In(time.FixedZone("CST", 8*60*60))}
	if store, ok := s.store.(XPStore); ok {
		dayStart := time.Date(event.CreatedAt.Year(), event.CreatedAt.Month(), event.CreatedAt.Day(), 0, 0, 0, 0, event.CreatedAt.Location())
		award, err := store.AwardXP(event, dailyXPCap, dayStart)
		if err != nil {
			return 0, err
		}
		return award.GrantedXP, nil
	}
	if err := s.store.AddUserXP(userID, event.XPEarned); err != nil {
		return 0, err
	}
	return event.XPEarned, nil
}

func (s *Service) XPEvents(userID string, limit int) ([]domain.XPEvent, error) {
	store, ok := s.store.(XPStore)
	if !ok {
		return []domain.XPEvent{}, nil
	}
	if limit <= 0 || limit > 30 {
		limit = 12
	}
	return store.ListXPEvents(userID, limit)
}

func learningXPAmount(activity string, quality int) int {
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}
	switch activity {
	case "THEATER_PRACTICE":
		return 10 + quality*22/100
	case "READING_PRACTICE":
		return 10 + quality*24/100
	case "ROLEPLAY_COMPLETE":
		return 16 + quality*30/100
	case "WRITING_COMPLETE":
		return 20 + quality*40/100
	case "SPEAKING_TURN":
		return 8 + quality*18/100
	default:
		return 8
	}
}

func learningProgress(totalXP int) domain.LearningProgress {
	if totalXP < 0 {
		totalXP = 0
	}
	remaining := totalXP
	for level := 1; level < domain.MaxLevel; level++ {
		required := xpForNextLevel(level)
		if remaining < required {
			return domain.LearningProgress{Level: level, XPIntoLevel: remaining, XPToNextLevel: required, LevelProgress: remaining * 100 / required, RankCode: rankForLevel(level), RankLabel: rankLabelForLevel(level)}
		}
		remaining -= required
	}
	return domain.LearningProgress{Level: domain.MaxLevel, XPIntoLevel: 0, XPToNextLevel: 0, LevelProgress: 100, RankCode: "LEGEND", RankLabel: "Lingua 传说"}
}

func xpForNextLevel(level int) int {
	switch {
	case level < 100:
		return 6 + (level-1)/25
	case level < 300:
		return 10 + (level-100)/20
	case level < 600:
		return 20 + (level-300)/18
	default:
		return 38 + (level-600)/16
	}
}

func rankForLevel(level int) string {
	switch {
	case level >= 999:
		return "LEGEND"
	case level >= 900:
		return "SOVEREIGN"
	case level >= 800:
		return "CELESTIAL"
	case level >= 650:
		return "MASTER"
	case level >= 500:
		return "EXPERT"
	case level >= 350:
		return "SCHOLAR"
	case level >= 200:
		return "ADEPT"
	case level >= 100:
		return "VOYAGER"
	case level >= 50:
		return "EXPLORER"
	default:
		return "NOVICE"
	}
}

func rankLabelForLevel(level int) string {
	labels := map[string]string{
		"NOVICE": "初学探索者", "EXPLORER": "语言行者", "VOYAGER": "表达旅人", "ADEPT": "进阶研习者", "SCHOLAR": "语境学者", "EXPERT": "表达专家", "MASTER": "语言大师", "CELESTIAL": "星耀宗师", "SOVEREIGN": "至尊领航者", "LEGEND": "Lingua 传说",
	}
	return labels[rankForLevel(level)]
}

func decorateLearningProgress(user domain.User) domain.User {
	progress := learningProgress(user.TotalXP)
	user.Level = progress.Level
	user.XPIntoLevel = progress.XPIntoLevel
	user.XPToNextLevel = progress.XPToNextLevel
	user.LevelProgress = progress.LevelProgress
	user.RankCode = progress.RankCode
	user.RankLabel = progress.RankLabel
	return user
}
