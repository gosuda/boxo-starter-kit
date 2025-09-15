package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubo_api "github.com/gosunuts/boxo-starter-kit/99-kubo-api-demo/pkg"
)

func TestKuboAPIOffline(t *testing.T) {
	// These tests don't require a running IPFS node

	t.Run("API Client Creation", func(t *testing.T) {
		// Test default endpoint
		api := kubo_api.NewKuboAPI("")
		assert.NotNil(t, api, "API client should be created")

		// Test custom endpoint
		api2 := kubo_api.NewKuboAPI("http://localhost:5001")
		assert.NotNil(t, api2, "API client should be created with custom endpoint")
	})

	t.Run("Data Structures", func(t *testing.T) {
		// Test NodeInfo structure
		nodeInfo := &kubo_api.NodeInfo{
			ID:        "QmTestNode123",
			PublicKey: "test-public-key",
			Addresses: []string{"/ip4/127.0.0.1/tcp/4001", "/ip6/::1/tcp/4001"},
			Version:   "0.14.0",
		}

		assert.Equal(t, "QmTestNode123", nodeInfo.ID)
		assert.Len(t, nodeInfo.Addresses, 2)

		// Test PinInfo structure
		pinInfo := &kubo_api.PinInfo{
			CID:  "QmTestCID123",
			Type: "recursive",
		}

		assert.Equal(t, "recursive", pinInfo.Type)

		// Test RepoStats structure
		repoStats := &kubo_api.RepoStats{
			RepoSize:   1024 * 1024,
			StorageMax: 10 * 1024 * 1024,
			NumObjects: 100,
			RepoPath:   "/home/user/.ipfs",
			Version:    "11",
		}

		assert.Equal(t, uint64(1024*1024), repoStats.RepoSize)
		assert.Equal(t, uint64(100), repoStats.NumObjects)
	})

	t.Run("CID Parsing", func(t *testing.T) {
		// Test valid CID parsing
		validCIDs := []string{
			"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",              // CIDv0
			"bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi", // CIDv1
		}

		for _, cidStr := range validCIDs {
			cid, err := ParseCID(cidStr)
			require.NoError(t, err, "Should parse valid CID: %s", cidStr)
			assert.True(t, cid.Defined(), "Parsed CID should be defined")
			assert.Equal(t, cidStr, cid.String(), "Parsed CID should match original")
		}

		// Test invalid CID
		invalidCIDs := []string{
			"",
			"invalid-cid",
			"QmInvalidCID",
			"bafyinvalid",
		}

		for _, cidStr := range invalidCIDs {
			_, err := ParseCID(cidStr)
			assert.Error(t, err, "Should fail to parse invalid CID: %s", cidStr)
		}
	})
}

