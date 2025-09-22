package security_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gosuda/boxo-starter-kit/pkg/security"
)

func TestRateLimiting(t *testing.T) {
	config := security.RateLimitConfig{
		RequestsPerSecond: 2.0, // Very low for testing
		BurstSize:         3,
		CleanupInterval:   time.Minute,
		KeyExtractor:      nil, // Use default
	}

	rateLimiter := security.NewRateLimiter(config)
	middleware := rateLimiter.Middleware(nil)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First few requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request %d should succeed, got status %d", i+1, rec.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Request should be rate limited, got status %d", rec.Code)
	}
}

func TestBasicAuth(t *testing.T) {
	middleware := security.BasicAuth("testuser", "testpass")

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authenticated"))
	}))

	// Test without auth
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Request without auth should be unauthorized, got status %d", rec.Code)
	}

	// Test with correct auth
	req = httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("testuser", "testpass")
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Request with correct auth should succeed, got status %d", rec.Code)
	}

	// Test with incorrect auth
	req = httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("testuser", "wrongpass")
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Request with incorrect auth should be unauthorized, got status %d", rec.Code)
	}
}

func TestRequestValidation(t *testing.T) {
	validator := security.NewRequestValidator()
	validator.MaxBodySize = 100 // Very small for testing
	validator.AllowedPaths = []string{"/api/"}
	validator.BlockedPaths = []string{"/admin/"}

	middleware := validator.Middleware()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("valid"))
	}))

	// Test allowed path
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Allowed path should succeed, got status %d", rec.Code)
	}

	// Test blocked path
	req = httptest.NewRequest("GET", "/admin/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Blocked path should be rejected, got status %d", rec.Code)
	}

	// Test disallowed path
	req = httptest.NewRequest("GET", "/other/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Disallowed path should be rejected, got status %d", rec.Code)
	}
}

func TestCIDValidation(t *testing.T) {
	validator := &security.CIDValidator{
		AllowedVersions: []int{1},
	}

	tests := []struct {
		cid   string
		valid bool
	}{
		{"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", false},             // CIDv0
		{"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi", true}, // CIDv1
		{"", false},        // Empty
		{"invalid", false}, // Invalid format
	}

	for _, test := range tests {
		err := validator.ValidateCID(test.cid)
		isValid := err == nil

		if isValid != test.valid {
			t.Errorf("CID %s: expected valid=%v, got valid=%v (error: %v)",
				test.cid, test.valid, isValid, err)
		}
	}
}

func TestIPFSPathValidation(t *testing.T) {
	validator := &security.IPFSPathValidator{
		MaxDepth:        3,
		AllowedPrefixes: []string{"/ipfs/"},
		BlockedPrefixes: []string{"/ipfs/QmBlocked"},
	}

	tests := []struct {
		path  string
		valid bool
	}{
		{"/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG", true},
		{"/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG/file.txt", true},
		{"/ipfs/QmBlocked123", false},   // Blocked prefix
		{"/ipns/example.com", false},    // Not allowed prefix
		{"/ipfs/QmTest/a/b/c/d", false}, // Too deep
		{"", false},                     // Empty
		{"/invalid", false},             // Invalid format
	}

	for _, test := range tests {
		err := validator.ValidateIPFSPath(test.path)
		isValid := err == nil

		if isValid != test.valid {
			t.Errorf("Path %s: expected valid=%v, got valid=%v (error: %v)",
				test.path, test.valid, isValid, err)
		}
	}
}

func TestSecurityMiddlewareStack(t *testing.T) {
	config := security.DefaultSecurityConfig()
	config.RateLimit.RequestsPerSecond = 100 // High enough for test
	config.EnableAuth = false                // Disable auth for this test

	sm := security.NewSecurityMiddleware(config)
	middleware := sm.Handler()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secure"))
	}))

	req := httptest.NewRequest("GET", "/ipfs/QmTest", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Secure request should succeed, got status %d", rec.Code)
	}

	// Check security headers
	headers := rec.Header()

	expectedHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Content-Security-Policy",
		"Referrer-Policy",
		"Access-Control-Allow-Origin",
	}

	for _, header := range expectedHeaders {
		if headers.Get(header) == "" {
			t.Errorf("Expected security header %s not found", header)
		}
	}
}

func TestUserContext(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	if security.IsAuthenticated(ctx) {
		t.Error("Empty context should not be authenticated")
	}

	if security.GetUserInfo(ctx) != nil {
		t.Error("Empty context should not have user info")
	}

	// Test with user info
	user := &security.UserInfo{
		ID:       "123",
		Username: "testuser",
		Scope:    "read",
	}

	ctx = security.WithUserInfo(ctx, user)

	if !security.IsAuthenticated(ctx) {
		t.Error("Context with user should be authenticated")
	}

	if !security.HasScope(ctx, "read") {
		t.Error("User should have read scope")
	}

	if security.HasScope(ctx, "write") {
		t.Error("User should not have write scope")
	}

	retrievedUser := security.GetUserInfo(ctx)
	if retrievedUser == nil {
		t.Error("Should retrieve user info from context")
	}

	if retrievedUser.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", retrievedUser.Username)
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"<script>alert('xss')</script>", "scriptalert(xss)/script"},
		{"user@example.com", "userexample.com"},
		{"'DROP TABLE users;'", "DROP TABLE users;"},
		{"safe-input_123", "safe-input_123"},
	}

	for _, test := range tests {
		result := security.SanitizeInput(test.input)
		if result != test.expected {
			t.Errorf("SanitizeInput(%q): expected %q, got %q",
				test.input, test.expected, result)
		}
	}
}

func TestCORS(t *testing.T) {
	config := security.DefaultCORSConfig()
	middleware := security.CORS(config)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test preflight request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Preflight request should return 204, got %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS headers should be set for preflight request")
	}

	// Test actual request
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("CORS request should succeed, got status %d", rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("CORS headers should be set for actual request")
	}
}
