package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCantoneseTransformer_DefaultBaseURL(t *testing.T) {
	ct := NewCantoneseTransformer("key", "", "")
	if ct.BaseURL != "http://43.172.5.210:3000/v1" {
		t.Errorf("BaseURL = %q, want default", ct.BaseURL)
	}
	if ct.Model != "gpt-5.4" {
		t.Errorf("Model = %q, want gpt-5.4", ct.Model)
	}
}

func TestNewCantoneseTransformer_CustomValues(t *testing.T) {
	ct := NewCantoneseTransformer("my-key", "gpt-4o", "https://api.example.com/v1/")
	if ct.APIKey != "my-key" {
		t.Errorf("APIKey = %q", ct.APIKey)
	}
	if ct.Model != "gpt-4o" {
		t.Errorf("Model = %q", ct.Model)
	}
	if ct.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q (trailing slash should be trimmed)", ct.BaseURL)
	}
}

func TestTransformCantonese_EmptyAPIKey(t *testing.T) {
	ct := NewCantoneseTransformer("", "gpt-4o", "https://api.example.com")
	_, err := ct.TransformCantonese(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if err.Error() != "transformer api key not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTransformCantonese_EmptyText(t *testing.T) {
	ct := NewCantoneseTransformer("key", "", "")
	_, err := ct.TransformCantonese(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if err.Error() != "empty text" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTransformCantonese_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "轉換後嘅文字"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ct := NewCantoneseTransformer("test-key", "gpt-4o", server.URL)
	result, err := ct.TransformCantonese(context.Background(), "你好世界")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "轉換後嘅文字" {
		t.Errorf("result = %q, want %q", result, "轉換後嘅文字")
	}
}

func TestTransformCantonese_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ct := NewCantoneseTransformer("key", "", server.URL)
	_, err := ct.TransformCantonese(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestTransformCantonese_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer server.Close()

	ct := NewCantoneseTransformer("key", "", server.URL)
	_, err := ct.TransformCantonese(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestTransformCantonese_StripsCodeFences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "```text\n你好\n```"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ct := NewCantoneseTransformer("key", "", server.URL)
	result, err := ct.TransformCantonese(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "你好" {
		t.Errorf("result = %q, expected code fences to be stripped", result)
	}
}

func TestTransformCantonese_URLConstruction(t *testing.T) {
	// When BaseURL ends with /v1, chat endpoint should be BaseURL + /chat/completions
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ct := NewCantoneseTransformer("key", "", server.URL+"/v1")
	ct.TransformCantonese(context.Background(), "test")
	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", gotPath)
	}
}
