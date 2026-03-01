package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/amalgamated-tools/sharer/internal/middleware"
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
	for i := 0; i < 5; i++ {
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
	for i := 0; i < 2; i++ {
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
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with X-Forwarded-For header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345" // Local proxy
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Second request with same X-Forwarded-For should be blocked
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}

func TestRateLimiter_ExtractsIPFromXRealIP(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
	defer rl.Stop()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Limit(testHandler)

	// Request with X-Real-IP header
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

func TestRateLimiter_XForwardedForTakesPrecedence(t *testing.T) {
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
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

	for i := 0; i < numRequests; i++ {
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
	for i := 0; i < 5; i++ {
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
	for i := 0; i < 3; i++ {
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
	for i := 0; i < 60; i++ {
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
	rl := middleware.NewRateLimiter(rate.Every(time.Second), 1)
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
	for i := 0; i < 3; i++ {
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
