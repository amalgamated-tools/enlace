package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides IP-based rate limiting for HTTP requests.
type RateLimiter struct {
	visitors       map[string]*visitor
	mu             sync.RWMutex
	rate           rate.Limit
	burst          int
	stopCh         chan struct{}
	trustedProxies []*net.IPNet
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter with the specified rate and burst.
// The rate parameter specifies the number of tokens added per second.
// The burst parameter specifies the maximum number of tokens that can be consumed at once.
// trustedProxyCIDRs is an optional list of CIDR strings (e.g. "10.0.0.0/8") whose
// requests are allowed to supply X-Forwarded-For / X-Real-IP headers that will be
// trusted for client-IP extraction. Requests arriving from any other address always
// use RemoteAddr, preventing header-spoofing by untrusted clients.
func NewRateLimiter(r rate.Limit, burst int, trustedProxyCIDRs ...string) *RateLimiter {
	rl := &RateLimiter{
		visitors:       make(map[string]*visitor),
		rate:           r,
		burst:          burst,
		stopCh:         make(chan struct{}),
		trustedProxies: parseCIDRs(trustedProxyCIDRs),
	}
	// Clean up old entries periodically
	go rl.cleanupVisitors()
	return rl
}

// parseCIDRs parses a slice of CIDR strings into *net.IPNet values.
// Invalid entries are logged as warnings and skipped.
func parseCIDRs(cidrs []string) []*net.IPNet {
	var networks []*net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("invalid trusted proxy CIDR, skipping", "cidr", cidr, "error", err)
			continue
		}
		networks = append(networks, network)
	}
	return networks
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
// Forwarded headers (X-Forwarded-For, X-Real-IP) are only trusted when
// the direct peer (RemoteAddr) belongs to the configured trusted-proxy list.
// The returned value is always a bare IP address with no port suffix.
func (rl *RateLimiter) extractIP(r *http.Request) string {
	// Resolve the direct peer address, stripping any port.
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr may already be a bare IP (no port); use it as-is.
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(strings.TrimSpace(remoteHost))

	// Only examine forwarded headers when the direct peer is a trusted proxy.
	if remoteIP != nil && rl.isTrustedProxy(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// X-Forwarded-For can contain multiple IPs; take the first one (client IP).
			clientIP := xff
			if i := strings.IndexByte(xff, ','); i >= 0 {
				clientIP = xff[:i]
			}
			clientIP = strings.TrimSpace(clientIP)
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}

		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	// Fall back to the direct peer address (no port).
	if remoteIP != nil {
		return remoteIP.String()
	}
	return r.RemoteAddr
}

// isTrustedProxy reports whether ip belongs to one of the configured trusted-proxy networks.
func (rl *RateLimiter) isTrustedProxy(ip net.IP) bool {
	for _, network := range rl.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
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
func LoginRateLimiter(trustedProxyCIDRs ...string) *RateLimiter {
	return NewRateLimiter(rate.Every(12*time.Second), 5, trustedProxyCIDRs...)
}

// RegisterRateLimiter returns a limiter for registration (3 per minute).
func RegisterRateLimiter(trustedProxyCIDRs ...string) *RateLimiter {
	return NewRateLimiter(rate.Every(20*time.Second), 3, trustedProxyCIDRs...)
}

// APIRateLimiter returns a general API limiter (60 per minute).
func APIRateLimiter(trustedProxyCIDRs ...string) *RateLimiter {
	return NewRateLimiter(rate.Every(time.Second), 60, trustedProxyCIDRs...)
}

// TFAVerifyRateLimiter returns a limiter for 2FA verification attempts (5 per minute).
func TFAVerifyRateLimiter(trustedProxyCIDRs ...string) *RateLimiter {
	return NewRateLimiter(rate.Every(12*time.Second), 5, trustedProxyCIDRs...)
}
