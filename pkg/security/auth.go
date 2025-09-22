package security

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig configures authentication behavior
type AuthConfig struct {
	JWTSecret     []byte
	TokenTTL      time.Duration
	RequiredScope string
	AdminUsers    []string
}

// AuthMiddleware provides authentication and authorization
type AuthMiddleware struct {
	config AuthConfig
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(config AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{config: config}
}

// BasicAuth provides HTTP Basic Authentication
func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Use constant-time comparison to prevent timing attacks
			userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
			passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

			if !userMatch || !passMatch {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// JWTAuth provides JWT-based authentication
func (am *AuthMiddleware) JWTAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractToken(r)
			if tokenString == "" {
				http.Error(w, "Missing token", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return am.config.JWTSecret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// Check scope if required
			if am.config.RequiredScope != "" {
				scope, exists := claims["scope"].(string)
				if !exists || scope != am.config.RequiredScope {
					http.Error(w, "Insufficient scope", http.StatusForbidden)
					return
				}
			}

			// Add user info to request context
			r = r.WithContext(WithUserInfo(r.Context(), &UserInfo{
				ID:       claims["sub"].(string),
				Username: claims["username"].(string),
				Scope:    claims["scope"].(string),
			}))

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly restricts access to admin users
func (am *AuthMiddleware) AdminOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userInfo := GetUserInfo(r.Context())
			if userInfo == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			isAdmin := false
			for _, adminUser := range am.config.AdminUsers {
				if userInfo.Username == adminUser {
					isAdmin = true
					break
				}
			}

			if !isAdmin {
				http.Error(w, "Admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GenerateToken creates a new JWT token
func (am *AuthMiddleware) GenerateToken(userID, username, scope string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"scope":    scope,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(am.config.TokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(am.config.JWTSecret)
}

// extractToken extracts JWT token from request
func extractToken(r *http.Request) string {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Check query parameter
	return r.URL.Query().Get("token")
}

// APIKeyAuth provides API key authentication
func APIKeyAuth(validKeys []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			if apiKey == "" {
				http.Error(w, "API key required", http.StatusUnauthorized)
				return
			}

			valid := false
			for _, validKey := range validKeys {
				if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
					valid = true
					break
				}
			}

			if !valid {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPWhitelistAuth restricts access to specific IP addresses
func IPWhitelistAuth(allowedIPs []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := extractClientIP(r)

			allowed := false
			for _, allowedIP := range allowedIPs {
				if clientIP == allowedIP {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig configures CORS behavior
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	MaxAge         time.Duration
	Credentials    bool
}

// DefaultCORSConfig returns sensible CORS defaults
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders: []string{"Link"},
		MaxAge:         12 * time.Hour,
		Credentials:    false,
	}
}

// CORS returns CORS middleware
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowedOrigin := ""
			for _, allowed := range config.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					allowedOrigin = allowed
					break
				}
			}

			if allowedOrigin != "" {
				if allowedOrigin == "*" && !config.Credentials {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}

			if config.Credentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if len(config.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			}

			if len(config.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			}

			if len(config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
			}

			if config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%.0f", config.MaxAge.Seconds()))
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeaders adds security headers to responses
func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable XSS protection
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Strict transport security (for HTTPS)
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			// Content Security Policy
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}
