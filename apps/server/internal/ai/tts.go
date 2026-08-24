package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/linguaquest/server/internal/contentquality"
	"github.com/linguaquest/server/internal/domain"
)

type APITTS struct {
	mu              sync.RWMutex
	Provider        string
	APIURL          string
	APIKey          string
	Voice           string
	Model           string
	AudioFormat     string
	UpdatedAt       time.Time
	UseUploadPrompt bool
	PromptAudioPath string
	ReturnJSON      bool
	MaxRetries      int
	Client          *http.Client
	xiaomiCloneSeed map[string]string
	xiaomiSeedWait  map[string]chan struct{}
	voiceSeedDir    string
}

const (
	ttsProviderCustom  = "CUSTOM"
	ttsProviderXiaomi  = "XIAOMI"
	ttsProviderMiniMax = "MINIMAX"
	ttsProviderAliyun  = "ALIYUN"

	defaultCustomTTSVoice        = "female-1"
	defaultXiaomiTTSModel        = "mimo-v2.5-tts"
	xiaomiTTSVoiceDesignModel    = "mimo-v2.5-tts-voicedesign"
	xiaomiTTSVoiceCloneModel     = "mimo-v2.5-tts-voiceclone"
	defaultXiaomiTTSVoice        = "mimo_default"
	defaultXiaomiVoiceDesignText = "20 多岁女性，声音亲和自然，吐字清晰，适合粤语和英文学习内容。"
	defaultXiaomiCantoneseStyle  = "温柔女生"
	defaultMiniMaxTTSModel       = "speech-2.8-hd"
	defaultMiniMaxTTSVoice       = "Cantonese_GentleLady"
	defaultAliyunTTSModel        = "cosyvoice-v3-flash"
	defaultAliyunTTSVoice        = "longjiaxin_v3"
	defaultTTSAudioFormat        = "mp3"
	xiaomiVoiceSeedVersion       = "cantonese-natural-v2"
)

var xiaomiPresetVoices = map[string]struct{}{
	"mimo_default": {},
	"冰糖":           {},
	"茉莉":           {},
	"苏打":           {},
	"白桦":           {},
	"Mia":          {},
	"Chloe":        {},
	"Milo":         {},
	"Dean":         {},
}

// Automatic theater styles use MiMo's stable preset voices. VoiceClone is
// reserved for an explicit user-library audio sample so ordinary theater
// generation does not inherit pauses and cadence drift from generated seeds.
var xiaomiAutomaticPresetByStyle = map[string]string{
	"温柔女生": "冰糖",
	"甜美女生": "茉莉",
	"播音男生": "白桦",
	"沉稳大叔": "Dean",
	"御姐音色": "Chloe",
	"清新少女": "Mia",
	"知性姐姐": "冰糖",
	"活力少年": "Milo",
	"港风店员": "茉莉",
	"电台暖男": "白桦",
	"冷静导师": "Dean",
	"俏皮同学": "苏打",
}

var defaultCantoneseAutoVoiceStyles = []string{
	"温柔女生", "甜美女生", "播音男生", "沉稳大叔", "御姐音色", "清新少女",
	"知性姐姐", "活力少年", "港风店员", "电台暖男", "冷静导师", "俏皮同学",
}

func NewAPITTS(provider string, apiURL string, apiKey string, voice string, model string, audioFormat string, useUploadPrompt bool, promptAudioPath string, returnJSON bool, timeoutSeconds int, maxRetries int) *APITTS {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	tts := &APITTS{
		APIURL:          apiURL,
		APIKey:          apiKey,
		UseUploadPrompt: useUploadPrompt,
		PromptAudioPath: strings.TrimSpace(promptAudioPath),
		ReturnJSON:      returnJSON,
		MaxRetries:      maxRetries,
		Client:          &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		xiaomiCloneSeed: make(map[string]string),
		xiaomiSeedWait:  make(map[string]chan struct{}),
	}
	tts.UpdateTTSConfig(domain.TTSConfig{
		Provider:    provider,
		BaseURL:     apiURL,
		APIKey:      apiKey,
		Voice:       voice,
		Model:       model,
		AudioFormat: audioFormat,
	})
	return tts
}

func (t *APITTS) GetTTSConfig() domain.TTSConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	model := normalizeTTSModel(t.Provider, t.Model)
	return domain.TTSConfig{
		Provider:    normalizeTTSProvider(t.Provider),
		BaseURL:     strings.TrimSpace(t.APIURL),
		APIKey:      strings.TrimSpace(t.APIKey),
		Voice:       normalizeTTSVoiceForModel(t.Provider, model, t.Voice),
		Model:       model,
		AudioFormat: normalizeTTSAudioFormat(t.AudioFormat),
		UpdatedAt:   t.UpdatedAt,
	}
}

func (t *APITTS) UpdateTTSConfig(config domain.TTSConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	provider := normalizeTTSProvider(config.Provider)
	model := normalizeTTSModel(provider, config.Model)
	t.Provider = provider
	t.APIURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	t.APIKey = strings.TrimSpace(config.APIKey)
	t.Voice = normalizeTTSVoiceForModel(provider, model, config.Voice)
	t.Model = model
	t.AudioFormat = normalizeTTSAudioFormat(config.AudioFormat)
	t.UpdatedAt = config.UpdatedAt
	clear(t.xiaomiCloneSeed)
}

func (t *APITTS) SetVoiceSeedDir(dir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.voiceSeedDir = strings.TrimSpace(dir)
}

func DefaultCantoneseAutoVoiceStyles() []string {
	return append([]string(nil), defaultCantoneseAutoVoiceStyles...)
}

