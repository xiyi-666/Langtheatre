package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

const maxASRAudioBytes = 10 * 1024 * 1024

// APIASR dispatches short WAV/MP3 recordings to the configured provider.
// Providers deliberately stay behind this interface so role-play processing
// remains independent from each vendor's authentication and wire format.
type APIASR struct {
	mu        sync.RWMutex
	provider  string
	baseURL   string
	apiKey    string
	appID     string
	model     string
	updatedAt time.Time
	client    *http.Client
}

func NewAPIASR(provider, baseURL, apiKey, appID, model string) *APIASR {
	asr := &APIASR{client: &http.Client{Timeout: 90 * time.Second}}
	asr.UpdateASRConfig(domain.ASRConfig{Provider: provider, BaseURL: baseURL, APIKey: apiKey, AppID: appID, Model: model})
	return asr
}

func (a *APIASR) GetASRConfig() domain.ASRConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return domain.ASRConfig{
		Provider: a.provider, BaseURL: a.baseURL, APIKey: a.apiKey, AppID: a.appID, Model: a.model, UpdatedAt: a.updatedAt,
	}
}

func (a *APIASR) UpdateASRConfig(config domain.ASRConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = normalizeASRProvider(config.Provider)
	a.baseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	a.apiKey = strings.TrimSpace(config.APIKey)
	a.appID = strings.TrimSpace(config.AppID)
	a.model = strings.TrimSpace(config.Model)
	if a.model == "" {
		a.model = defaultASRModel(a.provider)
	}
	if a.baseURL == "" {
		a.baseURL = defaultASRBaseURL(a.provider)
	}
	a.updatedAt = config.UpdatedAt
}

func (a *APIASR) Transcribe(ctx context.Context, audioDataURL, language string) (domain.TranscriptResult, error) {
	config := a.GetASRConfig()
	if config.APIKey == "" {
		return domain.TranscriptResult{}, errors.New("asr api key not configured")
	}
	if config.BaseURL == "" {
		return domain.TranscriptResult{}, errors.New("asr base url not configured")
	}
	if _, _, err := decodeASRAudio(audioDataURL); err != nil {
		return domain.TranscriptResult{}, err
	}

	switch normalizeASRProvider(config.Provider) {
	case "XIAOMI":
		return a.transcribeXiaomi(ctx, config, audioDataURL, language)
	case "ALIYUN":
		return a.transcribeAliyun(ctx, config, audioDataURL, language)
	case "DOUBAO":
		return a.transcribeDoubao(ctx, config, audioDataURL, language)
	case "GEMINI":
		return a.transcribeGemini(ctx, config, audioDataURL, language)
	default:
		return a.transcribeOpenAICompatible(ctx, config, audioDataURL, language)
	}
}

func (a *APIASR) transcribeXiaomi(ctx context.Context, config domain.ASRConfig, audioDataURL, language string) (domain.TranscriptResult, error) {
	requested := xiaomiASRLanguage(language)
	payload := map[string]any{
		"model": config.Model,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{{"type": "input_audio", "input_audio": map[string]string{"data": audioDataURL}}},
		}},
		"asr_options": map[string]string{"language": requested},
		"stream":      false,
	}
	endpoint := strings.TrimRight(config.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	req, err := newJSONRequest(ctx, endpoint, payload)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	req.Header.Set("api-key", config.APIKey)
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Seconds int `json:"seconds"`
		} `json:"usage"`
	}
	if err := a.doJSON(req, &parsed, "xiaomi asr"); err != nil {
		return domain.TranscriptResult{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return domain.TranscriptResult{}, errors.New("xiaomi asr returned an empty transcript")
	}
	return transcriptResult(strings.TrimSpace(parsed.Choices[0].Message.Content), config, language, requested, parsed.Usage.Seconds), nil
}

func (a *APIASR) transcribeAliyun(ctx context.Context, config domain.ASRConfig, audioDataURL, language string) (domain.TranscriptResult, error) {
	payload := map[string]any{
		"model": config.Model,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": "Transcribe this audio faithfully. Return only the transcript."},
				{"type": "input_audio", "input_audio": map[string]string{"data": audioDataURL}},
			},
		}},
		"stream": false,
	}
	endpoint := strings.TrimRight(config.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}
	req, err := newJSONRequest(ctx, endpoint, payload)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := a.doJSON(req, &parsed, "aliyun asr"); err != nil {
		return domain.TranscriptResult{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return domain.TranscriptResult{}, errors.New("aliyun asr returned an empty transcript")
	}
	return transcriptResult(strings.TrimSpace(parsed.Choices[0].Message.Content), config, language, transcriptionLanguageCode(language), 0), nil
}

