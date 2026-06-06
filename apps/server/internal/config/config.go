package config

import (
	"log"
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
	TTSProvider        string
	TTSAPIURL          string
	TTSAPIKey          string
	TTSVoice           string
	TTSModel           string
	TTSAudioFormat     string
	TTSUseUploadPrompt bool
	TTSPromptAudioPath string
	TTSReturnJSON      bool
	TTSTimeoutSeconds  int
	TTSMaxRetries      int
}

func Load() Config {
	// In local development, prefer values in .env over inherited shell variables.
	_ = godotenv.Overload()
	port := getenv("PORT", "8177")
	secret := getenv("JWT_SECRET", "")
	if secret == "" {
		secret = "dev-secret-change-me"
		log.Println("WARNING: JWT_SECRET is not set — using insecure default. Set JWT_SECRET in production!")
	}
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
	openAIModel := getenv("OPENAI_MODEL", "gpt-5.4")
	openAIBaseURL := getenv("OPENAI_BASE_URL", "http://43.172.5.210:3000/v1")
	ttsProvider := getenv("TTS_PROVIDER", "XIAOMI")
	ttsAPIURL := getenv("TTS_API_URL", defaultTTSBaseURL(ttsProvider))
	ttsAPIKey := getenv("TTS_API_KEY", "")
	ttsModel := getenv("TTS_MODEL", defaultTTSModel(ttsProvider))
	defaultTTSVoice := defaultTTSVoiceForProviderAndModel(ttsProvider, ttsModel)
	ttsVoice := getenv("TTS_VOICE", defaultTTSVoice)
	ttsAudioFormat := getenv("TTS_AUDIO_FORMAT", "wav")
	ttsUseUploadPrompt := getenvBool("TTS_USE_UPLOAD_PROMPT", false)
	ttsPromptAudioPath := getenv("TTS_PROMPT_AUDIO_PATH", "~/autodl-tmp/CosyVoice/test/test.225.wav")
	ttsReturnJSON := getenvBool("TTS_RETURN_JSON", true)
	ttsTimeoutSeconds := getenvInt("TTS_TIMEOUT_SECONDS", 300)
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
		TTSProvider:        ttsProvider,
		TTSAPIURL:          ttsAPIURL,
		TTSAPIKey:          ttsAPIKey,
		TTSVoice:           ttsVoice,
		TTSModel:           ttsModel,
		TTSAudioFormat:     ttsAudioFormat,
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

func defaultTTSBaseURL(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "https://token-plan-cn.xiaomimimo.com/v1"
	case "MINIMAX":
		return "https://api.minimax.io/v1/t2a_v2"
	case "ALIYUN":
		return "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer"
	default:
		return ""
	}
}

func defaultTTSVoiceForProvider(provider string) string {
	return defaultTTSVoiceForProviderAndModel(provider, defaultTTSModel(provider))
}

func defaultTTSVoiceForProviderAndModel(provider string, model string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		switch strings.ToLower(strings.TrimSpace(model)) {
		case "mimo-v2.5-tts-voicedesign":
			return "20 多岁女性，声音亲和自然，吐字清晰，适合粤语和英文学习内容。"
		case "mimo-v2.5-tts-voiceclone":
			return ""
		default:
			return "mimo_default"
		}
	case "MINIMAX":
		return "Cantonese_GentleLady"
	case "ALIYUN":
		return "longjiaxin_v3"
	default:
		return "female-1"
	}
}

func defaultTTSModel(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "mimo-v2.5-tts"
	case "MINIMAX":
		return "speech-2.8-hd"
	case "ALIYUN":
		return "cosyvoice-v3-flash"
	default:
		return ""
	}
}
