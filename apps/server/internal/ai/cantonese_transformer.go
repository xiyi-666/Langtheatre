package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type CantoneseTransformer struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewCantoneseTransformer(apiKey string, model string, baseURL string) *CantoneseTransformer {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com"
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}
	return &CantoneseTransformer{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (t *CantoneseTransformer) TransformCantonese(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(t.APIKey) == "" {
		return "", errors.New("transformer api key not configured")
	}
	input := strings.TrimSpace(text)
	if input == "" {
		return "", errors.New("empty text")
	}

	sys := "You are a Cantonese rewriting assistant. Convert Simplified Chinese input into natural colloquial Hong Kong Cantonese written in Traditional Chinese. Return plain text only."
	payload := map[string]any{
		"model": t.Model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": input},
		},
		"temperature": 0.3,
	}
	raw, _ := json.Marshal(payload)
	chatURL := t.BaseURL + "/v1/chat/completions"
	if strings.HasSuffix(t.BaseURL, "/v1") {
		chatURL = t.BaseURL + "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", errors.New("cantonese transform request failed")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("empty transform output")
	}
	out := strings.TrimSpace(parsed.Choices[0].Message.Content)
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimPrefix(out, "text")
	out = strings.TrimSuffix(out, "```")
	out = strings.TrimSpace(out)
	if out == "" {
		return "", errors.New("empty transform output")
	}
	return out, nil
}
