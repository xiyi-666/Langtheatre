package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Edition                      string
	Port                         string
	JWTSecret                    string
	RedisAddr                    string
	SentryDSN                    string
	DatabaseURL                  string
	SQLitePath                   string
	MigrationsDir                string
	OpenAIAPIKey                 string
	OpenAIModel                  string
	OpenAIBaseURL                string
	TTSProvider                  string
	TTSAPIURL                    string
	TTSAPIKey                    string
	TTSVoice                     string
	TTSModel                     string
	TTSAudioFormat               string
	TTSUseUploadPrompt           bool
	TTSPromptAudioPath           string
	TTSReturnJSON                bool
	TTSTimeoutSeconds            int
	TTSMaxRetries                int
	ASRProvider                  string
	ASRAPIURL                    string
	ASRAPIKey                    string
	ASRAppID                     string
	ASRModel                     string
	SMTPHost                     string
	SMTPPort                     int
	SMTPUsername                 string
	SMTPPassword                 string
	SMTPFrom                     string
	PublicAppURL                 string
	RequireEmailVerification     bool
	GenerationConcurrency        int
	BackgroundTaskTimeoutSeconds int
	HTTPRateLimitPerMinute       int
	AuthRateLimitPerMinute       int
	AIRequestRateLimitPerMinute  int
	GraphQLMaxBodyBytes          int
	TrustProxyHeaders            bool
	MediaProxyMaxBytes           int
	AnalyticsEnabled             bool
	AnalyticsTimezone            string
	AnalyticsAdminToken          string
	BillingEnabled               bool
	BillingFreeDailyCredits      int
	MiniAdFreeDailyUses          int
	MiniProgramDailyAIUses       int
	MiniProgramCooldownSeconds   int
	MiniProgramMaxActiveTasks    int
	BillingTimezone              string
	EpayGatewayURL               string
	EpayMerchantID               string
	EpayKey                      string
	EpayNotifyURL                string
	EpayDefaultChannel           string
	EpaySignatureMode            string
	AdProvider                   string
	AdScriptURL                  string
	AdCourseSlot                 string
	AdLibrarySlot                string
	AdResultSlot                 string
}

