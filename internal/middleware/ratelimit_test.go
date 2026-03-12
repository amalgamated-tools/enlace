package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/amalgamated-tools/enlace/internal/middleware"
	"golang.org/x/time/rate"
)

func TestNewRateLimiter(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 10)
	defer rl.Stop()

	if rl == nil {
		t.Fatal("expected rate limiter to be created")
	}
}

func TestRateLimiter_AllowsRequestsWithinLimit(t *testing.T) {
	// Allow 5 requests with burst of 5
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 5)
	defer rl.Stop()

	handlerCalled := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled++
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// First 5 requests should succeed (burst)
	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	if handlerCalled != 5 {
		t.Errorf("expected handler to be called 5 times, got %d", handlerCalled)
	}
}

func TestRateLimiter_BlocksExcessiveRequests(t *testing.T) {
	// Allow 2 requests with burst of 2
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 2)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Make requests from same IP
	ip := "192.168.1.1:12345"

	// First 2 requests should succeed
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	if rec.Body.String() != `{"error":"rate limit exceeded"}`+"\n" {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestRateLimiter_DifferentIPsHaveSeparateLimits(t *testing.T) {
	// Allow 1 request with burst of 1
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request from IP 1 - should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("IP1 first request: expected status %d, got %d", http.StatusOK, rec1.Code)
	}

	// Request from IP 2 - should also succeed (separate limit)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("IP2 first request: expected status %d, got %d", http.StatusOK, rec2.Code)
	}

	// Second request from IP 1 - should be blocked
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 second request: expected status %d, got %d", http.StatusTooManyRequests, rec3.Code)
	}
}

func TestRateLimiter_ExtractsIPFromXForwardedFor(t *testing.T) {
	// Trust 127.0.0.1 so the forwarded header is honoured.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Requests arrive from a trusted proxy with a spoofed leftmost entry prepended
	// by the client. The limiter must use the last untrusted IP in the chain
	// (203.0.113.195), not the spoofed first value.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345" // trusted proxy
	req.Header.Set("X-Forwarded-For", "198.51.100.10, 203.0.113.195")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// The spoofed first value changes, but the real client IP at the end stays the
	// same, so this request must still be blocked.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "198.51.100.11, 203.0.113.195")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestRateLimiter_SkipsTrustedProxiesInXForwardedForChain(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32", "10.0.0.0/8")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 10.1.2.3")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("first request: expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// The same client IP through the same trusted proxy chain must be blocked on
	// a second request.
	reqRepeat := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqRepeat.RemoteAddr = "127.0.0.1:12345"
	reqRepeat.Header.Set("X-Forwarded-For", "203.0.113.195, 10.1.2.3")
	recRepeat := httptest.NewRecorder()
	handler.ServeHTTP(recRepeat, reqRepeat)

	if recRepeat.Code != http.StatusTooManyRequests {
		t.Errorf("repeat request: expected status %d, got %d", http.StatusTooManyRequests, recRepeat.Code)
	}

	// A different client behind the same trusted upstream proxy should receive a
	// different bucket. If the limiter incorrectly used the last entry, this would
	// be blocked as 10.1.2.3.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "198.51.100.44, 10.1.2.3")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("second request: expected status %d, got %d", http.StatusOK, rec2.Code)
	}
}

