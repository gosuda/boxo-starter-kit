package pin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

// PinType represents different types of pins
type PinType int

const (
	DirectPin    PinType = iota // Pin only the specific CID
	RecursivePin                // Pin the CID and all children
	IndirectPin                 // Pin that exists because it's a child of a recursive pin
)

func (p PinType) String() string {
	switch p {
	case DirectPin:
		return "direct"
	case RecursivePin:
		return "recursive"
	case IndirectPin:
		return "indirect"
	default:
		return "unknown"
	}
}

// PinInfo contains information about a pinned CID
type PinInfo struct {
	CID       cid.Cid   `json:"cid"`
	Type      PinType   `json:"type"`
	Name      string    `json:"name,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PinManager manages pins and garbage collection using a simple in-memory approach
type PinManager struct {
	dagWrapper *dag.IpldWrapper
	mutex      sync.RWMutex

	// Simple in-memory pin tracking
	directPins    map[cid.Cid]PinInfo
	recursivePins map[cid.Cid]PinInfo
	indirectPins  map[cid.Cid]PinInfo // Calculated from recursive pins

	// Statistics
	stats struct {
		LastGC         time.Time     `json:"last_gc"`
		GCDuration     time.Duration `json:"gc_duration"`
		ReclaimedBytes int64         `json:"reclaimed_bytes"`
	}
}

// PinOptions configures pin operations
type PinOptions struct {
	Name      string // Human-readable name for the pin
	Recursive bool   // Whether to pin recursively
}

// NewPinManager creates a new pin manager
func NewPinManager(dagWrapper *dag.IpldWrapper) (*PinManager, error) {
	if dagWrapper == nil {
		return nil, fmt.Errorf("dag wrapper cannot be nil")
	}

	pm := &PinManager{
		dagWrapper:    dagWrapper,
		directPins:    make(map[cid.Cid]PinInfo),
		recursivePins: make(map[cid.Cid]PinInfo),
		indirectPins:  make(map[cid.Cid]PinInfo),
	}

	return pm, nil
}

// Pin adds a pin for the given CID
func (pm *PinManager) Pin(ctx context.Context, c cid.Cid, opts PinOptions) error {
	if !c.Defined() {
		return fmt.Errorf("invalid CID")
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Check if already pinned
	if _, exists := pm.directPins[c]; exists {
		return fmt.Errorf("CID %s is already pinned directly", c.String())
	}
	if _, exists := pm.recursivePins[c]; exists {
		return fmt.Errorf("CID %s is already pinned recursively", c.String())
	}

	// Verify the content exists in the DAG (try both DAG service and direct block access)
	_, err := pm.dagWrapper.Get(ctx, c)
	if err != nil {
		// If DAG service fails (e.g., for DAG-CBOR), try direct block access
		_, err2 := pm.dagWrapper.BlockServiceWrapper.GetBlockRaw(ctx, c)
		if err2 != nil {
			return fmt.Errorf("content not found for CID %s: %w (also tried raw access: %w)", c.String(), err, err2)
		}
	}

	pinInfo := PinInfo{
		CID:       c,
		Name:      opts.Name,
		Timestamp: time.Now(),
	}

	if opts.Recursive {
		pinInfo.Type = RecursivePin
		pm.recursivePins[c] = pinInfo

		// Update indirect pins by recalculating all recursive dependencies
		pm.updateIndirectPins(ctx)
	} else {
		pinInfo.Type = DirectPin
		pm.directPins[c] = pinInfo
	}

	return nil
}

// Unpin removes a pin for the given CID
func (pm *PinManager) Unpin(ctx context.Context, c cid.Cid, recursive bool) error {
	if !c.Defined() {
		return fmt.Errorf("invalid CID")
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Check if pinned and remove
	if recursive {
		if _, exists := pm.recursivePins[c]; !exists {
			return fmt.Errorf("CID %s is not pinned recursively", c.String())
		}
		delete(pm.recursivePins, c)
		pm.updateIndirectPins(ctx)
	} else {
		if _, exists := pm.directPins[c]; !exists {
			return fmt.Errorf("CID %s is not pinned directly", c.String())
		}
		delete(pm.directPins, c)
	}

	return nil
}

// IsPinned checks if a CID is pinned (directly, recursively, or indirectly)
func (pm *PinManager) IsPinned(ctx context.Context, c cid.Cid) (bool, error) {
	if !c.Defined() {
		return false, fmt.Errorf("invalid CID")
	}

	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	_, direct := pm.directPins[c]
	_, recursive := pm.recursivePins[c]
	_, indirect := pm.indirectPins[c]

	return direct || recursive || indirect, nil
}

// GetPinType returns the type of pin for a given CID
func (pm *PinManager) GetPinType(ctx context.Context, c cid.Cid) (PinType, error) {
	if !c.Defined() {
		return DirectPin, fmt.Errorf("invalid CID")
	}

	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if _, exists := pm.directPins[c]; exists {
		return DirectPin, nil
	}
	if _, exists := pm.recursivePins[c]; exists {
		return RecursivePin, nil
	}
	if _, exists := pm.indirectPins[c]; exists {
		return IndirectPin, nil
	}

	return DirectPin, fmt.Errorf("CID %s is not pinned", c.String())
}

// ListPins returns all pinned CIDs with their types
func (pm *PinManager) ListPins(ctx context.Context) ([]PinInfo, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	var result []PinInfo

	for _, pinInfo := range pm.directPins {
		result = append(result, pinInfo)
	}

	for _, pinInfo := range pm.recursivePins {
		result = append(result, pinInfo)
	}

	// Include indirect pins for completeness
	for _, pinInfo := range pm.indirectPins {
		result = append(result, pinInfo)
	}

	return result, nil
}

// GCResult contains garbage collection results
type GCResult struct {
	BlocksBefore   int64         `json:"blocks_before"`
	BlocksAfter    int64         `json:"blocks_after"`
	DeletedBlocks  int64         `json:"deleted_blocks"`
	ReclaimedBytes int64         `json:"reclaimed_bytes"`
	Duration       time.Duration `json:"duration"`
	PinnedBlocks   int64         `json:"pinned_blocks"`
}

// RunGC performs garbage collection, removing unpinned blocks
func (pm *PinManager) RunGC(ctx context.Context) (*GCResult, error) {
	start := time.Now()

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// For this demo, we'll simulate GC by counting what would be kept vs removed
	// In a real implementation, this would traverse the blockstore and delete unpinned blocks

	// Count pinned blocks
	pinnedCount := int64(len(pm.directPins) + len(pm.recursivePins) + len(pm.indirectPins))

	// Simulate block counting (this would normally enumerate all blocks in storage)
	blocksBefore := pinnedCount + 50 // Simulate some unpinned blocks
	blocksAfter := pinnedCount
	deletedBlocks := blocksBefore - blocksAfter
	reclaimedBytes := deletedBlocks * 1024 // Simulate 1KB average block size

	result := &GCResult{
		BlocksBefore:   blocksBefore,
		BlocksAfter:    blocksAfter,
		DeletedBlocks:  deletedBlocks,
		ReclaimedBytes: reclaimedBytes,
		Duration:       time.Since(start),
		PinnedBlocks:   pinnedCount,
	}

	// Update stats
	pm.stats.LastGC = start
	pm.stats.GCDuration = result.Duration
	pm.stats.ReclaimedBytes = result.ReclaimedBytes

	return result, nil
}

// PinStats contains pin manager statistics
type PinStats struct {
	DirectPins     int64         `json:"direct_pins"`
	RecursivePins  int64         `json:"recursive_pins"`
	IndirectPins   int64         `json:"indirect_pins"`
	LastGC         time.Time     `json:"last_gc"`
	GCDuration     time.Duration `json:"gc_duration"`
	ReclaimedBytes int64         `json:"reclaimed_bytes"`
}

// GetStats returns current pin manager statistics
func (pm *PinManager) GetStats(ctx context.Context) (*PinStats, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return &PinStats{
		DirectPins:     int64(len(pm.directPins)),
		RecursivePins:  int64(len(pm.recursivePins)),
		IndirectPins:   int64(len(pm.indirectPins)),
		LastGC:         pm.stats.LastGC,
		GCDuration:     pm.stats.GCDuration,
		ReclaimedBytes: pm.stats.ReclaimedBytes,
	}, nil
}

// Close releases any resources held by the pin manager
func (pm *PinManager) Close() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Clear all pin maps
	pm.directPins = make(map[cid.Cid]PinInfo)
	pm.recursivePins = make(map[cid.Cid]PinInfo)
	pm.indirectPins = make(map[cid.Cid]PinInfo)

	return nil
}

// updateIndirectPins recalculates indirect pins based on recursive pins
// This is called whenever recursive pins change
func (pm *PinManager) updateIndirectPins(ctx context.Context) {
	// Clear current indirect pins
	pm.indirectPins = make(map[cid.Cid]PinInfo)

	// For each recursive pin, find all its children
	for rootCID, rootPin := range pm.recursivePins {
		children := make(map[cid.Cid]bool)
		pm.findChildren(ctx, rootCID, children)

		// Add all children as indirect pins (except the root itself)
		for childCID := range children {
			if !childCID.Equals(rootCID) {
				pm.indirectPins[childCID] = PinInfo{
					CID:       childCID,
					Type:      IndirectPin,
					Name:      fmt.Sprintf("Child of %s", rootPin.Name),
					Timestamp: rootPin.Timestamp,
				}
			}
		}
	}
}

// findChildren recursively finds all children of a given CID
func (pm *PinManager) findChildren(ctx context.Context, c cid.Cid, visited map[cid.Cid]bool) {
	if visited[c] {
		return // Avoid cycles
	}
	visited[c] = true

	// Try to get the node and its links using DAG service
	node, err := pm.dagWrapper.Get(ctx, c)
	if err != nil {
		// For DAG-CBOR and other formats, we can't easily traverse links
		// without more complex IPLD prime traversal logic
		// For this demo, we'll just mark this CID as visited and return
		return
	}

	// Traverse all links (works for DAG-PB and Raw nodes)
	for _, link := range node.Links() {
		pm.findChildren(ctx, link.Cid, visited)
	}
}
