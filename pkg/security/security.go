// Package security provides comprehensive security utilities for IPFS applications
package security

import (
	"net/http"
	"time"
)

// SecurityConfig provides comprehensive security configuration
type SecurityConfig struct {
	// Rate limiting
	RateLimit       RateLimitConfig
	EnableRateLimit bool

	// Authentication
	Auth       AuthConfig
	EnableAuth bool

	// Request validation
	Validation       RequestValidator
	EnableValidation bool

	// CORS
	CORS       CORSConfig
	EnableCORS bool

	// Security headers
	EnableSecureHeaders bool

	// IP whitelist
	IPWhitelist   []string
	EnableIPWhite bool
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 10.0,
			BurstSize:         20,
			CleanupInterval:   time.Hour,
			KeyExtractor:      extractClientIP,
		},
		EnableRateLimit: true,

		Auth: AuthConfig{
			TokenTTL:      24 * time.Hour,
			RequiredScope: "read",
		},
		EnableAuth: false, // Disabled by default

		Validation: RequestValidator{
			MaxBodySize: 1024 * 1024, // 1MB
		},
		EnableValidation: true,

		CORS:       DefaultCORSConfig(),
		EnableCORS: true,

		EnableSecureHeaders: true,
		EnableIPWhite:       false,
	}
}

// SecurityMiddleware provides a complete security middleware stack
type SecurityMiddleware struct {
	config      SecurityConfig
	rateLimiter *RateLimiter
	auth        *AuthMiddleware
}

// NewSecurityMiddleware creates a new security middleware with the given config
func NewSecurityMiddleware(config SecurityConfig) *SecurityMiddleware {
	sm := &SecurityMiddleware{
		config: config,
	}

	if config.EnableRateLimit {
		sm.rateLimiter = NewRateLimiter(config.RateLimit)
	}

	if config.EnableAuth {
		sm.auth = NewAuthMiddleware(config.Auth)
	}

	return sm
}

// Handler returns a complete security middleware stack
func (sm *SecurityMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := next

		// Apply middleware in reverse order (last applied = first executed)

		// Security headers (always last)
		if sm.config.EnableSecureHeaders {
			handler = SecureHeaders()(handler)
		}

		// CORS
		if sm.config.EnableCORS {
			handler = CORS(sm.config.CORS)(handler)
		}

		// Request validation
		if sm.config.EnableValidation {
			handler = sm.config.Validation.Middleware()(handler)
		}

		// Authentication
		if sm.config.EnableAuth && sm.auth != nil {
			handler = sm.auth.JWTAuth()(handler)
		}

		// Rate limiting
		if sm.config.EnableRateLimit && sm.rateLimiter != nil {
			handler = sm.rateLimiter.Middleware(sm.config.RateLimit.KeyExtractor)(handler)
		}

		// IP whitelist (first check)
		if sm.config.EnableIPWhite && len(sm.config.IPWhitelist) > 0 {
			handler = IPWhitelistAuth(sm.config.IPWhitelist)(handler)
		}

		return handler
	}
}

// Quick helper functions for common security patterns

// SecureGateway applies security middleware suitable for IPFS gateways
func SecureGateway() func(http.Handler) http.Handler {
	config := DefaultSecurityConfig()
	config.RateLimit.RequestsPerSecond = 100 // Higher rate limit for gateways
	config.RateLimit.BurstSize = 200

	sm := NewSecurityMiddleware(config)
	return sm.Handler()
}

// SecureAPI applies security middleware suitable for APIs
func SecureAPI(jwtSecret []byte, adminUsers []string) func(http.Handler) http.Handler {
	config := DefaultSecurityConfig()
	config.EnableAuth = true
	config.Auth.JWTSecret = jwtSecret
	config.Auth.AdminUsers = adminUsers
	config.Auth.RequiredScope = "api"

	sm := NewSecurityMiddleware(config)
	return sm.Handler()
}

// SecureAdmin applies strict security for admin endpoints
func SecureAdmin(jwtSecret []byte, adminUsers []string, allowedIPs []string) func(http.Handler) http.Handler {
	config := DefaultSecurityConfig()
	config.EnableAuth = true
	config.Auth.JWTSecret = jwtSecret
	config.Auth.AdminUsers = adminUsers
	config.Auth.RequiredScope = "admin"

	config.EnableIPWhite = true
	config.IPWhitelist = allowedIPs

	config.RateLimit.RequestsPerSecond = 5 // Strict rate limiting
	config.RateLimit.BurstSize = 10

	sm := NewSecurityMiddleware(config)
	return sm.Handler()
}

// ReadOnlyGateway applies security for read-only IPFS gateways
func ReadOnlyGateway() func(http.Handler) http.Handler {
	config := DefaultSecurityConfig()
	config.RateLimit.RequestsPerSecond = 50
	config.RateLimit.BurstSize = 100

	// Restrict to read-only operations
	config.Validation.AllowedPaths = []string{"/ipfs/", "/ipns/"}
	config.Validation.BlockedPaths = []string{"/api/", "/admin/"}

	sm := NewSecurityMiddleware(config)
	return sm.Handler()
}