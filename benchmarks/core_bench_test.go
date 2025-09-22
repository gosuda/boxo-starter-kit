package benchmarks

import (
	"context"
	"testing"

	blockstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"

	blockpkg "github.com/gosuda/boxo-starter-kit/00-block-cid/pkg"
)

var benchConfig = DefaultConfig()

// Core benchmarks that focus on the fundamental building blocks
func BenchmarkCore_BlockCreation_Small(b *testing.B) {
	data := benchConfig.SmallTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := blockpkg.NewBlock(data, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCore_BlockCreation_Medium(b *testing.B) {
	data := benchConfig.MediumTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := blockpkg.NewBlock(data, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCore_BlockCreation_Large(b *testing.B) {
	data := benchConfig.LargeTestData()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := blockpkg.NewBlock(data, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCore_BlockStore_MemoryPutGet(b *testing.B) {
	// Setup memory blockstore
	ds := dsync.MutexWrap(datastore.NewMapDatastore())
	bs := blockstore.NewBlockstore(ds)
	ctx := context.Background()

	// Pre-create blocks
	testBlocks := make([]blocks.Block, benchConfig.SmallOpCount)
	for i := 0; i < benchConfig.SmallOpCount; i++ {
		block, err := blockpkg.NewBlock(benchConfig.SmallTestData(), nil)
		if err != nil {
			b.Fatal(err)
		}
		testBlocks[i] = block

		// Store the block
		err = bs.Put(ctx, block)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		blockIdx := i % benchConfig.SmallOpCount
		block := testBlocks[blockIdx]

		_, err := bs.Get(ctx, block.Cid())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCore_ConcurrentBlockCreation(b *testing.B) {
	data := benchConfig.SmallTestData()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := blockpkg.NewBlock(data, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCore_DatastorePut_Memory(b *testing.B) {
	ds := dsync.MutexWrap(datastore.NewMapDatastore())
	ctx := context.Background()
	data := benchConfig.SmallTestData()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := datastore.NewKey("bench").ChildString(string(rune(i)))
		err := ds.Put(ctx, key, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCore_DatastoreGet_Memory(b *testing.B) {
	ds := dsync.MutexWrap(datastore.NewMapDatastore())
	ctx := context.Background()
	data := benchConfig.SmallTestData()

	// Pre-populate data
	keys := make([]datastore.Key, benchConfig.SmallOpCount)
	for i := 0; i < benchConfig.SmallOpCount; i++ {
		key := datastore.NewKey("bench").ChildString(string(rune(i)))
		keys[i] = key
		err := ds.Put(ctx, key, data)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := keys[i%benchConfig.SmallOpCount]
		_, err := ds.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}
