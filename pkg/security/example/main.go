package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gosuda/boxo-starter-kit/pkg/security"
)

func main() {
	fmt.Println("🔒 Security Enhanced IPFS Gateway Example")
	fmt.Println("========================================")

	// Create different security configurations for different endpoints

	// 1. Public read-only gateway with basic security
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/ipfs/", handleIPFS)
	publicMux.HandleFunc("/ipns/", handleIPNS)
	publicMux.HandleFunc("/health", handleHealth)

	// Apply read-only gateway security
	securePublicHandler := security.ReadOnlyGateway()(publicMux)

	// 2. API endpoints with authentication
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/upload", handleUpload)
	apiMux.HandleFunc("/api/pin", handlePin)
	apiMux.HandleFunc("/api/stats", handleStats)

	// API security with JWT
	jwtSecret := []byte("your-secret-key-change-in-production")
	adminUsers := []string{"admin", "operator"}
	secureAPIHandler := security.SecureAPI(jwtSecret, adminUsers)(apiMux)

	// 3. Admin endpoints with strict security
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/users", handleUsers)
	adminMux.HandleFunc("/admin/config", handleConfig)
	adminMux.HandleFunc("/admin/metrics", handleMetrics)

	// Admin security with IP whitelist
	allowedIPs := []string{"127.0.0.1", "::1", "192.168.1.0/24"}
	secureAdminHandler := security.SecureAdmin(jwtSecret, adminUsers, allowedIPs)(adminMux)

	// 4. Main router
	mainMux := http.NewServeMux()
	mainMux.Handle("/ipfs/", securePublicHandler)
	mainMux.Handle("/ipns/", securePublicHandler)
	mainMux.Handle("/health", securePublicHandler)
	mainMux.Handle("/api/", secureAPIHandler)
	mainMux.Handle("/admin/", secureAdminHandler)
	mainMux.HandleFunc("/login", handleLogin)
	mainMux.HandleFunc("/", handleIndex)

	// 5. Apply global security middleware
	globalSecurity := security.NewSecurityMiddleware(security.SecurityConfig{
		RateLimit: security.RateLimitConfig{
			RequestsPerSecond: 50,
			BurstSize:         100,
			CleanupInterval:   time.Hour,
		},
		EnableRateLimit:     true,
		EnableValidation:    true,
		EnableCORS:          true,
		EnableSecureHeaders: true,
		CORS:                security.DefaultCORSConfig(),
		Validation: security.RequestValidator{
			MaxBodySize: 10 * 1024 * 1024, // 10MB
		},
	})

	finalHandler := globalSecurity.Handler()(mainMux)

	// Start server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      finalHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Println("🚀 Starting secure server on :8080")
	fmt.Println("📍 Endpoints:")
	fmt.Println("   • Public:  /ipfs/, /ipns/, /health")
	fmt.Println("   • API:     /api/* (requires JWT token)")
	fmt.Println("   • Admin:   /admin/* (requires JWT + IP whitelist)")
	fmt.Println("   • Auth:    /login (get JWT token)")
	fmt.Println()
	fmt.Println("🔑 To get a token:")
	fmt.Println("   curl -X POST http://localhost:8080/login -d '{\"username\":\"admin\",\"password\":\"secret\"}'")
	fmt.Println()
	fmt.Println("🌐 Example requests:")
	fmt.Println("   curl http://localhost:8080/health")
	fmt.Println("   curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/stats")

	log.Fatal(server.ListenAndServe())
}

// Public handlers

func handleIPFS(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would fetch from IPFS
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    r.URL.Path,
		"message": "IPFS content would be served here",
		"secure":  true,
	})
}

func handleIPNS(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would resolve IPNS
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    r.URL.Path,
		"message": "IPNS resolution would happen here",
		"secure":  true,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"security":  "enabled",
	})
}

// API handlers (require authentication)

func handleUpload(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "File upload would happen here",
		"user":    user.Username,
		"scope":   user.Scope,
	})
}

func handlePin(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Content pinning would happen here",
		"user":    user.Username,
	})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": map[string]interface{}{
			"total_requests": 12345,
			"active_pins":    678,
			"storage_used":   "1.2TB",
		},
		"user": user.Username,
	})
}

// Admin handlers (require admin privileges + IP whitelist)