func (t *APITTS) PrewarmCantoneseVoices(ctx context.Context, styles []string) error {
	config := t.GetTTSConfig()
	if !strings.EqualFold(config.Provider, ttsProviderXiaomi) || strings.TrimSpace(config.APIKey) == "" {
		return nil
	}
	for _, requestedStyle := range styles {
		style := normalizeVoiceStyle(requestedStyle)
		if style == "" {
			continue
		}
		if _, err := t.getXiaomiDesignedSeed(ctx, config, style, "CANTONESE"); err != nil {
			return fmt.Errorf("prewarm Cantonese voice %s: %w", style, err)
		}
	}
	return nil
}

func (t *APITTS) Synthesize(ctx context.Context, text string, language string, voice string) (string, error) {
	return t.SynthesizeWithContext(ctx, text, language, voice, "")
}

// SynthesizeWithContext supplies non-spoken scene and previous-turn context
// to providers that can use chat history for more natural conversational
// prosody. Other providers keep their existing behavior.
func (t *APITTS) SynthesizeWithContext(ctx context.Context, text string, language string, voice string, dialogueContext string) (string, error) {
	config := t.GetTTSConfig()
	if config.BaseURL == "" {
		return "", errors.New("tts api url not configured")
	}
	if isCantoneseLanguage(language) {
		text = contentquality.NormalizeCantoneseSpeechText(text)
	}
	switch {
	case strings.EqualFold(config.Provider, ttsProviderXiaomi):
		return t.synthesizeXiaomi(ctx, config, text, language, voice, dialogueContext)
	case strings.EqualFold(config.Provider, ttsProviderMiniMax):
		return t.synthesizeMiniMax(ctx, config, text, language, voice)
	case strings.EqualFold(config.Provider, ttsProviderAliyun):
		return t.synthesizeAliyun(ctx, config, text, language, voice)
	default:
		return t.synthesizeCustom(ctx, config, text, language, voice)
	}
}

// DesignVoice creates a reusable Xiaomi VoiceDesign sample. The returned data URL
// is persisted by the service as a voice-library profile and later passed to
// Xiaomi VoiceClone for consistent character speech.
func (t *APITTS) DesignVoice(ctx context.Context, prompt string, language string) (string, error) {
	config := t.GetTTSConfig()
	if !strings.EqualFold(config.Provider, ttsProviderXiaomi) {
		return "", errors.New("voice design requires Xiaomi TTS")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("voice design prompt is required")
	}
	messages, audio, err := buildXiaomiMessagesAndAudio(
		xiaomiTTSVoiceDesignModel,
		prompt,
		config.AudioFormat,
		xiaomiVoiceDesignSampleText(language),
		language,
		"",
	)
	if err != nil {
		return "", err
	}
	return t.doXiaomiSynthesis(ctx, config, xiaomiTTSVoiceDesignModel, messages, audio)
}

