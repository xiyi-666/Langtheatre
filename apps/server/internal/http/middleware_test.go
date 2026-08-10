package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewInMemoryRateLimiter(t *testing.T) {
	rl := NewInMemoryRateLimiter(5, time.Minute)
	if rl == nil {
		t.Fatal("rate limiter should not be nil")
	}
	if rl.limit != 5 {
		t.Errorf("limit = %d, want 5", rl.limit)
	}
}

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter(3, time.Minute)
	for i := range 3 {
		if !rl.Allow("client-1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow("client-1") {
		t.Error("4th request should be rate limited")
	}
}

func TestRateLimiter_IndependentClients(t *testing.T) {
	rl := NewInMemoryRateLimiter(2, time.Minute)
	rl.Allow("a")
	rl.Allow("a")
	if rl.Allow("a") {
		t.Error("client 'a' should be limited after 2 requests")
	}
	if !rl.Allow("b") {
		t.Error("client 'b' should not be limited")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewInMemoryRateLimiter(1, 50*time.Millisecond)
	if !rl.Allow("c") {
		t.Error("first request should be allowed")
	}
	if rl.Allow("c") {
		t.Error("second request should be rate limited")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.Allow("c") {
		t.Error("request after window expiry should be allowed")
	}
}

func TestWrapWithBaseMiddleware_SetsRequestID(t *testing.T) {
	handler := WrapWithBaseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	rid := rec.Header().Get("X-Request-ID")
	if rid == "" {
		t.Error("X-Request-ID header should be set")
	}
}

func TestWrapWithBaseMiddleware_RateLimits(t *testing.T) {
	handler := WrapWithBaseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send 181 requests (limit is 180/min)
	var lastCode int
	for range 181 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exceeding limit, got %d", lastCode)
	}
}