func Load() Config {
	// In local development, prefer values in .env over inherited shell variables.
	_ = godotenv.Overload()
	edition := normalizeEdition(getenv("APP_EDITION", "COMMERCIAL"))
	port := getenv("PORT", "8177")
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
	openAIModel := getenv("OPENAI_MODEL", "gpt-5.4")
	openAIBaseURL := getenv("OPENAI_BASE_URL", "http://43.172.5.210:3000/v1")
	ttsProvider := getenv("TTS_PROVIDER", "XIAOMI")
	ttsAPIURL := getenv("TTS_API_URL", defaultTTSBaseURL(ttsProvider))
	ttsAPIKey := getenv("TTS_API_KEY", "")
	ttsModel := getenv("TTS_MODEL", defaultTTSModel(ttsProvider))
	defaultTTSVoice := defaultTTSVoiceForProviderAndModel(ttsProvider, ttsModel)
	ttsVoice := getenv("TTS_VOICE", defaultTTSVoice)
	ttsAudioFormat := getenv("TTS_AUDIO_FORMAT", "mp3")
	ttsUseUploadPrompt := getenvBool("TTS_USE_UPLOAD_PROMPT", false)
	ttsPromptAudioPath := getenv("TTS_PROMPT_AUDIO_PATH", "~/autodl-tmp/CosyVoice/test/test.225.wav")
	ttsReturnJSON := getenvBool("TTS_RETURN_JSON", true)
	ttsTimeoutSeconds := getenvInt("TTS_TIMEOUT_SECONDS", 300)
	ttsMaxRetries := getenvInt("TTS_MAX_RETRIES", 1)
	asrProvider := getenv("ASR_PROVIDER", "XIAOMI")
	asrAPIURL := getenv("ASR_API_URL", defaultASRBaseURL(asrProvider))
	asrAPIKey := getenv("ASR_API_KEY", "")
	asrAppID := getenv("ASR_APP_ID", "")
	asrModel := getenv("ASR_MODEL", defaultASRModel(asrProvider))
	smtpHost := getenv("SMTP_HOST", "smtp.qq.com")
	smtpPort := getenvInt("SMTP_PORT", 465)
	smtpUsername := getenv("SMTP_USERNAME", "")
	smtpPassword := getenv("SMTP_PASSWORD", "")
	smtpFrom := getenv("SMTP_FROM", smtpUsername)
	publicAppURL := getenv("PUBLIC_APP_URL", "http://localhost:5174")
	requireEmailVerification := getenvBool("EMAIL_VERIFICATION_REQUIRED", true)
	generationConcurrency := getenvInt("GENERATION_CONCURRENCY", 30)
	backgroundTaskTimeoutSeconds := getenvInt("BACKGROUND_TASK_TIMEOUT_SECONDS", 1200)
	httpRateLimitPerMinute := getenvInt("HTTP_RATE_LIMIT_PER_MINUTE", 180)
	authRateLimitPerMinute := getenvInt("AUTH_RATE_LIMIT_PER_MINUTE", 12)
	aiRequestRateLimitPerMinute := getenvInt("AI_REQUEST_RATE_LIMIT_PER_MINUTE", 20)
	graphQLMaxBodyBytes := getenvInt("GRAPHQL_MAX_BODY_BYTES", 16*1024*1024)
	trustProxyHeaders := getenvBool("TRUST_PROXY_HEADERS", false)
	mediaProxyMaxBytes := getenvInt("MEDIA_PROXY_MAX_BYTES", 20*1024*1024)
	analyticsEnabled := getenvBool("ANALYTICS_ENABLED", true)
	analyticsTimezone := getenv("ANALYTICS_TIMEZONE", "Asia/Shanghai")
	analyticsAdminToken := getenv("ANALYTICS_ADMIN_TOKEN", "")
	billingEnabled := getenvBool("BILLING_ENABLED", false)
	billingFreeDailyCredits := getenvInt("BILLING_FREE_DAILY_CREDITS", 20)
	miniAdFreeDailyUses := getenvInt("MINI_AD_FREE_DAILY_USES", 3)
	miniProgramDailyAIUses := getenvInt("MINI_PROGRAM_DAILY_AI_USES", 20)
	miniProgramCooldownSeconds := getenvInt("MINI_PROGRAM_AI_COOLDOWN_SECONDS", 12)
	miniProgramMaxActiveTasks := getenvInt("MINI_PROGRAM_MAX_ACTIVE_TASKS", 2)
	billingTimezone := getenv("BILLING_TIMEZONE", "Asia/Shanghai")
	epayGatewayURL := getenv("EPAY_GATEWAY_URL", "")
	epayMerchantID := getenv("EPAY_MERCHANT_ID", "")
	epayKey := getenv("EPAY_KEY", "")
	epayNotifyURL := getenv("EPAY_NOTIFY_URL", "")
	epayDefaultChannel := getenv("EPAY_DEFAULT_CHANNEL", "alipay")
	epaySignatureMode := getenv("EPAY_SIGNATURE_MODE", "RAW_KEY")
	adProvider := getenv("AD_PROVIDER", "MOCK")
	adScriptURL := getenv("AD_SCRIPT_URL", "")
	adCourseSlot := getenv("AD_COURSE_SLOT", "")
	adLibrarySlot := getenv("AD_LIBRARY_SLOT", "")
	adResultSlot := getenv("AD_RESULT_SLOT", "")
	if edition == "OPEN_SOURCE" {
		billingEnabled = false
		adProvider = "NONE"
		epayGatewayURL = ""
		epayMerchantID = ""
		epayKey = ""
		epayNotifyURL = ""
	}
	return Config{
		Edition:                      edition,
		Port:                         port,
		JWTSecret:                    secret,
		RedisAddr:                    redisAddr,
		SentryDSN:                    sentryDsn,
		DatabaseURL:                  databaseURL,
		SQLitePath:                   sqlitePath,
		MigrationsDir:                migrationsDir,
		OpenAIAPIKey:                 openAIAPIKey,
		OpenAIModel:                  openAIModel,
		OpenAIBaseURL:                openAIBaseURL,
		TTSProvider:                  ttsProvider,
		TTSAPIURL:                    ttsAPIURL,
		TTSAPIKey:                    ttsAPIKey,
		TTSVoice:                     ttsVoice,
		TTSModel:                     ttsModel,
		TTSAudioFormat:               ttsAudioFormat,
		TTSUseUploadPrompt:           ttsUseUploadPrompt,
		TTSPromptAudioPath:           ttsPromptAudioPath,
		TTSReturnJSON:                ttsReturnJSON,
		TTSTimeoutSeconds:            ttsTimeoutSeconds,
		TTSMaxRetries:                ttsMaxRetries,
		ASRProvider:                  asrProvider,
		ASRAPIURL:                    asrAPIURL,
		ASRAPIKey:                    asrAPIKey,
		ASRAppID:                     asrAppID,
		ASRModel:                     asrModel,
		SMTPHost:                     smtpHost,
		SMTPPort:                     smtpPort,
		SMTPUsername:                 smtpUsername,
		SMTPPassword:                 smtpPassword,
		SMTPFrom:                     smtpFrom,
		PublicAppURL:                 publicAppURL,
		RequireEmailVerification:     requireEmailVerification,
		GenerationConcurrency:        generationConcurrency,
		BackgroundTaskTimeoutSeconds: backgroundTaskTimeoutSeconds,
		HTTPRateLimitPerMinute:       httpRateLimitPerMinute,
		AuthRateLimitPerMinute:       authRateLimitPerMinute,
		AIRequestRateLimitPerMinute:  aiRequestRateLimitPerMinute,
		GraphQLMaxBodyBytes:          graphQLMaxBodyBytes,
		TrustProxyHeaders:            trustProxyHeaders,
		MediaProxyMaxBytes:           mediaProxyMaxBytes,
		AnalyticsEnabled:             analyticsEnabled,
		AnalyticsTimezone:            analyticsTimezone,
		AnalyticsAdminToken:          analyticsAdminToken,
		BillingEnabled:               billingEnabled,
		BillingFreeDailyCredits:      billingFreeDailyCredits,
		MiniAdFreeDailyUses:          miniAdFreeDailyUses,
		MiniProgramDailyAIUses:       miniProgramDailyAIUses,
		MiniProgramCooldownSeconds:   miniProgramCooldownSeconds,
		MiniProgramMaxActiveTasks:    miniProgramMaxActiveTasks,
		BillingTimezone:              billingTimezone,
		EpayGatewayURL:               epayGatewayURL,
		EpayMerchantID:               epayMerchantID,
		EpayKey:                      epayKey,
		EpayNotifyURL:                epayNotifyURL,
		EpayDefaultChannel:           epayDefaultChannel,
		EpaySignatureMode:            epaySignatureMode,
		AdProvider:                   adProvider,
		AdScriptURL:                  adScriptURL,
		AdCourseSlot:                 adCourseSlot,
		AdLibrarySlot:                adLibrarySlot,
		AdResultSlot:                 adResultSlot,
	}
}