func (t *APITTS) synthesizeCustom(ctx context.Context, config domain.TTSConfig, text string, language string, voice string) (string, error) {
	apiVoice, voiceStyle := resolveVoiceSelection(voice, config.Voice)
	instruction := buildInstruction(text, language, voiceStyle)
	payload := map[string]any{
		"text":              text,
		"instruct":          instruction,
		"use_upload_prompt": t.UseUploadPrompt,
		"return_json":       t.ReturnJSON,
	}
	if t.PromptAudioPath != "" {
		payload["prompt_audio_path"] = t.PromptAudioPath
	}
	if strings.TrimSpace(apiVoice) != "" {
		payload["voice"] = strings.TrimSpace(apiVoice)
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return "", marshalErr
	}

	apiKey := strings.TrimSpace(config.APIKey)
	log.Printf("tts request provider=%s url=%s text_len=%d language=%s", config.Provider, config.BaseURL, len(text), language)
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, config.BaseURL, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			req.Header.Set("x-api-key", apiKey)
		}

		resp, err = t.Client.Do(req)
		if err == nil {
			break
		}
		if !isTimeoutErr(err) || attempt == t.MaxRetries {
			return "", err
		}
		log.Printf("tts timeout attempt=%d/%d err=%v", attempt+1, t.MaxRetries+1, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
		log.Printf("tts response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(errBody)))
		return "", fmt.Errorf("tts api request failed with status %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/json") || strings.Contains(contentType, "+json") {
		var parsed struct {
			AudioURL       string   `json:"audioUrl"`
			URL            string   `json:"url"`
			AudioURLAlt    string   `json:"audio_url"`
			AudioURLs      []string `json:"audio_urls"`
			AudioPaths     []string `json:"audio_paths"`
			RelativePath   string   `json:"relative_path"`
			Path           string   `json:"path"`
			FilePath       string   `json:"file_path"`
			AudioBase64    any      `json:"audio_base64"`
			AudioBase64Alt any      `json:"audioBase64"`
			ContentType    string   `json:"content_type"`
			MimeType       string   `json:"mime_type"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			return "", err
		}
		if parsed.AudioURL != "" {
			return parsed.AudioURL, nil
		}
		if parsed.URL != "" {
			return parsed.URL, nil
		}
		if parsed.AudioURLAlt != "" {
			return joinTTSURL(config.BaseURL, parsed.AudioURLAlt), nil
		}
		if len(parsed.AudioURLs) > 0 {
			first := strings.TrimSpace(parsed.AudioURLs[0])
			if first != "" {
				return joinTTSURL(config.BaseURL, first), nil
			}
		}
		if len(parsed.AudioPaths) > 0 {
			first := strings.TrimSpace(parsed.AudioPaths[0])
			if first != "" {
				return joinTTSURL(config.BaseURL, first), nil
			}
		}
		if parsed.RelativePath != "" {
			return joinTTSURL(config.BaseURL, parsed.RelativePath), nil
		}
		if parsed.Path != "" {
			mapped := mapLocalTTSPath(config.BaseURL, parsed.Path)
			if mapped != "" {
				return mapped, nil
			}
		}
		if parsed.FilePath != "" {
			mapped := mapLocalTTSPath(config.BaseURL, parsed.FilePath)
			if mapped != "" {
				return mapped, nil
			}
		}
		inlineAudio := firstNonEmptyAudioBase64(parsed.AudioBase64)
		if inlineAudio == "" {
			inlineAudio = firstNonEmptyAudioBase64(parsed.AudioBase64Alt)
		}
		if inlineAudio != "" {
			if strings.HasPrefix(inlineAudio, "data:audio/") {
				return inlineAudio, nil
			}
			mime := strings.TrimSpace(parsed.ContentType)
			if mime == "" {
				mime = strings.TrimSpace(parsed.MimeType)
			}
			if mime == "" || !strings.HasPrefix(strings.ToLower(mime), "audio/") {
				mime = ttsAudioMIME(config.AudioFormat)
			}
			return "data:" + mime + ";base64," + inlineAudio, nil
		}
		return "", errors.New("tts api missing audio url field")
	}
	if strings.HasPrefix(contentType, "audio/") || contentType == "application/octet-stream" {
		payload, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		if len(payload) == 0 {
			return "", errors.New("tts api returned empty audio payload")
		}
		encoded := base64.StdEncoding.EncodeToString(payload)
		return "data:" + contentType + ";base64," + encoded, nil
	}
	return "", errors.New("tts api returned unsupported content type")
}

func (t *APITTS) synthesizeXiaomi(ctx context.Context, config domain.TTSConfig, text string, language string, requestedVoice string, dialogueContext string) (string, error) {
	if isAudioDataURL(requestedVoice) {
		messages, audio, buildErr := buildXiaomiMessagesAndAudioWithContext(xiaomiTTSVoiceCloneModel, requestedVoice, config.AudioFormat, text, language, "", dialogueContext)
		if buildErr != nil {
			return "", buildErr
		}
		return t.doXiaomiSynthesis(ctx, config, xiaomiTTSVoiceCloneModel, messages, audio)
	}
	model := normalizeTTSModel(ttsProviderXiaomi, config.Model)
	// VoiceDesign and VoiceClone are explicit administrator choices. Do not
	// replace them with the automatic fallback below.
	if isXiaomiVoiceDesignModel(model) || isXiaomiVoiceCloneModel(model) {
		messages, audio, buildErr := buildXiaomiMessagesAndAudioWithContext(model, config.Voice, config.AudioFormat, text, language, requestedVoice, dialogueContext)
		if buildErr != nil {
			return "", buildErr
		}
		return t.doXiaomiSynthesis(ctx, config, model, messages, audio)
	}
	if presetVoice := xiaomiAutomaticPresetVoice(requestedVoice); presetVoice != "" {
		messages, audio, buildErr := buildXiaomiMessagesAndAudioWithContext(model, presetVoice, config.AudioFormat, text, language, requestedVoice, dialogueContext)
		if buildErr != nil {
			return "", buildErr
		}
		return t.doXiaomiSynthesis(ctx, config, model, messages, audio)
	}
	messages, audio, buildErr := buildXiaomiMessagesAndAudioWithContext(model, config.Voice, config.AudioFormat, text, language, requestedVoice, dialogueContext)
	if buildErr != nil {
		return "", buildErr
	}
	return t.doXiaomiSynthesis(ctx, config, model, messages, audio)
}

func (t *APITTS) synthesizeXiaomiWithDesignedClone(ctx context.Context, config domain.TTSConfig, text string, language string, requestedVoice string) (string, error) {
	style := normalizeVoiceStyle(requestedVoice)
	if style == "" {
		// MiMo does not expose a Cantonese locale or a Cantonese preset voice.
		// Seed an explicitly Cantonese VoiceDesign sample for automatic flows such
		// as reading materials, where no character style is provided.
		if isCantoneseLanguage(language) {
			style = defaultXiaomiCantoneseStyle
		} else {
			return "", errors.New("xiaomi designed clone flow requires a normalized voice style")
		}
	}
	seedAudio, seedErr := t.getXiaomiDesignedSeed(ctx, config, style, language)
	if seedErr != nil {
		return "", seedErr
	}
	messages, audio, buildErr := buildXiaomiMessagesAndAudio(xiaomiTTSVoiceCloneModel, seedAudio, config.AudioFormat, text, language, requestedVoice)
	if buildErr != nil {
		return "", buildErr
	}
	return t.doXiaomiSynthesis(ctx, config, xiaomiTTSVoiceCloneModel, messages, audio)
}

func (t *APITTS) getXiaomiDesignedSeed(ctx context.Context, config domain.TTSConfig, style string, language string) (string, error) {
	cacheKey := xiaomiVoiceSeedVersion + ":" + xiaomiDesignedCloneCacheKey(style, language) + ":" + normalizeTTSAudioFormat(config.AudioFormat)
	return t.getOrCreateXiaomiCloneSeed(ctx, cacheKey, config.AudioFormat, func() (string, error) {
		designPrompt := xiaomiVoiceDesignPrompt(style, language)
		seedText := xiaomiVoiceDesignSampleText(language)
		messages, audio, buildErr := buildXiaomiMessagesAndAudio(xiaomiTTSVoiceDesignModel, designPrompt, config.AudioFormat, seedText, language, "")
		if buildErr != nil {
			return "", buildErr
		}
		seed, err := t.doXiaomiSynthesis(ctx, config, xiaomiTTSVoiceDesignModel, messages, audio)
		if err != nil {
			return "", err
		}
		log.Printf("xiaomi theater voice seed generated style=%s language=%s", style, language)
		return seed, nil
	})
}

func (t *APITTS) doXiaomiSynthesis(ctx context.Context, config domain.TTSConfig, model string, messages []map[string]string, audio map[string]any) (string, error) {
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"audio":    prepareXiaomiAudioForEndpoint(model, config.BaseURL, audio),
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return "", marshalErr
	}

	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return "", errors.New("xiaomi tts api key not configured")
	}
	chatURL := xiaomiChatCompletionsURL(config.BaseURL)
	log.Printf("tts request provider=%s url=%s model=%s messages=%d", config.Provider, chatURL, payload["model"], len(messages))

	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		setXiaomiAuthHeader(req, config.BaseURL, apiKey)

		resp, err = t.Client.Do(req)
		if err == nil && resp != nil && shouldRetryTTSStatus(resp.StatusCode) && attempt < t.MaxRetries {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			log.Printf("xiaomi tts retryable status=%d attempt=%d/%d body=%s", resp.StatusCode, attempt+1, t.MaxRetries+1, strings.TrimSpace(string(body)))
			if sleepErr := sleepBeforeTTSRetry(ctx, attempt); sleepErr != nil {
				return "", sleepErr
			}
			continue
		}
		if err == nil {
			break
		}
		if !isRetryableTTSError(err) || attempt == t.MaxRetries {
			return "", err
		}
		log.Printf("xiaomi tts retryable error attempt=%d/%d err=%v", attempt+1, t.MaxRetries+1, err)
		if sleepErr := sleepBeforeTTSRetry(ctx, attempt); sleepErr != nil {
			return "", sleepErr
		}
	}
	if resp == nil {
		return "", errors.New("xiaomi tts request failed")
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		log.Printf("xiaomi tts response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		return "", fmt.Errorf("xiaomi tts api request failed with status %d", resp.StatusCode)
	}
	audioData, err := extractXiaomiAudioData(body)
	if err != nil {
		return "", err
	}
	return "data:" + ttsAudioMIME(config.AudioFormat) + ";base64," + audioData, nil
}

func (t *APITTS) getOrCreateXiaomiCloneSeed(ctx context.Context, key string, format string, create func() (string, error)) (string, error) {
	t.mu.Lock()
	if t.xiaomiCloneSeed == nil {
		t.xiaomiCloneSeed = make(map[string]string)
	}
	if seed := strings.TrimSpace(t.xiaomiCloneSeed[key]); seed != "" {
		t.mu.Unlock()
		return seed, nil
	}
	t.mu.Unlock()
	if seed := t.loadXiaomiCloneSeed(key, format); seed != "" {
		t.mu.Lock()
		t.xiaomiCloneSeed[key] = seed
		t.mu.Unlock()
		return seed, nil
	}
	t.mu.Lock()
	if seed := strings.TrimSpace(t.xiaomiCloneSeed[key]); seed != "" {
		t.mu.Unlock()
		return seed, nil
	}
	if t.xiaomiSeedWait == nil {
		t.xiaomiSeedWait = make(map[string]chan struct{})
	}
	if wait, exists := t.xiaomiSeedWait[key]; exists {
		t.mu.Unlock()
		select {
		case <-wait:
			t.mu.RLock()
			seed := strings.TrimSpace(t.xiaomiCloneSeed[key])
			t.mu.RUnlock()
			if seed == "" {
				return "", errors.New("xiaomi voice seed generation failed")
			}
			return seed, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	wait := make(chan struct{})
	t.xiaomiSeedWait[key] = wait
	t.mu.Unlock()

	seed, err := create()
	seed = strings.TrimSpace(seed)
	t.mu.Lock()
	if err == nil && seed != "" {
		t.xiaomiCloneSeed[key] = seed
	} else if err == nil {
		err = errors.New("xiaomi voice design returned empty seed audio")
	}
	delete(t.xiaomiSeedWait, key)
	close(wait)
	t.mu.Unlock()
	if err == nil {
		if persistErr := t.persistXiaomiCloneSeed(key, format, seed); persistErr != nil {
			log.Printf("persist xiaomi voice seed failed key=%s err=%v", key, persistErr)
		}
	}
	return seed, err
}

func (t *APITTS) loadXiaomiCloneSeed(key string, format string) string {
	path := t.xiaomiCloneSeedPath(key, format)
	if path == "" {
		return ""
	}
	payload, err := os.ReadFile(path)
	if err != nil || len(payload) == 0 {
		return ""
	}
	return "data:" + ttsAudioMIME(format) + ";base64," + base64.StdEncoding.EncodeToString(payload)
}

func (t *APITTS) persistXiaomiCloneSeed(key string, format string, seed string) error {
	path := t.xiaomiCloneSeedPath(key, format)
	if path == "" {
		return nil
	}
	payload, err := base64.StdEncoding.DecodeString(xiaomiVoiceCloneSample(seed))
	if err != nil || len(payload) == 0 {
		return errors.New("invalid xiaomi voice seed audio")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporaryPath := path + ".tmp"
	if err = os.WriteFile(temporaryPath, payload, 0o644); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	return nil
}

func (t *APITTS) xiaomiCloneSeedPath(key string, format string) string {
	t.mu.RLock()
	dir := strings.TrimSpace(t.voiceSeedDir)
	t.mu.RUnlock()
	if dir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	extension := "." + normalizeTTSAudioFormat(format)
	return filepath.Join(dir, hex.EncodeToString(sum[:])+extension)
}

func setXiaomiAuthHeader(req *http.Request, baseURL string, apiKey string) {
	if isXiaomiTokenPlanURL(baseURL) {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return
	}
	req.Header.Set("api-key", apiKey)
}

func prepareXiaomiAudioForEndpoint(model string, baseURL string, audio map[string]any) map[string]any {
	if !isXiaomiVoiceCloneModel(model) || isXiaomiTokenPlanURL(baseURL) {
		return audio
	}
	voice, _ := audio["voice"].(string)
	voice = strings.TrimSpace(voice)
	if voice == "" || isAudioDataURL(voice) {
		return audio
	}
	format, _ := audio["format"].(string)
	prepared := make(map[string]any, len(audio))
	for key, value := range audio {
		prepared[key] = value
	}
	prepared["voice"] = "data:" + ttsAudioMIME(format) + ";base64," + voice
	return prepared
}

func isXiaomiTokenPlanURL(baseURL string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(baseURL)), "token-plan")
}

func (t *APITTS) synthesizeMiniMax(ctx context.Context, config domain.TTSConfig, text string, language string, requestedVoice string) (string, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return "", errors.New("minimax tts api key not configured")
	}
	content := strings.TrimSpace(text)
	if content == "" {
		return "", errors.New("minimax tts text is empty")
	}
	payload := map[string]any{
		"model":          normalizeTTSModel(ttsProviderMiniMax, config.Model),
		"text":           content,
		"stream":         false,
		"output_format":  "hex",
		"language_boost": minimaxLanguageBoost(language),
		"voice_setting": map[string]any{
			"voice_id": resolveTTSVoice(ttsProviderMiniMax, config.Voice, requestedVoice),
			"speed":    1,
			"vol":      1,
			"pitch":    0,
		},
		"audio_setting": map[string]any{
			"sample_rate": 32000,
			"bitrate":     128000,
			"format":      normalizeTTSAudioFormat(config.AudioFormat),
			"channel":     1,
		},
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return "", marshalErr
	}

	ttsURL := minimaxTTSURL(config.BaseURL)
	log.Printf("tts request provider=%s url=%s model=%s text_len=%d language=%s", config.Provider, ttsURL, payload["model"], len(text), language)
	resp, err := t.doTTSRequestWithRetry(ctx, ttsURL, raw, func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		log.Printf("minimax tts response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		return "", fmt.Errorf("minimax tts api request failed with status %d", resp.StatusCode)
	}

	audioHex, err := extractMiniMaxAudioHex(body)
	if err != nil {
		return "", err
	}
	decoded, err := hex.DecodeString(audioHex)
	if err != nil {
		return "", err
	}
	return "data:" + ttsAudioMIME(config.AudioFormat) + ";base64," + base64.StdEncoding.EncodeToString(decoded), nil
}

func (t *APITTS) synthesizeAliyun(ctx context.Context, config domain.TTSConfig, text string, language string, requestedVoice string) (string, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return "", errors.New("aliyun tts api key not configured")
	}
	content := strings.TrimSpace(text)
	if content == "" {
		return "", errors.New("aliyun tts text is empty")
	}
	payload := map[string]any{
		"model": normalizeTTSModel(ttsProviderAliyun, config.Model),
		"input": map[string]any{
			"text":        content,
			"voice":       resolveTTSVoice(ttsProviderAliyun, config.Voice, requestedVoice),
			"format":      normalizeTTSAudioFormat(config.AudioFormat),
			"sample_rate": 24000,
		},
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return "", marshalErr
	}

	ttsURL := aliyunTTSURL(config.BaseURL)
	log.Printf("tts request provider=%s url=%s model=%s text_len=%d language=%s", config.Provider, ttsURL, payload["model"], len(text), language)
	resp, err := t.doTTSRequestWithRetry(ctx, ttsURL, raw, func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode >= 400 {
		log.Printf("aliyun tts response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		return "", fmt.Errorf("aliyun tts api request failed with status %d", resp.StatusCode)
	}

	return extractAliyunAudioURL(body, config.AudioFormat)
}

func buildXiaomiMessagesAndAudio(model string, configuredVoice string, format string, text string, language string, requestedVoice string) ([]map[string]string, map[string]any, error) {
	return buildXiaomiMessagesAndAudioWithContext(model, configuredVoice, format, text, language, requestedVoice, "")
}

func buildXiaomiMessagesAndAudioWithContext(model string, configuredVoice string, format string, text string, language string, requestedVoice string, dialogueContext string) ([]map[string]string, map[string]any, error) {
	audioFormat := normalizeTTSAudioFormat(format)
	synthesisText := xiaomiSynthesisText(text, language)
	switch {
	case isXiaomiVoiceDesignModel(model):
		voiceDescription := normalizeTTSVoiceForModel(ttsProviderXiaomi, model, configuredVoice)
		return []map[string]string{
				{"role": "user", "content": voiceDescription},
				{"role": "assistant", "content": synthesisText},
			},
			map[string]any{"format": audioFormat, "optimize_text_preview": true}, nil
	case isXiaomiVoiceCloneModel(model):
		cloneSample := strings.TrimSpace(configuredVoice)
		if cloneSample == "" {
			return nil, nil, errors.New("xiaomi voice clone sample not configured")
		}
		if !isAudioDataURL(cloneSample) {
			return nil, nil, errors.New("xiaomi voice clone sample must be a data:audio URL")
		}
		instruction := buildInstructionWithContext(text, language, requestedVoice, dialogueContext)
		return []map[string]string{
				{"role": "user", "content": instruction},
				{"role": "assistant", "content": synthesisText},
			},
			map[string]any{
				"format": audioFormat,
				"voice":  xiaomiVoiceCloneSample(cloneSample),
			}, nil
	default:
		instruction := buildInstructionWithContext(text, language, requestedVoice, dialogueContext)
		return []map[string]string{
				{"role": "user", "content": instruction},
				{"role": "assistant", "content": synthesisText},
			},
			map[string]any{
				"format": audioFormat,
				"voice":  normalizeTTSVoiceForModel(ttsProviderXiaomi, model, configuredVoice),
			}, nil
	}
}

func xiaomiAutomaticPresetVoice(requestedVoice string) string {
	clean := strings.TrimSpace(requestedVoice)
	if clean == "" || isAudioDataURL(clean) {
		return ""
	}
	for preset := range xiaomiPresetVoices {
		if strings.EqualFold(clean, preset) {
			return preset
		}
	}
	return xiaomiAutomaticPresetByStyle[normalizeVoiceStyle(clean)]
}

func shouldUseXiaomiDesignedCloneFlow(requestedVoice string) bool {
	return normalizeVoiceStyle(requestedVoice) != ""
}

func isCantoneseLanguage(language string) bool {
	return strings.EqualFold(strings.TrimSpace(language), "CANTONESE")
}

func xiaomiDesignedCloneCacheKey(style string, language string) string {
	return strings.ToUpper(strings.TrimSpace(language)) + ":" + normalizeVoiceAlias(style)
}

func xiaomiVoiceDesignPrompt(style string, language string) string {
	normalizedStyle := normalizeVoiceStyle(style)
	if normalizedStyle == "" {
		normalizedStyle = "温柔女生"
	}
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CANTONESE":
		return fmt.Sprintf("請設計一把香港本地%s。人物聲線設定：%s。角色標籤只用嚟區分音色，唔好刻意表演、扮聲或者誇張情緒。必須由頭到尾使用自然香港粵語／廣東話發音，粵語聲調、連讀同句尾語氣要清楚，唔可以有普通話、國語或者書面朗讀腔。用平穩自然嘅日常傾偈節奏一次讀完，唔好自行加入笑聲、氣聲、戲劇停頓、拖長尾音或者突然變速。", normalizedStyle, cantoneseVoicePersona(normalizedStyle))
	case "ENGLISH":
		return fmt.Sprintf("Design a distinct %s character voice for natural English learning dialogues. Keep the voice realistic, emotionally stable and clearly differentiated from other characters, with steady pacing and no announcer tone.", normalizedStyle)
	default:
		return fmt.Sprintf("请设计一个%s的声音，音色真实自然，情绪稳定，吐字清晰，保持稳定语速，不要播报腔。", normalizedStyle)
	}
}

func cantoneseVoicePersona(style string) string {
	switch normalizeVoiceStyle(style) {
	case "温柔女生":
		return "二十多歲香港女聲，中音偏明亮，柔和親切，氣息自然"
	case "甜美女生":
		return "年輕香港女聲，中高音，聲線清甜但唔嗲，語氣自然"
	case "播音男生":
		return "三十歲左右香港男聲，中低音，厚實清楚，只保留清晰度而唔用播音腔"
	case "沉稳大叔":
		return "四十歲左右香港男聲，低音沉穩，共鳴自然，說話從容但唔拖慢"
	case "御姐音色":
		return "三十歲左右香港女聲，中低音，沉着自信，語尾俐落但唔強勢"
	case "清新少女":
		return "年輕香港女聲，中高音，清爽輕盈，節奏自然明快"
	case "知性姐姐":
		return "成熟香港女聲，中音溫暖，冷靜清晰，表達自然有分寸"
	case "活力少年":
		return "年輕香港男聲，中高音，精神自然，語氣爽快但唔誇張"
	case "港风店员":
		return "二十多歲香港女聲，中音，親切利落，生活感自然"
	case "电台暖男":
		return "成熟香港男聲，中低音溫暖，放鬆自然，唔用電台或者播音腔"
	case "冷静导师":
		return "成熟香港男聲，中低音，耐心清楚，語氣平穩冷靜"
	case "俏皮同学":
		return "年輕香港中性聲線，中高音，輕快有互動感，但唔賣萌唔拖尾音"
	default:
		return "香港本地成年人聲線，音色自然清楚，人物特徵鮮明"
	}
}

func xiaomiVoiceDesignSampleText(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CANTONESE":
		return "早晨，唔該你幫我留一張枱。我想要一杯熱奶茶，少甜，拎走呀。我哋兩點左右到，想坐近窗邊。今日交通有啲慢，不過我會提早十分鐘出門口。你聽日得唔得閒一齊食飯？如果落雨，我哋就改約下星期，唔使急，慢慢傾都得。你收到訊息之後覆我一聲，好嗎？"
	case "ENGLISH":
		return "Hello, welcome to LinguaQuest. Today we are going to practise a natural everyday conversation. I will speak clearly, but I will keep the rhythm relaxed. If you need more time, pause and listen again. When you are ready, answer in your own words, and we can continue together."
	default:
		return "你好，欢迎来到 LinguaQuest，我们开始自然对话练习。"
	}
}

func (t *APITTS) doTTSRequestWithRetry(ctx context.Context, url string, raw []byte, prepare func(*http.Request)) (*http.Response, error) {
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		prepare(req)
		resp, err = t.Client.Do(req)
		if err == nil && resp != nil && shouldRetryTTSStatus(resp.StatusCode) && attempt < t.MaxRetries {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			log.Printf("tts retryable status provider_url=%s status=%d attempt=%d/%d body=%s", url, resp.StatusCode, attempt+1, t.MaxRetries+1, strings.TrimSpace(string(body)))
			if sleepErr := sleepBeforeTTSRetry(ctx, attempt); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		if err == nil {
			return resp, nil
		}
		if !isRetryableTTSError(err) || attempt == t.MaxRetries {
			return nil, err
		}
		log.Printf("tts retryable error provider_url=%s attempt=%d/%d err=%v", url, attempt+1, t.MaxRetries+1, err)
		if sleepErr := sleepBeforeTTSRetry(ctx, attempt); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return resp, err
}

func extractXiaomiAudioData(body []byte) (string, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	for _, choice := range parsed.Choices {
		data := strings.TrimSpace(choice.Message.Audio.Data)
		if data != "" {
			return data, nil
		}
	}
	return "", errors.New("xiaomi tts response missing choices[0].message.audio.data")
}

func extractMiniMaxAudioHex(body []byte) (string, error) {
	var parsed struct {
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.BaseResp.StatusCode != 0 {
		if strings.TrimSpace(parsed.BaseResp.StatusMsg) != "" {
			return "", errors.New(strings.TrimSpace(parsed.BaseResp.StatusMsg))
		}
		return "", fmt.Errorf("minimax tts request failed with status code %d", parsed.BaseResp.StatusCode)
	}
	audioHex := strings.TrimSpace(parsed.Data.Audio)
	if audioHex == "" {
		return "", errors.New("minimax tts response missing data.audio")
	}
	return audioHex, nil
}

func extractAliyunAudioURL(body []byte, format string) (string, error) {
	var parsed struct {
		Output struct {
			Audio struct {
				URL  string `json:"url"`
				Data string `json:"data"`
			} `json:"audio"`
		} `json:"output"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.Code) != "" && !strings.EqualFold(strings.TrimSpace(parsed.Code), "ok") {
		if strings.TrimSpace(parsed.Message) != "" {
			return "", errors.New(strings.TrimSpace(parsed.Message))
		}
		return "", fmt.Errorf("aliyun tts request failed with code %s", strings.TrimSpace(parsed.Code))
	}
	if url := strings.TrimSpace(parsed.Output.Audio.URL); url != "" {
		return url, nil
	}
	if data := strings.TrimSpace(parsed.Output.Audio.Data); data != "" {
		if strings.HasPrefix(strings.ToLower(data), "data:audio/") {
			return data, nil
		}
		return "data:" + ttsAudioMIME(format) + ";base64," + data, nil
	}
	return "", errors.New("aliyun tts response missing output.audio.url")
}

func xiaomiSynthesisText(text string, language string) string {
	content := strings.TrimSpace(text)
	if !isCantoneseLanguage(language) || content == "" {
		return content
	}
	if hasXiaomiCantoneseTag(content) {
		content = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(content, "(粤语)"), "（粤语）"))
	}
	content = contentquality.NormalizeCantoneseSpeechText(content)
	if content == "" {
		return ""
	}
	// MiMo 官方文档将“粤语”列为 assistant 文本开头的音频标签。使用内置
	// TTS 加该标签可避免自动 VoiceDesign/VoiceClone 带来的额外时延和音色漂移。
	return "(粤语)" + content
}