func TestRateLimiter_ExtractsIPFromXRealIP(t *testing.T) {
	// Trust 127.0.0.1 so the X-Real-IP header is honoured.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with X-Real-IP header from a trusted proxy
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.195")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Second request with same X-Real-IP should be blocked
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Real-IP", "203.0.113.195")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestRateLimiter_IgnoresTrustedProxyXRealIPFallback(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32", "10.0.0.0/8")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	makeRequest := func(xri string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "10.1.2.3")
		req.Header.Set("X-Real-IP", xri)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := makeRequest("10.1.2.3"); rec.Code != http.StatusOK {
		t.Errorf("first request: expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Rotating a trusted-proxy X-Real-IP value must not create a fresh bucket when
	// X-Forwarded-For contains only trusted hops and the limiter falls back to
	// RemoteAddr.
	if rec := makeRequest("10.1.2.4"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimiter_XForwardedForTakesPrecedence(t *testing.T) {
	// Trust 127.0.0.1 so forwarded headers are honoured.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with both headers - X-Forwarded-For should take precedence
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.100")
	req.Header.Set("X-Real-IP", "203.0.113.200")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Second request with X-Real-IP (different from X-Forwarded-For) should succeed
	// because X-Forwarded-For IP is different
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Real-IP", "203.0.113.200")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec2.Code)
	}
}

func TestRateLimiter_VisitorCount(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 10)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Initially no visitors
	if count := rl.VisitorCount(); count != 0 {
		t.Errorf("expected 0 visitors, got %d", count)
	}

	// Make request from IP 1
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if count := rl.VisitorCount(); count != 1 {
		t.Errorf("expected 1 visitor, got %d", count)
	}

	// Make request from IP 2
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if count := rl.VisitorCount(); count != 2 {
		t.Errorf("expected 2 visitors, got %d", count)
	}

	// Another request from IP 1 should not increase count
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if count := rl.VisitorCount(); count != 2 {
		t.Errorf("expected 2 visitors (no increase), got %d", count)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	// High burst to allow concurrent requests
	rl := middleware.NewRateLimiter(rate.Every(time.Millisecond), 100)
	defer rl.Stop()

	var successCount int
	var mu sync.Mutex

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		successCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Launch concurrent requests
	var wg sync.WaitGroup
	numRequests := 50

	for i := range numRequests {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			// Use different IPs to avoid rate limiting blocking the test
			req.RemoteAddr = "192.168.1." + string(rune(idx%255+1)) + ":12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}(i)
	}

	wg.Wait()

	// All requests should succeed since each has unique IP
	if successCount != numRequests {
		t.Errorf("expected %d successful requests, got %d", numRequests, successCount)
	}
}

func TestRateLimiter_ResponseContentType(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// First request succeeds
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Second request gets rate limited - check content type
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	contentType := rec2.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}
}

func TestLoginRateLimiter(t *testing.T) {
	rl := middleware.LoginRateLimiter()
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)
	ip := "192.168.1.1:12345"

	// Should allow 5 requests (burst)
	for i := range 5 {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// 6th request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRegisterRateLimiter(t *testing.T) {
	rl := middleware.RegisterRateLimiter()
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)
	ip := "192.168.1.1:12345"

	// Should allow 3 requests (burst)
	for i := range 3 {
		req := httptest.NewRequest(http.MethodPost, "/register", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// 4th request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestAPIRateLimiter(t *testing.T) {
	rl := middleware.APIRateLimiter()
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)
	ip := "192.168.1.1:12345"

	// Should allow 60 requests (burst)
	for i := range 60 {
		req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// 61st request should be blocked
	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 10)

	// Stop should not panic
	rl.Stop()

	// Calling Stop multiple times should be safe (select on closed channel)
	// Note: This would panic if Stop is called twice without proper handling
	// The current implementation closes the channel, so we don't call Stop again
}

func TestRateLimiter_EmptyXForwardedFor(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with empty X-Forwarded-For should fall back to RemoteAddr
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Second request from same RemoteAddr should be blocked
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	req2.Header.Set("X-Forwarded-For", "")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestRateLimiter_SingleIPInXForwardedFor(t *testing.T) {
	// Trust 127.0.0.1 so the forwarded header is honoured.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "127.0.0.1/32")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with single IP in X-Forwarded-For
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.195")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRateLimiter_ChainWithOtherMiddleware(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 5)
	defer rl.Stop()

	// Create a simple logging middleware
	var logCalls int
	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logCalls++
			next.ServeHTTP(w, r)
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: logging -> rate limit -> final handler
	handler := loggingMiddleware(rl.Limit(finalHandler))

	// Make requests
	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	// Logging middleware should have been called for each request
	if logCalls != 3 {
		t.Errorf("expected logging middleware to be called 3 times, got %d", logCalls)
	}
}

// TestRateLimiter_IgnoresSpoofedHeadersFromUntrustedProxy verifies that a client
// connecting directly (not through a trusted proxy) cannot spoof X-Forwarded-For or
// X-Real-IP to evade rate limiting.
func TestRateLimiter_IgnoresSpoofedHeadersFromUntrustedProxy(t *testing.T) {
	// No trusted proxies configured: headers are never trusted.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Client spoofs X-Forwarded-For to appear as a different IP on each request.
	// Without trusted-proxy validation the rate limiter would use the spoofed IP,
	// allowing the client to bypass the limit. With the fix it must use RemoteAddr.
	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "203.0.113.50:9000" // same real client every time
		// Rotate the spoofed IP to try to get a fresh bucket each request.
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
		req.Header.Set("X-Real-IP", fmt.Sprintf("10.0.0.%d", i+1))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if i == 0 {
			if rec.Code != http.StatusOK {
				t.Errorf("first request: expected status %d, got %d", http.StatusOK, rec.Code)
			}
		} else {
			// All subsequent requests must be blocked because RemoteAddr is the key.
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: expected status %d, got %d (header spoofing should be ignored)",
					i+1, http.StatusTooManyRequests, rec.Code)
			}
		}
	}
}

