package ipni

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Security handles cryptographic operations for IPNI
type Security struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	peerID     peer.ID
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	RequireSignatures bool          `json:"require_signatures"`
	KeyRotationPeriod time.Duration `json:"key_rotation_period"`
	TrustThreshold    float64       `json:"trust_threshold"`
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		RequireSignatures: true,
		KeyRotationPeriod: 24 * time.Hour,
		TrustThreshold:    0.7,
	}
}

// NewSecurity creates a new security manager
func NewSecurity(config *SecurityConfig) (*Security, error) {
	// Generate Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create peer ID from public key (simplified)
	peerID := peer.ID(fmt.Sprintf("12D3KooW%x", publicKey[:8]))

	return &Security{
		privateKey: privateKey,
		publicKey:  publicKey,
		peerID:     peerID,
	}, nil
}

// GetPeerID returns the peer ID for this security instance
func (s *Security) GetPeerID() peer.ID {
	return s.peerID
}

// SignData signs data with the private key
func (s *Security) SignData(data []byte) ([]byte, error) {
	signature := ed25519.Sign(s.privateKey, data)
	return signature, nil
}

// VerifySignature verifies a signature against data and public key
func (s *Security) VerifySignature(data, signature, publicKey []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, data, signature)
}

// GetPublicKey returns the public key
func (s *Security) GetPublicKey() []byte {
	return s.publicKey
}

// SignedAnnouncement represents a cryptographically signed provider announcement
type SignedAnnouncement struct {
	ProviderID peer.ID           `json:"provider_id"`
	ContextID  []byte            `json:"context_id"`
	Metadata   map[string]string `json:"metadata"`
	CIDs       []string          `json:"cids"`
	Timestamp  time.Time         `json:"timestamp"`
	Signature  []byte            `json:"signature"`
	PublicKey  []byte            `json:"public_key"`
}

// CreateSignedAnnouncement creates a cryptographically signed announcement
func (s *Security) CreateSignedAnnouncement(providerID peer.ID, contextID []byte, metadata map[string]string, cids []string) (*SignedAnnouncement, error) {
	announcement := &SignedAnnouncement{
		ProviderID: providerID,
		ContextID:  contextID,
		Metadata:   metadata,
		CIDs:       cids,
		Timestamp:  time.Now(),
		PublicKey:  s.publicKey,
	}

	// Create data to sign (simplified)
	dataToSign := fmt.Sprintf("%s:%x:%v:%d",
		providerID, contextID, cids, announcement.Timestamp.Unix())

	signature, err := s.SignData([]byte(dataToSign))
	if err != nil {
		return nil, fmt.Errorf("failed to sign announcement: %w", err)
	}

	announcement.Signature = signature
	return announcement, nil
}

// VerifyAnnouncement verifies a signed announcement
func (s *Security) VerifyAnnouncement(announcement *SignedAnnouncement) bool {
	// Recreate the data that was signed
	dataToSign := fmt.Sprintf("%s:%x:%v:%d",
		announcement.ProviderID, announcement.ContextID,
		announcement.CIDs, announcement.Timestamp.Unix())

	return s.VerifySignature([]byte(dataToSign), announcement.Signature, announcement.PublicKey)
}

// TrustScore calculates a trust score for a provider
func (s *Security) TrustScore(providerID peer.ID) float64 {
	// Simplified trust calculation
	// In practice, this would consider:
	// - Historical reliability
	// - Signature verification success rate
	// - Network reputation
	// - Time since last verification

	// For demo, return a random-ish but deterministic score
	hash := string(providerID)
	score := 0.0
	for _, char := range hash {
		score += float64(char)
	}

	// Normalize to 0-1 range
	normalized := (score / 1000.0)
	if normalized > 1.0 {
		normalized = 1.0 - (normalized - 1.0)
	}
	if normalized < 0.0 {
		normalized = -normalized
	}

	return normalized
}

// IsProviderTrusted checks if a provider meets the trust threshold
func (s *Security) IsProviderTrusted(providerID peer.ID, config *SecurityConfig) bool {
	score := s.TrustScore(providerID)
	return score >= config.TrustThreshold
}

// AntiSpamFilter provides basic spam protection
type AntiSpamFilter struct {
	rateLimits map[peer.ID][]time.Time
	maxRate    int
	window     time.Duration
}

// NewAntiSpamFilter creates a new anti-spam filter
func NewAntiSpamFilter(maxRate int, window time.Duration) *AntiSpamFilter {
	return &AntiSpamFilter{
		rateLimits: make(map[peer.ID][]time.Time),
		maxRate:    maxRate,
		window:     window,
	}
}

// CheckRateLimit checks if a provider has exceeded rate limits
func (f *AntiSpamFilter) CheckRateLimit(providerID peer.ID) bool {
	now := time.Now()

	// Get existing timestamps for this provider
	timestamps := f.rateLimits[providerID]

	// Remove old timestamps outside the window
	var validTimestamps []time.Time
	for _, ts := range timestamps {
		if now.Sub(ts) <= f.window {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	// Check if adding this request would exceed the limit
	if len(validTimestamps) >= f.maxRate {
		return false // Rate limit exceeded
	}

	// Add current timestamp and update
	validTimestamps = append(validTimestamps, now)
	f.rateLimits[providerID] = validTimestamps

	return true // Request allowed
}
