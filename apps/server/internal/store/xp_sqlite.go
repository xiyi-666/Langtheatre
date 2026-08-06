package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

func (s *SQLiteStore) AwardXP(event domain.XPEvent, dailyCap int, dayStart time.Time) (domain.XPAward, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return domain.XPAward{}, err
	}
	defer tx.Rollback()
	var prior domain.XPEvent
	var priorCreated string
	err = tx.QueryRow(`SELECT id, user_id, activity, source_id, xp_earned, created_at FROM xp_events WHERE user_id = ? AND activity = ? AND source_id = ?`, event.UserID, event.Activity, event.SourceID).Scan(&prior.ID, &prior.UserID, &prior.Activity, &prior.SourceID, &prior.XPEarned, &priorCreated)
	if err == nil {
		prior.CreatedAt, err = parseSQLiteBillingTime(priorCreated)
		if err != nil {
			return domain.XPAward{}, err
		}
		if err = tx.Commit(); err != nil {
			return domain.XPAward{}, err
		}
		return domain.XPAward{Event: prior, Duplicate: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.XPAward{}, err
	}
	var awardedToday int
	if err = tx.QueryRow(`SELECT COALESCE(SUM(xp_earned), 0) FROM xp_events WHERE user_id = ? AND created_at >= ?`, event.UserID, dayStart.Format(sqliteTimeLayout)).Scan(&awardedToday); err != nil {
		return domain.XPAward{}, err
	}
	granted := event.XPEarned
	if remaining := dailyCap - awardedToday; remaining < granted {
		granted = max(0, remaining)
	}
	event.XPEarned = granted
	if _, err = tx.Exec(`INSERT INTO xp_events (id, user_id, activity, source_id, xp_earned, created_at) VALUES (?, ?, ?, ?, ?, ?)`, event.ID, event.UserID, event.Activity, event.SourceID, event.XPEarned, event.CreatedAt.Format(sqliteTimeLayout)); err != nil {
		return domain.XPAward{}, err
	}
	result, err := tx.Exec(`UPDATE users SET total_xp = total_xp + ? WHERE id = ?`, granted, event.UserID)
	if err != nil {
		return domain.XPAward{}, err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return domain.XPAward{}, errors.New("user not found")
	}
	if err = tx.Commit(); err != nil {
		return domain.XPAward{}, err
	}
	return domain.XPAward{Event: event, GrantedXP: granted}, nil
}

func (s *SQLiteStore) ListXPEvents(userID string, limit int) ([]domain.XPEvent, error) {
	rows, err := s.db.Query(`SELECT id, user_id, activity, source_id, xp_earned, created_at FROM xp_events WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.XPEvent, 0)
	for rows.Next() {
		var event domain.XPEvent
		var created string
		if err = rows.Scan(&event.ID, &event.UserID, &event.Activity, &event.SourceID, &event.XPEarned, &created); err != nil {
			return nil, err
		}
		if event.CreatedAt, err = parseSQLiteBillingTime(created); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}