// TestRateLimiter_TrustedProxyCIDRRange verifies that a limiter configured with
// a CIDR range trusts headers from any address inside that range.
func TestRateLimiter_TrustedProxyCIDRRange(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "10.0.0.0/8")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Proxy at 10.1.2.3 is inside the trusted CIDR — header should be honoured.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.1.2.3:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("first request: expected %d, got %d", http.StatusOK, rec.Code)
	}

	// Same client IP from the same trusted proxy — should be rate-limited.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "10.1.2.3:8081"
	req2.Header.Set("X-Forwarded-For", "203.0.113.77")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

// TestRateLimiter_UntrustedProxyHeaderIgnored verifies that a request arriving from
// outside the trusted-proxy CIDR list is rate-limited by its own RemoteAddr even
// when it sets X-Forwarded-For.
func TestRateLimiter_UntrustedProxyHeaderIgnored(t *testing.T) {
	// Only trust the 10.0.0.0/8 range.
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1, "10.0.0.0/8")
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Requests come from 203.0.113.50 — outside trusted range.
	// They supply different X-Forwarded-For values to try to evade the limiter.
	makeReq := func(xff string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "203.0.113.50:1234"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := makeReq("1.2.3.4"); rec.Code != http.StatusOK {
		t.Errorf("first request: expected %d, got %d", http.StatusOK, rec.Code)
	}
	// Header changes, but RemoteAddr is the same — must still be blocked.
	if rec := makeReq("5.6.7.8"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request (different XFF, same RemoteAddr): expected %d, got %d",
			http.StatusTooManyRequests, rec.Code)
	}
}

// TestRateLimiter_RemoteAddrNormalization verifies that two connections from the same
// IP but different source ports are treated as the same visitor.
func TestRateLimiter_RemoteAddrNormalization(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// First connection from 192.0.2.1 — port 1111.
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.0.2.1:1111"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected %d, got %d", http.StatusOK, rec1.Code)
	}

	// Second connection from 192.0.2.1 — different port, same IP; must be blocked.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.0.2.1:2222"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request (different port, same IP): expected %d, got %d",
			http.StatusTooManyRequests, rec2.Code)
	}
}

// TestRateLimiter_RealIPMiddlewareUnderminesProtection is a regression test
// proving that chi's middleware.RealIP must NOT be placed before the rate
// limiter. When it is, clients can spoof X-Forwarded-For to bypass limits.
// The router must not include middleware.RealIP in its middleware stack.
func TestRateLimiter_RealIPMiddlewareUnderminesProtection(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// DANGEROUS: middleware.RealIP before rate limiter allows spoofing.
	// This half of the test documents the bypass so no one re-adds RealIP.
	rlDangerous := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rlDangerous.Stop()
	dangerousChain := chiMiddleware.RealIP(rlDangerous.Limit(handler))

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "203.0.113.50:9000"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
		rec := httptest.NewRecorder()
		dangerousChain.ServeHTTP(rec, req)

		// All requests succeed because middleware.RealIP rewrites RemoteAddr
		// to the spoofed IP, giving each request a fresh rate-limit bucket.
		if rec.Code != http.StatusOK {
			t.Errorf("dangerous chain request %d: expected %d, got %d — middleware.RealIP behavior may have changed",
				i+1, http.StatusOK, rec.Code)
		}
	}

	// SAFE: rate limiter alone (no middleware.RealIP) ignores spoofed headers.
	rlSafe := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rlSafe.Stop()
	safeChain := rlSafe.Limit(handler)

	for i := range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "203.0.113.50:9000"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i+1))
		rec := httptest.NewRecorder()
		safeChain.ServeHTTP(rec, req)

		if i == 0 {
			if rec.Code != http.StatusOK {
				t.Errorf("safe chain first request: expected %d, got %d", http.StatusOK, rec.Code)
			}
		} else {
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("safe chain request %d: expected %d, got %d — spoofed headers should be ignored",
					i+1, http.StatusTooManyRequests, rec.Code)
			}
		}
	}
}
