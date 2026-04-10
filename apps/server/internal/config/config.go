package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	JWTSecret          string
	RedisAddr          string
	SentryDSN          string
	DatabaseURL        string
	SQLitePath         string
	MigrationsDir      string
	OpenAIAPIKey       string
	OpenAIModel        string
	OpenAIBaseURL      string
	TTSAPIURL          string
	TTSAPIKey          string
	TTSVoice           string
	TTSUseUploadPrompt bool
	TTSPromptAudioPath string
	TTSReturnJSON      bool
	TTSTimeoutSeconds  int
	TTSMaxRetries      int
}

func Load() Config {
	// In local development, prefer values in .env over inherited shell variables.
	_ = godotenv.Overload()
	port := getenv("PORT", "8080")
	secret := getenv("JWT_SECRET", "dev-secret-change-me")
	redisAddr := getenv("REDIS_ADDR", "localhost:6379")
	sentryDsn := getenv("SENTRY_DSN", "")
	databaseURL := getenv("SUPABASE_DB_URL", "")
	if databaseURL == "" {
		databaseURL = getenv("SUPBASE_DB_URL", "")
	}
	if databaseURL == "" {
		databaseURL = getenv("DATABASE_URL", "")
	}
	migrationsDir := getenv("MIGRATIONS_DIR", "migrations")
	sqlitePath := getenv("SQLITE_PATH", "")
	openAIAPIKey := getenv("OPENAI_API_KEY", "")
	openAIModel := getenv("OPENAI_MODEL", "gpt-4o-mini")
	openAIBaseURL := getenv("OPENAI_BASE_URL", "https://api.openai.com")
	ttsAPIURL := getenv("TTS_API_URL", "")
	ttsAPIKey := getenv("TTS_API_KEY", "")
	ttsVoice := getenv("TTS_VOICE", "female-1")
	ttsUseUploadPrompt := getenvBool("TTS_USE_UPLOAD_PROMPT", false)
	ttsPromptAudioPath := getenv("TTS_PROMPT_AUDIO_PATH", "~/autodl-tmp/CosyVoice/test/test.225.wav")
	ttsReturnJSON := getenvBool("TTS_RETURN_JSON", true)
	ttsTimeoutSeconds := getenvInt("TTS_TIMEOUT_SECONDS", 45)
	ttsMaxRetries := getenvInt("TTS_MAX_RETRIES", 1)
	return Config{
		Port:               port,
		JWTSecret:          secret,
		RedisAddr:          redisAddr,
		SentryDSN:          sentryDsn,
		DatabaseURL:        databaseURL,
		SQLitePath:         sqlitePath,
		MigrationsDir:      migrationsDir,
		OpenAIAPIKey:       openAIAPIKey,
		OpenAIModel:        openAIModel,
		OpenAIBaseURL:      openAIBaseURL,
		TTSAPIURL:          ttsAPIURL,
		TTSAPIKey:          ttsAPIKey,
		TTSVoice:           ttsVoice,
		TTSUseUploadPrompt: ttsUseUploadPrompt,
		TTSPromptAudioPath: ttsPromptAudioPath,
		TTSReturnJSON:      ttsReturnJSON,
		TTSTimeoutSeconds:  ttsTimeoutSeconds,
		TTSMaxRetries:      ttsMaxRetries,
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	value = strings.ToLower(value)
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
