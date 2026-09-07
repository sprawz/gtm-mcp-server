package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 20, false)

	if rl == nil {
		t.Fatal("expected non-nil RateLimiter")
	}

	if rl.rate != 10 {
		t.Errorf("expected rate 10, got %v", rl.rate)
	}

	if rl.burst != 20 {
		t.Errorf("expected burst 20, got %v", rl.burst)
	}

	if rl.visitors == nil {
		t.Error("expected non-nil visitors map")
	}
}

func TestRateLimiter_BasicAllowDeny(t *testing.T) {
	// Create rate limiter: 1 request per second, burst of 2
	rl := NewRateLimiter(1, 2, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request should succeed (within burst)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", w2.Code)
	}

	// Third request should be rate limited (burst exhausted)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.1:1234"
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("third request: expected status 429, got %d", w3.Code)
	}

	// Verify response headers and body for rate limited request
	if w3.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w3.Header().Get("Content-Type"))
	}

	if w3.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After 1, got %s", w3.Header().Get("Retry-After"))
	}

	expectedBody := `{"error":"rate_limit_exceeded","error_description":"Too many requests. Please retry later."}`
	if strings.TrimSpace(w3.Body.String()) != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w3.Body.String())
	}
}

func TestRateLimiter_PerIPIsolation(t *testing.T) {
	// Create rate limiter: 1 request per second, burst of 1
	rl := NewRateLimiter(1, 1, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// IP 1 - first request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("IP1 first request: expected status 200, got %d", w1.Code)
	}

	// IP 1 - second request (should be rate limited)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 second request: expected status 429, got %d", w2.Code)
	}

	// IP 2 - first request (should succeed, different IP)
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.2:5678"
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("IP2 first request: expected status 200, got %d", w3.Code)
	}
}

func TestRateLimiter_XForwardedFor(t *testing.T) {
	// Create rate limiter: 1 request per second, burst of 1
	rl := NewRateLimiter(1, 1, true)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request with X-Forwarded-For
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "10.0.0.1:1234" // Proxy IP
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request with same X-Forwarded-For (should be rate limited)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "10.0.0.2:5678"                 // Different proxy IP
	req2.Header.Set("X-Forwarded-For", "203.0.113.1") // Same client IP
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status 429, got %d", w2.Code)
	}
}

func TestRateLimiter_BurstHandling(t *testing.T) {
	// Create rate limiter: 10 requests per second, burst of 5
	rl := NewRateLimiter(10, 5, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.1:1234"

	// Should be able to make 5 requests immediately (burst)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("burst request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// 6th request should be rate limited
	req6 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req6.RemoteAddr = ip
	w6 := httptest.NewRecorder()
	handler.ServeHTTP(w6, req6)

	if w6.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: expected status 429, got %d", w6.Code)
	}
}

func TestRateLimiter_MiddlewareFunc(t *testing.T) {
	// Test the MiddlewareFunc variant
	rl := NewRateLimiter(1, 1, false)

	handler := rl.MiddlewareFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status 429, got %d", w2.Code)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to the rate limiter
	rl := NewRateLimiter(100, 200, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				// Each goroutine uses a different IP
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.RemoteAddr = "192.168.1." + string(rune(id+1)) + ":1234"
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// With high rate and burst, most requests should succeed
				if w.Code != http.StatusOK && w.Code != http.StatusTooManyRequests {
					t.Errorf("unexpected status code: %d", w.Code)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestRateLimiter_ConcurrentSameIP(t *testing.T) {
	// Test concurrent requests from the same IP
	rl := NewRateLimiter(10, 20, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 30
	const ip = "192.168.1.1:1234"

	var wg sync.WaitGroup
	var successCount, rateLimitedCount int
	var mu sync.Mutex

	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = ip
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			mu.Lock()
			if w.Code == http.StatusOK {
				successCount++
			} else if w.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// We expect some requests to succeed (up to burst) and some to be rate limited
	if successCount == 0 {
		t.Error("expected some successful requests")
	}
	if rateLimitedCount == 0 {
		t.Error("expected some rate limited requests")
	}
	if successCount+rateLimitedCount != numGoroutines {
		t.Errorf("expected %d total requests, got %d", numGoroutines, successCount+rateLimitedCount)
	}
}

func TestMaxBytesMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		maxBytes      int64
		bodySize      int
		expectSuccess bool
	}{
		{
			name:          "body within limit",
			maxBytes:      1024,
			bodySize:      512,
			expectSuccess: true,
		},
		{
			name:          "body at limit",
			maxBytes:      1024,
			bodySize:      1024,
			expectSuccess: true,
		},
		{
			name:          "body exceeds limit",
			maxBytes:      1024,
			bodySize:      2048,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := MaxBytesMiddleware(tt.maxBytes, func(w http.ResponseWriter, r *http.Request) {
				// Try to read the entire body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					// Error reading body (likely exceeds limit)
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}

				// Verify body was read correctly
				if len(body) != tt.bodySize {
					t.Errorf("expected body size %d, got %d", tt.bodySize, len(body))
				}

				w.WriteHeader(http.StatusOK)
			})

			// Create request with body
			body := make([]byte, tt.bodySize)
			for i := range body {
				body[i] = 'a'
			}

			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(string(body)))
			w := httptest.NewRecorder()

			handler(w, req)

			if tt.expectSuccess {
				if w.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", w.Code)
				}
			} else {
				if w.Code == http.StatusOK {
					t.Error("expected request to fail due to body size limit")
				}
			}
		})
	}
}

