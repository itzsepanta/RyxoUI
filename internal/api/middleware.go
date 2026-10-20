package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ryxoui/ryxoui/internal/auth"
	"github.com/ryxoui/ryxoui/internal/authz"
)

// contextKey is the type of the keys used for request scoped values.
type contextKey string

const (
	contextKeyRequestID contextKey = "requestID"
	contextKeyPrincipal contextKey = "principal"
)

// RequestIDHeader carries the correlation identifier.
const RequestIDHeader = "X-Request-Id"

// Principal is the authenticated caller of a request.
type Principal struct {
	Operator  auth.Operator
	Role      authz.Role
	Scopes    []string
	SessionID string
	TokenID   string
	viaToken  bool
}

// HasScope reports whether the principal may perform the action.
func (p *Principal) HasScope(scope authz.Scope) bool {
	if p == nil {
		return false
	}
	return authz.Matches(p.Scopes, scope)
}

// IsToken reports whether the request was authenticated with an API token.
func (p *Principal) IsToken() bool { return p != nil && p.viaToken }

// requestID returns the correlation identifier of a request.
func requestID(ctx context.Context) string {
	value, _ := ctx.Value(contextKeyRequestID).(string)
	return value
}

// principal returns the authenticated caller of a request.
func principal(ctx context.Context) *Principal {
	value, _ := ctx.Value(contextKeyPrincipal).(*Principal)
	return value
}

// withRequestID attaches a correlation identifier to every request.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withRecover turns a panic into a 500 response.
func withRecover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic recovered", "path", r.URL.Path, "panic", p)
					writeError(w, http.StatusInternalServerError, CodeInternal, "internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// withLogging writes one structured line per request.
func withLogging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestID(r.Context()),
				"remote", clientIP(r),
			)
		})
	}
}

// withSecurityHeaders applies the baseline response headers.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

// statusRecorder remembers the status code written to a response.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

// WriteHeader records the status code.
func (r *statusRecorder) WriteHeader(status int) {
	if !r.written {
		r.status = status
		r.written = true
	}
	r.ResponseWriter.WriteHeader(status)
}

// Write records an implicit 200.
func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

// clientIP returns the address of the caller, honouring the proxy headers
// only for requests that come from a trusted proxy.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiter is a small per key rate limiter.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   int
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(rate float64, burst int) *limiter {
	if rate <= 0 {
		return nil
	}
	return &limiter{buckets: map[string]*bucket{}, rate: rate, burst: burst}
}

// allow reports whether a request from key may proceed.
func (l *limiter) allow(key string, now time.Time) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.burst), last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// withRateLimit rejects requests that arrive too quickly.
func withRateLimit(l *limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l != nil && !l.allow(clientIP(r), time.Now()) {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, CodeRateLimited, "too many requests", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newRequestID returns a random correlation identifier.
func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}

// bearerToken extracts the token from an Authorization header.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
