package ipns

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
)

// IPNSManager manages IPNS records and name resolution
type IPNSManager struct {
	dagWrapper *dag.DagWrapper
	records    map[string]*IPNSRecord
	keys       map[string]crypto.PrivKey
	mutex      sync.RWMutex
}

// IPNSRecord represents an IPNS record with metadata
type IPNSRecord struct {
	Name       string         `json:"name"`       // IPNS name (peer ID)
	Value      string         `json:"value"`      // CID or path this name points to
	CreatedAt  time.Time      `json:"created_at"` // When record was created
	UpdatedAt  time.Time      `json:"updated_at"` // Last update time
	TTL        uint64         `json:"ttl"`        // Time to live in seconds
	Sequence   uint64         `json:"sequence"`   // Sequence number for updates
	PrivateKey crypto.PrivKey `json:"-"`          // Private key (not exported)
}

// NewIPNSManager creates a new IPNS manager
func NewIPNSManager(dagWrapper *dag.DagWrapper) *IPNSManager {
	return &IPNSManager{
		dagWrapper: dagWrapper,
		records:    make(map[string]*IPNSRecord),
		keys:       make(map[string]crypto.PrivKey),
	}
}

// GenerateKey generates a new keypair for IPNS
func (m *IPNSManager) GenerateKey(ctx context.Context, keyName string) (peer.ID, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Generate RSA keypair
	privKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Get peer ID from public key
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return "", fmt.Errorf("failed to get peer ID: %w", err)
	}

	// Store the key
	m.keys[keyName] = privKey

	return peerID, nil
}

// PublishIPNS publishes a new IPNS record
func (m *IPNSManager) PublishIPNS(ctx context.Context, keyName string, value cid.Cid, ttl time.Duration) (*IPNSRecord, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the private key
	privKey, exists := m.keys[keyName]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyName)
	}

	// Get peer ID
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer ID: %w", err)
	}

	ipnsName := peerID.String()

	// Check if record exists for sequence number
	var sequence uint64 = 0
	if existingRecord, exists := m.records[ipnsName]; exists {
		sequence = existingRecord.Sequence + 1
	}

	// Create IPNS record
	now := time.Now()
	eol := now.Add(ttl)

	// Create path from CID
	ipfsPath := path.FromCid(value)

	// Create the actual IPNS record using boxo
	ipnsRecord, err := ipns.NewRecord(privKey, ipfsPath, sequence, eol, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPNS record: %w", err)
	}

	// Create IPNS name from peer ID
	ipnsNameObj := ipns.NameFromPeer(peerID)

	// Validate the record
	err = ipns.ValidateWithName(ipnsRecord, ipnsNameObj)
	if err != nil {
		return nil, fmt.Errorf("invalid IPNS record: %w", err)
	}

	// Store our record
	record := &IPNSRecord{
		Name:       ipnsName,
		Value:      "/ipfs/" + value.String(),
		CreatedAt:  now,
		UpdatedAt:  now,
		TTL:        uint64(ttl.Seconds()),
		Sequence:   sequence,
		PrivateKey: privKey,
	}

	m.records[ipnsName] = record

	return record, nil
}

// ResolveIPNS resolves an IPNS name to its current value
func (m *IPNSManager) ResolveIPNS(ctx context.Context, name string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Clean the name (remove /ipns/ prefix if present)
	name = cleanIPNSName(name)

	record, exists := m.records[name]
	if !exists {
		return "", fmt.Errorf("IPNS name not found: %s", name)
	}

	// Check if record has expired
	expirationTime := record.CreatedAt.Add(time.Duration(record.TTL) * time.Second)
	if time.Now().After(expirationTime) {
		return "", fmt.Errorf("IPNS record expired: %s", name)
	}

	return record.Value, nil
}

// UpdateIPNS updates an existing IPNS record
func (m *IPNSManager) UpdateIPNS(ctx context.Context, keyName string, newValue cid.Cid, ttl time.Duration) (*IPNSRecord, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the private key
	privKey, exists := m.keys[keyName]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyName)
	}

	// Get peer ID
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer ID: %w", err)
	}

	ipnsName := peerID.String()

	// Get existing record for sequence number
	existingRecord, exists := m.records[ipnsName]
	if !exists {
		return nil, fmt.Errorf("IPNS record not found: %s", ipnsName)
	}

	// Create updated record
	now := time.Now()
	sequence := existingRecord.Sequence + 1
	eol := now.Add(ttl)

	// Create path from CID
	ipfsPath := path.FromCid(newValue)

	// Create the actual IPNS record using boxo
	ipnsRecord, err := ipns.NewRecord(privKey, ipfsPath, sequence, eol, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPNS record: %w", err)
	}

	// Create IPNS name from peer ID
	ipnsNameObj := ipns.NameFromPeer(peerID)

	// Validate the record
	err = ipns.ValidateWithName(ipnsRecord, ipnsNameObj)
	if err != nil {
		return nil, fmt.Errorf("invalid IPNS record: %w", err)
	}

	// Update our record
	record := &IPNSRecord{
		Name:       ipnsName,
		Value:      "/ipfs/" + newValue.String(),
		CreatedAt:  existingRecord.CreatedAt,
		UpdatedAt:  now,
		TTL:        uint64(ttl.Seconds()),
		Sequence:   sequence,
		PrivateKey: privKey,
	}

	m.records[ipnsName] = record

	return record, nil
}

