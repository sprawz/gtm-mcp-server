package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const maxVisitors = 10000

// RateLimiter provides per-IP rate limiting for HTTP endpoints.
type RateLimiter struct {
	mu         sync.Mutex
	visitors   map[string]*visitor
	rate       rate.Limit
	burst      int
	done       chan struct{}
	trustProxy bool
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter with the given requests per second and burst.
// trustProxy controls whether X-Forwarded-For is used for IP extraction.
// Set to true only when the server is behind a trusted reverse proxy (e.g. Caddy).
func NewRateLimiter(rps float64, burst int, trustProxy bool) *RateLimiter {
	rl := &RateLimiter{
		visitors:   make(map[string]*visitor),
		rate:       rate.Limit(rps),
		burst:      burst,
		done:       make(chan struct{}),
		trustProxy: trustProxy,
	}
	go rl.cleanup()
	return rl
}

// Close stops the cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.done)
}

func (rl *RateLimiter) getVisitor(ip string) (*rate.Limiter, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		if len(rl.visitors) >= maxVisitors {
			return nil, false
		}
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter, true
	}
	v.lastSeen = time.Now()
	return v.limiter, true
}

// cleanup removes stale visitors every minute.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// extractClientIP returns the client IP from the request.
// When trustProxy is true, the rightmost IP from X-Forwarded-For is used.
// When false, only RemoteAddr is used to prevent spoofing.
//
// The rightmost entry, not the leftmost: a reverse proxy appends the peer
// address to whatever X-Forwarded-For the client already sent (nginx's
// $proxy_add_x_forwarded_for), so every entry left of the last one is
// attacker-controlled. Keying on one would let a client send a fresh value per
// request and get a fresh bucket each time, defeating the limit entirely. The
// last entry is the one our own proxy wrote.
//
// This assumes exactly one trusted proxy hop in front of the server, which is
// the deployment. A second proxy would mean skipping that many entries.
func extractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Joining the header lines rather than reading the first one: a proxy
		// that appends a line instead of rewriting one leaves the client's own
		// line first, and RFC 7230 makes repeated field lines equivalent to
		// one comma-joined line anyway.
		if forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ","); forwarded != "" {
			hops := strings.Split(forwarded, ",")
			for i := len(hops) - 1; i >= 0; i-- {
				if hop := strings.TrimSpace(hops[i]); hop != "" {
					return hop
				}
			}
		}
	}
	// RemoteAddr is "ip:port"; strip the port so limiting is per-IP,
	// not per-connection.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func rateLimitReject(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"rate_limit_exceeded","error_description":"Too many requests. Please retry later."}`))
}

// Middleware returns an HTTP middleware that rate limits by client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r, rl.trustProxy)

		limiter, ok := rl.getVisitor(ip)
		if !ok || !limiter.Allow() {
			rateLimitReject(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc wraps an http.HandlerFunc with rate limiting.
func (rl *RateLimiter) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r, rl.trustProxy)

		limiter, ok := rl.getVisitor(ip)
		if !ok || !limiter.Allow() {
			rateLimitReject(w)
			return
		}

		next(w, r)
	}
}

// MaxBytesMiddleware wraps a handler with a request body size limit.
func MaxBytesMiddleware(maxBytes int64, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next(w, r)
	}
}
