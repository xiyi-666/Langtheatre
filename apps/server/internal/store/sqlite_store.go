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
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
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
	if err := migrateLegacySQLiteUsers(db); err != nil {
		return err
	}
	stmts := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		`CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            username TEXT NOT NULL COLLATE NOCASE UNIQUE,
            email TEXT NOT NULL,
            email_verified INTEGER NOT NULL DEFAULT 0,
            password_hash TEXT NOT NULL,
            nickname TEXT,
            avatar_url TEXT,
            bio TEXT,
            total_xp INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS model_configs (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            provider TEXT NOT NULL DEFAULT 'OPENAI',
            model TEXT NOT NULL DEFAULT 'gpt-5.4',
            base_url TEXT NOT NULL DEFAULT 'http://43.172.5.210:3000/v1',
            api_key TEXT NOT NULL DEFAULT '',
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS tts_configs (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            provider TEXT NOT NULL DEFAULT 'XIAOMI',
            model TEXT NOT NULL DEFAULT 'mimo-v2.5-tts',
            base_url TEXT NOT NULL DEFAULT 'https://api.xiaomimimo.com/v1',
            api_key TEXT NOT NULL DEFAULT '',
            voice TEXT NOT NULL DEFAULT 'mimo_default',
            audio_format TEXT NOT NULL DEFAULT 'mp3',
            updated_at TEXT NOT NULL
        )`,
<<<<<<< HEAD
		`CREATE TABLE IF NOT EXISTS oauth_accounts (
            id TEXT PRIMARY KEY,
            email TEXT NOT NULL,
            provider TEXT NOT NULL,
            client_id TEXT NOT NULL,
            refresh_token TEXT NOT NULL,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_accounts_provider_email ON oauth_accounts(provider, email)`,
=======
		`CREATE TABLE IF NOT EXISTS asr_configs (
            id INTEGER PRIMARY KEY CHECK (id = 1),
            provider TEXT NOT NULL DEFAULT 'XIAOMI',
            model TEXT NOT NULL DEFAULT 'mimo-v2.5-asr',
            base_url TEXT NOT NULL DEFAULT 'https://api.xiaomimimo.com/v1',
            api_key TEXT NOT NULL DEFAULT '',
            app_id TEXT NOT NULL DEFAULT '',
            updated_at TEXT NOT NULL
        )`,
