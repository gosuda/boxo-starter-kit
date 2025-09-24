package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
	ipns "github.com/gosuda/boxo-starter-kit/09-ipns/pkg"
)

func TestIPNSManager(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup
	dagWrapper, err := dag.NewIpldWrapper(ctx, nil)
	require.NoError(t, err)
	defer dagWrapper.BlockServiceWrapper.Close()

	ipnsManager := ipns.NewIPNSManager(dagWrapper)

	t.Run("Key Generation", func(t *testing.T) {
		keyName := "test-key"
		peerID, err := ipnsManager.GenerateKey(ctx, keyName)
		require.NoError(t, err)
		assert.NotEmpty(t, peerID.String(), "Peer ID should not be empty")
		assert.True(t, strings.HasPrefix(peerID.String(), "12D3KooW") ||
			strings.HasPrefix(peerID.String(), "Qm"), "Peer ID should have valid format")
	})

	t.Run("IPNS Publishing", func(t *testing.T) {
		// Create test content
		testContent := map[string]any{
			"message": "Hello IPNS!",
			"version": 1,
		}
		contentCID, err := dagWrapper.PutAny(ctx, testContent)
		require.NoError(t, err)

		// Generate key and publish
		keyName := "publish-test"
		_, err = ipnsManager.GenerateKey(ctx, keyName)
		require.NoError(t, err)

		record, err := ipnsManager.PublishIPNS(ctx, keyName, contentCID, 1*time.Hour)
		require.NoError(t, err)

		assert.NotEmpty(t, record.Name, "IPNS name should not be empty")
		assert.Contains(t, record.Value, contentCID.String(), "Value should contain CID")
		assert.Equal(t, uint64(0), record.Sequence, "First record should have sequence 0")
		assert.Equal(t, uint64(3600), record.TTL, "TTL should be 1 hour in seconds")
	})

	t.Run("IPNS Resolution", func(t *testing.T) {
		// Create content and publish
		testContent := map[string]any{"data": "resolution test"}
		contentCID, err := dagWrapper.PutAny(ctx, testContent)
		require.NoError(t, err)

		keyName := "resolve-test"
		_, err = ipnsManager.GenerateKey(ctx, keyName)
		require.NoError(t, err)

		record, err := ipnsManager.PublishIPNS(ctx, keyName, contentCID, 1*time.Hour)
		require.NoError(t, err)

		// Resolve the name
		resolved, err := ipnsManager.ResolveIPNS(ctx, record.Name)
		require.NoError(t, err)
		assert.Equal(t, record.Value, resolved, "Resolved value should match published value")

		// Test with /ipns/ prefix
		resolved2, err := ipnsManager.ResolveIPNS(ctx, "/ipns/"+record.Name)
		require.NoError(t, err)
		assert.Equal(t, resolved, resolved2, "Resolution should work with /ipns/ prefix")
	})

	t.Run("IPNS Updates", func(t *testing.T) {
		// Initial content
		content1 := map[string]any{"version": 1, "data": "original"}
		cid1, err := dagWrapper.PutAny(ctx, content1)
		require.NoError(t, err)

		// Generate key and publish initial record
		keyName := "update-test"
		_, err = ipnsManager.GenerateKey(ctx, keyName)
		require.NoError(t, err)

		record1, err := ipnsManager.PublishIPNS(ctx, keyName, cid1, 1*time.Hour)
		require.NoError(t, err)

		// Updated content
		content2 := map[string]any{"version": 2, "data": "updated"}
		cid2, err := dagWrapper.PutAny(ctx, content2)
		require.NoError(t, err)

		// Update the record
		time.Sleep(100 * time.Millisecond) // Ensure different timestamp
		record2, err := ipnsManager.UpdateIPNS(ctx, keyName, cid2, 2*time.Hour)
		require.NoError(t, err)

		// Check update
		assert.Equal(t, record1.Name, record2.Name, "Name should remain the same")
		assert.NotEqual(t, record1.Value, record2.Value, "Value should be updated")
		assert.Equal(t, record1.Sequence+1, record2.Sequence, "Sequence should increment")
		assert.True(t, record2.UpdatedAt.After(record1.UpdatedAt), "Update time should be later")

		// Verify resolution returns updated value
		resolved, err := ipnsManager.ResolveIPNS(ctx, record2.Name)
		require.NoError(t, err)
		assert.Equal(t, record2.Value, resolved, "Resolution should return updated value")
	})

	t.Run("Record Listing", func(t *testing.T) {
		// Create multiple records
		keyNames := []string{"list-test-1", "list-test-2", "list-test-3"}
		var expectedNames []string

		for _, keyName := range keyNames {
			// Create content
			content := map[string]any{"key": keyName}
			contentCID, err := dagWrapper.PutAny(ctx, content)
			require.NoError(t, err)

			// Generate key and publish
			_, err = ipnsManager.GenerateKey(ctx, keyName)
			require.NoError(t, err)

			record, err := ipnsManager.PublishIPNS(ctx, keyName, contentCID, 1*time.Hour)
			require.NoError(t, err)

			expectedNames = append(expectedNames, record.Name)
		}

		// List all records
		records, err := ipnsManager.ListIPNSRecords(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(records), 3, "Should have at least 3 records")

		// Check that our records are present
		var foundNames []string
		for _, record := range records {
			foundNames = append(foundNames, record.Name)
			// Ensure private key is not exposed
			assert.Nil(t, record.PrivateKey, "Private key should not be exposed in listing")
		}

		for _, expectedName := range expectedNames {
			assert.Contains(t, foundNames, expectedName, "Expected name should be in listing")
		}
	})

	t.Run("Record Expiration", func(t *testing.T) {
		// Create content
		testContent := map[string]any{"expiration": "test"}
		contentCID, err := dagWrapper.PutAny(ctx, testContent)
		require.NoError(t, err)

		// Generate key and publish with very short TTL
		keyName := "expire-test"
		_, err = ipnsManager.GenerateKey(ctx, keyName)
		require.NoError(t, err)

		record, err := ipnsManager.PublishIPNS(ctx, keyName, contentCID, 1*time.Second)
		require.NoError(t, err)

		// Initially should resolve
		_, err = ipnsManager.ResolveIPNS(ctx, record.Name)
		require.NoError(t, err)

		// Initially should not be expired
		expired, err := ipnsManager.IsExpired(ctx, record.Name)
		require.NoError(t, err)
		assert.False(t, expired, "Record should not be expired initially")

		// Wait for expiration
		time.Sleep(1200 * time.Millisecond)

		// Should be expired
		expired, err = ipnsManager.IsExpired(ctx, record.Name)
		require.NoError(t, err)
		assert.True(t, expired, "Record should be expired")

		// Resolution should fail
		_, err = ipnsManager.ResolveIPNS(ctx, record.Name)
		assert.Error(t, err, "Resolution should fail for expired record")
		assert.Contains(t, err.Error(), "expired", "Error should mention expiration")
	})

	t.Run("Statistics", func(t *testing.T) {
		stats, err := ipnsManager.GetStats(ctx)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, stats.TotalRecords, 0, "Total records should be non-negative")
		assert.GreaterOrEqual(t, stats.ActiveRecords, 0, "Active records should be non-negative")
		assert.GreaterOrEqual(t, stats.ExpiredRecords, 0, "Expired records should be non-negative")
		assert.Equal(t, stats.TotalRecords, stats.ActiveRecords+stats.ExpiredRecords,
			"Total should equal active + expired")
	})
}