func hasXiaomiCantoneseTag(text string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	return strings.HasPrefix(trimmed, "(粤语)") ||
		strings.HasPrefix(trimmed, "（粤语）") ||
		strings.HasPrefix(trimmed, "(cantonese)") ||
		strings.HasPrefix(trimmed, "（cantonese）")
}

func minimaxLanguageBoost(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CANTONESE":
		return "Chinese,Yue"
	case "ENGLISH":
		return "English"
	default:
		return "auto"
	}
}

func resolveTTSVoice(provider string, configuredVoice string, requestedVoice string) string {
	if voice := strings.TrimSpace(requestedVoice); voice != "" {
		return voice
	}
	return normalizeTTSVoice(provider, configuredVoice)
}

func xiaomiChatCompletionsURL(baseURL string) string {
	cleaned := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(cleaned, "/chat/completions") {
		return cleaned
	}
	if strings.HasSuffix(cleaned, "/v1") {
		return cleaned + "/chat/completions"
	}
	return cleaned + "/v1/chat/completions"
}

func minimaxTTSURL(baseURL string) string {
	cleaned := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(cleaned, "/v1/t2a_v2") {
		return cleaned
	}
	if strings.HasSuffix(cleaned, "/v1") {
		return cleaned + "/t2a_v2"
	}
	return cleaned + "/v1/t2a_v2"
}