func handleUsers(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": []map[string]interface{}{
			{"id": 1, "username": "admin", "role": "administrator"},
			{"id": 2, "username": "operator", "role": "operator"},
			{"id": 3, "username": "user", "role": "user"},
		},
		"admin": user.Username,
	})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": map[string]interface{}{
			"rate_limit": "50 req/s",
			"auth":       "JWT",
			"cors":       "enabled",
			"validation": "enabled",
		},
		"admin": user.Username,
	})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	user := security.GetUserInfo(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"metrics": map[string]interface{}{
			"cpu_usage":    "25%",
			"memory_usage": "60%",
			"disk_usage":   "45%",
			"uptime":       "72h",
		},
		"admin": user.Username,
	})
}

// Auth handler

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Simple auth check (use proper auth in production!)
	if creds.Username == "admin" && creds.Password == "secret" {
		// Create auth middleware to generate token
		authConfig := security.AuthConfig{
			JWTSecret:     []byte("your-secret-key-change-in-production"),
			TokenTTL:      24 * time.Hour,
			RequiredScope: "admin",
			AdminUsers:    []string{"admin"},
		}
		auth := security.NewAuthMiddleware(authConfig)

		token, err := auth.GenerateToken("1", creds.Username, "admin")
		if err != nil {
			http.Error(w, "Token generation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":    token,
			"username": creds.Username,
			"scope":    "admin",
			"expires":  time.Now().Add(24 * time.Hour).Unix(),
		})
	} else if creds.Username == "user" && creds.Password == "userpass" {
		authConfig := security.AuthConfig{
			JWTSecret:     []byte("your-secret-key-change-in-production"),
			TokenTTL:      24 * time.Hour,
			RequiredScope: "api",
		}
		auth := security.NewAuthMiddleware(authConfig)

		token, err := auth.GenerateToken("2", creds.Username, "api")
		if err != nil {
			http.Error(w, "Token generation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":    token,
			"username": creds.Username,
			"scope":    "api",
			"expires":  time.Now().Add(24 * time.Hour).Unix(),
		})
	} else {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
	}
}

// Home handler

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Secure IPFS Gateway</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 20px 0; padding: 15px; background: #f5f5f5; border-radius: 5px; }
        .method { color: #007acc; font-weight: bold; }
        .path { color: #d73a49; font-family: monospace; }
        .description { color: #6f42c1; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 3px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>🔒 Secure IPFS Gateway</h1>
    <p>This gateway demonstrates comprehensive security features including rate limiting, authentication, validation, CORS, and secure headers.</p>

    <h2>Public Endpoints</h2>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/health</span></div>
        <div class="description">Health check endpoint</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/ipfs/{cid}</span></div>
        <div class="description">Access IPFS content (read-only)</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/ipns/{name}</span></div>
        <div class="description">Resolve IPNS names (read-only)</div>
    </div>

    <h2>API Endpoints (Authentication Required)</h2>
    <div class="endpoint">
        <div><span class="method">POST</span> <span class="path">/api/upload</span></div>
        <div class="description">Upload content to IPFS</div>
    </div>
    <div class="endpoint">
        <div><span class="method">POST</span> <span class="path">/api/pin</span></div>
        <div class="description">Pin content</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/api/stats</span></div>
        <div class="description">Get statistics</div>
    </div>

    <h2>Admin Endpoints (Admin + IP Whitelist Required)</h2>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/admin/users</span></div>
        <div class="description">Manage users</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/admin/config</span></div>
        <div class="description">View configuration</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/admin/metrics</span></div>
        <div class="description">System metrics</div>
    </div>

    <h2>Authentication</h2>
    <div class="endpoint">
        <div><span class="method">POST</span> <span class="path">/login</span></div>
        <div class="description">Get JWT token</div>
        <pre>curl -X POST http://localhost:8080/login -d '{"username":"admin","password":"secret"}'</pre>
        <pre>curl -X POST http://localhost:8080/login -d '{"username":"user","password":"userpass"}'</pre>
    </div>

    <h2>Security Features</h2>
    <ul>
        <li><strong>Rate Limiting:</strong> 50 requests/second with burst capacity</li>
        <li><strong>CORS:</strong> Configurable cross-origin resource sharing</li>
        <li><strong>Security Headers:</strong> XSS protection, content type sniffing prevention, etc.</li>
        <li><strong>Request Validation:</strong> Path validation, content size limits</li>
        <li><strong>Authentication:</strong> JWT-based authentication for API endpoints</li>
        <li><strong>Authorization:</strong> Role-based access control</li>
        <li><strong>IP Whitelisting:</strong> IP-based access control for admin endpoints</li>
    </ul>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
