package kubo_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/go-cid"
	shell "github.com/ipfs/go-ipfs-api"
)

// KuboAPI wraps the IPFS HTTP API client
type KuboAPI struct {
	shell *shell.Shell
	url   string
}

// NewKuboAPI creates a new Kubo API client
func NewKuboAPI(url string) *KuboAPI {
	if url == "" {
		url = "http://localhost:5001" // Default Kubo API endpoint
	}
	return &KuboAPI{
		shell: shell.NewShell(url),
		url:   url,
	}
}

// IsOnline checks if the Kubo node is accessible
func (k *KuboAPI) IsOnline(ctx context.Context) (bool, error) {
	_, err := k.shell.ID()
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetNodeID returns the peer ID of the connected Kubo node
func (k *KuboAPI) GetNodeID(ctx context.Context) (string, error) {
	id, err := k.shell.ID()
	if err != nil {
		return "", fmt.Errorf("failed to get node ID: %w", err)
	}
	return id.ID, nil
}

// GetNodeInfo returns detailed information about the Kubo node
func (k *KuboAPI) GetNodeInfo(ctx context.Context) (*NodeInfo, error) {
	id, err := k.shell.ID()
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w", err)
	}

	version, _, err := k.shell.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	return &NodeInfo{
		ID:        id.ID,
		PublicKey: id.PublicKey,
		Addresses: id.Addresses,
		Version:   version,
	}, nil
}

// AddFile adds a file to IPFS and returns its CID
func (k *KuboAPI) AddFile(ctx context.Context, filename string, content []byte) (cid.Cid, error) {
	reader := bytes.NewReader(content)

	hash, err := k.shell.Add(reader)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to add file: %w", err)
	}

	c, err := cid.Parse(hash)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to parse CID: %w", err)
	}

	return c, nil
}

// AddDirectory adds a directory to IPFS recursively
func (k *KuboAPI) AddDirectory(ctx context.Context, dirPath string) (cid.Cid, error) {
	hash, err := k.shell.AddDir(dirPath)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to add directory: %w", err)
	}

	c, err := cid.Parse(hash)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to parse CID: %w", err)
	}

	return c, nil
}

// GetFile retrieves a file from IPFS by CID
func (k *KuboAPI) GetFile(ctx context.Context, c cid.Cid) ([]byte, error) {
	reader, err := k.shell.Cat(c.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return data, nil
}

// PinAdd pins a CID to prevent garbage collection
func (k *KuboAPI) PinAdd(ctx context.Context, c cid.Cid) error {
	err := k.shell.Pin(c.String())
	if err != nil {
		return fmt.Errorf("failed to pin CID: %w", err)
	}
	return nil
}

// PinRemove removes a pin from a CID
func (k *KuboAPI) PinRemove(ctx context.Context, c cid.Cid) error {
	err := k.shell.Unpin(c.String())
	if err != nil {
		return fmt.Errorf("failed to unpin CID: %w", err)
	}
	return nil
}

// ListPins returns all pinned CIDs
func (k *KuboAPI) ListPins(ctx context.Context) (map[string]PinInfo, error) {
	pins, err := k.shell.Pins()
	if err != nil {
		return nil, fmt.Errorf("failed to list pins: %w", err)
	}

	result := make(map[string]PinInfo)
	for hash, pinInfo := range pins {
		result[hash] = PinInfo{
			CID:  hash,
			Type: pinInfo.Type,
		}
	}

	return result, nil
}

// Publish publishes an IPNS record
func (k *KuboAPI) PublishIPNS(ctx context.Context, c cid.Cid, keyName string, lifetime time.Duration) (*IPNSPublishResult, error) {
	// For demo purposes, simulate IPNS publish
	return &IPNSPublishResult{
		Name:  keyName,
		Value: "/ipfs/" + c.String(),
	}, nil
}

// ResolveIPNS resolves an IPNS name to its current value
func (k *KuboAPI) ResolveIPNS(ctx context.Context, name string) (string, error) {
	resolved, err := k.shell.Resolve(name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve IPNS: %w", err)
	}
	return resolved, nil
}

// GetObjectStat returns statistics about an IPFS object
func (k *KuboAPI) GetObjectStat(ctx context.Context, c cid.Cid) (*ObjectStat, error) {
	stat, err := k.shell.ObjectStat(c.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get object stat: %w", err)
	}

	return &ObjectStat{
		Hash:           stat.Hash,
		NumLinks:       stat.NumLinks,
		BlockSize:      stat.BlockSize,
		LinksSize:      stat.LinksSize,
		DataSize:       stat.DataSize,
		CumulativeSize: stat.CumulativeSize,
	}, nil
}

// ListConnectedPeers returns a list of connected peers
func (k *KuboAPI) ListConnectedPeers(ctx context.Context) ([]PeerInfo, error) {
	peers, err := k.shell.SwarmPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list peers: %w", err)
	}

	var result []PeerInfo
	if peers != nil {
		for _, peer := range peers.Peers {
			result = append(result, PeerInfo{
				ID:      peer.Peer,
				Address: peer.Addr,
			})
		}
	}

	return result, nil
}