func TestMaxBytesMiddleware_NilBody(t *testing.T) {
	// Test that middleware doesn't panic with nil body
	handler := MaxBytesMiddleware(1024, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRateLimiter_RecoveryAfterWait(t *testing.T) {
	// Test that rate limiter allows requests again after waiting
	rl := NewRateLimiter(10, 2, false) // 10 req/sec, burst of 2

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.1:1234"

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("burst request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Next request should be rate limited
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = ip
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w3.Code)
	}

	// Wait for rate limit to recover (100ms should allow 1 request at 10 req/sec)
	time.Sleep(150 * time.Millisecond)

	// Should be able to make request again
	req4 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req4.RemoteAddr = ip
	w4 := httptest.NewRecorder()
	handler.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("after recovery: expected status 200, got %d", w4.Code)
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(1, 1, false)

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create multiple IPs and verify they're independent
	ips := []string{
		"192.168.1.1:1234",
		"192.168.1.2:1234",
		"192.168.1.3:1234",
		"192.168.1.4:1234",
		"192.168.1.5:1234",
	}

	// Each IP should be able to make 1 request successfully
	for i, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP %d first request: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Each IP's second request should be rate limited
	for i, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("IP %d second request: expected status 429, got %d", i+1, w.Code)
		}
	}
}

// The proxy appends the peer address to whatever X-Forwarded-For the client
// sent, so the leftmost entry is attacker-controlled. A client rotating that
// value must not get a fresh bucket per request.
func TestRateLimiter_SpoofedLeftmostXFFCannotEscapeTheBucket(t *testing.T) {
	rl := NewRateLimiter(1, 1, true)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Each request carries a different spoofed leftmost entry; the rightmost
	// entry is the one our own proxy wrote and is the same every time.
	send := func(spoofed string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", spoofed+", 203.0.113.7")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	if code := send("198.51.100.1"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	if code := send("198.51.100.2"); code != http.StatusTooManyRequests {
		t.Errorf("second request with a rotated leftmost entry: expected status 429, got %d", code)
	}
}

// A header that ends in a separator must not collapse the bucket key to the
// empty string, which would put unrelated clients in one shared bucket.
func TestRateLimiter_TrailingSeparatorInXFFKeepsTheProxyEntry(t *testing.T) {
	rl := NewRateLimiter(1, 1, true)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(xff, remoteAddr string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Forwarded-For", xff)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// Spends the single token held by 203.0.113.8.
	if code := send("203.0.113.8, ", "10.0.0.1:1234"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	// A different client, whose header also ends in a separator, must have its
	// own bucket rather than sharing an empty-string key with the first.
	if code := send("203.0.113.9, ", "10.0.0.1:5678"); code != http.StatusOK {
		t.Errorf("unrelated client with a trailing separator: expected status 200, got %d", code)
	}
}

// Pinning test: without TRUST_PROXY the header is client-supplied noise and
// must not affect the bucket. Passes before and after the rightmost fix.
func TestRateLimiter_XForwardedForIgnoredWhenProxyNotTrusted(t *testing.T) {
	rl := NewRateLimiter(1, 1, false)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(xff string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		req.Header.Set("X-Forwarded-For", xff)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	if code := send("198.51.100.1"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	if code := send("198.51.100.2"); code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status 429, got %d", code)
	}
}

// A proxy that appends a second X-Forwarded-For header line rather than
// rewriting one leaves the attacker's value in the first line. Reading only
// the first line would hand the bucket key straight back to the client.
func TestRateLimiter_SpoofedFirstXFFHeaderLineCannotEscapeTheBucket(t *testing.T) {
	rl := NewRateLimiter(1, 1, true)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(spoofed string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Add("X-Forwarded-For", spoofed) // the client's own line
		req.Header.Add("X-Forwarded-For", "203.0.113.7")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	if code := send("198.51.100.1"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	if code := send("198.51.100.2"); code != http.StatusTooManyRequests {
		t.Errorf("second request with a rotated first header line: expected status 429, got %d", code)
	}
}

// A client may legitimately send a multi-value X-Forwarded-For of its own, so
// after the proxy appends there can be three or more entries. Only the last is
// trustworthy; the ones the client supplied must not reach the bucket key.
func TestRateLimiter_RotatingMiddleXFFEntryCannotEscapeTheBucket(t *testing.T) {
	rl := NewRateLimiter(1, 1, true)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(spoofed string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", "9.9.9.9, "+spoofed+", 203.0.113.7")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	if code := send("198.51.100.1"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	if code := send("198.51.100.2"); code != http.StatusTooManyRequests {
		t.Errorf("second request with a rotated middle entry: expected status 429, got %d", code)
	}
}

// Without a trusted proxy the key comes from RemoteAddr, which carries an
// ephemeral source port. Keying on the port would make the bucket
// per-connection, so a new connection per request would evade the limit.
func TestRateLimiter_SamePortlessIPSharesABucketAcrossConnections(t *testing.T) {
	rl := NewRateLimiter(1, 1, false)
	defer rl.Close()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	if code := send("203.0.113.9:1111"); code != http.StatusOK {
		t.Fatalf("first request: expected status 200, got %d", code)
	}
	if code := send("203.0.113.9:2222"); code != http.StatusTooManyRequests {
		t.Errorf("second request from a new source port: expected status 429, got %d", code)
	}
}