func aliyunTTSURL(baseURL string) string {
	cleaned := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(cleaned), "/api/v1/services/audio/tts/speechsynthesizer") {
		return cleaned
	}
	return cleaned + "/api/v1/services/audio/tts/SpeechSynthesizer"
}

func normalizeTTSProvider(provider string) string {
	switch strings.ToUpper(strings.TrimSpace(provider)) {
	case ttsProviderXiaomi:
		return ttsProviderXiaomi
	case ttsProviderMiniMax:
		return ttsProviderMiniMax
	case ttsProviderAliyun:
		return ttsProviderAliyun
	default:
		return ttsProviderCustom
	}
}

func normalizeTTSModel(provider string, model string) string {
	cleaned := strings.TrimSpace(model)
	if cleaned != "" {
		return cleaned
	}
	switch {
	case strings.EqualFold(provider, ttsProviderXiaomi):
		return defaultXiaomiTTSModel
	case strings.EqualFold(provider, ttsProviderMiniMax):
		return defaultMiniMaxTTSModel
	case strings.EqualFold(provider, ttsProviderAliyun):
		return defaultAliyunTTSModel
	}
	return ""
}

func normalizeTTSVoice(provider string, voice string) string {
	cleaned := strings.TrimSpace(voice)
	if cleaned != "" {
		return cleaned
	}
	switch {
	case strings.EqualFold(provider, ttsProviderXiaomi):
		return defaultXiaomiTTSVoice
	case strings.EqualFold(provider, ttsProviderMiniMax):
		return defaultMiniMaxTTSVoice
	case strings.EqualFold(provider, ttsProviderAliyun):
		return defaultAliyunTTSVoice
	}
	return defaultCustomTTSVoice
}

