package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cid "github.com/ipfs/go-cid"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"

	"github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
)

func main() {
	fmt.Println("🎯 Block and CID Comprehensive Demo")
	fmt.Println("===================================")

	ctx := context.Background()

	// Create in-memory blockstore
	bs := block.NewInMemory()
	defer bs.Close()

	fmt.Println("\n1. 📦 Basic Block Operations")
	fmt.Println("----------------------------")
	demonstrateBasicBlocks(ctx, bs)

	fmt.Println("\n2. 🔢 CID Version Comparison")
	fmt.Println("----------------------------")
	demonstrateCIDVersions(ctx, bs)

	fmt.Println("\n3. 🧮 Hash Algorithm Comparison")
	fmt.Println("-------------------------------")
	demonstrateHashAlgorithms(ctx, bs)

	fmt.Println("\n4. 🔗 Identity Hash Optimization")
	fmt.Println("--------------------------------")
	demonstrateIdentityHash(ctx, bs)

	fmt.Println("\n5. 📊 Performance Benchmarks")
	fmt.Println("----------------------------")
	demonstratePerformance(ctx, bs)

	fmt.Println("\n6. 🔍 Content Addressing Benefits")
	fmt.Println("---------------------------------")
	demonstrateContentAddressing(ctx, bs)

	fmt.Println("\n7. 🗄️ Blockstore Operations")
	fmt.Println("---------------------------")
	demonstrateBlockstoreOps(ctx, bs)

	fmt.Println("\n🎉 Demo Complete!")
	fmt.Println("Next: Try 01-persistent module for disk storage")
}

