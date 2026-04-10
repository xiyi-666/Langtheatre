package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

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