func normalizeTTSVoiceForModel(provider string, model string, voice string) string {
	if !strings.EqualFold(provider, ttsProviderXiaomi) {
		return normalizeTTSVoice(provider, voice)
	}
	normalizedModel := normalizeTTSModel(provider, model)
	cleaned := strings.TrimSpace(voice)
	switch {
	case isXiaomiVoiceDesignModel(normalizedModel):
		if cleaned != "" && !isAudioDataURL(cleaned) {
			return cleaned
		}
		return defaultXiaomiVoiceDesignText
	case isXiaomiVoiceCloneModel(normalizedModel):
		if isAudioDataURL(cleaned) {
			return cleaned
		}
		return ""
	default:
		if _, ok := xiaomiPresetVoices[cleaned]; ok {
			return cleaned
		}
		return defaultXiaomiTTSVoice
	}
}

func isXiaomiVoiceDesignModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), xiaomiTTSVoiceDesignModel)
}

func isXiaomiVoiceCloneModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), xiaomiTTSVoiceCloneModel)
}

func isAudioDataURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:audio/")
}

func xiaomiVoiceCloneSample(dataURL string) string {
	trimmed := strings.TrimSpace(dataURL)
	if markerIndex := strings.Index(strings.ToLower(trimmed), ";base64,"); markerIndex >= 0 {
		return strings.TrimSpace(trimmed[markerIndex+len(";base64,"):])
	}
	return trimmed
}

