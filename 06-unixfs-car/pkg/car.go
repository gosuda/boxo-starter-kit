package unixfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/storage"
)

func (u *UnixFsWrapper) CarExport(ctx context.Context, roots []cid.Cid, w io.Writer) error {
	ws, ok := w.(io.WriteSeeker)
	if !ok {
		return fmt.Errorf("car v2 export needs io.WriteSeeker; got %T", w)
	}

	writable, err := storage.NewWritable(ws, roots)
	if err != nil {
		return fmt.Errorf("failed to create writable car storage: %w", err)
	}
	defer writable.Finalize()
	bs := u.IpldWrapper.BlockServiceWrapper.Blockstore()
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

		nd, err := u.IpldWrapper.Get(ctx, c) // format.Node
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

func (u *UnixFsWrapper) CarExportBytes(ctx context.Context, roots []cid.Cid) ([]byte, error) {
	f, err := os.CreateTemp("", "export-*.car")
	if err != nil {
		return nil, fmt.Errorf("create temp car: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := u.CarExport(ctx, roots, f); err != nil {
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

func (u *UnixFsWrapper) CarExportToPath(ctx context.Context, roots []cid.Cid, path string) error {
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

	return u.CarExport(ctx, roots, file)
}

func (u *UnixFsWrapper) CarImport(ctx context.Context, r io.Reader) ([]cid.Cid, error) {
	br, err := car.NewBlockReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open car reader: %w", err)
	}

	bs := u.IpldWrapper.BlockServiceWrapper.Blockstore()

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

func (u *UnixFsWrapper) CarImportBytes(ctx context.Context, data []byte) ([]cid.Cid, error) {
	return u.CarImport(ctx, bytes.NewReader(data))
}

func (u *UnixFsWrapper) CarImportPath(ctx context.Context, path string) ([]cid.Cid, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return u.CarImport(ctx, file)
}
