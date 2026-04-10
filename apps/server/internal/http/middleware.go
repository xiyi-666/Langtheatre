package httpserver

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type InMemoryRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	cleanup time.Time
}

func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		hits:   map[string][]time.Time{},
		limit:  limit,
		window: window,
	}
}

func (r *InMemoryRateLimiter) Allow(clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.cleanup) > r.window {
		r.cleanup = now
		for key, ts := range r.hits {
			valid := make([]time.Time, 0, len(ts))
			for _, item := range ts {
				if now.Sub(item) <= r.window {
					valid = append(valid, item)
				}
			}
			if len(valid) == 0 {
				delete(r.hits, key)
			} else {
				r.hits[key] = valid
			}
		}
	}
	list := r.hits[clientID]
	valid := make([]time.Time, 0, len(list)+1)
	for _, item := range list {
		if now.Sub(item) <= r.window {
			valid = append(valid, item)
		}
	}
	if len(valid) >= r.limit {
		r.hits[clientID] = valid
		return false
	}
	valid = append(valid, now)
	r.hits[clientID] = valid
	return true
}

func WrapWithBaseMiddleware(next http.Handler) http.Handler {
	limiter := NewInMemoryRateLimiter(180, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.RemoteAddr
		if !limiter.Allow(clientID) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		requestID := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		start := time.Now()
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Printf("request_id=%s method=%s path=%s duration_ms=%d", requestID, r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}
