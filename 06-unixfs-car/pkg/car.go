package unixfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/storage"
)

func CarExport(ctx context.Context, ipldWrapper *dag.IpldWrapper, roots []cid.Cid, w io.Writer) error {
	ws, ok := w.(io.WriteSeeker)
	if !ok {
		return fmt.Errorf("car v2 export needs io.WriteSeeker; got %T", w)
	}

	writable, err := storage.NewWritable(ws, roots)
	if err != nil {
		return fmt.Errorf("failed to create writable car storage: %w", err)
	}
	defer writable.Finalize()
	bs := ipldWrapper.BlockServiceWrapper.Blockstore()
	seen := make(map[cid.Cid]struct{}, 1024)

	var walk func(c cid.Cid) error
	walk = func(c cid.Cid) error {
		if _, ok := seen[c]; ok {
			return nil
		}
		seen[c] = struct{}{}

		blk, err := bs.Get(ctx, c)
		if err != nil {
			return fmt.Errorf("get block %s: %w", c, err)
		}
		if err := writable.Put(ctx, blk.Cid().KeyString(), blk.RawData()); err != nil {
			return fmt.Errorf("write block %s: %w", blk.Cid(), err)
		}

		nd, err := ipldWrapper.Get(ctx, c) // format.Node
		if err != nil {
			return fmt.Errorf("load node %s: %w", c, err)
		}
		for _, l := range nd.Links() {
			if err := walk(l.Cid); err != nil {
				return err
			}
		}
		return nil
	}

	for _, r := range roots {
		if err := walk(r); err != nil {
			return err
		}
	}
	return nil
}

func CarExportBytes(ctx context.Context, ipldWrapper *dag.IpldWrapper, roots []cid.Cid) ([]byte, error) {
	f, err := os.CreateTemp("", "export-*.car")
	if err != nil {
		return nil, fmt.Errorf("create temp car: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := CarExport(ctx, ipldWrapper, roots, f); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek temp car: %w", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read temp car: %w", err)
	}
	return data, nil
}

func CarExportToPath(ctx context.Context, ipldWrapper *dag.IpldWrapper, roots []cid.Cid, path string) error {
	if filepath.Ext(path) != ".car" {
		path = filepath.Join(path, "default.car")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return CarExport(ctx, ipldWrapper, roots, file)
}

func CarImport(ctx context.Context, bs blockstore.Blockstore, r io.Reader) ([]cid.Cid, error) {
	br, err := car.NewBlockReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open car reader: %w", err)
	}

	for {
		blk, err := br.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read block: %w", err)
		}
		if err := bs.Put(ctx, blk); err != nil {
			return nil, fmt.Errorf("failed to store block %s: %w", blk.Cid(), err)
		}
	}

	return br.Roots, nil
}

func CarImportBytes(ctx context.Context, bs blockstore.Blockstore, data []byte) ([]cid.Cid, error) {
	return CarImport(ctx, bs, bytes.NewReader(data))
}

func CarImportPath(ctx context.Context, bs blockstore.Blockstore, path string) ([]cid.Cid, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return CarImport(ctx, bs, file)
}
