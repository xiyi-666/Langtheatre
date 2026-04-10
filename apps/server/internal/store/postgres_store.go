package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linguaquest/server/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) CreateUser(email string, passwordHash string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id::text, email, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) GetUserByEmail(email string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, email, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) GetUserByID(id string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, email, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at
		 FROM users WHERE id = $1::uuid`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) UpdateUserProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`UPDATE users
		 SET nickname = COALESCE(NULLIF($2, ''), nickname),
		     avatar_url = $3,
		     bio = $4
		 WHERE id = $1::uuid
		 RETURNING id::text, email, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at`,
		userID, nickname, avatarURL, bio,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) SaveTheater(theater domain.Theater) (domain.Theater, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialoguesJSON, err := json.Marshal(theater.Dialogues)
	if err != nil {
		return domain.Theater{}, err
	}
	quizJSON, err := json.Marshal(theater.QuizQuestions)
	if err != nil {
		return domain.Theater{}, err
	}
	charactersJSON, err := json.Marshal(theater.Characters)
	if err != nil {
		return domain.Theater{}, err
	}
	err = s.pool.QueryRow(
		ctx,
		`INSERT INTO theaters (id, user_id, language, topic, difficulty, mode, status, scene_description, characters, dialogues, quiz_questions, is_favorite, share_code)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, false, '')
		 RETURNING id::text, user_id::text, language, topic, difficulty, mode, status, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), created_at`,
		theater.ID, theater.UserID, theater.Language, theater.Topic, theater.Difficulty, theater.Mode, theater.Status, theater.SceneDescription, string(charactersJSON), string(dialoguesJSON), string(quizJSON),
	).Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.IsFavorite, &theater.ShareCode, &theater.SceneDescription, &charactersJSON, &theater.CreatedAt)
	if err != nil {
		return domain.Theater{}, err
	}
	_ = json.Unmarshal(charactersJSON, &theater.Characters)
	return theater, nil
}

func (s *PostgresStore) GetTheater(id string) (domain.Theater, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	theater := domain.Theater{}
	var charactersRaw []byte
	var dialoguesRaw []byte
	var quizRaw []byte
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, user_id::text, language, topic, difficulty, mode, status, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), dialogues, COALESCE(quiz_questions, '[]'::jsonb), created_at
		 FROM theaters WHERE id = $1::uuid`,
		id,
	).Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.IsFavorite, &theater.ShareCode, &theater.SceneDescription, &charactersRaw, &dialoguesRaw, &quizRaw, &theater.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Theater{}, errors.New("theater not found")
	}
	if err != nil {
		return domain.Theater{}, err
	}
	if len(charactersRaw) > 0 {
		if err = json.Unmarshal(charactersRaw, &theater.Characters); err != nil {
			return domain.Theater{}, err
		}
	}
	if len(dialoguesRaw) > 0 {
		if err = json.Unmarshal(dialoguesRaw, &theater.Dialogues); err != nil {
			return domain.Theater{}, err
		}
	}
	if len(quizRaw) > 0 {
		if err = json.Unmarshal(quizRaw, &theater.QuizQuestions); err != nil {
			return domain.Theater{}, err
		}
	}
	return theater, nil
}