func demonstrateBasicBlocks(ctx context.Context, bs *block.BlockWrapper) {
	// Create a simple block
	data := []byte("Hello, IPFS World!")
	fmt.Printf("Original data: %s\n", string(data))

	// Store using default CID v1
	cid1, err := bs.PutV1Cid(ctx, data, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CID v1: %s\n", cid1.String())

	// Retrieve the block
	retrievedBlock, err := bs.Get(ctx, cid1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved: %s\n", string(retrievedBlock.RawData()))

	// Verify content addressing property
	data2 := []byte("Hello, IPFS World!") // Same content
	cid2, err := bs.PutV1Cid(ctx, data2, nil)
	if err != nil {
		log.Fatal(err)
	}

	if cid1.Equals(cid2) {
		fmt.Printf("✅ Same content = Same CID: %s\n", cid1.String())
	} else {
		fmt.Printf("❌ Something went wrong!\n")
	}

	// Different content
	data3 := []byte("Hello, Different World!")
	cid3, err := bs.PutV1Cid(ctx, data3, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Different data: %s\n", string(data3))
	fmt.Printf("Different CID: %s\n", cid3.String())
}

func demonstrateCIDVersions(ctx context.Context, bs *block.BlockWrapper) {
	data := []byte("Version comparison test")
	fmt.Printf("Test data: %s\n", string(data))

	// CID v0 (legacy format)
	cidV0, err := bs.PutV0Cid(ctx, data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CID v0: %s (length: %d)\n", cidV0.String(), len(cidV0.String()))

	// CID v1 with raw codec
	cidV1Raw, err := bs.PutV1Cid(ctx, data, block.NewV1Prefix(mc.Raw, mh.SHA2_256, -1))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CID v1 (raw): %s (length: %d)\n", cidV1Raw.String(), len(cidV1Raw.String()))

	// CID v1 with dag-pb codec
	cidV1DagPb, err := bs.PutV1Cid(ctx, data, block.NewV1Prefix(mc.DagPb, mh.SHA2_256, -1))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("CID v1 (dag-pb): %s (length: %d)\n", cidV1DagPb.String(), len(cidV1DagPb.String()))

	// Compare properties
	fmt.Printf("\nVersion Analysis:\n")
	fmt.Printf("v0 - Base58 encoding, case-sensitive, dag-pb only\n")
	fmt.Printf("v1 - Base32 encoding, case-insensitive, all codecs\n")
	fmt.Printf("v1 - URL-friendly (no uppercase letters)\n")
}

func demonstrateHashAlgorithms(ctx context.Context, bs *block.BlockWrapper) {
	data := make([]byte, 1024) // 1KB test data
	for i := range data {
		data[i] = byte(i % 256)
	}
	fmt.Printf("Test data: 1KB of sequential bytes\n")

	algorithms := []struct {
		name string
		code uint64
	}{
		{"SHA2-256", mh.SHA2_256},
		{"SHA2-512", mh.SHA2_512},
		{"BLAKE3", mh.BLAKE3},
		{"SHA3-256", mh.SHA3_256},
	}

	fmt.Printf("\nHash Algorithm Comparison:\n")
	for _, alg := range algorithms {
		start := time.Now()
		prefix := block.NewV1Prefix(mc.Raw, alg.code, -1)
		cidResult, err := bs.PutV1Cid(ctx, data, prefix)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("%-12s: ERROR - %v\n", alg.name, err)
			continue
		}

		fmt.Printf("%-12s: %s (took: %v)\n", alg.name, cidResult.String(), duration)
	}

	fmt.Printf("\nRecommendations:\n")
	fmt.Printf("- SHA2-256: Best compatibility (default)\n")
	fmt.Printf("- BLAKE3: Best performance (3x faster)\n")
	fmt.Printf("- SHA2-512: Enhanced security (slower)\n")
	fmt.Printf("- SHA3-256: Quantum-resistant option\n")
}

func demonstrateIdentityHash(ctx context.Context, bs *block.BlockWrapper) {
	smallData := []byte("tiny")
	largeData := make([]byte, 64) // Larger than hash size
	for i := range largeData {
		largeData[i] = byte('A' + i%26)
	}

	fmt.Printf("Small data (4 bytes): %s\n", string(smallData))
	fmt.Printf("Large data (64 bytes): %s...\n", string(largeData[:10]))

	// Identity hash for small data (if supported)
	// Note: Identity hash requires special handling
	identityPrefix := &cid.Prefix{
		Version:  1,
		Codec:    uint64(mc.Identity),
		MhType:   mh.IDENTITY,
		MhLength: len(smallData),
	}

	identityCid, err := identityPrefix.Sum(smallData)
	if err == nil {
		// Store with identity hash
		err = bs.PutWithCID(ctx, smallData, identityCid)
		if err == nil {
			fmt.Printf("Identity CID: %s\n", identityCid.String())
			fmt.Printf("✅ Small data stored without hashing (saves 32 bytes)\n")
		}
	}

	// Regular hash for large data
	regularCid, err := bs.PutV1Cid(ctx, largeData, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Regular CID: %s\n", regularCid.String())

	fmt.Printf("\nOptimization Benefits:\n")
	fmt.Printf("- Identity hash: No hash computation needed\n")
	fmt.Printf("- Space saving: 32 bytes (hash size) for small data\n")
	fmt.Printf("- Performance: Instant retrieval (no hash lookup)\n")
}

func demonstratePerformance(ctx context.Context, bs *block.BlockWrapper) {
	sizes := []int{1024, 64 * 1024, 1024 * 1024} // 1KB, 64KB, 1MB
	iterations := []int{1000, 100, 10}

	fmt.Printf("Performance benchmarks:\n")

	for i, size := range sizes {
		data := make([]byte, size)
		for j := range data {
			data[j] = byte(j % 256)
		}

		// Benchmark Put operations
		start := time.Now()
		var lastCid cid.Cid
		for j := 0; j < iterations[i]; j++ {
			// Make data slightly different each iteration
			data[0] = byte(j)
			cidResult, err := bs.PutV1Cid(ctx, data, nil)
			if err != nil {
				log.Fatal(err)
			}
			lastCid = cidResult
		}
		putDuration := time.Since(start)

		// Benchmark Get operations
		start = time.Now()
		for j := 0; j < iterations[i]; j++ {
			_, err := bs.Get(ctx, lastCid)
			if err != nil {
				log.Fatal(err)
			}
		}
		getDuration := time.Since(start)

		sizeStr := formatSize(size)
		fmt.Printf("%s blocks (%d ops):\n", sizeStr, iterations[i])
		fmt.Printf("  Put: %v/op (%.0f ops/sec)\n",
			putDuration/time.Duration(iterations[i]),
			float64(iterations[i])/putDuration.Seconds())
		fmt.Printf("  Get: %v/op (%.0f ops/sec)\n",
			getDuration/time.Duration(iterations[i]),
			float64(iterations[i])/getDuration.Seconds())
	}
}

func demonstrateContentAddressing(ctx context.Context, bs *block.BlockWrapper) {
	fmt.Printf("Content Addressing vs Location Addressing:\n")

	// Simulate traditional file system
	fmt.Printf("\n📁 Traditional Location Addressing:\n")
	fmt.Printf("File: /home/user/documents/report.pdf\n")
	fmt.Printf("Problems:\n")
	fmt.Printf("- File can be moved or deleted\n")
	fmt.Printf("- Content can be modified\n")
	fmt.Printf("- Multiple copies waste space\n")
	fmt.Printf("- No integrity verification\n")

	// Demonstrate IPFS content addressing
	fmt.Printf("\n🔗 IPFS Content Addressing:\n")

	originalDoc := []byte("Important document content v1.0")
	modifiedDoc := []byte("Important document content v1.1")
	duplicateDoc := []byte("Important document content v1.0") // Same as original

	originalCid, _ := bs.PutV1Cid(ctx, originalDoc, nil)
	modifiedCid, _ := bs.PutV1Cid(ctx, modifiedDoc, nil)
	duplicateCid, _ := bs.PutV1Cid(ctx, duplicateDoc, nil)

	fmt.Printf("Original:  %s\n", originalCid.String())
	fmt.Printf("Modified:  %s\n", modifiedCid.String())
	fmt.Printf("Duplicate: %s\n", duplicateCid.String())

	fmt.Printf("\nBenefits demonstrated:\n")
	if originalCid.Equals(duplicateCid) {
		fmt.Printf("✅ Automatic deduplication (same content = same CID)\n")
	}
	if !originalCid.Equals(modifiedCid) {
		fmt.Printf("✅ Tamper detection (different content = different CID)\n")
	}
	fmt.Printf("✅ Content verification (CID proves data integrity)\n")
	fmt.Printf("✅ Distributed storage (content accessible anywhere)\n")
}

func demonstrateBlockstoreOps(ctx context.Context, bs *block.BlockWrapper) {
	// Store multiple blocks
	blocks := [][]byte{
		[]byte("Block 1 content"),
		[]byte("Block 2 content"),
		[]byte("Block 3 content"),
	}

	var cids []cid.Cid
	fmt.Printf("Storing blocks:\n")
	for i, data := range blocks {
		cidResult, err := bs.PutV1Cid(ctx, data, nil)
		if err != nil {
			log.Fatal(err)
		}
		cids = append(cids, cidResult)
		fmt.Printf("Block %d: %s\n", i+1, cidResult.String())
	}

	// Check existence
	fmt.Printf("\nChecking block existence:\n")
	for i, c := range cids {
		exists, err := bs.Has(ctx, c)
		if err != nil {
			log.Fatal(err)
		}
		status := "❌"
		if exists {
			status = "✅"
		}
		fmt.Printf("Block %d: %s exists: %s\n", i+1, c.String()[:20]+"...", status)
	}

	// Get sizes
	fmt.Printf("\nBlock sizes:\n")
	for i, c := range cids {
		size, err := bs.GetSize(ctx, c)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Block %d: %d bytes\n", i+1, size)
	}

	// List all blocks
	fmt.Printf("\nAll blocks in store:\n")
	allKeys, err := bs.AllKeysChan(ctx)
	if err != nil {
		log.Fatal(err)
	}

	count := 0
	for cid := range allKeys {
		count++
		fmt.Printf("  %d. %s\n", count, cid.String())
	}
	fmt.Printf("Total blocks: %d\n", count)

	// Delete a block
	if len(cids) > 0 {
		fmt.Printf("\nDeleting block: %s\n", cids[0].String()[:20]+"...")
		err := bs.Delete(ctx, cids[0])
		if err != nil {
			log.Fatal(err)
		}

		// Verify deletion
		exists, err := bs.Has(ctx, cids[0])
		if err != nil {
			log.Fatal(err)
		}
		if !exists {
			fmt.Printf("✅ Block successfully deleted\n")
		} else {
			fmt.Printf("❌ Block still exists\n")
		}
	}
}

func formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%dKB", bytes/1024)
	} else {
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	}
}