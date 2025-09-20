package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	network "github.com/gosuda/boxo-starter-kit/02-network/pkg"
)

func TestHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Node Creation", func(t *testing.T) {
		// Create node
		node, err := network.New(&network.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node.Close()

		// Verify node properties
		nodeID := node.ID()
		assert.NotEmpty(t, nodeID.String())

		addrs := node.Addrs()
		assert.Greater(t, len(addrs), 0, "Node should have at least one address")

		fullAddrs := node.GetFullAddresses()
		assert.Greater(t, len(fullAddrs), 0, "Node should have full addresses with peer ID")
	})

	t.Run("Two Node Connection", func(t *testing.T) {
		// Create first node
		node1, err := network.New(&network.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		require.NoError(t, err)
		defer node1.Close()

		// Create second node
		node2, err := network.New(&network.Config{
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
		peers1 := node1.Peers()
		peers2 := node2.Peers()

		// Both nodes should see each other as connected
		assert.Contains(t, peers1, node2.ID(), "Node 1 should see Node 2 as connected")
		assert.Contains(t, peers2, node1.ID(), "Node 2 should see Node 1 as connected")
	})

	t.Run("Two Nodes Connection: Send/Receive", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		node1, _ := network.New(&network.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		defer node1.Close()
		node2, _ := network.New(&network.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		})
		defer node2.Close()

		require.NoError(t, node2.ConnectToPeer(ctx, node1.GetFullAddresses()[0]))

		payload := []byte("hi data")
		cid, err := node2.Send(ctx, node1.ID(), payload)
		require.NoError(t, err)

		from, got, err := node1.Receive(ctx, cid)
		require.NoError(t, err)
		assert.Equal(t, from, node2.ID())
		assert.Equal(t, payload, got)
	})

}

func TestConfig(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		// Test with empty config (should use defaults)
		node, err := network.New(&network.Config{})
		require.NoError(t, err)
		defer node.Close()

		addrs := node.GetFullAddresses()
		assert.Greater(t, len(addrs), 0, "Should have default addresses")
	})

	t.Run("Custom Listen Addresses", func(t *testing.T) {
		node, err := network.New(&network.Config{
			ListenAddrs: []string{
				"/ip4/127.0.0.1/tcp/0",
				"/ip6/::1/tcp/0",
			},
		})
		require.NoError(t, err)
		defer node.Close()

		addrs := node.GetFullAddresses()
		assert.GreaterOrEqual(t, len(addrs), 1, "Should have at least one address")
	})

	t.Run("Invalid Listen Address", func(t *testing.T) {
		_, err := network.New(&network.Config{
			ListenAddrs: []string{"invalid-address"},
		})
		assert.Error(t, err, "Should fail with invalid address")
	})
}
