package config

import (
	"os"
	"testing"
)

func setenvClean(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestGetenv_FallbackWhenEmpty(t *testing.T) {
	os.Unsetenv("TEST_GETENV_KEY")
	got := getenv("TEST_GETENV_KEY", "default-val")
	if got != "default-val" {
		t.Errorf("getenv returned %q, want %q", got, "default-val")
	}
}

func TestGetenv_ReturnsSetValue(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "custom")
	got := getenv("TEST_GETENV_KEY", "default-val")
	if got != "custom" {
		t.Errorf("getenv returned %q, want %q", got, "custom")
	}
}

func TestGetenvBool(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		fallback bool
		want     bool
	}{
		{"empty_true_fallback", "", true, true},
		{"empty_false_fallback", "", false, false},
		{"true", "true", false, true},
		{"TRUE", "TRUE", false, true},
		{"1", "1", false, true},
		{"yes", "yes", false, true},
		{"on", "on", false, true},
		{"false", "false", true, false},
		{"0", "0", true, false},
		{"no", "no", true, false},
		{"off", "off", true, false},
		{"random_string", "random", true, false},
		{"whitespace_true", "  true  ", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				os.Unsetenv("TEST_BOOL_KEY")
			} else {
				t.Setenv("TEST_BOOL_KEY", tc.value)
			}
			got := getenvBool("TEST_BOOL_KEY", tc.fallback)
			if got != tc.want {
				t.Errorf("getenvBool(%q, %v) = %v, want %v", tc.value, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestGetenvInt(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		fallback int
		want     int
	}{
		{"empty_fallback", "", 42, 42},
		{"valid_int", "100", 42, 100},
		{"negative_int", "-5", 42, -5},
		{"invalid_string", "abc", 42, 42},
		{"float_string", "3.14", 42, 42},
		{"whitespace", "  50  ", 42, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				os.Unsetenv("TEST_INT_KEY")
			} else {
				t.Setenv("TEST_INT_KEY", tc.value)
			}
			got := getenvInt("TEST_INT_KEY", tc.fallback)
			if got != tc.want {
				t.Errorf("getenvInt(%q, %d) = %d, want %d", tc.value, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestDefaultTTSBaseURL(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{"XIAOMI", "https://token-plan-cn.xiaomimimo.com/v1"},
		{"xiaomi", "https://token-plan-cn.xiaomimimo.com/v1"},
		{"  XIAOMI  ", "https://token-plan-cn.xiaomimimo.com/v1"},
		{"MINIMAX", "https://api.minimax.io/v1/t2a_v2"},
		{"ALIYUN", "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer"},
		{"UNKNOWN", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got := defaultTTSBaseURL(tc.provider)
			if got != tc.want {
				t.Errorf("defaultTTSBaseURL(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestDefaultTTSModel(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{"XIAOMI", "mimo-v2.5-tts"},
		{"MINIMAX", "speech-2.8-hd"},
		{"ALIYUN", "cosyvoice-v3-flash"},
		{"OTHER", ""},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got := defaultTTSModel(tc.provider)
			if got != tc.want {
				t.Errorf("defaultTTSModel(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestDefaultTTSVoiceForProvider(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		{"XIAOMI", "mimo_default"},
		{"MINIMAX", "Cantonese_GentleLady"},
		{"ALIYUN", "longjiaxin_v3"},
		{"OTHER", "female-1"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			got := defaultTTSVoiceForProvider(tc.provider)
			if got != tc.want {
				t.Errorf("defaultTTSVoiceForProvider(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestDefaultTTSVoiceForProviderAndModel(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     string
	}{
		{"XIAOMI", "mimo-v2.5-tts", "mimo_default"},
		{"XIAOMI", "mimo-v2.5-tts-voicedesign", "20 多岁女性，声音亲和自然，吐字清晰，适合粤语和英文学习内容。"},
		{"XIAOMI", "mimo-v2.5-tts-voiceclone", ""},
		{"MINIMAX", "speech-2.8-hd", "Cantonese_GentleLady"},
		{"ALIYUN", "cosyvoice-v3-flash", "longjiaxin_v3"},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"_"+tc.model, func(t *testing.T) {
			got := defaultTTSVoiceForProviderAndModel(tc.provider, tc.model)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Clear all env vars that Load() reads so we get defaults.
	for _, key := range []string{
		"PORT", "JWT_SECRET", "REDIS_ADDR", "SENTRY_DSN",
		"SUPABASE_DB_URL", "SUPBASE_DB_URL", "DATABASE_URL",
		"MIGRATIONS_DIR", "SQLITE_PATH",
		"OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL",
		"TTS_PROVIDER", "TTS_API_URL", "TTS_API_KEY", "TTS_MODEL", "TTS_VOICE",
		"TTS_AUDIO_FORMAT", "TTS_USE_UPLOAD_PROMPT", "TTS_PROMPT_AUDIO_PATH",
		"TTS_RETURN_JSON", "TTS_TIMEOUT_SECONDS", "TTS_MAX_RETRIES",
	} {
		os.Unsetenv(key)
	}
	cfg := Load()
	if cfg.Port != "8177" {
		t.Errorf("Port = %q, want 8177", cfg.Port)
	}
	if cfg.JWTSecret != "dev-secret-change-me" {
		t.Errorf("JWTSecret = %q, want dev-secret-change-me", cfg.JWTSecret)
	}
	if cfg.TTSProvider != "XIAOMI" {
		t.Errorf("TTSProvider = %q, want XIAOMI", cfg.TTSProvider)
	}
	if cfg.TTSReturnJSON != true {
		t.Errorf("TTSReturnJSON = %v, want true", cfg.TTSReturnJSON)
	}
	if cfg.TTSTimeoutSeconds != 300 {
		t.Errorf("TTSTimeoutSeconds = %d, want 300", cfg.TTSTimeoutSeconds)
	}
}
