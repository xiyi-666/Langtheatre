package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/linguaquest/server/internal/analytics"
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

type PaymentNotifier interface {
	HandleEpayNotification(values url.Values) error
}

type MuxOptions struct {
	Security            SecurityOptions
	Analytics           *analytics.Reporter
	AnalyticsAdminToken string
	MediaDir            string
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

func NewMux(schema graphql.Schema, jwtSecret string, paymentNotifier PaymentNotifier, healthFunc func(context.Context) HealthResult, optionValues ...SecurityOptions) *http.ServeMux {
	security := SecurityOptions{}
	if len(optionValues) > 0 {
		security = optionValues[0]
	}
	return NewMuxWithOptions(schema, jwtSecret, paymentNotifier, healthFunc, MuxOptions{Security: security})
}

func NewMuxWithOptions(schema graphql.Schema, jwtSecret string, paymentNotifier PaymentNotifier, healthFunc func(context.Context) HealthResult, muxOptions MuxOptions) *http.ServeMux {
	security := muxOptions.Security.normalized()
	authLimiter := NewInMemoryRateLimiter(security.AuthRateLimitPerMinute, time.Minute)
	aiLimiter := NewInMemoryRateLimiter(security.AIRequestRateLimitPerMinute, time.Minute)
	mux := http.NewServeMux()
	mediaDir := strings.TrimSpace(muxOptions.MediaDir)
	if mediaDir == "" {
		mediaDir = "media"
	}
	mediaHandler := http.StripPrefix("/media/", http.FileServer(http.Dir(mediaDir)))
	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "仅支持 GET 和 HEAD 请求", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		if isInlineAudioPath(r.URL.Path) {
			w.Header().Set("Content-Disposition", "inline")
		}
		mediaHandler.ServeHTTP(w, r)
	})
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
	if paymentNotifier != nil {
		mux.HandleFunc("/payments/easypay/notify", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodPost {
				http.Error(w, "only GET and POST are supported", http.StatusMethodNotAllowed)
				return
			}
			if paymentNotifier == nil {
				http.Error(w, "payment is unavailable", http.StatusServiceUnavailable)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid payment callback", http.StatusBadRequest)
				return
			}
			if err := paymentNotifier.HandleEpayNotification(r.Form); err != nil {
				http.Error(w, "fail", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("success"))
		})
	}
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "仅支持 POST 请求", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, security.GraphQLMaxBodyBytes))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		var payload GraphQLRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.Query) > 64*1024 || strings.Count(payload.Query, "{") > 24 {
			http.Error(w, "graphql query is too complex", http.StatusBadRequest)
			return
		}
		if isSensitiveAuthOperation(payload.Query) && !authLimiter.Allow(clientIPFromRequest(r, security.TrustProxyHeaders)) {
			http.Error(w, "认证请求过于频繁，请 1 分钟后再试。", http.StatusTooManyRequests)
			return
		}
		if isAIRequestOperation(payload.Query) && !aiLimiter.Allow(clientIPFromRequest(r, security.TrustProxyHeaders)) {
			http.Error(w, "AI 请求过于频繁，请稍后再试。", http.StatusTooManyRequests)
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
	if muxOptions.Analytics != nil {
		mux.HandleFunc("/telemetry/event", func(w http.ResponseWriter, r *http.Request) {
			setCORSHeaders(w, r)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Method != http.MethodPost {
				http.Error(w, "only POST is supported", http.StatusMethodNotAllowed)
				return
			}
			ctx := withAuth(r.Context(), r.Header.Get("Authorization"), jwtSecret)
			if userID, _ := ctx.Value(graph.UserIDKey).(string); userID == "" {
				http.Error(w, "未授权，请重新登录。", http.StatusUnauthorized)
				return
			}
			defer r.Body.Close()
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
			if err != nil {
				http.Error(w, "invalid telemetry event", http.StatusBadRequest)
				return
			}
			var event struct {
				Category string `json:"category"`
				Name     string `json:"name"`
			}
			if err = json.Unmarshal(body, &event); err != nil || !isAllowedTelemetryEvent(event.Category, event.Name) {
				http.Error(w, "invalid telemetry event", http.StatusBadRequest)
				return
			}
			event.Category = strings.ToUpper(strings.TrimSpace(event.Category))
			event.Name = strings.TrimSpace(event.Name)
			muxOptions.Analytics.RecordProductMetric(event.Category, event.Name)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	if muxOptions.Analytics != nil && strings.TrimSpace(muxOptions.AnalyticsAdminToken) != "" {
		mux.HandleFunc("/internal/analytics/daily", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "仅支持 GET 请求", http.StatusMethodNotAllowed)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Analytics-Token")), []byte(muxOptions.AnalyticsAdminToken)) != 1 {
				http.Error(w, "未授权，请重新登录。", http.StatusUnauthorized)
				return
			}
			fromDay, toDay, err := analyticsDateRange(muxOptions.Analytics.CurrentDay(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			report, err := muxOptions.Analytics.DailyReport(fromDay, toDay)
			if err != nil {
				http.Error(w, "failed to load analytics", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(report)
		})
	}
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
		if err := validatePublicURL(r.Context(), target); err != nil {
			http.Error(w, "upstream url is not allowed", http.StatusForbidden)
			return
		}

		client := newSafeMediaClient()
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
		if resp.ContentLength > security.MediaProxyMaxBytes {
			http.Error(w, "upstream media is too large", http.StatusRequestEntityTooLarge)
			return
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, security.MediaProxyMaxBytes+1))
		if err != nil {
			http.Error(w, "failed to read upstream media", http.StatusBadGateway)
			return
		}
		if int64(len(body)) > security.MediaProxyMaxBytes {
			http.Error(w, "upstream media is too large", http.StatusRequestEntityTooLarge)
			return
		}

		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("Cache-Control", "public, max-age=300")
		if _, err = w.Write(body); err != nil {
			http.Error(w, "failed to stream media", http.StatusBadGateway)
			return
		}
	})
	return mux
}

func isSensitiveAuthOperation(query string) bool {
	query = strings.ToLower(query)
	for _, operation := range []string{
		"logincandidates", "register", "login", "requestemailverification",
		"verifyemail", "requestpasswordreset", "resetpassword", "requestusernamerecovery",
	} {
		if strings.Contains(query, operation) {
			return true
		}
	}
	return false
}

func isInlineAudioPath(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, extension := range []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".webm"} {
		if strings.HasSuffix(lower, extension) {
			return true
		}
	}
	return false
}

