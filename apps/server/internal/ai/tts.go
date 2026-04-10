package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type APITTS struct {
	APIURL          string
	APIKey          string
	Voice           string
	UseUploadPrompt bool
	PromptAudioPath string
	ReturnJSON      bool
	MaxRetries      int
	Client          *http.Client
}

func NewAPITTS(apiURL string, apiKey string, voice string, useUploadPrompt bool, promptAudioPath string, returnJSON bool, timeoutSeconds int, maxRetries int) *APITTS {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 45
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &APITTS{
		APIURL:          apiURL,
		APIKey:          apiKey,
		Voice:           voice,
		UseUploadPrompt: useUploadPrompt,
		PromptAudioPath: strings.TrimSpace(promptAudioPath),
		ReturnJSON:      returnJSON,
		MaxRetries:      maxRetries,
		Client:          &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

func (t *APITTS) Synthesize(ctx context.Context, text string, language string, voice string) (string, error) {
	if t.APIURL == "" {
		return "", errors.New("tts api url not configured")
	}
	if voice == "" {
		voice = t.Voice
	}
	instruction := buildInstruction(text, language)
	payload := map[string]any{
		"text":              text,
		"instruct":          instruction,
		"use_upload_prompt": t.UseUploadPrompt,
		"return_json":       t.ReturnJSON,
	}
	if t.PromptAudioPath != "" {
		payload["prompt_audio_path"] = t.PromptAudioPath
	}
	if strings.TrimSpace(voice) != "" {
		payload["voice"] = strings.TrimSpace(voice)
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return "", marshalErr
	}

	apiKey := strings.TrimSpace(t.APIKey)
	log.Printf("tts request url=%s text_len=%d language=%s", t.APIURL, len(text), language)
	var resp *http.Response
	var err error
	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, t.APIURL, bytes.NewReader(raw))
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
			AudioURL     string `json:"audioUrl"`
			URL          string `json:"url"`
			AudioURLAlt  string `json:"audio_url"`
			AudioURLs    []string `json:"audio_urls"`
			AudioPaths   []string `json:"audio_paths"`
			RelativePath string `json:"relative_path"`
			Path         string `json:"path"`
			FilePath     string `json:"file_path"`
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
			return joinTTSURL(t.APIURL, parsed.AudioURLAlt), nil
		}
		if len(parsed.AudioURLs) > 0 {
			first := strings.TrimSpace(parsed.AudioURLs[0])
			if first != "" {
				return joinTTSURL(t.APIURL, first), nil
			}
		}
		if len(parsed.AudioPaths) > 0 {
			first := strings.TrimSpace(parsed.AudioPaths[0])
			if first != "" {
				return joinTTSURL(t.APIURL, first), nil
			}
		}
		if parsed.RelativePath != "" {
			return joinTTSURL(t.APIURL, parsed.RelativePath), nil
		}
		if parsed.Path != "" {
			mapped := mapLocalTTSPath(t.APIURL, parsed.Path)
			if mapped != "" {
				return mapped, nil
			}
		}
		if parsed.FilePath != "" {
			mapped := mapLocalTTSPath(t.APIURL, parsed.FilePath)
			if mapped != "" {
				return mapped, nil
			}
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

func buildInstruction(text string, language string) string {
	base := "温柔地说"
	lang := strings.ToUpper(strings.TrimSpace(language))
	if lang == "CANTONESE" && !englishLetter.MatchString(text) {
		return "请用广东话说，" + base
	}
	return base
}