func (a *APIASR) transcribeDoubao(ctx context.Context, config domain.ASRConfig, audioDataURL, language string) (domain.TranscriptResult, error) {
	if strings.TrimSpace(config.AppID) == "" {
		return domain.TranscriptResult{}, errors.New("doubao asr app id not configured")
	}
	mimeType, audio, err := decodeASRAudio(audioDataURL)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	format := "wav"
	if strings.Contains(mimeType, "mpeg") || strings.Contains(mimeType, "mp3") {
		format = "mp3"
	}
	payload := map[string]any{
		"user": map[string]string{"uid": "linguaquest"},
		"audio": map[string]string{"data": base64.StdEncoding.EncodeToString(audio), "format": format},
		"request": map[string]any{
			"model_name":  config.Model,
			"enable_itn":  true,
			"enable_punc": true,
			"language":    doubaoASRLanguage(language),
		},
	}
	req, err := newJSONRequest(ctx, strings.TrimRight(config.BaseURL, "/"), payload)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	req.Header.Set("X-Api-App-Key", config.AppID)
	req.Header.Set("X-Api-Access-Key", config.APIKey)
	req.Header.Set("X-Api-Resource-Id", "volc.bigasr.auc")
	var parsed struct {
		Text   string `json:"text"`
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
	}
	if err := a.doJSON(req, &parsed, "doubao asr"); err != nil {
		return domain.TranscriptResult{}, err
	}
	text := strings.TrimSpace(parsed.Result.Text)
	if text == "" {
		text = strings.TrimSpace(parsed.Text)
	}
	if text == "" {
		return domain.TranscriptResult{}, errors.New("doubao asr returned an empty transcript")
	}
	return transcriptResult(text, config, language, doubaoASRLanguage(language), 0), nil
}

func (a *APIASR) transcribeGemini(ctx context.Context, config domain.ASRConfig, audioDataURL, language string) (domain.TranscriptResult, error) {
	mimeType, audio, err := decodeASRAudio(audioDataURL)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	model := strings.TrimPrefix(strings.TrimSpace(config.Model), "models/")
	if model == "" {
		return domain.TranscriptResult{}, errors.New("gemini asr model is required")
	}
	endpoint := strings.TrimRight(config.BaseURL, "/")
	if strings.HasSuffix(endpoint, "/models") {
		endpoint += "/" + model + ":generateContent"
	} else {
		endpoint += "/models/" + model + ":generateContent"
	}
	prompt := "Transcribe this audio faithfully. Return only the transcript, with no commentary."
	if hint := geminiLanguageHint(language); hint != "" {
		prompt += " The expected language is " + hint + "."
	}
	payload := map[string]any{
		"contents": []map[string]any{{
			"role": "user",
			"parts": []map[string]any{
				{"text": prompt},
				{"inline_data": map[string]string{"mime_type": mimeType, "data": base64.StdEncoding.EncodeToString(audio)}},
			},
		}},
		"generationConfig": map[string]any{"temperature": 0},
	}
	req, err := newJSONRequest(ctx, endpoint, payload)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	req.Header.Set("x-goog-api-key", config.APIKey)
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := a.doJSON(req, &parsed, "gemini asr"); err != nil {
		return domain.TranscriptResult{}, err
	}
	if len(parsed.Candidates) == 0 {
		return domain.TranscriptResult{}, errors.New("gemini asr returned no transcript candidates")
	}
	parts := make([]string, 0, len(parsed.Candidates[0].Content.Parts))
	for _, part := range parsed.Candidates[0].Content.Parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			parts = append(parts, text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return domain.TranscriptResult{}, errors.New("gemini asr returned an empty transcript")
	}
	return transcriptResult(text, config, language, geminiLanguageHint(language), 0), nil
}