func normalizeTTSAudioFormat(format string) string {
	cleaned := strings.ToLower(strings.TrimSpace(format))
	if cleaned == "" {
		return defaultTTSAudioFormat
	}
	switch cleaned {
	case "mp3", "mpeg", "ogg", "opus", "wav":
		if cleaned == "mpeg" {
			return "mp3"
		}
		return cleaned
	default:
		return defaultTTSAudioFormat
	}
}

func ttsAudioMIME(format string) string {
	switch normalizeTTSAudioFormat(format) {
	case "mp3":
		return "audio/mpeg"
	case "ogg", "opus":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}

func firstNonEmptyAudioBase64(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		for _, item := range v {
			candidate := strings.TrimSpace(item)
			if candidate != "" {
				return candidate
			}
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				candidate := strings.TrimSpace(s)
				if candidate != "" {
					return candidate
				}
			}
		}
	}
	return ""
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

func isRetryableTTSError(err error) bool {
	if err == nil {
		return false
	}
	if isTimeoutErr(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Temporary() {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "tls handshake timeout") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "unexpected eof") ||
		strings.Contains(lower, "temporary")
}

func shouldRetryTTSStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500
}

func sleepBeforeTTSRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(250*(attempt+1)) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func joinTTSURL(apiURL string, path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	base := strings.TrimRight(apiURL, "/")
	if idx := strings.Index(base, "/vapi/"); idx != -1 {
		base = base[:idx]
	}
	if idx := strings.Index(base, "/v1/"); idx != -1 {
		base = base[:idx]
	}
	if strings.HasSuffix(base, "/vapi") {
		base = strings.TrimSuffix(base, "/vapi")
	}
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	if strings.HasPrefix(trimmed, "/") {
		return base + trimmed
	}
	return base + "/" + trimmed
}