func isAIRequestOperation(query string) bool {
	query = strings.ToLower(query)
	for _, operation := range []string{
		"generatetheater", "generatereading", "createvoiceprofile", "startwritingsession",
		"submitwritingsession", "startroleplay", "submitroleplayreply", "submitroleplayaudio", "endroleplay",
	} {
		if strings.Contains(query, operation) {
			return true
		}
	}
	return false
}

func isAllowedTelemetryEvent(category string, name string) bool {
	category = strings.ToUpper(strings.TrimSpace(category))
	if category != analytics.MetricCategoryFeature && category != analytics.MetricCategoryClick {
		return false
	}
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 64 {
		return false
	}
	for _, char := range name {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func analyticsDateRange(today string, fromValue string, toValue string) (string, string, error) {
	fromDay := strings.TrimSpace(fromValue)
	toDay := strings.TrimSpace(toValue)
	if toDay == "" {
		toDay = today
	}
	if fromDay == "" {
		fromDay = toDay
	}
	from, err := time.Parse("2006-01-02", fromDay)
	if err != nil {
		return "", "", errors.New("from must use YYYY-MM-DD")
	}
	to, err := time.Parse("2006-01-02", toDay)
	if err != nil {
		return "", "", errors.New("to must use YYYY-MM-DD")
	}
	if from.After(to) || to.Sub(from) > 366*24*time.Hour {
		return "", "", errors.New("date range must be ordered and within 366 days")
	}
	return fromDay, toDay, nil
}

func newSafeMediaClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ip, err := resolvePublicHost(ctx, host)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	return &http.Client{
		Timeout:   25 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validatePublicURL(req.Context(), req.URL)
		},
	}
}

func validatePublicURL(ctx context.Context, target *url.URL) error {
	if target == nil || target.Hostname() == "" {
		return errors.New("missing upstream host")
	}
	_, err := resolvePublicHost(ctx, target.Hostname())
	return err
}

func resolvePublicHost(ctx context.Context, host string) (net.IP, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, errors.New("local host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, errors.New("private address is not allowed")
		}
		return ip, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("unable to resolve upstream host")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return nil, errors.New("private address is not allowed")
		}
	}
	return addresses[0].IP, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return !(ipv4[0] == 0 || ipv4[0] >= 224 || (ipv4[0] == 100 && ipv4[1]&0xc0 == 0x40) || (ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 0) || (ipv4[0] == 198 && (ipv4[1] == 18 || ipv4[1] == 19)) || (ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 2) || (ipv4[0] == 198 && ipv4[1] == 51 && ipv4[2] == 100) || (ipv4[0] == 203 && ipv4[1] == 0 && ipv4[2] == 113))
	}
	return true
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
