package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
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

func (s *PostgresStore) CreateUser(username string, email string, passwordHash string, emailVerified bool) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`INSERT INTO users (username, email, email_verified, password_hash) VALUES ($1, $2, $3, $4)
		 RETURNING id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at`,
		username, email, emailVerified, passwordHash,
	).Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
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
		`SELECT id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at
		 FROM users WHERE LOWER(email) = LOWER($1) ORDER BY created_at LIMIT 1`,
		email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) ListUsersByEmail(email string) ([]domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at FROM users WHERE LOWER(email) = LOWER($1) ORDER BY created_at`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err = rows.Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *PostgresStore) GetUserByUsername(username string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var user domain.User
	err := s.pool.QueryRow(ctx, `SELECT id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at FROM users WHERE LOWER(username) = LOWER($1)`, username).Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.New("user not found")
	}
	return user, err
}

func (s *PostgresStore) GetUserByID(id string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	user := domain.User{}
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at
		 FROM users WHERE id = $1::uuid`,
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
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
		 RETURNING id::text, username, email, email_verified, password_hash, COALESCE(nickname, ''), COALESCE(avatar_url, ''), COALESCE(bio, ''), total_xp, created_at`,
		userID, nickname, avatarURL, bio,
	).Scan(&user.ID, &user.Username, &user.Email, &user.EmailVerified, &user.PasswordHash, &user.Nickname, &user.AvatarURL, &user.Bio, &user.TotalXP, &user.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *PostgresStore) UpdateUserPassword(userID string, passwordHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1::uuid`, userID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *PostgresStore) SetUserEmailVerified(userID string, verified bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.pool.Exec(ctx, `UPDATE users SET email_verified = $2 WHERE id = $1::uuid`, userID, verified)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *PostgresStore) CreateAuthToken(userID string, purpose string, tokenHash string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM auth_tokens WHERE user_id = $1::uuid AND purpose = $2`, userID, purpose); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_tokens (token_hash, user_id, purpose, expires_at) VALUES ($1, $2::uuid, $3, $4)`, tokenHash, userID, purpose, expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ConsumeAuthToken(tokenHash string, purpose string, now time.Time) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var userID string
	err := s.pool.QueryRow(ctx, `DELETE FROM auth_tokens WHERE token_hash = $1 AND purpose = $2 AND expires_at > $3 RETURNING user_id::text`, tokenHash, purpose, now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("token not found")
	}
	return userID, err
}

func (s *PostgresStore) SaveVoiceProfile(profile domain.VoiceProfile) (domain.VoiceProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved := domain.VoiceProfile{}
	err := s.pool.QueryRow(ctx, `INSERT INTO voice_profiles (id, user_id, name, prompt, language, provider, model, preview_audio_url, status, generation_message, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, prompt = EXCLUDED.prompt, language = EXCLUDED.language,
		provider = EXCLUDED.provider, model = EXCLUDED.model, preview_audio_url = EXCLUDED.preview_audio_url,
		status = EXCLUDED.status, generation_message = EXCLUDED.generation_message
		RETURNING id::text, user_id::text, name, prompt, language, provider, model, preview_audio_url, status, generation_message, created_at`,
		profile.ID, profile.UserID, profile.Name, profile.Prompt, profile.Language, profile.Provider, profile.Model, profile.PreviewAudioURL, profile.Status, profile.GenerationMessage, profile.CreatedAt,
	).Scan(&saved.ID, &saved.UserID, &saved.Name, &saved.Prompt, &saved.Language, &saved.Provider, &saved.Model, &saved.PreviewAudioURL, &saved.Status, &saved.GenerationMessage, &saved.CreatedAt)
	if err != nil {
		return domain.VoiceProfile{}, err
	}
	return saved, nil
}

