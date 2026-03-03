package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides IP-based rate limiting for HTTP requests.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter with the specified rate and burst.
// The rate parameter specifies the number of tokens added per second.
// The burst parameter specifies the maximum number of tokens that can be consumed at once.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	// Clean up old entries periodically
	go rl.cleanupVisitors()
	return rl
}

// getVisitor retrieves the rate limiter for the given IP address.
// If no limiter exists for the IP, a new one is created.
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// Limit returns middleware that enforces rate limiting per IP address.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.extractIP(r)

		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}` + "\n"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP extracts the client IP address from the request.
// It checks X-Forwarded-For and X-Real-IP headers first for proxied requests.
func (rl *RateLimiter) extractIP(r *http.Request) string {
	// Check for forwarded headers (used when behind proxy/load balancer)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs; take the first one (client IP)
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// cleanupVisitors removes visitors that haven't been seen for 3 minutes.
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(time.Minute)
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
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine. Call this when the rate limiter is no longer needed.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// VisitorCount returns the current number of tracked visitors.
// This is primarily useful for testing and monitoring.
func (rl *RateLimiter) VisitorCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return len(rl.visitors)
}

// LoginRateLimiter returns a limiter for login attempts (5 per minute).
func LoginRateLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(12*time.Second), 5)
}

// RegisterRateLimiter returns a limiter for registration (3 per minute).
func RegisterRateLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(20*time.Second), 3)
}

// APIRateLimiter returns a general API limiter (60 per minute).
func APIRateLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(time.Second), 60)
}

// TFAVerifyRateLimiter returns a limiter for 2FA verification attempts (5 per minute).
func TFAVerifyRateLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(12*time.Second), 5)
}
