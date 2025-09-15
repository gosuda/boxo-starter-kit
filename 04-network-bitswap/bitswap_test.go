package main

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dag "github.com/gosuda/boxo-starter-kit/02-dag-ipld/pkg"
	bitswap "github.com/gosuda/boxo-starter-kit/04-network-bitswap/pkg"
)

func TestBitswapNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Node Creation", func(t *testing.T) {
		// Create bitswap node
		node, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node.Close()

		// Verify node properties
		nodeID := node.GetID()
		assert.NotEmpty(t, nodeID.String())

		addrs := node.GetAddresses()
		assert.Greater(t, len(addrs), 0, "Node should have at least one address")

		fullAddrs := node.GetFullAddresses()
		assert.Greater(t, len(fullAddrs), 0, "Node should have full addresses with peer ID")
	})

	t.Run("Block Storage and Retrieval", func(t *testing.T) {
		// Create bitswap node
		node, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node.Close()
		// Store a block
		testData := []byte("Test block data for bitswap")
		cid, err := node.PutBlock(ctx, testData)
		require.NoError(t, err)
		assert.True(t, cid.Defined(), "CID should be valid")

		// Retrieve the block locally (from own store)
		retrievedData, err := node.GetBlock(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, testData, retrievedData, "Retrieved data should match original")

		// Check stats
		stats := node.GetStats()
		assert.Greater(t, stats.BlocksSent, int64(0), "Should have sent at least one block")
		assert.Greater(t, stats.BlocksReceived, int64(0), "Should have received at least one block")
	})

	t.Run("Two Node Connection", func(t *testing.T) {
		// Create first node
		node1, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node1.Close()

		// Create second node
		node2, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node2.Close()

		// Connect nodes
		node2Addrs := node2.GetFullAddresses()
		require.Greater(t, len(node2Addrs), 0, "Node 2 should have addresses")

		err = node1.ConnectToPeer(ctx, node2Addrs[0])
		require.NoError(t, err)

		// Wait for connection
		time.Sleep(1 * time.Second)

		// Check connection stats
		peers1 := node1.GetConnectedPeers()
		peers2 := node2.GetConnectedPeers()

		// Both nodes should see each other as connected
		assert.Contains(t, peers1, node2.GetID(), "Node 1 should see Node 2 as connected")
		assert.Contains(t, peers2, node1.GetID(), "Node 2 should see Node 1 as connected")
	})

	t.Run("Error Handling", func(t *testing.T) {
		// Test with nil DAG wrapper
		_, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{})
		assert.Error(t, err, "Should fail with nil DAG wrapper")

		// Create valid node for other error tests
		dagWrapper, err := dag.New(nil, "")
		require.NoError(t, err)
		defer dagWrapper.Close()
		node, err := bitswap.NewBitswapNode(dagWrapper, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node.Close()

		// Test with empty data
		_, err = node.PutBlock(ctx, []byte{})
		assert.Error(t, err, "Should fail with empty data")

		// Test with invalid CID
		_, err = node.GetBlock(ctx, cid.Undef)
		assert.Error(t, err, "Should fail with undefined CID")
	})

	t.Run("Statistics Tracking", func(t *testing.T) {
		// Create bitswap node
		node, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node.Close()

		// Initial stats
		initialStats := node.GetStats()
		assert.Equal(t, int64(0), initialStats.BlocksSent)
		assert.Equal(t, int64(0), initialStats.BlocksReceived)
		assert.NotEmpty(t, initialStats.NodeID)

		// Store a block
		testData := []byte("Statistics test data")
		cid, err := node.PutBlock(ctx, testData)
		require.NoError(t, err)

		// Retrieve the block
		_, err = node.GetBlock(ctx, cid)
		require.NoError(t, err)

		// Check updated stats
		finalStats := node.GetStats()
		assert.Greater(t, finalStats.BlocksSent, initialStats.BlocksSent, "Blocks sent should increase")
		assert.Greater(t, finalStats.BlocksReceived, initialStats.BlocksReceived, "Blocks received should increase")
	})
}

func TestBitswapNodeConfig(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		// Test with empty config (should use defaults)
		node, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{})
		require.NoError(t, err)
		defer node.Close()

		addrs := node.GetAddresses()
		assert.Greater(t, len(addrs), 0, "Should have default addresses")
	})

	t.Run("Custom Listen Addresses", func(t *testing.T) {
		node, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{
				"/ip4/127.0.0.1/tcp/0",
				"/ip6/::1/tcp/0",
			},
		})
		require.NoError(t, err)
		defer node.Close()

		addrs := node.GetAddresses()
		assert.GreaterOrEqual(t, len(addrs), 1, "Should have at least one address")
	})

	t.Run("Invalid Listen Address", func(t *testing.T) {
		_, err := bitswap.NewBitswapNode(nil, &bitswap.NodeConfig{
			ListenAddrs: []string{"invalid-address"},
		})
		assert.Error(t, err, "Should fail with invalid address")
	})
}