func TestKuboAPIOnline(t *testing.T) {
	// These tests require a running IPFS node
	// Skip if IPFS node is not available
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	api := kubo_api.NewKuboAPI("")

	// Check if node is online
	online, err := api.IsOnline(ctx)
	if !online || err != nil {
		t.Skip("Kubo node not available - skipping online tests")
		return
	}

	t.Run("Node Connection", func(t *testing.T) {
		online, err := api.IsOnline(ctx)
		require.NoError(t, err)
		assert.True(t, online, "Node should be online")
	})

	t.Run("Node Information", func(t *testing.T) {
		nodeInfo, err := api.GetNodeInfo(ctx)
		require.NoError(t, err)

		assert.NotEmpty(t, nodeInfo.ID, "Node ID should not be empty")
		assert.NotEmpty(t, nodeInfo.Version, "Version should not be empty")
		assert.NotNil(t, nodeInfo.Addresses, "Addresses should not be nil")
	})

	t.Run("Repository Statistics", func(t *testing.T) {
		stats, err := api.GetRepoStats(ctx)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, stats.RepoSize, uint64(0), "Repo size should be non-negative")
		assert.Greater(t, stats.StorageMax, uint64(0), "Storage max should be positive")
		assert.GreaterOrEqual(t, stats.NumObjects, uint64(0), "Num objects should be non-negative")
		assert.NotEmpty(t, stats.RepoPath, "Repo path should not be empty")
	})

	t.Run("File Operations", func(t *testing.T) {
		// Add a test file
		testContent := []byte("Hello from Kubo API test!")

		cid, err := api.AddFile(ctx, "test.txt", testContent)
		require.NoError(t, err)
		assert.True(t, cid.Defined(), "CID should be defined")

		// Retrieve the file
		retrieved, err := api.GetFile(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, testContent, retrieved, "Retrieved content should match original")

		// Get object statistics
		stat, err := api.GetObjectStat(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, cid.String(), stat.Hash, "Hash should match CID")
		assert.GreaterOrEqual(t, stat.DataSize, len(testContent), "Data size should be at least content length")
	})

	t.Run("Pin Operations", func(t *testing.T) {
		// Add a file for pinning
		testContent := []byte("Pin test content")
		cid, err := api.AddFile(ctx, "pin-test.txt", testContent)
		require.NoError(t, err)

		// Pin the file
		err = api.PinAdd(ctx, cid)
		require.NoError(t, err)

		// List pins and verify it's there
		pins, err := api.ListPins(ctx)
		require.NoError(t, err)

		_, found := pins[cid.String()]
		assert.True(t, found, "Pinned CID should be in pin list")

		// Unpin the file
		err = api.PinRemove(ctx, cid)
		require.NoError(t, err)
	})

	t.Run("Network Information", func(t *testing.T) {
		// Get connected peers
		peers, err := api.ListConnectedPeers(ctx)
		require.NoError(t, err)
		// Note: May be empty if node has no peers
		assert.NotNil(t, peers, "Peers list should not be nil")

		// Get bootstrap peers
		bootstrap, err := api.GetBootstrapPeers(ctx)
		require.NoError(t, err)
		assert.NotNil(t, bootstrap, "Bootstrap peers should not be nil")
		// Usually has some default bootstrap peers
		assert.Greater(t, len(bootstrap), 0, "Should have some bootstrap peers")
	})

	t.Run("Key Management", func(t *testing.T) {
		// List existing keys
		keys, err := api.ListKeys(ctx)
		require.NoError(t, err)
		assert.NotNil(t, keys, "Keys list should not be nil")

		// Should at least have the 'self' key
		found := false
		for _, key := range keys {
			if key.Name == "self" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have 'self' key")

		// Create a test key
		testKeyName := "test-key-" + time.Now().Format("150405")
		newKey, err := api.CreateKey(ctx, testKeyName, "rsa")
		require.NoError(t, err)
		assert.Equal(t, testKeyName, newKey.Name, "Key name should match")
		assert.NotEmpty(t, newKey.ID, "Key ID should not be empty")
	})

	t.Run("Provider Search", func(t *testing.T) {
		// Add a file and search for providers
		testContent := []byte("Provider search test")
		cid, err := api.AddFile(ctx, "provider-test.txt", testContent)
		require.NoError(t, err)

		// Search for providers (might be empty if content is new)
		providers, err := api.FindProviders(ctx, cid, 5)
		require.NoError(t, err)
		assert.NotNil(t, providers, "Providers list should not be nil")
		// Note: May be empty for newly added content
	})
}

func TestErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with invalid endpoint
	api := kubo_api.NewKuboAPI("http://invalid:9999")

	t.Run("Invalid Node Connection", func(t *testing.T) {
		online, err := api.IsOnline(ctx)
		assert.Error(t, err, "Should fail to connect to invalid endpoint")
		assert.False(t, online, "Node should not be online")
	})

	t.Run("Operations on Invalid Node", func(t *testing.T) {
		// These should all fail gracefully
		_, err := api.GetNodeInfo(ctx)
		assert.Error(t, err, "Should fail to get node info")

		_, err = api.GetRepoStats(ctx)
		assert.Error(t, err, "Should fail to get repo stats")

		_, err = api.ListKeys(ctx)
		assert.Error(t, err, "Should fail to list keys")

		_, err = api.ListConnectedPeers(ctx)
		assert.Error(t, err, "Should fail to list peers")
	})
}
