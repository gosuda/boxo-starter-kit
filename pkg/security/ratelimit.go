package security

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality for HTTP endpoints
type RateLimiter struct {
	mu              sync.RWMutex
	limiters        map[string]*rate.Limiter
	rate            rate.Limit
	burst           int
	cleanupInterval time.Duration
	lastSeen        map[string]time.Time
}

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	RequestsPerSecond float64                    // Requests per second allowed
	BurstSize         int                        // Maximum burst size
	CleanupInterval   time.Duration              // How often to cleanup old entries
	KeyExtractor      func(*http.Request) string // Function to extract rate limit key
}

// DefaultRateLimitConfig returns sensible default configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 10.0,
		BurstSize:         20,
		CleanupInterval:   time.Hour,
		KeyExtractor:      extractClientIP,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*rate.Limiter),
		rate:            rate.Limit(config.RequestsPerSecond),
		burst:           config.BurstSize,
		cleanupInterval: config.CleanupInterval,
		lastSeen:        make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go rl.cleanupRoutine()

	return rl
}

// getLimiter returns the rate limiter for a specific key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
	}

	rl.lastSeen[key] = time.Now()
	return limiter
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	limiter := rl.getLimiter(key)
	return limiter.Allow()
}

// Middleware returns HTTP middleware that enforces rate limiting
func (rl *RateLimiter) Middleware(keyExtractor func(*http.Request) string) func(http.Handler) http.Handler {
	if keyExtractor == nil {
		keyExtractor = extractClientIP
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyExtractor(r)
			if !rl.Allow(key) {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", float64(rl.rate)))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			limiter := rl.getLimiter(key)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", float64(rl.rate)))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", limiter.Tokens()))

			next.ServeHTTP(w, r)
		})
	}
}

// cleanupRoutine periodically removes old rate limiters
func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes rate limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.cleanupInterval)

	for key, lastSeen := range rl.lastSeen {
		if lastSeen.Before(cutoff) {
			delete(rl.limiters, key)
			delete(rl.lastSeen, key)
		}
	}
}

// extractClientIP extracts client IP from request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if firstIP := extractFirstIP(xff); firstIP != "" {
			return firstIP
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return extractIPFromAddr(r.RemoteAddr)
}

// extractFirstIP extracts the first IP from X-Forwarded-For header
func extractFirstIP(xff string) string {
	for i, char := range xff {
		if char == ',' || char == ' ' {
			return xff[:i]
		}
	}
	return xff
}

// extractIPFromAddr extracts IP from address:port format
func extractIPFromAddr(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// PerPathRateLimiter provides path-specific rate limiting
type PerPathRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

// NewPerPathRateLimiter creates a rate limiter with different limits per path
func NewPerPathRateLimiter() *PerPathRateLimiter {
	return &PerPathRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

// AddPath adds rate limiting for a specific path
func (prl *PerPathRateLimiter) AddPath(path string, config RateLimitConfig) {
	prl.mu.Lock()
	defer prl.mu.Unlock()
	prl.limiters[path] = NewRateLimiter(config)
}

// Middleware returns middleware that applies different rate limits per path
func (prl *PerPathRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prl.mu.RLock()
			limiter, exists := prl.limiters[r.URL.Path]
			prl.mu.RUnlock()

			if exists {
				key := extractClientIP(r)
				if !limiter.Allow(key) {
					http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