func (s *PostgresStore) ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	filterFavorite := false
	favoriteEnabled := false
	if favorite != nil {
		favoriteEnabled = true
		filterFavorite = *favorite
	}
	rows, err := s.pool.Query(
		ctx,
		`SELECT id::text, user_id::text, language, topic, difficulty, mode, status, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), dialogues, COALESCE(quiz_questions, '[]'::jsonb), created_at
		 FROM theaters
		 WHERE user_id = $1::uuid
		   AND ($2 = '' OR language = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4 = false OR is_favorite = $5)
		 ORDER BY created_at DESC`,
		userID, language, status, favoriteEnabled, filterFavorite,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Theater, 0)
	for rows.Next() {
		var item domain.Theater
		var charactersRaw []byte
		var dialoguesRaw []byte
		var quizRaw []byte
		if scanErr := rows.Scan(
			&item.ID, &item.UserID, &item.Language, &item.Topic, &item.Difficulty, &item.Mode,
			&item.Status, &item.IsFavorite, &item.ShareCode, &item.SceneDescription, &charactersRaw, &dialoguesRaw, &quizRaw, &item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		if len(charactersRaw) > 0 {
			if unmarshalErr := json.Unmarshal(charactersRaw, &item.Characters); unmarshalErr != nil {
				return nil, unmarshalErr
			}
		}
		if len(dialoguesRaw) > 0 {
			if unmarshalErr := json.Unmarshal(dialoguesRaw, &item.Dialogues); unmarshalErr != nil {
				return nil, unmarshalErr
			}
		}
		if len(quizRaw) > 0 {
			if unmarshalErr := json.Unmarshal(quizRaw, &item.QuizQuestions); unmarshalErr != nil {
				return nil, unmarshalErr
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) SetTheaterFavorite(userID string, theaterID string, favorite bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(
		ctx,
		`UPDATE theaters SET is_favorite = $3 WHERE id = $1::uuid AND user_id = $2::uuid`,
		theaterID, userID, favorite,
	)
	return err
}

func (s *PostgresStore) SetTheaterShareCode(userID string, theaterID string, shareCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(
		ctx,
		`UPDATE theaters SET share_code = $3 WHERE id = $1::uuid AND user_id = $2::uuid`,
		theaterID, userID, shareCode,
	)
	return err
}

func (s *PostgresStore) DeleteTheater(userID string, theaterID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `DELETE FROM practice_records WHERE user_id = $1::uuid AND theater_id = $2::uuid`, userID, theaterID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM roleplay_sessions WHERE user_id = $1::uuid AND theater_id = $2::uuid`, userID, theaterID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `DELETE FROM theaters WHERE id = $1::uuid AND user_id = $2::uuid`, theaterID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("theater not found")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) AddUserXP(userID string, xp int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(
		ctx,
		`UPDATE users SET total_xp = total_xp + $2 WHERE id = $1::uuid`,
		userID, xp,
	)
	return err
}

func (s *PostgresStore) SavePracticeRecord(userID string, theaterID string, score int, answers []string, xpEarned int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(
		ctx,
		`INSERT INTO practice_records (user_id, theater_id, score, answers, xp_earned)
		 VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5)`,
		userID, theaterID, score, string(answersJSON), xpEarned,
	)
	return err
}

func (s *PostgresStore) ListCourses(language string) ([]domain.Course, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(
		ctx,
		`SELECT id::text, language, category, title, description, min_level, max_level, is_active
		 FROM courses
		 WHERE is_active = true AND ($1 = '' OR language = $1)
		 ORDER BY min_level ASC, title ASC`,
		language,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Course, 0)
	for rows.Next() {
		var item domain.Course
		if scanErr := rows.Scan(&item.ID, &item.Language, &item.Category, &item.Title, &item.Description, &item.MinLevel, &item.MaxLevel, &item.IsActive); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) CreateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := json.Marshal(session.Transcript)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	err = s.pool.QueryRow(
		ctx,
		`INSERT INTO roleplay_sessions (id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, final_feedback)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb, $8, $9)
		 RETURNING id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(final_feedback, ''), created_at, updated_at`,
		session.ID, session.UserID, session.TheaterID, session.UserRole, session.TurnIndex, session.CurrentScore, string(raw), session.Status, session.FinalFeedback,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	_ = json.Unmarshal(raw, &session.Transcript)
	return session, nil
}

func (s *PostgresStore) GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var session domain.RoleplaySession
	var raw []byte
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(final_feedback, ''), created_at, updated_at
		 FROM roleplay_sessions
		 WHERE id = $1::uuid AND user_id = $2::uuid`,
		sessionID, userID,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoleplaySession{}, errors.New("roleplay session not found")
	}
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	_ = json.Unmarshal(raw, &session.Transcript)
	return session, nil
}

func (s *PostgresStore) UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := json.Marshal(session.Transcript)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	err = s.pool.QueryRow(
		ctx,
		`UPDATE roleplay_sessions
		 SET turn_index = $3, current_score = $4, transcript = $5::jsonb, status = $6, final_feedback = $7, updated_at = NOW()
		 WHERE id = $1::uuid AND user_id = $2::uuid
		 RETURNING id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(final_feedback, ''), created_at, updated_at`,
		session.ID, session.UserID, session.TurnIndex, session.CurrentScore, string(raw), session.Status, session.FinalFeedback,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	_ = json.Unmarshal(raw, &session.Transcript)
	return session, nil
}