// ListIPNSRecords lists all IPNS records
func (m *IPNSManager) ListIPNSRecords(ctx context.Context) ([]*IPNSRecord, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var records []*IPNSRecord
	for _, record := range m.records {
		// Create a copy without the private key for safety
		recordCopy := &IPNSRecord{
			Name:      record.Name,
			Value:     record.Value,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
			TTL:       record.TTL,
			Sequence:  record.Sequence,
		}
		records = append(records, recordCopy)
	}

	return records, nil
}

// GetIPNSRecord gets a specific IPNS record
func (m *IPNSManager) GetIPNSRecord(ctx context.Context, name string) (*IPNSRecord, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	name = cleanIPNSName(name)

	record, exists := m.records[name]
	if !exists {
		return nil, fmt.Errorf("IPNS record not found: %s", name)
	}

	// Return a copy without the private key
	return &IPNSRecord{
		Name:      record.Name,
		Value:     record.Value,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		TTL:       record.TTL,
		Sequence:  record.Sequence,
	}, nil
}

// DeleteIPNS deletes an IPNS record
func (m *IPNSManager) DeleteIPNS(ctx context.Context, keyName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the private key to find the peer ID
	privKey, exists := m.keys[keyName]
	if !exists {
		return fmt.Errorf("key not found: %s", keyName)
	}

	// Get peer ID
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to get peer ID: %w", err)
	}

	ipnsName := peerID.String()

	// Delete the record and key
	delete(m.records, ipnsName)
	delete(m.keys, keyName)

	return nil
}

// IsExpired checks if an IPNS record has expired
func (m *IPNSManager) IsExpired(ctx context.Context, name string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	name = cleanIPNSName(name)

	record, exists := m.records[name]
	if !exists {
		return true, fmt.Errorf("IPNS record not found: %s", name)
	}

	expirationTime := record.CreatedAt.Add(time.Duration(record.TTL) * time.Second)
	return time.Now().After(expirationTime), nil
}

// GetStats returns IPNS manager statistics
func (m *IPNSManager) GetStats(ctx context.Context) (*IPNSStats, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var totalRecords, expiredRecords, activeRecords int
	var oldestRecord, newestRecord time.Time

	now := time.Now()
	for _, record := range m.records {
		totalRecords++

		// Check expiration
		expirationTime := record.CreatedAt.Add(time.Duration(record.TTL) * time.Second)
		if now.After(expirationTime) {
			expiredRecords++
		} else {
			activeRecords++
		}

		// Track oldest and newest
		if oldestRecord.IsZero() || record.CreatedAt.Before(oldestRecord) {
			oldestRecord = record.CreatedAt
		}
		if newestRecord.IsZero() || record.CreatedAt.After(newestRecord) {
			newestRecord = record.CreatedAt
		}
	}

	return &IPNSStats{
		TotalRecords:   totalRecords,
		ActiveRecords:  activeRecords,
		ExpiredRecords: expiredRecords,
		TotalKeys:      len(m.keys),
		OldestRecord:   oldestRecord,
		NewestRecord:   newestRecord,
	}, nil
}

// IPNSStats contains statistics about IPNS records
type IPNSStats struct {
	TotalRecords   int       `json:"total_records"`
	ActiveRecords  int       `json:"active_records"`
	ExpiredRecords int       `json:"expired_records"`
	TotalKeys      int       `json:"total_keys"`
	OldestRecord   time.Time `json:"oldest_record"`
	NewestRecord   time.Time `json:"newest_record"`
}

// cleanIPNSName removes /ipns/ prefix from name if present
func cleanIPNSName(name string) string {
	if len(name) > 6 && name[:6] == "/ipns/" {
		return name[6:]
	}
	return name
}

// ValidateIPNSName validates an IPNS name format
func ValidateIPNSName(name string) error {
	name = cleanIPNSName(name)

	// Parse as peer ID to validate format
	_, err := peer.Decode(name)
	if err != nil {
		return fmt.Errorf("invalid IPNS name format: %w", err)
	}

	return nil
}

// FormatIPNSPath formats a path for IPNS usage
func FormatIPNSPath(name string) string {
	name = cleanIPNSName(name)
	return "/ipns/" + name
}

// ExtractCIDFromIPFSPath extracts CID from /ipfs/CID path
func ExtractCIDFromIPFSPath(path string) (cid.Cid, error) {
	if len(path) < 7 || path[:6] != "/ipfs/" {
		return cid.Undef, fmt.Errorf("not an IPFS path: %s", path)
	}

	cidStr := path[6:]

	// Handle paths with additional segments
	if slashIndex := len(cidStr); slashIndex > 0 {
		for i, ch := range cidStr {
			if ch == '/' {
				cidStr = cidStr[:i]
				break
			}
		}
	}

	return cid.Parse(cidStr)
}
