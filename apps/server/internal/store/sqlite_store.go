package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const sqliteTimeLayout = time.RFC3339Nano

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return nil, errors.New("sqlite path is required")
	}
	db, err := sql.Open("sqlite", cleaned)
	if err != nil {
		return nil, err
	}
	if err = applySQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func applySQLiteSchema(db *sql.DB) error {
	stmts := []string{
		"PRAGMA foreign_keys = ON",
		`CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            nickname TEXT,
            avatar_url TEXT,
            bio TEXT,
            total_xp INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS theaters (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            language TEXT,
            topic TEXT,
            difficulty REAL,
            mode TEXT,
            status TEXT,
            is_favorite INTEGER NOT NULL DEFAULT 0,
            share_code TEXT,
            scene_description TEXT,
            characters TEXT NOT NULL DEFAULT '[]',
            dialogues TEXT NOT NULL,
            quiz_questions TEXT NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_theaters_user ON theaters(user_id)`,
		`CREATE TABLE IF NOT EXISTS practice_records (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id TEXT NOT NULL,
            theater_id TEXT NOT NULL,
            score INTEGER,
            answers TEXT,
            xp_earned INTEGER,
            created_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS roleplay_sessions (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            theater_id TEXT NOT NULL,
            user_role TEXT,
            turn_index INTEGER,
            current_score INTEGER,
            transcript TEXT NOT NULL,
            status TEXT,
            final_feedback TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`ALTER TABLE theaters ADD COLUMN scene_description TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE theaters ADD COLUMN characters TEXT NOT NULL DEFAULT '[]'`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	return nil
}

func (s *SQLiteStore) CreateUser(email string, passwordHash string) (domain.User, error) {
	user := domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash, nickname, avatar_url, bio, total_xp, created_at)
         VALUES (?, ?, ?, '', '', '', 0, ?)`,
		user.ID, user.Email, user.PasswordHash, user.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		if isUniqueConstraint(err, "users.email") {
			return domain.User{}, errors.New("email already exists")
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *SQLiteStore) UpdateUserProfile(userID string, nickname string, avatarURL string, bio string) (domain.User, error) {
	res, err := s.db.Exec(`UPDATE users SET nickname = ?, avatar_url = ?, bio = ? WHERE id = ?`, nickname, avatarURL, bio, userID)
	if err != nil {
		return domain.User{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.User{}, errors.New("user not found")
	}
	return s.GetUserByID(userID)
}

func (s *SQLiteStore) GetUserByEmail(email string) (domain.User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByID(id string) (domain.User, error) {
	row := s.db.QueryRow(`SELECT id, email, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *SQLiteStore) SaveTheater(theater domain.Theater) (domain.Theater, error) {
	if theater.ID == "" {
		theater.ID = uuid.NewString()
	}
	if theater.CreatedAt.IsZero() {
		theater.CreatedAt = time.Now().UTC()
	}
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
	_, err = s.db.Exec(
		`INSERT INTO theaters (id, user_id, language, topic, difficulty, mode, status, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            user_id=excluded.user_id,
            language=excluded.language,
            topic=excluded.topic,
            difficulty=excluded.difficulty,
            mode=excluded.mode,
            status=excluded.status,
            is_favorite=excluded.is_favorite,
            share_code=excluded.share_code,
            scene_description=excluded.scene_description,
            characters=excluded.characters,
            dialogues=excluded.dialogues,
            quiz_questions=excluded.quiz_questions,
            created_at=excluded.created_at`,
		theater.ID,
		theater.UserID,
		theater.Language,
		theater.Topic,
		theater.Difficulty,
		theater.Mode,
		theater.Status,
		boolToInt(theater.IsFavorite),
		theater.ShareCode,
		theater.SceneDescription,
		string(charactersJSON),
		string(dialoguesJSON),
		string(quizJSON),
		theater.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.Theater{}, err
	}
	return theater, nil
}

func (s *SQLiteStore) GetTheater(id string) (domain.Theater, error) {
	row := s.db.QueryRow(`SELECT id, user_id, language, topic, difficulty, mode, status, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at FROM theaters WHERE id = ?`, id)
	return scanTheater(row)
}

func (s *SQLiteStore) ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	query := `SELECT id, user_id, language, topic, difficulty, mode, status, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at FROM theaters WHERE user_id = ?`
	args := []any{userID}
	if language != "" {
		query += " AND language = ?"
		args = append(args, language)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if favorite != nil {
		query += " AND is_favorite = ?"
		args = append(args, boolToInt(*favorite))
	}
	query += " ORDER BY datetime(created_at) DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Theater, 0)
	for rows.Next() {
		theater, err := scanTheater(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, theater)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SQLiteStore) SetTheaterFavorite(userID string, theaterID string, favorite bool) error {
	res, err := s.db.Exec(`UPDATE theaters SET is_favorite = ? WHERE id = ? AND user_id = ?`, boolToInt(favorite), theaterID, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("theater not found")
	}
	return nil
}

func (s *SQLiteStore) SetTheaterShareCode(userID string, theaterID string, shareCode string) error {
	res, err := s.db.Exec(`UPDATE theaters SET share_code = ? WHERE id = ? AND user_id = ?`, shareCode, theaterID, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("theater not found")
	}
	return nil
}

func (s *SQLiteStore) DeleteTheater(userID string, theaterID string) error {
	if _, err := s.db.Exec(`DELETE FROM practice_records WHERE user_id = ? AND theater_id = ?`, userID, theaterID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM roleplay_sessions WHERE user_id = ? AND theater_id = ?`, userID, theaterID); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM theaters WHERE id = ? AND user_id = ?`, theaterID, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("theater not found")
	}
	return nil
}

func (s *SQLiteStore) AddUserXP(userID string, xp int) error {
	res, err := s.db.Exec(`UPDATE users SET total_xp = total_xp + ? WHERE id = ?`, xp, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *SQLiteStore) SavePracticeRecord(userID string, theaterID string, score int, answers []string, xpEarned int) error {
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO practice_records (user_id, theater_id, score, answers, xp_earned, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, theaterID, score, string(answersJSON), xpEarned, time.Now().UTC().Format(sqliteTimeLayout))
	return err
}

func (s *SQLiteStore) ListCourses(language string) ([]domain.Course, error) {
	seed := []domain.Course{
		{ID: "c1", Language: "CANTONESE", Category: "daily", Title: "茶餐厅点单", Description: "日常场景对话", MinLevel: 4.0, MaxLevel: 6.0, IsActive: true},
		{ID: "c2", Language: "ENGLISH", Category: "ielts", Title: "Describe a memorable trip", Description: "IELTS 口语主题", MinLevel: 5.5, MaxLevel: 8.0, IsActive: true},
	}
	result := make([]domain.Course, 0)
	for _, course := range seed {
		if language == "" || course.Language == language {
			result = append(result, course)
		}
	}
	return result, nil
}

func (s *SQLiteStore) CreateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	transcriptJSON, err := json.Marshal(session.Transcript)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	_, err = s.db.Exec(`INSERT INTO roleplay_sessions (id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, final_feedback, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.TheaterID, session.UserRole, session.TurnIndex, session.CurrentScore, string(transcriptJSON), session.Status, session.FinalFeedback, session.CreatedAt.Format(sqliteTimeLayout), session.UpdatedAt.Format(sqliteTimeLayout))
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	return session, nil
}

func (s *SQLiteStore) GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error) {
	row := s.db.QueryRow(`SELECT id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, final_feedback, created_at, updated_at FROM roleplay_sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	return scanRoleplay(row)
}

func (s *SQLiteStore) UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	session.UpdatedAt = time.Now().UTC()
	transcriptJSON, err := json.Marshal(session.Transcript)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	res, err := s.db.Exec(`UPDATE roleplay_sessions SET turn_index = ?, current_score = ?, transcript = ?, status = ?, final_feedback = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		session.TurnIndex, session.CurrentScore, string(transcriptJSON), session.Status, session.FinalFeedback, session.UpdatedAt.Format(sqliteTimeLayout), session.ID, session.UserID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.RoleplaySession{}, errors.New("roleplay session not found")
	}
	return session, nil
}

func scanUser(scanner interface{ Scan(dest ...any) error }) (domain.User, error) {
	var user domain.User
	var createdAt string
	if err := scanner.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, errors.New("user not found")
		}
		return domain.User{}, err
	}
	user.CreatedAt = parseSQLiteTime(createdAt)
	return user, nil
}

func scanTheater(scanner interface{ Scan(dest ...any) error }) (domain.Theater, error) {
	var theater domain.Theater
	var favorite int
	var charactersJSON, dialoguesJSON, quizJSON, createdAt string
	if err := scanner.Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &favorite, &theater.ShareCode, &theater.SceneDescription, &charactersJSON, &dialoguesJSON, &quizJSON, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Theater{}, errors.New("theater not found")
		}
		return domain.Theater{}, err
	}
	_ = json.Unmarshal([]byte(charactersJSON), &theater.Characters)
	_ = json.Unmarshal([]byte(dialoguesJSON), &theater.Dialogues)
	_ = json.Unmarshal([]byte(quizJSON), &theater.QuizQuestions)
	theater.IsFavorite = favorite != 0
	theater.CreatedAt = parseSQLiteTime(createdAt)
	return theater, nil
}

func scanRoleplay(scanner interface{ Scan(dest ...any) error }) (domain.RoleplaySession, error) {
	var session domain.RoleplaySession
	var transcriptJSON, createdAt, updatedAt string
	if err := scanner.Scan(&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &transcriptJSON, &session.Status, &session.FinalFeedback, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.RoleplaySession{}, errors.New("roleplay session not found")
		}
		return domain.RoleplaySession{}, err
	}
	_ = json.Unmarshal([]byte(transcriptJSON), &session.Transcript)
	session.CreatedAt = parseSQLiteTime(createdAt)
	session.UpdatedAt = parseSQLiteTime(updatedAt)
	return session, nil
}

func parseSQLiteTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(sqliteTimeLayout, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isUniqueConstraint(err error, column string) bool {
	if err == nil {
		return false
	}
	needle := "UNIQUE constraint failed: " + column
	return strings.Contains(err.Error(), needle)
}