func mapLocalTTSPath(apiURL string, localPath string) string {
	cleaned := strings.TrimSpace(localPath)
	if cleaned == "" {
		return ""
	}
	parts := strings.Split(cleaned, "/output/")
	if len(parts) < 2 {
		return ""
	}
	relative := strings.TrimPrefix(parts[len(parts)-1], "/")
	if relative == "" {
		return ""
	}
	return joinTTSURL(apiURL, "/vapi/audio/"+relative)
}

var englishLetter = regexp.MustCompile(`[A-Za-z]`)

func buildInstruction(text string, language string, voiceStyle string) string {
	return buildInstructionWithContext(text, language, voiceStyle, "")
}

func buildInstructionWithContext(text string, language string, voiceStyle string, dialogueContext string) string {
	style := normalizeVoiceStyle(voiceStyle)
	if style == "" {
		style = "温柔女生"
	}
	base := voiceStyleInstruction(style) + "，保持中速偏自然的稳定语速，按标点做短停顿，不要忽快忽慢，不要拖长句尾，像真实生活场景中的对话，不要播报腔。"
	lang := strings.ToUpper(strings.TrimSpace(language))
	if lang == "CANTONESE" {
		persona := cantoneseVoicePersona(style)
		contextNote := normalizeTTSDialogueContext(dialogueContext)
		if contextNote != "" {
			contextNote = "對話背景只作語氣參考，唔需要讀出：" + contextNote + "。"
		}
		englishNote := ""
		if englishLetter.MatchString(text) {
			englishNote = "英文字母按前後粵語語意自然帶過。"
		}
		return contextNote + "使用自然香港粵語，以真實傾偈方式講出 assistant 原文。角色聲線：" + persona + "。語速自然略快，句子連貫，逗號輕輕帶過，句尾自然收束。" + englishNote
	}
	return base
}

func normalizeTTSDialogueContext(value string) string {
	clean := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if clean == "" {
		return ""
	}
	const maxRunes = 320
	runes := []rune(clean)
	if len(runes) > maxRunes {
		clean = string(runes[:maxRunes])
	}
	return clean
}

func resolveVoiceSelection(requestedVoice string, defaultVoice string) (string, string) {
	voice := strings.TrimSpace(requestedVoice)
	if voice == "" {
		return strings.TrimSpace(defaultVoice), normalizeVoiceStyle(defaultVoice)
	}
	style := normalizeVoiceStyle(voice)
	if style != "" {
		return strings.TrimSpace(defaultVoice), style
	}
	return voice, normalizeVoiceStyle(voice)
}

func normalizeVoiceStyle(input string) string {
	return normalizeVoiceStyleFromCatalog(input)
}

func voiceStyleInstruction(style string) string {
	instruction := voiceStyleInstructionFromCatalog(style)
	if instruction != "" {
		return instruction
	}
	return "请用温柔女生音色说"
}
