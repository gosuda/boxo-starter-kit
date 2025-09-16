package unixfs

import (
	chunker "github.com/ipfs/boxo/chunker"
	bal "github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	h "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

const (
	KiB = 1 << 10
	MiB = KiB << 10
	GiB = MiB << 10
)

func GetChunkSize(size int, defaultChunkSize int64) (chunkSize int64) {
	if size < 0 {
		size = 0
	}

	switch {
	case size <= 1*MiB:
		return max(32*KiB, min(defaultChunkSize, int64(size)))
	case size <= 64*MiB:
		return defaultChunkSize
	case size <= 1*GiB:
		return max(defaultChunkSize, 1*MiB)
	default:
		return 4 * MiB
	}
}

func BuildDagFromReader(prefix *cid.Prefix, ds ipld.DAGService, spl chunker.Splitter) (ipld.Node, error) {
	dbp := h.DagBuilderParams{
		Dagserv:    ds,
		Maxlinks:   h.DefaultLinksPerBlock,
		CidBuilder: prefix,
	}
	db, err := dbp.New(spl)
	if err != nil {
		return nil, err
	}
	return bal.Layout(db)
}