func (a *APIASR) transcribeOpenAICompatible(ctx context.Context, config domain.ASRConfig, audioDataURL, language string) (domain.TranscriptResult, error) {
	mimeType, audio, err := decodeASRAudio(audioDataURL)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filename := "roleplay.wav"
	if strings.Contains(mimeType, "mpeg") || strings.Contains(mimeType, "mp3") {
		filename = "roleplay.mp3"
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	if _, err = part.Write(audio); err != nil {
		return domain.TranscriptResult{}, err
	}
	_ = writer.WriteField("model", config.Model)
	if languageCode := transcriptionLanguageCode(language); languageCode != "" {
		_ = writer.WriteField("language", languageCode)
	}
	if err := writer.Close(); err != nil {
		return domain.TranscriptResult{}, err
	}
	endpoint := transcriptionEndpoint(config)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return domain.TranscriptResult{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	var parsed struct {
		Text     string  `json:"text"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
	}
	if err = a.doJSON(req, &parsed, "asr"); err != nil {
		return domain.TranscriptResult{}, err
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return domain.TranscriptResult{}, errors.New("asr returned an empty transcript")
	}
	return transcriptResult(strings.TrimSpace(parsed.Text), config, language, parsed.Language, int(parsed.Duration)), nil
}

func newJSONRequest(ctx context.Context, endpoint string, payload any) (*http.Request, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (a *APIASR) doJSON(req *http.Request, destination any, source string) error {
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1200))
		return fmt.Errorf("%s request failed: %d %s", source, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(destination)
}

func transcriptResult(text string, config domain.ASRConfig, requested, detected string, duration int) domain.TranscriptResult {
	if detected == "" {
		detected = requested
	}
	return domain.TranscriptResult{
		Text: text, RequestedLanguage: requested, DetectedLanguage: detected, DurationSeconds: duration, Provider: config.Provider, Model: config.Model,
	}
}

func transcriptionEndpoint(config domain.ASRConfig) string {
	endpoint := strings.TrimRight(config.BaseURL, "/")
	if strings.HasSuffix(endpoint, "/audio/transcriptions") || strings.HasSuffix(endpoint, "/audio_transcriptions") {
		return endpoint
	}
	if normalizeASRProvider(config.Provider) == "MINIMAX" {
		return endpoint + "/audio_transcriptions"
	}
	return endpoint + "/audio/transcriptions"
}

func decodeASRAudio(value string) (string, []byte, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ",", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "data:") || !strings.Contains(parts[0], ";base64") {
		return "", nil, errors.New("audio must be a base64 data URL")
	}
	mimeType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
	if mimeType != "audio/wav" && mimeType != "audio/mpeg" && mimeType != "audio/mp3" {
		return "", nil, errors.New("ASR accepts WAV or MP3 audio only")
	}
	audio, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil, errors.New("invalid base64 audio")
	}
	if len(audio) == 0 || len(audio) > maxASRAudioBytes {
		return "", nil, errors.New("audio must be between 1 byte and 10MB")
	}
	return mimeType, audio, nil
}

func normalizeASRProvider(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "XIAOMI", "ALIYUN", "OPENAI", "DOUBAO", "GEMINI", "MINIMAX":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "OPENAI_COMPATIBLE"
	}
}

func defaultASRBaseURL(provider string) string {
	switch normalizeASRProvider(provider) {
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
	default:
		return "https://api.openai.com/v1"
	}
}

func defaultASRModel(provider string) string {
	switch normalizeASRProvider(provider) {
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
	default:
		return "gpt-4o-mini-transcribe"
	}
}

func xiaomiASRLanguage(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CHINESE", "ZH":
		return "zh"
	case "ENGLISH", "EN":
		return "en"
	default:
		return "auto"
	}
}

func doubaoASRLanguage(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CHINESE", "ZH":
		return "zh-CN"
	case "CANTONESE", "YUE":
		return "yue-Hant-HK"
	case "ENGLISH", "EN":
		return "en-US"
	default:
		return ""
	}
}

func geminiLanguageHint(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CHINESE", "ZH":
		return "Chinese"
	case "CANTONESE", "YUE":
		return "Cantonese"
	case "ENGLISH", "EN":
		return "English"
	default:
		return ""
	}
}

func transcriptionLanguageCode(language string) string {
	switch strings.ToUpper(strings.TrimSpace(language)) {
	case "CHINESE", "ZH":
		return "zh"
	case "CANTONESE", "YUE":
		return "yue"
	case "ENGLISH", "EN":
		return "en"
	default:
		return ""
	}
}