func (s *PostgresStore) ListVoiceProfiles(userID string) ([]domain.VoiceProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT id::text, user_id::text, name, prompt, language, provider, model, preview_audio_url, status, generation_message, created_at FROM voice_profiles WHERE user_id = $1::uuid ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]domain.VoiceProfile, 0)
	for rows.Next() {
		var profile domain.VoiceProfile
		if err = rows.Scan(&profile.ID, &profile.UserID, &profile.Name, &profile.Prompt, &profile.Language, &profile.Provider, &profile.Model, &profile.PreviewAudioURL, &profile.Status, &profile.GenerationMessage, &profile.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *PostgresStore) DeleteVoiceProfile(userID string, profileID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.pool.Exec(ctx, `DELETE FROM voice_profiles WHERE id = $1::uuid AND user_id = $2::uuid`, profileID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("voice profile not found")
	}
	return nil
}

func (s *PostgresStore) GetModelConfig() (domain.ModelConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := domain.ModelConfig{}
	err := s.pool.QueryRow(
		ctx,
		`SELECT provider, model, base_url, api_key, updated_at
		 FROM model_configs
		 WHERE id = 1`,
	).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ModelConfig{}, errors.New("model config not found")
	}
	if err != nil {
		return domain.ModelConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) SaveModelConfig(config domain.ModelConfig) (domain.ModelConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	err := s.pool.QueryRow(
		ctx,
		`INSERT INTO model_configs (id, provider, model, base_url, api_key, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5)
		 ON CONFLICT(id) DO UPDATE SET
		    provider = EXCLUDED.provider,
		    model = EXCLUDED.model,
		    base_url = EXCLUDED.base_url,
		    api_key = EXCLUDED.api_key,
		    updated_at = EXCLUDED.updated_at
		 RETURNING provider, model, base_url, api_key, updated_at`,
		config.Provider, config.Model, config.BaseURL, config.APIKey, config.UpdatedAt,
	).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.UpdatedAt)
	if err != nil {
		return domain.ModelConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) GetTTSConfig() (domain.TTSConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := domain.TTSConfig{}
	err := s.pool.QueryRow(
		ctx,
		`SELECT provider, model, base_url, api_key, voice, audio_format, updated_at
		 FROM tts_configs
		 WHERE id = 1`,
	).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.Voice, &config.AudioFormat, &config.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TTSConfig{}, errors.New("tts config not found")
	}
	if err != nil {
		return domain.TTSConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) SaveTTSConfig(config domain.TTSConfig) (domain.TTSConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	err := s.pool.QueryRow(
		ctx,
		`INSERT INTO tts_configs (id, provider, model, base_url, api_key, voice, audio_format, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT(id) DO UPDATE SET
		    provider = EXCLUDED.provider,
		    model = EXCLUDED.model,
		    base_url = EXCLUDED.base_url,
		    api_key = EXCLUDED.api_key,
		    voice = EXCLUDED.voice,
		    audio_format = EXCLUDED.audio_format,
		    updated_at = EXCLUDED.updated_at
		 RETURNING provider, model, base_url, api_key, voice, audio_format, updated_at`,
		config.Provider, config.Model, config.BaseURL, config.APIKey, config.Voice, config.AudioFormat, config.UpdatedAt,
	).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.Voice, &config.AudioFormat, &config.UpdatedAt)
	if err != nil {
		return domain.TTSConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) GetASRConfig() (domain.ASRConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var config domain.ASRConfig
	err := s.pool.QueryRow(ctx, `SELECT provider, model, base_url, api_key, app_id, updated_at FROM asr_configs WHERE id = 1`).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.AppID, &config.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ASRConfig{}, errors.New("asr config not found")
	}
	if err != nil {
		return domain.ASRConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) SaveASRConfig(config domain.ASRConfig) (domain.ASRConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now().UTC()
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO asr_configs (id, provider, model, base_url, api_key, app_id, updated_at) VALUES (1, $1, $2, $3, $4, $5, $6)
        ON CONFLICT(id) DO UPDATE SET provider=EXCLUDED.provider, model=EXCLUDED.model, base_url=EXCLUDED.base_url, api_key=EXCLUDED.api_key, app_id=EXCLUDED.app_id, updated_at=EXCLUDED.updated_at
        RETURNING provider, model, base_url, api_key, app_id, updated_at`, config.Provider, config.Model, config.BaseURL, config.APIKey, config.AppID, config.UpdatedAt).Scan(&config.Provider, &config.Model, &config.BaseURL, &config.APIKey, &config.AppID, &config.UpdatedAt)
	if err != nil {
		return domain.ASRConfig{}, err
	}
	return config, nil
}

func (s *PostgresStore) ListOAuthAccounts(provider string) ([]domain.OAuthAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.pool.Query(
		ctx,
		`SELECT id::text, email, provider, client_id, refresh_token, created_at, updated_at
		 FROM oauth_accounts
		 WHERE ($1 = '' OR LOWER(provider) = LOWER($1))
		 ORDER BY LOWER(email), LOWER(provider), client_id`,
		strings.TrimSpace(provider),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.OAuthAccount, 0)
	for rows.Next() {
		var account domain.OAuthAccount
		if scanErr := rows.Scan(
			&account.ID,
			&account.Email,
			&account.Provider,
			&account.ClientID,
			&account.RefreshToken,
			&account.CreatedAt,
			&account.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, account)
	}
	return result, rows.Err()
}

func (s *PostgresStore) SaveTheater(theater domain.Theater) (domain.Theater, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if theater.ID == "" {
		theater.ID = uuid.NewString()
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
	err = s.pool.QueryRow(
		ctx,
		`INSERT INTO theaters (id, user_id, language, topic, difficulty, mode, status, generation_progress, generation_message, scene_description, characters, dialogues, quiz_questions, is_favorite, share_code)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13::jsonb, $14, $15)
		 ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			language = EXCLUDED.language,
			topic = EXCLUDED.topic,
			difficulty = EXCLUDED.difficulty,
			mode = EXCLUDED.mode,
			status = EXCLUDED.status,
			generation_progress = EXCLUDED.generation_progress,
			generation_message = EXCLUDED.generation_message,
			scene_description = EXCLUDED.scene_description,
			characters = EXCLUDED.characters,
			dialogues = EXCLUDED.dialogues,
			quiz_questions = EXCLUDED.quiz_questions,
			is_favorite = EXCLUDED.is_favorite,
			share_code = EXCLUDED.share_code
		 RETURNING id::text, user_id::text, language, topic, difficulty, mode, status, generation_progress, generation_message, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), created_at`,
		theater.ID, theater.UserID, theater.Language, theater.Topic, theater.Difficulty, theater.Mode, theater.Status, theater.GenerationProgress, theater.GenerationMessage, theater.SceneDescription, string(charactersJSON), string(dialoguesJSON), string(quizJSON), theater.IsFavorite, theater.ShareCode,
	).Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.GenerationProgress, &theater.GenerationMessage, &theater.IsFavorite, &theater.ShareCode, &theater.SceneDescription, &charactersJSON, &theater.CreatedAt)
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
		`SELECT id::text, user_id::text, language, topic, difficulty, mode, status, generation_progress, generation_message, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), dialogues, COALESCE(quiz_questions, '[]'::jsonb), created_at
		 FROM theaters WHERE id = $1::uuid`,
		id,
	).Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.GenerationProgress, &theater.GenerationMessage, &theater.IsFavorite, &theater.ShareCode, &theater.SceneDescription, &charactersRaw, &dialoguesRaw, &quizRaw, &theater.CreatedAt)
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

func (s *PostgresStore) GetTheaterByShareCode(shareCode string) (domain.Theater, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	theater := domain.Theater{}
	var charactersRaw []byte
	var dialoguesRaw []byte
	var quizRaw []byte
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, user_id::text, language, topic, difficulty, mode, status, generation_progress, generation_message, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), dialogues, COALESCE(quiz_questions, '[]'::jsonb), created_at
		 FROM theaters WHERE UPPER(share_code) = UPPER($1) AND share_code <> ''`,
		shareCode,
	).Scan(&theater.ID, &theater.UserID, &theater.Language, &theater.Topic, &theater.Difficulty, &theater.Mode, &theater.Status, &theater.GenerationProgress, &theater.GenerationMessage, &theater.IsFavorite, &theater.ShareCode, &theater.SceneDescription, &charactersRaw, &dialoguesRaw, &quizRaw, &theater.CreatedAt)
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
		`SELECT id::text, user_id::text, language, topic, difficulty, mode, status, generation_progress, generation_message, COALESCE(is_favorite, false), COALESCE(share_code, ''), COALESCE(scene_description, ''), COALESCE(characters, '[]'::jsonb), dialogues, COALESCE(quiz_questions, '[]'::jsonb), created_at
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
			&item.Status, &item.GenerationProgress, &item.GenerationMessage, &item.IsFavorite, &item.ShareCode, &item.SceneDescription, &charactersRaw, &dialoguesRaw, &quizRaw, &item.CreatedAt,
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

func (s *PostgresStore) SaveReadingPracticeRecord(userID string, materialID string, score int, answers []string, xpEarned int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(
		ctx,
		`INSERT INTO reading_practice_records (user_id, material_id, score, answers, xp_earned)
		 VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5)`,
		userID, materialID, score, string(answersJSON), xpEarned,
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

func (s *PostgresStore) SaveReadingMaterial(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	err = s.pool.QueryRow(
		ctx,
		`INSERT INTO reading_materials (
            id, user_id, exam, language, level, topic, title, passage, vocabulary, questions, source_ids,
			generation_note, audio_url, audio_urls, audio_status, status, generation_progress, generation_message, vocabulary_items, association_sentences, grammar_insights, created_at
        ) VALUES (
            $1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb,
			$12, $13, $14::jsonb, $15, $16, $17, $18, $19::jsonb, $20::jsonb, $21::jsonb, $22
        )
        ON CONFLICT (id) DO UPDATE SET
            user_id = EXCLUDED.user_id,
            exam = EXCLUDED.exam,
            language = EXCLUDED.language,
            level = EXCLUDED.level,
            topic = EXCLUDED.topic,
            band = EXCLUDED.band,
            stage = EXCLUDED.stage,
            section = EXCLUDED.section,
            skill_focus = EXCLUDED.skill_focus,
            question_type = EXCLUDED.question_type,
            scenario_family = EXCLUDED.scenario_family,
            title = EXCLUDED.title,
            passage = EXCLUDED.passage,
            vocabulary = EXCLUDED.vocabulary,
            questions = EXCLUDED.questions,
            source_ids = EXCLUDED.source_ids,
            generation_note = EXCLUDED.generation_note,
            audio_url = EXCLUDED.audio_url,
			audio_urls = EXCLUDED.audio_urls,
			audio_status = EXCLUDED.audio_status,
			status = EXCLUDED.status,
			generation_progress = EXCLUDED.generation_progress,
			generation_message = EXCLUDED.generation_message,
            vocabulary_items = EXCLUDED.vocabulary_items,
            association_sentences = EXCLUDED.association_sentences,
            grammar_insights = EXCLUDED.grammar_insights,
            created_at = EXCLUDED.created_at
        RETURNING id::text, user_id::text, exam, language, level, topic, title, passage, vocabulary, questions, source_ids,
			COALESCE(generation_note, ''), COALESCE(audio_url, ''), COALESCE(audio_urls, '[]'::jsonb), audio_status, status, generation_progress, generation_message,
			COALESCE(vocabulary_items, '[]'::jsonb), COALESCE(association_sentences, '[]'::jsonb), COALESCE(grammar_insights, '[]'::jsonb), created_at`,
		material.ID, material.UserID, material.Exam, material.Language, material.Level, material.Topic, material.Title, material.Passage,
		string(vocabularyJSON), string(questionsJSON), string(sourceIDsJSON), material.GenerationNote, material.AudioURL,
		string(audioURLsJSON), material.AudioStatus, material.Status, material.GenerationProgress, material.GenerationMessage, string(vocabularyItemsJSON), string(associationJSON), string(grammarJSON), material.CreatedAt,
	).Scan(
		&material.ID, &material.UserID, &material.Exam, &material.Language, &material.Level, &material.Topic, &material.Title, &material.Passage,
		&vocabularyJSON, &questionsJSON, &sourceIDsJSON, &material.GenerationNote, &material.AudioURL, &audioURLsJSON, &material.AudioStatus, &material.Status, &material.GenerationProgress, &material.GenerationMessage,
		&vocabularyItemsJSON, &associationJSON, &grammarJSON, &material.CreatedAt,
	)
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	_ = json.Unmarshal(vocabularyJSON, &material.Vocabulary)
	_ = json.Unmarshal(questionsJSON, &material.Questions)
	_ = json.Unmarshal(sourceIDsJSON, &material.SourceIDs)
	_ = json.Unmarshal(audioURLsJSON, &material.AudioURLs)
	_ = json.Unmarshal(vocabularyItemsJSON, &material.VocabularyItems)
	_ = json.Unmarshal(associationJSON, &material.AssociationSentences)
	_ = json.Unmarshal(grammarJSON, &material.GrammarInsights)
	return material, nil
}

func (s *PostgresStore) UpdateReadingMaterialExisting(material domain.ReadingMaterial) (domain.ReadingMaterial, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	result, err := s.pool.Exec(ctx, `UPDATE reading_materials SET exam = $3, language = $4, level = $5, topic = $6, title = $7, passage = $8, vocabulary = $9::jsonb, questions = $10::jsonb, source_ids = $11::jsonb, generation_note = $12, audio_url = $13, audio_urls = $14::jsonb, audio_status = $15, status = $16, generation_progress = $17, generation_message = $18, vocabulary_items = $19::jsonb, association_sentences = $20::jsonb, grammar_insights = $21::jsonb WHERE id = $1::uuid AND user_id = $2::uuid`,
		material.ID, material.UserID, material.Exam, material.Language, material.Level, material.Topic, material.Title, material.Passage, string(vocabularyJSON), string(questionsJSON), string(sourceIDsJSON), material.GenerationNote, material.AudioURL, string(audioURLsJSON), material.AudioStatus, material.Status, material.GenerationProgress, material.GenerationMessage, string(vocabularyItemsJSON), string(associationJSON), string(grammarJSON))
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	if result.RowsAffected() == 0 {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	return material, nil
}

func (s *PostgresStore) GetReadingMaterial(id string, userID string) (domain.ReadingMaterial, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var material domain.ReadingMaterial
	var vocabularyJSON, questionsJSON, sourceIDsJSON []byte
	var audioURLsJSON, vocabularyItemsJSON, associationJSON, grammarJSON []byte
	err := s.pool.QueryRow(
		ctx,
		`SELECT id::text, user_id::text, exam, language, level, topic, title, passage, vocabulary, questions, source_ids,
			COALESCE(generation_note, ''), COALESCE(audio_url, ''), COALESCE(audio_urls, '[]'::jsonb), audio_status, status, generation_progress, generation_message,
			COALESCE(vocabulary_items, '[]'::jsonb), COALESCE(association_sentences, '[]'::jsonb), COALESCE(grammar_insights, '[]'::jsonb), created_at
         FROM reading_materials
         WHERE id = $1::uuid AND ($2 = '' OR user_id = NULLIF($2, '')::uuid)`,
		id, userID,
	).Scan(
		&material.ID, &material.UserID, &material.Exam, &material.Language, &material.Level, &material.Topic, &material.Title, &material.Passage,
		&vocabularyJSON, &questionsJSON, &sourceIDsJSON, &material.GenerationNote, &material.AudioURL, &audioURLsJSON, &material.AudioStatus, &material.Status, &material.GenerationProgress, &material.GenerationMessage,
		&vocabularyItemsJSON, &associationJSON, &grammarJSON, &material.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReadingMaterial{}, errors.New("reading material not found")
	}
	if err != nil {
		return domain.ReadingMaterial{}, err
	}
	_ = json.Unmarshal(vocabularyJSON, &material.Vocabulary)
	_ = json.Unmarshal(questionsJSON, &material.Questions)
	_ = json.Unmarshal(sourceIDsJSON, &material.SourceIDs)
	_ = json.Unmarshal(audioURLsJSON, &material.AudioURLs)
	_ = json.Unmarshal(vocabularyItemsJSON, &material.VocabularyItems)
	_ = json.Unmarshal(associationJSON, &material.AssociationSentences)
	_ = json.Unmarshal(grammarJSON, &material.GrammarInsights)
	return material, nil
}

func (s *PostgresStore) ListReadingMaterialsByUser(userID string, exam string) ([]domain.ReadingMaterial, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(
		ctx,
		`SELECT id::text, user_id::text, exam, language, level, topic, title, passage, vocabulary, questions, source_ids,
			COALESCE(generation_note, ''), COALESCE(audio_url, ''), COALESCE(audio_urls, '[]'::jsonb), audio_status, status, generation_progress, generation_message,
			COALESCE(vocabulary_items, '[]'::jsonb), COALESCE(association_sentences, '[]'::jsonb), COALESCE(grammar_insights, '[]'::jsonb), created_at
         FROM reading_materials
         WHERE user_id = $1::uuid AND ($2 = '' OR exam = $2)
         ORDER BY created_at DESC`,
		userID, exam,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.ReadingMaterial, 0)
	for rows.Next() {
		var item domain.ReadingMaterial
		var vocabularyJSON, questionsJSON, sourceIDsJSON []byte
		var audioURLsJSON, vocabularyItemsJSON, associationJSON, grammarJSON []byte
		if scanErr := rows.Scan(
			&item.ID, &item.UserID, &item.Exam, &item.Language, &item.Level, &item.Topic, &item.Title, &item.Passage,
			&vocabularyJSON, &questionsJSON, &sourceIDsJSON, &item.GenerationNote, &item.AudioURL, &audioURLsJSON, &item.AudioStatus, &item.Status, &item.GenerationProgress, &item.GenerationMessage,
			&vocabularyItemsJSON, &associationJSON, &grammarJSON, &item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		_ = json.Unmarshal(vocabularyJSON, &item.Vocabulary)
		_ = json.Unmarshal(questionsJSON, &item.Questions)
		_ = json.Unmarshal(sourceIDsJSON, &item.SourceIDs)
		_ = json.Unmarshal(audioURLsJSON, &item.AudioURLs)
		_ = json.Unmarshal(vocabularyItemsJSON, &item.VocabularyItems)
		_ = json.Unmarshal(associationJSON, &item.AssociationSentences)
		_ = json.Unmarshal(grammarJSON, &item.GrammarInsights)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) DeleteReadingMaterial(userID string, materialID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM reading_practice_records WHERE user_id = $1::uuid AND material_id = $2::uuid`, userID, materialID); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `DELETE FROM reading_materials WHERE id = $1::uuid AND user_id = $2::uuid`, materialID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("reading material not found")
	}
	return tx.Commit(ctx)
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
		`INSERT INTO roleplay_sessions (id, user_id, theater_id, user_role, turn_index, current_score, transcript, status, processing_message, final_feedback)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb, $8, $9, $10)
		 RETURNING id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(processing_message, ''), COALESCE(final_feedback, ''), created_at, updated_at`,
		session.ID, session.UserID, session.TheaterID, session.UserRole, session.TurnIndex, session.CurrentScore, string(raw), session.Status, session.ProcessingMessage, session.FinalFeedback,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.ProcessingMessage, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
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
		`SELECT id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(processing_message, ''), COALESCE(final_feedback, ''), created_at, updated_at
		 FROM roleplay_sessions
		 WHERE id = $1::uuid AND user_id = $2::uuid`,
		sessionID, userID,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.ProcessingMessage, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
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
		 SET turn_index = $3, current_score = $4, transcript = $5::jsonb, status = $6, processing_message = $7, final_feedback = $8, updated_at = NOW()
		 WHERE id = $1::uuid AND user_id = $2::uuid
		 RETURNING id::text, user_id::text, theater_id::text, user_role, turn_index, current_score, transcript, status, COALESCE(processing_message, ''), COALESCE(final_feedback, ''), created_at, updated_at`,
		session.ID, session.UserID, session.TurnIndex, session.CurrentScore, string(raw), session.Status, session.ProcessingMessage, session.FinalFeedback,
	).Scan(
		&session.ID, &session.UserID, &session.TheaterID, &session.UserRole, &session.TurnIndex, &session.CurrentScore, &raw, &session.Status, &session.ProcessingMessage, &session.FinalFeedback, &session.CreatedAt, &session.UpdatedAt,
	)
	if err != nil {
		return domain.RoleplaySession{}, err
	}
	_ = json.Unmarshal(raw, &session.Transcript)
	return session, nil
}

func (s *PostgresStore) SaveWritingSession(session domain.WritingSession) (domain.WritingSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	prompt, err := json.Marshal(session.Prompt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	var evaluation []byte
	if session.Evaluation != nil {
		evaluation, err = json.Marshal(session.Evaluation)
		if err != nil {
			return domain.WritingSession{}, err
		}
	}
	var submittedAt *time.Time
	err = s.pool.QueryRow(ctx, `INSERT INTO writing_sessions (id, user_id, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at)
        VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb, $6, $7, $8, $9, $10::jsonb, $11, NULLIF($12::text, '')::timestamptz, $13, NOW())
        ON CONFLICT(id) DO UPDATE SET essay=EXCLUDED.essay, word_count=EXCLUDED.word_count, status=EXCLUDED.status, progress_message=EXCLUDED.progress_message, evaluation=EXCLUDED.evaluation, submitted_at=EXCLUDED.submitted_at, updated_at=NOW()
        RETURNING id::text, user_id::text, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at`,
		session.ID, session.UserID, session.Exam, session.TimeLimitSeconds, string(prompt), session.Essay, session.WordCount, session.Status, session.ProgressMessage, nullableJSON(evaluation), session.StartedAt, nullablePostgresTime(session.SubmittedAt), session.CreatedAt).Scan(
		&session.ID, &session.UserID, &session.Exam, &session.TimeLimitSeconds, &prompt, &session.Essay, &session.WordCount, &session.Status, &session.ProgressMessage, &evaluation, &session.StartedAt, &submittedAt, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if submittedAt != nil {
		session.SubmittedAt = *submittedAt
	}
	_ = json.Unmarshal(prompt, &session.Prompt)
	if len(evaluation) > 0 && string(evaluation) != "null" {
		var e domain.WritingEvaluation
		if json.Unmarshal(evaluation, &e) == nil {
			session.Evaluation = &e
		}
	}
	return session, nil
}

func (s *PostgresStore) UpdateWritingSessionExisting(session domain.WritingSession) (domain.WritingSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session.UpdatedAt = time.Now().UTC()
	prompt, err := json.Marshal(session.Prompt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	var evaluation []byte
	if session.Evaluation != nil {
		evaluation, err = json.Marshal(session.Evaluation)
		if err != nil {
			return domain.WritingSession{}, err
		}
	}
	result, err := s.pool.Exec(ctx, `UPDATE writing_sessions SET exam = $3, time_limit_seconds = $4, prompt = $5::jsonb, essay = $6, word_count = $7, status = $8, progress_message = $9, evaluation = $10::jsonb, started_at = $11, submitted_at = NULLIF($12::text, '')::timestamptz, updated_at = $13 WHERE id = $1::uuid AND user_id = $2::uuid`,
		session.ID, session.UserID, session.Exam, session.TimeLimitSeconds, string(prompt), session.Essay, session.WordCount, session.Status, session.ProgressMessage, nullableJSON(evaluation), session.StartedAt, nullablePostgresTime(session.SubmittedAt), session.UpdatedAt)
	if err != nil {
		return domain.WritingSession{}, err
	}
	if result.RowsAffected() == 0 {
		return domain.WritingSession{}, errors.New("writing session not found")
	}
	return session, nil
}

func (s *PostgresStore) GetWritingSession(sessionID string, userID string) (domain.WritingSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.scanWritingSession(s.pool.QueryRow(ctx, `SELECT id::text, user_id::text, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at FROM writing_sessions WHERE id=$1::uuid AND user_id=$2::uuid`, sessionID, userID))
}

func (s *PostgresStore) ListWritingSessions(userID string) ([]domain.WritingSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT id::text, user_id::text, exam, time_limit_seconds, prompt, essay, word_count, status, progress_message, evaluation, started_at, submitted_at, created_at, updated_at FROM writing_sessions WHERE user_id=$1::uuid ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.WritingSession, 0)
	for rows.Next() {
		item, scanErr := s.scanWritingSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *PostgresStore) DeleteWritingSession(userID string, sessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := s.pool.Exec(ctx, `DELETE FROM writing_sessions WHERE id = $1::uuid AND user_id = $2::uuid`, sessionID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("writing session not found")
	}
	return nil
}

func (s *PostgresStore) scanWritingSession(scanner interface{ Scan(dest ...any) error }) (domain.WritingSession, error) {
	var item domain.WritingSession
	var prompt, evaluation []byte
	var submittedAt *time.Time
	err := scanner.Scan(&item.ID, &item.UserID, &item.Exam, &item.TimeLimitSeconds, &prompt, &item.Essay, &item.WordCount, &item.Status, &item.ProgressMessage, &evaluation, &item.StartedAt, &submittedAt, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WritingSession{}, errors.New("writing session not found")
	}
	if err != nil {
		return domain.WritingSession{}, err
	}
	if submittedAt != nil {
		item.SubmittedAt = *submittedAt
	}
	_ = json.Unmarshal(prompt, &item.Prompt)
	if len(evaluation) > 0 && string(evaluation) != "null" {
		var value domain.WritingEvaluation
		if json.Unmarshal(evaluation, &value) == nil {
			item.Evaluation = &value
		}
	}
	return item, nil
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
func nullablePostgresTime(value time.Time) any {
	if value.IsZero() {
		return ""
	}
	return value
}