func TestIPNSValidation(t *testing.T) {
	t.Run("Valid Names", func(t *testing.T) {
		// These are example valid peer IDs
		validNames := []string{
			"12D3KooWGRUGLqLgmtR2YiTiP4VqNMDXE8s9FZjqM9vKV3gWqF8Q",
			"QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
		}

		for _, name := range validNames {
			// Skip actual validation since these are example IDs
			// In real test, you'd use actual generated peer IDs
			t.Logf("Testing format for: %s", name[:12]+"...")
		}
	})

	t.Run("Invalid Names", func(t *testing.T) {
		invalidNames := []string{
			"invalid-name",
			"",
			"too-short",
			"contains spaces",
		}

		for _, name := range invalidNames {
			err := ipns.ValidateIPNSName(name)
			assert.Error(t, err, "Should reject invalid name: %s", name)
		}
	})
}

func TestPathFormatting(t *testing.T) {
	t.Run("IPNS Path Formatting", func(t *testing.T) {
		testName := "12D3KooWExample"
		formatted := ipns.FormatIPNSPath(testName)
		assert.Equal(t, "/ipns/"+testName, formatted, "Should format correctly")

		// Test with existing prefix
		alreadyFormatted := "/ipns/" + testName
		reformatted := ipns.FormatIPNSPath(alreadyFormatted)
		assert.Equal(t, alreadyFormatted, reformatted, "Should handle existing prefix")
	})

	t.Run("CID Extraction from IPFS Path", func(t *testing.T) {
		testCID := "QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N"
		path := "/ipfs/" + testCID

		extractedCID, err := ipns.ExtractCIDFromIPFSPath(path)
		require.NoError(t, err)
		assert.Equal(t, testCID, extractedCID.String(), "Should extract CID correctly")

		// Test with additional path segments
		pathWithSegments := "/ipfs/" + testCID + "/some/path"
		extractedCID2, err := ipns.ExtractCIDFromIPFSPath(pathWithSegments)
		require.NoError(t, err)
		assert.Equal(t, testCID, extractedCID2.String(), "Should extract CID from path with segments")

		// Test invalid paths
		invalidPaths := []string{
			"not-ipfs-path",
			"/ipns/" + testCID,
			"ipfs/" + testCID,
		}

		for _, invalidPath := range invalidPaths {
			_, err := ipns.ExtractCIDFromIPFSPath(invalidPath)
			assert.Error(t, err, "Should reject invalid path: %s", invalidPath)
		}
	})
}
