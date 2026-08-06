package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/linguaquest/server/internal/domain"
)

func (s *PostgresStore) AwardXP(event domain.XPEvent, dailyCap int, dayStart time.Time) (domain.XPAward, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.XPAward{}, err
	}
	defer tx.Rollback(ctx)
	var prior domain.XPEvent
	err = tx.QueryRow(ctx, `SELECT id, user_id::text, activity, source_id, xp_earned, created_at FROM xp_events WHERE user_id=$1::uuid AND activity=$2 AND source_id=$3`, event.UserID, event.Activity, event.SourceID).Scan(&prior.ID, &prior.UserID, &prior.Activity, &prior.SourceID, &prior.XPEarned, &prior.CreatedAt)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return domain.XPAward{}, err
		}
		return domain.XPAward{Event: prior, Duplicate: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.XPAward{}, err
	}
	var awardedToday int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(xp_earned), 0) FROM xp_events WHERE user_id=$1::uuid AND created_at >= $2`, event.UserID, dayStart).Scan(&awardedToday); err != nil {
		return domain.XPAward{}, err
	}
	granted := event.XPEarned
	if remaining := dailyCap - awardedToday; remaining < granted {
		granted = max(0, remaining)
	}
	event.XPEarned = granted
	if _, err = tx.Exec(ctx, `INSERT INTO xp_events (id, user_id, activity, source_id, xp_earned, created_at) VALUES ($1, $2::uuid, $3, $4, $5, $6)`, event.ID, event.UserID, event.Activity, event.SourceID, event.XPEarned, event.CreatedAt); err != nil {
		return domain.XPAward{}, err
	}
	result, err := tx.Exec(ctx, `UPDATE users SET total_xp = total_xp + $2 WHERE id=$1::uuid`, event.UserID, granted)
	if err != nil {
		return domain.XPAward{}, err
	}
	if result.RowsAffected() == 0 {
		return domain.XPAward{}, errors.New("user not found")
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.XPAward{}, err
	}
	return domain.XPAward{Event: event, GrantedXP: granted}, nil
}

func (s *PostgresStore) ListXPEvents(userID string, limit int) ([]domain.XPEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT id, user_id::text, activity, source_id, xp_earned, created_at FROM xp_events WHERE user_id=$1::uuid ORDER BY created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.XPEvent, 0)
	for rows.Next() {
		var event domain.XPEvent
		if err = rows.Scan(&event.ID, &event.UserID, &event.Activity, &event.SourceID, &event.XPEarned, &event.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}