func (c Config) IsOpenSourceEdition() bool {
	return c.Edition == "OPEN_SOURCE"
}

func (c Config) IsMiniProgramEdition() bool {
	return c.Edition == "MINI_PROGRAM"
}

func normalizeEdition(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "OPEN_SOURCE", "OPEN", "OSS":
		return "OPEN_SOURCE"
	case "MINI_PROGRAM", "MINIPROGRAM", "MINI", "WECHAT_MINIPROGRAM":
		return "MINI_PROGRAM"
	default:
		return "COMMERCIAL"
	}
}

func defaultASRBaseURL(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "https://api.xiaomimimo.com/v1"
	case "ALIYUN":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "DOUBAO":
		return "https://openspeech.bytedance.com/api/v3/auc/bigmodel/recognize/flash"
	case "GEMINI":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "MINIMAX":
		return "https://api.minimax.io/v1"
	case "OPENAI", "OPENAI_COMPATIBLE":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

func defaultASRModel(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case "XIAOMI":
		return "mimo-v2.5-asr"
	case "ALIYUN":
		return "qwen3-asr-flash"
	case "DOUBAO":
		return "bigmodel"
	case "GEMINI":
		return "gemini-2.5-flash"
	case "MINIMAX":
		return "speech-2.8-hd"
	case "OPENAI", "OPENAI_COMPATIBLE":
		return "gpt-4o-mini-transcribe"
	default:
		return ""
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
		return "https://api.xiaomimimo.com/v1"
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
