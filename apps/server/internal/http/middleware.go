package httpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	clientIPKey  contextKey = "client_ip"
)

// SecurityOptions contains the public HTTP boundary controls. A reverse proxy
// header is deliberately ignored unless TrustProxyHeaders is explicitly enabled.
type SecurityOptions struct {
	GlobalRateLimitPerMinute    int
	AuthRateLimitPerMinute      int
	AIRequestRateLimitPerMinute int
	GraphQLMaxBodyBytes         int64
	MediaProxyMaxBytes          int64
	TrustProxyHeaders           bool
}

func (o SecurityOptions) normalized() SecurityOptions {
	if o.GlobalRateLimitPerMinute <= 0 {
		o.GlobalRateLimitPerMinute = 180
	}
	if o.AuthRateLimitPerMinute <= 0 {
		o.AuthRateLimitPerMinute = 12
	}
	if o.AIRequestRateLimitPerMinute <= 0 {
		o.AIRequestRateLimitPerMinute = 20
	}
	if o.GraphQLMaxBodyBytes <= 0 {
		o.GraphQLMaxBodyBytes = 16 * 1024 * 1024
	}
	if o.MediaProxyMaxBytes <= 0 {
		o.MediaProxyMaxBytes = 20 * 1024 * 1024
	}
	return o
}

type InMemoryRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	cleanup time.Time
}

func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	if limit <= 0 {
		limit = 1
	}
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

func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		for _, value := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
			if ip := strings.TrimSpace(value); net.ParseIP(ip) != nil {
				return ip
			}
		}
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	if ip := net.ParseIP(strings.TrimSpace(r.RemoteAddr)); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func clientIPFromRequest(r *http.Request, trustProxyHeaders bool) string {
	if value, ok := r.Context().Value(clientIPKey).(string); ok && value != "" {
		return value
	}
	return clientIP(r, trustProxyHeaders)
}

func WrapWithBaseMiddleware(next http.Handler, optionValues ...SecurityOptions) http.Handler {
	options := SecurityOptions{}
	if len(optionValues) > 0 {
		options = optionValues[0]
	}
	options = options.normalized()
	limiter := NewInMemoryRateLimiter(options.GlobalRateLimitPerMinute, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := clientIP(r, options.TrustProxyHeaders)
		if !limiter.Allow(clientID) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		requestID := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, clientIPKey, clientID)
		start := time.Now()
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Printf("request_id=%s method=%s path=%s duration_ms=%d", requestID, r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}
