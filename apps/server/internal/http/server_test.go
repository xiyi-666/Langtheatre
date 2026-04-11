package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linguaquest/server/internal/graph"
	"github.com/linguaquest/server/internal/service"
	"github.com/linguaquest/server/internal/store"
)

func TestGraphQLRegisterAndLogin(t *testing.T) {
	svc := service.New(store.NewMemoryStore(), nil, nil, nil, "integration-secret")
	schema, err := graph.NewSchema(svc)
	if err != nil {
		t.Fatalf("schema init failed: %v", err)
	}
	mux := NewMux(schema, "integration-secret", nil)

	payload := map[string]any{
		"query": `mutation { register(email: "int@linguaquest.app", password: "pass1234") { accessToken } }`,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("accessToken")) {
		t.Fatalf("expected accessToken in response")
	}

	optReq := httptest.NewRequest(http.MethodOptions, "/graphql", nil)
	optReq.Header.Set("Origin", "http://localhost:5173")
	optReq.Header.Set("Access-Control-Request-Method", "POST")
	optRes := httptest.NewRecorder()
	mux.ServeHTTP(optRes, optReq)
	if optRes.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", optRes.Code)
	}
}
