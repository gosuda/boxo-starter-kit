package mfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	unixfs "github.com/gosuda/boxo-starter-kit/05-unixfs-car/pkg"
)

type MFSWrapper struct {
	*unixfs.UnixFsWrapper
	root *mfs.Root

	cur cid.Cid
}

func New(ctx context.Context, ufs *unixfs.UnixFsWrapper, c cid.Cid) (*MFSWrapper, error) {
	var err error
	if ufs == nil {
		ufs, err = unixfs.New(0, nil)
		if err != nil {
			return nil, err
		}
	}

	var root *mfs.Root
	if c == cid.Undef {
		root, err = mfs.NewEmptyRoot(ctx, ufs.DAGService, dummypf, nil, mfs.MkdirOpts{})
	} else {
		nd, err := ufs.IpldWrapper.Get(ctx, c) // format.Node
		if err != nil {
			return nil, err
		}
		protond, ok := nd.(*merkledag.ProtoNode)
		if !ok {
			return nil, fmt.Errorf("node is not a ProtoNode")
		}
		root, err = mfs.NewRoot(ctx, ufs.DAGService, protond, dummypf, nil)
	}
	if err != nil {
		return nil, err
	}

	return &MFSWrapper{
		UnixFsWrapper: ufs,
		root:          root,
	}, nil
}

func (m *MFSWrapper) Mkdir(ctx context.Context, path string, opts mfs.MkdirOpts) error {
	return mfs.Mkdir(m.root, normPath(path), opts)
}

func (w *MFSWrapper) RefreshRootCID(ctx context.Context) error {
	nd, err := w.root.GetDirectory().GetNode()
	if err != nil {
		return err
	}
	if err := w.IpldWrapper.Add(ctx, nd); err != nil {
		return err
	}
	w.cur = nd.Cid()
	return nil
}

type WriteOptions struct {
	Create   bool
	Truncate bool
	Append   bool
	Parents  bool
	Mtime    *time.Time
}

var DefaultWriteOptions = WriteOptions{
	Create:   true,
	Truncate: true,
	Append:   false,
	Parents:  true,
	Mtime:    nil,
}

func (m *MFSWrapper) WriteBytes(ctx context.Context, dst string, data []byte, trunc bool) error {
	dirp, _ := path.Split(normPath(dst))
	if err := mfs.Mkdir(m.root, normPath(dirp), mfs.MkdirOpts{Mkparents: true}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("mkdir parents for %s: %w", dst, err)
	}

	c, err := m.PutBytes(ctx, data)
	if err != nil {
		return fmt.Errorf("put bytes: %w", err)
	}

	ipldNode, err := m.IpldWrapper.Get(ctx, c) // format.Node
	if err != nil {
		return fmt.Errorf("load node: %w", err)
	}
	if err := mfs.PutNode(m.root, normPath(dst), ipldNode); err != nil {
		return fmt.Errorf("mfs.PutNode(%s): %w", dst, err)
	}

	return nil
}

func (m *MFSWrapper) Move(_ context.Context, src, dst string) error {
	return mfs.Mv(m.root, normPath(src), normPath(dst))
}

func (m *MFSWrapper) Remove(_ context.Context, target string) error {
	target = normPath(target)
	dirp, name := path.Split(target)
	fsn, err := mfs.Lookup(m.root, dirp)
	if err != nil {
		return err
	}
	d, ok := fsn.(*mfs.Directory)
	if !ok {
		return fmt.Errorf("%s is not a directory", dirp)
	}
	return d.Unlink(name)
}

func (m *MFSWrapper) ReadBytes(ctx context.Context, path string) ([]byte, error) {
	fsn, err := mfs.Lookup(m.root, normPath(path))
	if err != nil {
		return nil, err
	}
	ipldNode, err := fsn.GetNode()
	if err != nil {
		return nil, err
	}
	c := ipldNode.Cid()
	n, err := m.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	defer n.Close()

	f, ok := n.(files.File)
	if !ok {
		return nil, fmt.Errorf("%s is not a file", path)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *MFSWrapper) Chmod(_ context.Context, path string, mode uint32) error {
	return mfs.Chmod(m.root, normPath(path), os.FileMode(mode))
}

func (m *MFSWrapper) Touch(_ context.Context, path string, ts time.Time) error {
	return mfs.Touch(m.root, normPath(path), ts)
}

func (m *MFSWrapper) FlushPath(ctx context.Context, path string) (format.Node, error) {
	return mfs.FlushPath(ctx, m.root, normPath(path))
}

func (m *MFSWrapper) SnapshotCID(ctx context.Context) (cid.Cid, error) {
	nd, err := m.FlushPath(ctx, "/")
	if err != nil {
		return cid.Undef, err
	}
	return nd.Cid(), nil
}

func (m *MFSWrapper) ExportCAR(ctx context.Context, ws io.WriteSeeker) error {
	root, err := m.SnapshotCID(ctx)
	if err != nil {
		return err
	}
	return m.CarExport(ctx, []cid.Cid{root}, ws)
}

func (m *MFSWrapper) ImportCAR(ctx context.Context, r io.Reader, choose func([]cid.Cid) cid.Cid) (cid.Cid, error) {
	roots, err := m.CarImport(ctx, r)
	if err != nil {
		return cid.Undef, err
	}
	if len(roots) == 0 {
		return cid.Undef, fmt.Errorf("no roots in CAR")
	}
	root := roots[0]
	if choose != nil {
		root = choose(roots)
	}
	nd, err := m.IpldWrapper.Get(ctx, root)
	if err != nil {
		return cid.Undef, err
	}
	pn, ok := nd.(*merkledag.ProtoNode)
	if !ok {
		return cid.Undef, fmt.Errorf("root is not ProtoNode")
	}
	newRoot, err := mfs.NewRoot(ctx, m.IpldWrapper, pn, dummypf, nil)
	if err != nil {
		return cid.Undef, err
	}
	m.root = newRoot
	return m.SnapshotCID(ctx)
}