>>>>>>> 73c0fbd (feat: prepare mini program production release)
		`CREATE TABLE IF NOT EXISTS theaters (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            language TEXT,
            topic TEXT,
            difficulty REAL,
            mode TEXT,
            status TEXT,
			generation_progress INTEGER NOT NULL DEFAULT 0,
			generation_message TEXT NOT NULL DEFAULT '',
            is_favorite INTEGER NOT NULL DEFAULT 0,
            share_code TEXT,
            scene_description TEXT,
            characters TEXT NOT NULL DEFAULT '[]',
            dialogues TEXT NOT NULL,
            quiz_questions TEXT NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_theaters_user ON theaters(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_theaters_share_code ON theaters(share_code)`,
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
			processing_message TEXT NOT NULL DEFAULT '',
            final_feedback TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS writing_sessions (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            exam TEXT NOT NULL,
            time_limit_seconds INTEGER NOT NULL,
            prompt TEXT NOT NULL,
            essay TEXT NOT NULL DEFAULT '',
            word_count INTEGER NOT NULL DEFAULT 0,
            status TEXT NOT NULL,
            progress_message TEXT NOT NULL DEFAULT '',
            evaluation TEXT,
            started_at TEXT NOT NULL,
            submitted_at TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_writing_sessions_user_created ON writing_sessions(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS reading_materials (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            exam TEXT NOT NULL,
            language TEXT NOT NULL,
            level TEXT NOT NULL,
            topic TEXT NOT NULL,
            band REAL NOT NULL DEFAULT 0,
            stage TEXT NOT NULL DEFAULT '',
            section TEXT NOT NULL DEFAULT '',
            skill_focus TEXT NOT NULL DEFAULT '',
            question_type TEXT NOT NULL DEFAULT '',
            scenario_family TEXT NOT NULL DEFAULT '',
            title TEXT NOT NULL,
            passage TEXT NOT NULL,
            vocabulary TEXT NOT NULL DEFAULT '[]',
            questions TEXT NOT NULL DEFAULT '[]',
            source_ids TEXT NOT NULL DEFAULT '[]',
            generation_note TEXT NOT NULL DEFAULT '',
            audio_url TEXT NOT NULL DEFAULT '',
            audio_urls TEXT NOT NULL DEFAULT '[]',
            audio_status TEXT NOT NULL DEFAULT 'PENDING',
			status TEXT NOT NULL DEFAULT 'READY',
			generation_progress INTEGER NOT NULL DEFAULT 100,
			generation_message TEXT NOT NULL DEFAULT '',
            vocabulary_items TEXT NOT NULL DEFAULT '[]',
            association_sentences TEXT NOT NULL DEFAULT '[]',
            grammar_insights TEXT NOT NULL DEFAULT '[]',
            created_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS reading_practice_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			material_id TEXT NOT NULL,
			score INTEGER,
			answers TEXT,
			xp_earned INTEGER,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reading_materials_user_exam ON reading_materials(user_id, exam, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_reading_practice_user_material ON reading_practice_records(user_id, material_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE TABLE IF NOT EXISTS auth_tokens (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			purpose TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_purpose ON auth_tokens(user_id, purpose)`,
		`CREATE TABLE IF NOT EXISTS voice_profiles (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			prompt TEXT NOT NULL,
			language TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			preview_audio_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			generation_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_voice_profiles_user_created ON voice_profiles(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS payment_orders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			product_code TEXT NOT NULL,
			amount_cents INTEGER NOT NULL,
			payment_channel TEXT NOT NULL,
			status TEXT NOT NULL,
			provider_trade_no TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			paid_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_orders_user_created ON payment_orders(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS billing_entitlements (
			user_id TEXT PRIMARY KEY,
			product_code TEXT NOT NULL,
			product_name TEXT NOT NULL,
			is_lifetime INTEGER NOT NULL DEFAULT 0,
			ads_free INTEGER NOT NULL DEFAULT 0,
			credit_balance INTEGER NOT NULL DEFAULT 0,
			credit_allowance INTEGER NOT NULL DEFAULT 0,
			credit_reset_at TEXT,
			expires_at TEXT,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS credit_usages (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			activity TEXT NOT NULL,
			source_id TEXT NOT NULL,
			amount INTEGER NOT NULL,
			is_free INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE(user_id, activity, source_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_credit_usages_user_created ON credit_usages(user_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS xp_events (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			activity TEXT NOT NULL,
			source_id TEXT NOT NULL,
			xp_earned INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(user_id, activity, source_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_xp_events_user_created ON xp_events(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS model_usage_daily (
			day TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			operation TEXT NOT NULL,
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			request_count INTEGER NOT NULL DEFAULT 0,
			reported_request_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			total_latency_ms INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, provider, model, operation)
		)`,
		`CREATE TABLE IF NOT EXISTS product_metrics_daily (
			day TEXT NOT NULL,
			category TEXT NOT NULL,
			name TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, category, name)
		)`,
		`CREATE TRIGGER IF NOT EXISTS users_email_account_limit
		BEFORE INSERT ON users
		WHEN (SELECT COUNT(*) FROM users WHERE LOWER(email) = LOWER(NEW.email)) >= 3
		BEGIN
			SELECT RAISE(ABORT, 'email account limit reached');
		END`,
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
<<<<<<< HEAD
	readingMetadataColumns := []string{
		`ALTER TABLE reading_materials ADD COLUMN band REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE reading_materials ADD COLUMN stage TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reading_materials ADD COLUMN section TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reading_materials ADD COLUMN skill_focus TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reading_materials ADD COLUMN question_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reading_materials ADD COLUMN scenario_family TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range readingMetadataColumns {
=======
	if _, err := db.Exec(`ALTER TABLE asr_configs ADD COLUMN app_id TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE theaters ADD COLUMN generation_progress INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE theaters ADD COLUMN generation_message TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reading_materials ADD COLUMN status TEXT NOT NULL DEFAULT 'READY'`,
		`ALTER TABLE reading_materials ADD COLUMN generation_progress INTEGER NOT NULL DEFAULT 100`,
		`ALTER TABLE reading_materials ADD COLUMN generation_message TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE roleplay_sessions ADD COLUMN processing_message TEXT NOT NULL DEFAULT ''`,
	} {
>>>>>>> 73c0fbd (feat: prepare mini program production release)
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// Legacy SQLite databases used a UNIQUE email column. Rebuild only that table
// once so one address can own up to three independent accounts.
func migrateLegacySQLiteUsers(db *sql.DB) error {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'`).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	rows, err := db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasUsername := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == "username" {
			hasUsername = true
		}
	}
	if hasUsername {
		return rows.Err()
	}
	if _, err = db.Exec(`CREATE TABLE users_auth_upgrade (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL COLLATE NOCASE UNIQUE,
		email TEXT NOT NULL,
		email_verified INTEGER NOT NULL DEFAULT 1,
		password_hash TEXT NOT NULL,
		nickname TEXT,
		avatar_url TEXT,
		bio TEXT,
		total_xp INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`); err != nil {
		return err
	}
	if _, err = db.Exec(`INSERT INTO users_auth_upgrade (id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at)
		SELECT id, 'legacy_' || substr(replace(id, '-', ''), 1, 12), email, 1, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users`); err != nil {
		return err
	}
	if _, err = db.Exec(`DROP TABLE users`); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE users_auth_upgrade RENAME TO users`)
	return err
}

func (s *SQLiteStore) CreateUser(username string, email string, passwordHash string, emailVerified bool) (domain.User, error) {
	user := domain.User{
		ID:            uuid.NewString(),
		Username:      username,
		Email:         email,
		EmailVerified: emailVerified,
		PasswordHash:  passwordHash,
		CreatedAt:     time.Now().UTC(),
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at)
		 VALUES (?, ?, ?, ?, ?, '', '', '', 0, ?)`,
		user.ID, user.Username, user.Email, boolToInt(user.EmailVerified), user.PasswordHash, user.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		if isUniqueConstraint(err, "users.username") {
			return domain.User{}, errors.New("username already exists")
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

func (s *SQLiteStore) GetModelConfig() (domain.ModelConfig, error) {
	row := s.db.QueryRow(`SELECT provider, model, base_url, api_key, updated_at FROM model_configs WHERE id = 1`)
	var config domain.ModelConfig
	var updatedAt string
	if err := row.Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ModelConfig{}, errors.New("model config not found")
		}
		return domain.ModelConfig{}, err
	}
	parsed, err := time.Parse(sqliteTimeLayout, updatedAt)
	if err != nil {
		return domain.ModelConfig{}, err
	}
	config.UpdatedAt = parsed
	return config, nil
}

func (s *SQLiteStore) SaveModelConfig(config domain.ModelConfig) (domain.ModelConfig, error) {
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO model_configs (id, provider, model, base_url, api_key, updated_at)
         VALUES (1, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            provider = excluded.provider,
            model = excluded.model,
            base_url = excluded.base_url,
            api_key = excluded.api_key,
            updated_at = excluded.updated_at`,
		config.Provider,
		config.Model,
		config.BaseURL,
		config.APIKey,
		config.UpdatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.ModelConfig{}, err
	}
	return config, nil
}

func (s *SQLiteStore) GetTTSConfig() (domain.TTSConfig, error) {
	row := s.db.QueryRow(`SELECT provider, model, base_url, api_key, voice, audio_format, updated_at FROM tts_configs WHERE id = 1`)
	var config domain.TTSConfig
	var updatedAt string
	if err := row.Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.Voice, &config.AudioFormat, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TTSConfig{}, errors.New("tts config not found")
		}
		return domain.TTSConfig{}, err
	}
	parsed, err := time.Parse(sqliteTimeLayout, updatedAt)
	if err != nil {
		return domain.TTSConfig{}, err
	}
	config.UpdatedAt = parsed
	return config, nil
}

func (s *SQLiteStore) SaveTTSConfig(config domain.TTSConfig) (domain.TTSConfig, error) {
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO tts_configs (id, provider, model, base_url, api_key, voice, audio_format, updated_at)
         VALUES (1, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            provider = excluded.provider,
            model = excluded.model,
            base_url = excluded.base_url,
            api_key = excluded.api_key,
            voice = excluded.voice,
            audio_format = excluded.audio_format,
            updated_at = excluded.updated_at`,
		config.Provider,
		config.Model,
		config.BaseURL,
		config.APIKey,
		config.Voice,
		config.AudioFormat,
		config.UpdatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.TTSConfig{}, err
	}
	return config, nil
}

<<<<<<< HEAD
func (s *SQLiteStore) ListOAuthAccounts(provider string) ([]domain.OAuthAccount, error) {
	query := `SELECT id, email, provider, client_id, refresh_token, created_at, updated_at FROM oauth_accounts`
	args := []any{}
	if strings.TrimSpace(provider) != "" {
		query += ` WHERE LOWER(provider) = LOWER(?)`
		args = append(args, strings.TrimSpace(provider))
	}
	query += ` ORDER BY LOWER(email), LOWER(provider), client_id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.OAuthAccount, 0)
	for rows.Next() {
		item, scanErr := scanOAuthAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
=======
func (s *SQLiteStore) GetASRConfig() (domain.ASRConfig, error) {
	row := s.db.QueryRow(`SELECT provider, model, base_url, api_key, app_id, updated_at FROM asr_configs WHERE id = 1`)
	var config domain.ASRConfig
	var updatedAt string
	if err := row.Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.AppID, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ASRConfig{}, errors.New("asr config not found")
		}
		return domain.ASRConfig{}, err
	}
	config.UpdatedAt = parseSQLiteTime(updatedAt)
	return config, nil
}

func (s *SQLiteStore) SaveASRConfig(config domain.ASRConfig) (domain.ASRConfig, error) {
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO asr_configs (id, provider, model, base_url, api_key, app_id, updated_at) VALUES (1, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET provider = excluded.provider, model = excluded.model, base_url = excluded.base_url, api_key = excluded.api_key, app_id = excluded.app_id, updated_at = excluded.updated_at`,
		config.Provider, config.Model, config.BaseURL, config.APIKey, config.AppID, config.UpdatedAt.Format(sqliteTimeLayout))
	if err != nil {
		return domain.ASRConfig{}, err
	}
	return config, nil
>>>>>>> 73c0fbd (feat: prepare mini program production release)
}

func (s *SQLiteStore) GetUserByEmail(email string) (domain.User, error) {
	row := s.db.QueryRow(`SELECT id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE LOWER(email) = LOWER(?) ORDER BY datetime(created_at) LIMIT 1`, email)
	return scanUser(row)
}

func (s *SQLiteStore) ListUsersByEmail(email string) ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE LOWER(email) = LOWER(?) ORDER BY datetime(created_at)`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		user, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *SQLiteStore) GetUserByUsername(username string) (domain.User, error) {
	row := s.db.QueryRow(`SELECT id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE LOWER(username) = LOWER(?)`, username)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByID(id string) (domain.User, error) {
	row := s.db.QueryRow(`SELECT id, username, email, email_verified, password_hash, nickname, avatar_url, bio, total_xp, created_at FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *SQLiteStore) UpdateUserPassword(userID string, passwordHash string) error {
	res, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *SQLiteStore) SetUserEmailVerified(userID string, verified bool) error {
	res, err := s.db.Exec(`UPDATE users SET email_verified = ? WHERE id = ?`, boolToInt(verified), userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *SQLiteStore) CreateAuthToken(userID string, purpose string, tokenHash string, expiresAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM auth_tokens WHERE user_id = ? AND purpose = ?`, userID, purpose); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO auth_tokens (token_hash, user_id, purpose, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`, tokenHash, userID, purpose, expiresAt.UTC().Format(sqliteTimeLayout), time.Now().UTC().Format(sqliteTimeLayout)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) ConsumeAuthToken(tokenHash string, purpose string, now time.Time) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var userID, expiresAt string
	if err = tx.QueryRow(`SELECT user_id, expires_at FROM auth_tokens WHERE token_hash = ? AND purpose = ?`, tokenHash, purpose).Scan(&userID, &expiresAt); err != nil {
		return "", errors.New("token not found")
	}
	expires := parseSQLiteTime(expiresAt)
	if !expires.After(now) {
		_, _ = tx.Exec(`DELETE FROM auth_tokens WHERE token_hash = ?`, tokenHash)
		return "", errors.New("token expired")
	}
	if _, err = tx.Exec(`DELETE FROM auth_tokens WHERE token_hash = ?`, tokenHash); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *SQLiteStore) SaveVoiceProfile(profile domain.VoiceProfile) (domain.VoiceProfile, error) {
	if strings.TrimSpace(profile.ID) == "" || strings.TrimSpace(profile.UserID) == "" {
		return domain.VoiceProfile{}, errors.New("voice profile id and user id are required")
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO voice_profiles (id, user_id, name, prompt, language, provider, model, preview_audio_url, status, generation_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, prompt = excluded.prompt, language = excluded.language,
		provider = excluded.provider, model = excluded.model, preview_audio_url = excluded.preview_audio_url,
		status = excluded.status, generation_message = excluded.generation_message`,
		profile.ID, profile.UserID, profile.Name, profile.Prompt, profile.Language, profile.Provider, profile.Model, profile.PreviewAudioURL, profile.Status, profile.GenerationMessage, profile.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.VoiceProfile{}, err
	}
	return profile, nil
}

func (s *SQLiteStore) ListVoiceProfiles(userID string) ([]domain.VoiceProfile, error) {
	rows, err := s.db.Query(`SELECT id, user_id, name, prompt, language, provider, model, preview_audio_url, status, generation_message, created_at FROM voice_profiles WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]domain.VoiceProfile, 0)
	for rows.Next() {
		var profile domain.VoiceProfile
		var createdAt string
		if err = rows.Scan(&profile.ID, &profile.UserID, &profile.Name, &profile.Prompt, &profile.Language, &profile.Provider, &profile.Model, &profile.PreviewAudioURL, &profile.Status, &profile.GenerationMessage, &createdAt); err != nil {
			return nil, err
		}
		profile.CreatedAt = parseSQLiteTime(createdAt)
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *SQLiteStore) DeleteVoiceProfile(userID string, profileID string) error {
	result, err := s.db.Exec(`DELETE FROM voice_profiles WHERE id = ? AND user_id = ?`, profileID, userID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return errors.New("voice profile not found")
	}
	return nil
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
		`INSERT INTO theaters (id, user_id, language, topic, difficulty, mode, status, generation_progress, generation_message, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET
            user_id=excluded.user_id,
            language=excluded.language,
            topic=excluded.topic,
            difficulty=excluded.difficulty,
            mode=excluded.mode,
            status=excluded.status,
			generation_progress=excluded.generation_progress,
			generation_message=excluded.generation_message,
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
		theater.GenerationProgress,
		theater.GenerationMessage,
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
	row := s.db.QueryRow(`SELECT id, user_id, language, topic, difficulty, mode, status, generation_progress, generation_message, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at FROM theaters WHERE id = ?`, id)
	return scanTheater(row)
}

func (s *SQLiteStore) GetTheaterByShareCode(shareCode string) (domain.Theater, error) {
	row := s.db.QueryRow(`SELECT id, user_id, language, topic, difficulty, mode, status, generation_progress, generation_message, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at FROM theaters WHERE UPPER(share_code) = UPPER(?) AND share_code <> ''`, shareCode)
	return scanTheater(row)
}

func (s *SQLiteStore) ListTheatersByUser(userID string, language string, status string, favorite *bool) ([]domain.Theater, error) {
	query := `SELECT id, user_id, language, topic, difficulty, mode, status, generation_progress, generation_message, is_favorite, share_code, scene_description, characters, dialogues, quiz_questions, created_at FROM theaters WHERE user_id = ?`
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

func (s *SQLiteStore) SaveReadingPracticeRecord(userID string, materialID string, score int, answers []string, xpEarned int) error {
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO reading_practice_records (user_id, material_id, score, answers, xp_earned, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, materialID, score, string(answersJSON), xpEarned, time.Now().UTC().Format(sqliteTimeLayout))
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

func (s *SQLiteStore) SaveReadingMaterial(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	if material.ID == "" {
		material.ID = uuid.NewString()
	}
	if material.CreatedAt.IsZero() {
		material.CreatedAt = time.Now().UTC()
	}
	vocabularyJSON, err := json.Marshal(material.Vocabulary)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	questionsJSON, err := json.Marshal(material.Questions)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	sourceIDsJSON, err := json.Marshal(material.SourceIDs)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	audioURLsJSON, err := json.Marshal(material.AudioURLs)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	vocabularyItemsJSON, err := json.Marshal(material.VocabularyItems)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	associationJSON, err := json.Marshal(material.AssociationSentences)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	grammarJSON, err := json.Marshal(material.GrammarInsights)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	_, err = s.db.Exec(
		`INSERT INTO reading_materials (
<<<<<<< HEAD
            id, user_id, exam, language, level, topic, band, stage, section, skill_focus, question_type, scenario_family,
            title, passage, vocabulary, questions, source_ids,
            generation_note, audio_url, audio_urls, audio_status, vocabulary_items, association_sentences, grammar_insights, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
=======
            id, user_id, exam, language, level, topic, title, passage, vocabulary, questions, source_ids,
            generation_note, audio_url, audio_urls, audio_status, status, generation_progress, generation_message, vocabulary_items, association_sentences, grammar_insights, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
>>>>>>> 73c0fbd (feat: prepare mini program production release)
        ON CONFLICT(id) DO UPDATE SET
            user_id=excluded.user_id,
            exam=excluded.exam,
            language=excluded.language,
            level=excluded.level,
            topic=excluded.topic,
            band=excluded.band,
            stage=excluded.stage,
            section=excluded.section,
            skill_focus=excluded.skill_focus,
            question_type=excluded.question_type,
            scenario_family=excluded.scenario_family,
            title=excluded.title,
            passage=excluded.passage,
            vocabulary=excluded.vocabulary,
            questions=excluded.questions,
            source_ids=excluded.source_ids,
            generation_note=excluded.generation_note,
            audio_url=excluded.audio_url,
            audio_urls=excluded.audio_urls,
            audio_status=excluded.audio_status,
			status=excluded.status,
			generation_progress=excluded.generation_progress,
			generation_message=excluded.generation_message,
            vocabulary_items=excluded.vocabulary_items,
            association_sentences=excluded.association_sentences,
            grammar_insights=excluded.grammar_insights,
            created_at=excluded.created_at`,
		material.ID,
		material.UserID,
		material.Exam,
		material.Language,
		material.Level,
		material.Topic,
		material.Band,
		material.Stage,
		material.Section,
		material.SkillFocus,
		material.QuestionType,
		material.ScenarioFamily,
		material.Title,
		material.Passage,
		string(vocabularyJSON),
		string(questionsJSON),
		string(sourceIDsJSON),
		material.GenerationNote,
		material.AudioURL,
		string(audioURLsJSON),
		material.AudioStatus,
		material.Status,
		material.GenerationProgress,
		material.GenerationMessage,
		string(vocabularyItemsJSON),
		string(associationJSON),
		string(grammarJSON),
		material.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	return material, nil
}

func (s *SQLiteStore) UpdateReadingMaterialExisting(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	vocabularyJSON, err := json.Marshal(material.Vocabulary)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	questionsJSON, err := json.Marshal(material.Questions)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	sourceIDsJSON, err := json.Marshal(material.SourceIDs)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	audioURLsJSON, err := json.Marshal(material.AudioURLs)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	vocabularyItemsJSON, err := json.Marshal(material.VocabularyItems)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	associationJSON, err := json.Marshal(material.AssociationSentences)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	grammarJSON, err := json.Marshal(material.GrammarInsights)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}

	res, err := s.db.Exec(`UPDATE reading_materials SET exam = ?, language = ?, level = ?, topic = ?, title = ?, passage = ?, vocabulary = ?, questions = ?, source_ids = ?, generation_note = ?, audio_url = ?, audio_urls = ?, audio_status = ?, status = ?, generation_progress = ?, generation_message = ?, vocabulary_items = ?, association_sentences = ?, grammar_insights = ? WHERE id = ? AND user_id = ?`,
		material.Exam, material.Language, material.Level, material.Topic, material.Title, material.Passage, string(vocabularyJSON), string(questionsJSON), string(sourceIDsJSON), material.GenerationNote, material.AudioURL, string(audioURLsJSON), material.AudioStatus, material.Status, material.GenerationProgress, material.GenerationMessage, string(vocabularyItemsJSON), string(associationJSON), string(grammarJSON), material.ID, material.UserID)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	return material, nil
}

func (s *SQLiteStore) GetReadingMaterial(id string, userID string) (domain.ReadingMaterial, error) {
	row := s.db.QueryRow(
<<<<<<< HEAD
		`SELECT id, user_id, exam, language, level, topic, band, stage, section, skill_focus, question_type, scenario_family,
            title, passage, vocabulary, questions, source_ids, generation_note,
            audio_url, audio_urls, audio_status, vocabulary_items, association_sentences, grammar_insights, created_at
=======
		`SELECT id, user_id, exam, language, level, topic, title, passage, vocabulary, questions, source_ids, generation_note,
			audio_url, audio_urls, audio_status, status, generation_progress, generation_message, vocabulary_items, association_sentences, grammar_insights, created_at
>>>>>>> 73c0fbd (feat: prepare mini program production release)
         FROM reading_materials WHERE id = ? AND (? = '' OR user_id = ?)`,
		id, userID, userID,
	)
	return scanReadingMaterial(row)
}

func (s *SQLiteStore) ListReadingMaterialsByUser(userID string, exam string) ([]domain.ReadingMaterial, error) {
	rows, err := s.db.Query(
<<<<<<< HEAD
		`SELECT id, user_id, exam, language, level, topic, band, stage, section, skill_focus, question_type, scenario_family,
            title, passage, vocabulary, questions, source_ids, generation_note,
            audio_url, audio_urls, audio_status, vocabulary_items, association_sentences, grammar_insights, created_at
=======
		`SELECT id, user_id, exam, language, level, topic, title, passage, vocabulary, questions, source_ids, generation_note,
			audio_url, audio_urls, audio_status, status, generation_progress, generation_message, vocabulary_items, association_sentences, grammar_insights, created_at
>>>>>>> 73c0fbd (feat: prepare mini program production release)
         FROM reading_materials
         WHERE user_id = ? AND (? = '' OR exam = ?)
         ORDER BY datetime(created_at) DESC`,
		userID, exam, exam,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ReadingMaterial, 0)
	for rows.Next() {
		item, scanErr := scanReadingMaterial(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) DeleteReadingMaterial(userID string, materialID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM reading_practice_records WHERE user_id = ? AND material_id = ?`, userID, materialID); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM reading_materials WHERE id = ? AND user_id = ?`, materialID, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("reading material not found")
	}
	return tx.Commit()
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
	_, err = s.db.Exec(`INSERT INTO roleplay_sessions (id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, processing_message, final_feedback, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.TheaterID, session.UserRole, session.TurnIndex, session.CurrentScore, string(transcriptJSON), session.Status, session.ProcessingMessage, session.FinalFeedback, session.CreatedAt.Format(sqliteTimeLayout), session.UpdatedAt.Format(sqliteTimeLayout))
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	return session, nil
}

func (s *SQLiteStore) GetRoleplaySession(sessionID string, userID string) (domain.RoleplaySession, error) {
	row := s.db.QueryRow(`SELECT id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, processing_message, final_feedback, created_at, updated_at FROM roleplay_sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	return scanRoleplay(row)
}

func (s *SQLiteStore) UpdateRoleplaySession(session domain.RoleplaySession) (domain.RoleplaySession, error) {
	session.UpdatedAt = time.Now().UTC()
	transcriptJSON, err := json.Marshal(session.Transcript)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	res, err := s.db.Exec(`UPDATE roleplay_sessions SET turn_index = ?, current_score = ?, transcript = ?, status = ?, processing_message = ?, final_feedback = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		session.TurnIndex, session.CurrentScore, string(transcriptJSON), session.Status, session.ProcessingMessage, session.FinalFeedback, session.UpdatedAt.Format(sqliteTimeLayout), session.ID, session.UserID)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.RoleplaySession{}, errors.New("roleplay session not found")
	}
	return session, nil
}

func (s *SQLiteStore) SaveWritingSession(session domain.WritingSession) (domain.WritingSession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	session.UpdatedAt = now
	prompt, err := json.Marshal(session.Prompt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	var evaluation any
	if session.Evaluation != nil {
		evaluation, err = json.Marshal(session.Evaluation)
		if err != nil {
			return domain.WritingSession{}, err
		}
	}
	_, err = s.db.Exec(`INSERT INTO writing_sessions (id, user_id, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET essay=excluded.essay, word_count=excluded.word_count, status=excluded.status, progress_message=excluded.progress_message, evaluation=excluded.evaluation, submitted_at=excluded.submitted_at, updated_at=excluded.updated_at`,
		session.ID, session.UserID, session.Exam, session.TimeLimitSeconds, string(prompt), session.Essay, session.WordCount, session.Status, session.ProgressMessage, evaluation, session.StartedAt.Format(sqliteTimeLayout), nullableSQLiteTime(session.SubmittedAt), session.CreatedAt.Format(sqliteTimeLayout), session.UpdatedAt.Format(sqliteTimeLayout))
	if err != nil {
		return domain.WritingSession{}, err
	}
	return session, nil
}

func (s *SQLiteStore) UpdateWritingSessionExisting(session domain.WritingSession) (domain.WritingSession, error) {
	session.UpdatedAt = time.Now().UTC()
	prompt, err := json.Marshal(session.Prompt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	var evaluation any
	if session.Evaluation != nil {
		evaluation, err = json.Marshal(session.Evaluation)
		if err != nil {
			return domain.WritingSession{}, err
		}
	}
	res, err := s.db.Exec(`UPDATE writing_sessions SET exam = ?, time_limit_seconds = ?, prompt = ?, essay = ?, word_count = ?, status = ?, progress_message = ?, evaluation = ?, started_at = ?, submitted_at = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		session.Exam, session.TimeLimitSeconds, string(prompt), session.Essay, session.WordCount, session.Status, session.ProgressMessage, evaluation, session.StartedAt.Format(sqliteTimeLayout), nullableSQLiteTime(session.SubmittedAt), session.UpdatedAt.Format(sqliteTimeLayout), session.ID, session.UserID)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return domain.WritingSession{}, errors.New("writing session not found")
	}
	return session, nil
}

func (s *SQLiteStore) GetWritingSession(sessionID string, userID string) (domain.WritingSession, error) {
	row := s.db.QueryRow(`SELECT id, user_id, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at FROM writing_sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	return scanWritingSession(row)
}

func (s *SQLiteStore) ListWritingSessions(userID string) ([]domain.WritingSession, error) {
	rows, err := s.db.Query(`SELECT id, user_id, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at FROM writing_sessions WHERE user_id = ? ORDER BY datetime(created_at) DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.WritingSession, 0)
	for rows.Next() {
		item, scanErr := scanWritingSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) DeleteWritingSession(userID string, sessionID string) error {
	res, err := s.db.Exec(`DELETE FROM writing_sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New("writing session not found")
	}
	return nil
}

func scanUser(scanner interface{ Scan(dest ...any) error }) (domain.User, error) {
	var user domain.User
	var createdAt string
	var emailVerified int
	if err := scanner.Scan(&user.ID, &user.Username, &user.Email, &emailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, errors.New("user not found")
		}
		return domain.User{}, err
	}
	user.CreatedAt = parseSQLiteTime(createdAt)
	user.EmailVerified = emailVerified != 0
	return user, nil
}

func scanOAuthAccount(scanner interface{ Scan(dest ...any) error }) (domain.OAuthAccount, error) {
	var account domain.OAuthAccount
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&account.ID, &account.Email, &account.Provider, &account.ClientID, &account.RefreshToken, &createdAt, &updatedAt); err != nil {
		return domain.OAuthAccount{}, err
	}
	account.CreatedAt = parseSQLiteTime(createdAt)
	account.UpdatedAt = parseSQLiteTime(updatedAt)
	return account, nil
}

func scanTheater(scanner interface{ Scan(dest ...any) error }) (domain.Theater, error) {
	var theater domain.Theater
	var favorite int
	var charactersJSON, dialoguesJSON, quizJSON, createdAt string
	if err := scanner.Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.GenerationProgress, &theater.GenerationMessage, &favorite, &theater.ShareCode, &theater.SceneDescription, &charactersJSON, &dialoguesJSON, &quizJSON, &createdAt); err != nil {
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
	if err := scanner.Scan(&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &transcriptJSON, &session.Status, &session.ProcessingMessage, &session.FinalFeedback, &createdAt, &updatedAt); err != nil {
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

func scanWritingSession(scanner interface{ Scan(dest ...any) error }) (domain.WritingSession, error) {
	var item domain.WritingSession
	var promptJSON string
	var evaluationJSON sql.NullString
	var startedAt, createdAt, updatedAt string
	var submittedAt sql.NullString
	if err := scanner.Scan(&item.ID, &item.UserID, &item.Exam, &item.TimeLimitSeconds, &promptJSON, &item.Essay, &item.WordCount, &item.Status, &item.ProgressMessage, &evaluationJSON, &startedAt, &submittedAt, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WritingSession{}, errors.New("writing session not found")
		}
		return domain.WritingSession{}, err
	}
	_ = json.Unmarshal([]byte(promptJSON), &item.Prompt)
	if evaluationJSON.Valid && evaluationJSON.String != "" {
		var evaluation domain.WritingEvaluation
		if json.Unmarshal([]byte(evaluationJSON.String), &evaluation) == nil {
			item.Evaluation = &evaluation
		}
	}
	item.StartedAt = parseSQLiteTime(startedAt)
	item.CreatedAt = parseSQLiteTime(createdAt)
	item.UpdatedAt = parseSQLiteTime(updatedAt)
	if submittedAt.Valid {
		item.SubmittedAt = parseSQLiteTime(submittedAt.String)
	}
	return item, nil
}

func nullableSQLiteTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.Format(sqliteTimeLayout)
}

func scanReadingMaterial(scanner interface{ Scan(dest ...any) error }) (domain.ReadingMaterial, error) {
	var item domain.ReadingMaterial
	var vocabularyJSON, questionsJSON, sourceIDsJSON string
	var audioURLsJSON, vocabularyItemsJSON, associationJSON, grammarJSON string
	var createdAt string
	if err := scanner.Scan(
		&item.ID, &item.UserID, &item.Exam, &item.Language, &item.Level, &item.Topic, &item.Band, &item.Stage,
		&item.Section, &item.SkillFocus, &item.QuestionType, &item.ScenarioFamily, &item.Title, &item.Passage,
		&vocabularyJSON, &questionsJSON, &sourceIDsJSON, &item.GenerationNote, &item.AudioURL, &audioURLsJSON,
		&item.AudioStatus, &item.Status, &item.GenerationProgress, &item.GenerationMessage, &vocabularyItemsJSON, &associationJSON, &grammarJSON, &createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ReadingMaterial{}, errors.New("reading material not found")
		}
		return domain.ReadingMaterial{}, err
	}
	_ = json.Unmarshal([]byte(vocabularyJSON), &item.Vocabulary)
	_ = json.Unmarshal([]byte(questionsJSON), &item.Questions)
	_ = json.Unmarshal([]byte(sourceIDsJSON), &item.SourceIDs)
	_ = json.Unmarshal([]byte(audioURLsJSON), &item.AudioURLs)
	_ = json.Unmarshal([]byte(vocabularyItemsJSON), &item.VocabularyItems)
	_ = json.Unmarshal([]byte(associationJSON), &item.AssociationSentences)
	_ = json.Unmarshal([]byte(grammarJSON), &item.GrammarInsights)
	item.CreatedAt = parseSQLiteTime(createdAt)
	return item, nil
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