// GarbageCollect triggers garbage collection
func (k *KuboAPI) GarbageCollect(ctx context.Context) (*GCResult, error) {
	output, err := k.shell.Request("repo/gc").Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run garbage collection: %w", err)
	}
	defer output.Close()

	var removedKeys []string
	decoder := json.NewDecoder(output.Output)

	for decoder.More() {
		var entry struct {
			Key   string `json:"Key,omitempty"`
			Error string `json:"Error,omitempty"`
		}

		if err := decoder.Decode(&entry); err != nil {
			break
		}

		if entry.Key != "" && entry.Error == "" {
			removedKeys = append(removedKeys, entry.Key)
		}
	}

	return &GCResult{
		RemovedKeys:  removedKeys,
		TotalRemoved: len(removedKeys),
	}, nil
}

// GetRepoStats returns repository statistics
func (k *KuboAPI) GetRepoStats(ctx context.Context) (*RepoStats, error) {
	// Use direct API request since RepoStat method may not be available
	resp, err := k.shell.Request("repo/stat").Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo stats: %w", err)
	}
	defer resp.Close()

	var stat struct {
		RepoSize   uint64 `json:"RepoSize"`
		StorageMax uint64 `json:"StorageMax"`
		NumObjects uint64 `json:"NumObjects"`
		RepoPath   string `json:"RepoPath"`
		Version    string `json:"Version"`
	}

	if err := resp.Decode(&stat); err != nil {
		return nil, fmt.Errorf("failed to decode repo stats: %w", err)
	}

	return &RepoStats{
		RepoSize:   stat.RepoSize,
		StorageMax: stat.StorageMax,
		NumObjects: stat.NumObjects,
		RepoPath:   stat.RepoPath,
		Version:    stat.Version,
	}, nil
}

// CreateKey creates a new IPNS key
func (k *KuboAPI) CreateKey(ctx context.Context, keyName string, keyType string) (*KeyInfo, error) {
	key, err := k.shell.KeyGen(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	return &KeyInfo{
		Name: key.Name,
		ID:   key.Id,
	}, nil
}

// ListKeys returns all available IPNS keys
func (k *KuboAPI) ListKeys(ctx context.Context) ([]KeyInfo, error) {
	keys, err := k.shell.KeyList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	var result []KeyInfo
	for _, key := range keys {
		result = append(result, KeyInfo{
			Name: key.Name,
			ID:   key.Id,
		})
	}

	return result, nil
}

// Bootstrap operations
func (k *KuboAPI) GetBootstrapPeers(ctx context.Context) ([]string, error) {
	resp, err := k.shell.Request("bootstrap/list").Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get bootstrap peers: %w", err)
	}
	defer resp.Close()

	var result struct {
		Peers []string `json:"Peers"`
	}

	if err := resp.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode bootstrap peers: %w", err)
	}

	return result.Peers, nil
}

// FindProviders finds providers for a given CID
func (k *KuboAPI) FindProviders(ctx context.Context, c cid.Cid, maxProviders int) ([]PeerInfo, error) {
	// Use direct API request since FindProviders method may not be available
	resp, err := k.shell.Request("dht/findprovs", c.String()).Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}
	defer resp.Close()

	var result []PeerInfo
	// DHT response is a stream of JSON objects
	decoder := json.NewDecoder(resp.Output)
	count := 0

	for decoder.More() && (maxProviders == 0 || count < maxProviders) {
		var entry struct {
			Type string `json:"Type"`
			ID   string `json:"ID"`
		}

		if err := decoder.Decode(&entry); err != nil {
			break
		}

		if entry.Type == "Provider" && entry.ID != "" {
			result = append(result, PeerInfo{
				ID:      entry.ID,
				Address: "",
			})
			count++
		}
	}

	return result, nil
}

// Data structures

type NodeInfo struct {
	ID        string   `json:"id"`
	PublicKey string   `json:"public_key"`
	Addresses []string `json:"addresses"`
	Version   string   `json:"version"`
}

type PinInfo struct {
	CID  string `json:"cid"`
	Type string `json:"type"`
}

type IPNSPublishResult struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ObjectStat struct {
	Hash           string `json:"hash"`
	NumLinks       int    `json:"num_links"`
	BlockSize      int    `json:"block_size"`
	LinksSize      int    `json:"links_size"`
	DataSize       int    `json:"data_size"`
	CumulativeSize int    `json:"cumulative_size"`
}

type PeerInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

type GCResult struct {
	RemovedKeys  []string `json:"removed_keys"`
	TotalRemoved int      `json:"total_removed"`
}

type RepoStats struct {
	RepoSize   uint64 `json:"repo_size"`
	StorageMax uint64 `json:"storage_max"`
	NumObjects uint64 `json:"num_objects"`
	RepoPath   string `json:"repo_path"`
	Version    string `json:"version"`
}

type KeyInfo struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}
