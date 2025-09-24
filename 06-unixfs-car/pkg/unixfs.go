package unixfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	chunk "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/files"
	ufs "github.com/ipfs/boxo/ipld/unixfs"
	uio "github.com/ipfs/boxo/ipld/unixfs/file"
	"github.com/ipfs/boxo/ipld/unixfs/importer"
	"github.com/ipfs/go-cid"

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

type UnixFsWrapper struct {
	defaultChunkSize int64
	*dag.IpldWrapper
}

func New(defaultChunkSize int64, dagWrapper *dag.IpldWrapper) (*UnixFsWrapper, error) {
	var err error
	if defaultChunkSize <= 0 {
		defaultChunkSize = 1024 * 256
	}
	if dagWrapper == nil {
		ctx := context.Background()
		dagWrapper, err = dag.NewIpldWrapper(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create DAG wrapper: %w", err)
		}
	}
	return &UnixFsWrapper{
		defaultChunkSize: defaultChunkSize,
		IpldWrapper:      dagWrapper,
	}, nil
}

func (u *UnixFsWrapper) Put(ctx context.Context, node files.Node) (cid.Cid, error) {
	switch v := node.(type) {
	case files.File:
		return u.putFile(ctx, v)
	case files.Directory:
		return u.putDir(ctx, v)
	default:
		return cid.Undef, fmt.Errorf("unsupported node type %T", v)
	}
}

func (u *UnixFsWrapper) PutBytes(ctx context.Context, b []byte) (cid.Cid, error) {
	file := files.NewBytesFile(b)
	return u.Put(ctx, file)
}

func (u *UnixFsWrapper) PutPath(ctx context.Context, path string) (cid.Cid, error) {
	info, err := os.Stat(path)
	if err != nil {
		return cid.Undef, err
	}

	var node files.Node
	if !info.IsDir() { // put file
		f, err := os.Open(path)
		if err != nil {
			return cid.Undef, fmt.Errorf("open %q: %w", path, err)
		}
		node = files.NewReaderFile(f)
	} else { // put directory
		node, err = files.NewSerialFile(path, false, info)
		if err != nil {
			return cid.Undef, fmt.Errorf("new serial file %q: %w", path, err)
		}
	}
	defer node.Close()

	return u.Put(ctx, node)
}

func (u *UnixFsWrapper) putFile(ctx context.Context, file files.File) (cid.Cid, error) {
	size, _ := file.Size()
	if size <= 0 {
		size = u.defaultChunkSize
	}
	splitter := chunk.NewSizeSplitter(file, GetChunkSize(int(size), u.defaultChunkSize))

	nd, err := importer.BuildDagFromReader(u.IpldWrapper, splitter)
	if err != nil {
		return cid.Undef, fmt.Errorf("build dag from file: %w", err)
	}
	return nd.Cid(), nil
}

func (u *UnixFsWrapper) putDir(ctx context.Context, d files.Directory) (cid.Cid, error) {
	root := ufs.EmptyDirNode()

	type child struct {
		name string
		cid  cid.Cid
	}
	var children []child

	it := d.Entries()
	for it.Next() {
		select {
		case <-ctx.Done():
			return cid.Undef, ctx.Err()
		default:
		}

		name := it.Name()
		n := it.Node()

		childCid, err := u.Put(ctx, n)
		_ = n.Close()
		if err != nil {
			return cid.Undef, fmt.Errorf("put child %q: %w", name, err)
		}
		children = append(children, child{name: name, cid: childCid})
	}
	if err := it.Err(); err != nil {
		return cid.Undef, err
	}

	sort.Slice(children, func(i, j int) bool { return children[i].name < children[j].name })

	for _, c := range children {
		childNode, err := u.IpldWrapper.Get(ctx, c.cid)
		if err != nil {
			return cid.Undef, fmt.Errorf("get child %q (%s): %w", c.name, c.cid, err)
		}
		if err := root.AddNodeLink(c.name, childNode); err != nil {
			return cid.Undef, fmt.Errorf("add link %q: %w", c.name, err)
		}
	}

	if err := u.IpldWrapper.Add(ctx, root); err != nil {
		return cid.Undef, fmt.Errorf("dag add dir root: %w", err)
	}
	return root.Cid(), nil
}

func (u *UnixFsWrapper) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
	nd, err := u.IpldWrapper.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	return uio.NewUnixfsFile(ctx, u.IpldWrapper, nd)
}

func (u *UnixFsWrapper) GetBytes(ctx context.Context, c cid.Cid) ([]byte, error) {
	node, err := u.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	defer node.Close()

	file, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("cid %s is not a file", c)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (u *UnixFsWrapper) GetPath(ctx context.Context, c cid.Cid, dstPath string) error {
	node, err := u.Get(ctx, c)
	if err != nil {
		return err
	}
	defer node.Close()

	switch n := node.(type) {
	case files.File:
		return u.writeFileToPath(n, dstPath)
	case files.Directory:
		return u.writeDirToPath(ctx, n, dstPath)
	default:
		return fmt.Errorf("unsupported node type")
	}
}

func (u *UnixFsWrapper) writeFileToPath(file files.File, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	return err
}

func (u *UnixFsWrapper) writeDirToPath(ctx context.Context, dir files.Directory, dstPath string) error {
	if err := os.MkdirAll(dstPath, 0755); err != nil {
		return err
	}

	entries := dir.Entries()
	for entries.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		name := entries.Name()
		subNode := entries.Node()
		defer subNode.Close()
		subPath := filepath.Join(dstPath, name)

		var err error
		switch n := subNode.(type) {
		case files.Directory:
			err = u.writeDirToPath(ctx, n, subPath)
		case files.File:
			err = u.writeFileToPath(n, subPath)
		default:
			err = fmt.Errorf("unsupported node type %T for %q", n, name)
		}
		if err != nil {
			return err
		}
	}
	return entries.Err()
}

func (u *UnixFsWrapper) List(ctx context.Context, dirCID cid.Cid) ([]string, error) {
	node, err := u.Get(ctx, dirCID)
	if err != nil {
		return nil, err
	}
	defer node.Close()

	dir, ok := node.(files.Directory)
	if !ok {
		return nil, fmt.Errorf("cid %s is not a directory", dirCID)
	}

	var entries []string
	it := dir.Entries()
	for it.Next() {
		entries = append(entries, it.Name())
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	sort.Strings(entries)
	return entries, nil
}
