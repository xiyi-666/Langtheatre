package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/auth"
	"github.com/linguaquest/server/internal/graph"
)

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type HealthResult struct {
	OK        bool              `json:"ok"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func NewMux(schema graphql.Schema, jwtSecret string, healthFunc func(context.Context) HealthResult) *http.ServeMux {
	mux := http.NewServeMux()
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if healthFunc == nil {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(HealthResult{
				OK:        true,
				Timestamp: "",
				Checks: map[string]string{
					"postgres": "not_configured",
					"redis":    "not_configured",
				},
			})
			return
		}
		result := healthFunc(r.Context())
		if result.OK {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(result)
	}
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", healthHandler)
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "only POST is supported", http.StatusMethodNotAllowed)
			return
		}
		var payload GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx := withAuth(r.Context(), r.Header.Get("Authorization"), jwtSecret)
		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  payload.Query,
			VariableValues: payload.Variables,
			Context:        ctx,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("/media-proxy", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "only GET is supported", http.StatusMethodNotAllowed)
			return
		}

		rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
		if rawURL == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}
		target, err := url.Parse(rawURL)
		if err != nil || target == nil || target.Host == "" {
			http.Error(w, "invalid url", http.StatusBadRequest)
			return
		}
		if target.Scheme != "http" && target.Scheme != "https" {
			http.Error(w, "unsupported url scheme", http.StatusBadRequest)
			return
		}

		client := &http.Client{Timeout: 25 * time.Second}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
		if err != nil {
			http.Error(w, "failed to build upstream request", http.StatusBadGateway)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "failed to fetch upstream media", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			http.Error(w, "upstream media unavailable", http.StatusBadGateway)
			return
		}

		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		if contentLength := strings.TrimSpace(resp.Header.Get("Content-Length")); contentLength != "" {
			w.Header().Set("Content-Length", contentLength)
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		if _, err = io.Copy(w, resp.Body); err != nil {
			http.Error(w, "failed to stream media", http.StatusBadGateway)
			return
		}
	})
	return mux
}

func withAuth(ctx context.Context, authHeader string, jwtSecret string) context.Context {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return ctx
	}
	token := strings.TrimPrefix(authHeader, prefix)
	claims, err := auth.ParseAccessToken(jwtSecret, token)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, graph.UserIDKey, claims.UserID)
}
