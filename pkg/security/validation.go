package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/ipfs/go-cid"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%d validation errors: %s", len(e), e[0].Message)
}

// RequestValidator provides request validation functionality
type RequestValidator struct {
	MaxBodySize     int64
	AllowedPaths    []string
	BlockedPaths    []string
	RequiredHeaders []string
}

// NewRequestValidator creates a new request validator
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		MaxBodySize: 1024 * 1024, // 1MB default
	}
}

// ValidateRequest validates an HTTP request
func (rv *RequestValidator) ValidateRequest(r *http.Request) error {
	var errors ValidationErrors

	// Validate content length
	if r.ContentLength > rv.MaxBodySize {
		errors = append(errors, ValidationError{
			Field:   "content-length",
			Message: fmt.Sprintf("request body too large, max %d bytes", rv.MaxBodySize),
			Value:   fmt.Sprintf("%d", r.ContentLength),
		})
	}

	// Validate path
	if err := rv.validatePath(r.URL.Path); err != nil {
		errors = append(errors, *err)
	}

	// Validate required headers
	for _, header := range rv.RequiredHeaders {
		if r.Header.Get(header) == "" {
			errors = append(errors, ValidationError{
				Field:   header,
				Message: "required header missing",
			})
		}
	}

	// Validate method
	if err := rv.validateMethod(r.Method); err != nil {
		errors = append(errors, *err)
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validatePath validates request path
func (rv *RequestValidator) validatePath(path string) *ValidationError {
	// Check blocked paths
	for _, blocked := range rv.BlockedPaths {
		if strings.HasPrefix(path, blocked) {
			return &ValidationError{
				Field:   "path",
				Message: "path is blocked",
				Value:   path,
			}
		}
	}

	// Check allowed paths (if specified)
	if len(rv.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range rv.AllowedPaths {
			if strings.HasPrefix(path, allowedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ValidationError{
				Field:   "path",
				Message: "path not allowed",
				Value:   path,
			}
		}
	}

	return nil
}

// validateMethod validates HTTP method
func (rv *RequestValidator) validateMethod(method string) *ValidationError {
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}

	for _, allowed := range allowedMethods {
		if method == allowed {
			return nil
		}
	}

	return &ValidationError{
		Field:   "method",
		Message: "unsupported HTTP method",
		Value:   method,
	}
}

// Middleware returns HTTP middleware for request validation
func (rv *RequestValidator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := rv.ValidateRequest(r); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":   "validation failed",
					"details": err,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CIDValidator validates IPFS CIDs
type CIDValidator struct {
	AllowedVersions []int
	AllowedCodecs   []uint64
	AllowedHashes   []uint64
}

// ValidateCID validates a CID string
func (cv *CIDValidator) ValidateCID(cidStr string) error {
	if cidStr == "" {
		return ValidationError{
			Field:   "cid",
			Message: "CID cannot be empty",
		}
	}

	parsedCID, err := cid.Decode(cidStr)
	if err != nil {
		return ValidationError{
			Field:   "cid",
			Message: "invalid CID format",
			Value:   cidStr,
		}
	}

	// Validate version
	if len(cv.AllowedVersions) > 0 {
		versionAllowed := false
		for _, version := range cv.AllowedVersions {
			if int(parsedCID.Version()) == version {
				versionAllowed = true
				break
			}
		}
		if !versionAllowed {
			return ValidationError{
				Field:   "cid",
				Message: fmt.Sprintf("CID version %d not allowed", parsedCID.Version()),
				Value:   cidStr,
			}
		}
	}

	// Validate codec
	if len(cv.AllowedCodecs) > 0 {
		codecAllowed := false
		for _, codec := range cv.AllowedCodecs {
			if parsedCID.Type() == codec {
				codecAllowed = true
				break
			}
		}
		if !codecAllowed {
			return ValidationError{
				Field:   "cid",
				Message: fmt.Sprintf("CID codec %d not allowed", parsedCID.Type()),
				Value:   cidStr,
			}
		}
	}

	return nil
}

// IPFSPathValidator validates IPFS paths
type IPFSPathValidator struct {
	MaxDepth        int
	AllowedPrefixes []string
	BlockedPrefixes []string
}

// ValidateIPFSPath validates an IPFS path
func (pv *IPFSPathValidator) ValidateIPFSPath(path string) error {
	if path == "" {
		return ValidationError{
			Field:   "path",
			Message: "path cannot be empty",
		}
	}

	// Basic format validation
	if !strings.HasPrefix(path, "/ipfs/") && !strings.HasPrefix(path, "/ipns/") {
		return ValidationError{
			Field:   "path",
			Message: "path must start with /ipfs/ or /ipns/",
			Value:   path,
		}
	}

	// Check depth
	if pv.MaxDepth > 0 {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) > pv.MaxDepth {
			return ValidationError{
				Field:   "path",
				Message: fmt.Sprintf("path depth %d exceeds maximum %d", len(parts), pv.MaxDepth),
				Value:   path,
			}
		}
	}

	// Check blocked prefixes
	for _, blocked := range pv.BlockedPrefixes {
		if strings.HasPrefix(path, blocked) {
			return ValidationError{
				Field:   "path",
				Message: "path prefix is blocked",
				Value:   path,
			}
		}
	}

	// Check allowed prefixes
	if len(pv.AllowedPrefixes) > 0 {
		allowed := false
		for _, prefix := range pv.AllowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ValidationError{
				Field:   "path",
				Message: "path prefix not allowed",
				Value:   path,
			}
		}
	}

	return nil
}

// SanitizeInput sanitizes user input to prevent injection attacks
func SanitizeInput(input string) string {
	// Remove potentially dangerous characters
	reg := regexp.MustCompile(`[<>'"&]`)
	return reg.ReplaceAllString(input, "")
}

// ValidateContentType validates HTTP Content-Type header
func ValidateContentType(contentType string, allowed []string) error {
	if contentType == "" {
		return ValidationError{
			Field:   "content-type",
			Message: "content type is required",
		}
	}

	for _, allowedType := range allowed {
		if strings.HasPrefix(contentType, allowedType) {
			return nil
		}
	}

	return ValidationError{
		Field:   "content-type",
		Message: "unsupported content type",
		Value:   contentType,
	}
}

// ValidateUserAgent validates HTTP User-Agent header
func ValidateUserAgent(userAgent string, blocked []string) error {
	if userAgent == "" {
		return nil // User-Agent is optional
	}

	userAgentLower := strings.ToLower(userAgent)
	for _, blockedAgent := range blocked {
		if strings.Contains(userAgentLower, strings.ToLower(blockedAgent)) {
			return ValidationError{
				Field:   "user-agent",
				Message: "user agent is blocked",
				Value:   userAgent,
			}
		}
	}

	return nil
}
