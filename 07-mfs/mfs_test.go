package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	mfs "github.com/gosuda/boxo-starter-kit/07-mfs/pkg"
)

func TestMFSSetGet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m, err := mfs.New(ctx, nil, cid.Undef)
	require.NoError(t, err)

	const p = "/docs/readme.txt"
	err = m.WriteBytes(ctx, p, []byte("hello, world"), true)
	require.NoError(t, err)

	cid1, err := m.SnapshotCID(ctx)
	require.NoError(t, err)
	require.NotEqual(t, cid.Undef, cid1)

	got, err := m.ReadBytes(ctx, p)
	require.NoError(t, err)
	require.Equal(t, []byte("hello, world"), got)

	err = m.Move(ctx, "/docs/readme.txt", "/docs/README.md")
	require.NoError(t, err)

	cid2, err := m.SnapshotCID(ctx)
	require.NoError(t, err)
	require.NotEqual(t, cid1, cid2)

	err = m.Remove(ctx, "/docs/README.md")
	require.NoError(t, err)

	cid3, err := m.SnapshotCID(ctx)
	require.NoError(t, err)
	require.NotEqual(t, cid2, cid3)
}

func TestMFSTouchAndChmod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m, err := mfs.New(ctx, nil, cid.Undef)
	require.NoError(t, err)

	err = m.WriteBytes(ctx, "/bin/run.sh", []byte("#!/bin/sh\necho hi\n"), true)
	require.NoError(t, err)

	// chmod(0755)
	err = m.Chmod(ctx, "/bin/run.sh", 0o755)
	require.NoError(t, err)

	// touch(now)
	err = m.Touch(ctx, "/bin/run.sh", time.Now())
	require.NoError(t, err)

	_, err = m.SnapshotCID(ctx)
	require.NoError(t, err)
}

func TestMFSCAR(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	m1, err := mfs.New(ctx, nil, cid.Undef)
	require.NoError(t, err)

	err = m1.WriteBytes(ctx, "/notes/today.md", []byte("day 1"), true)
	require.NoError(t, err)

	root1, err := m1.SnapshotCID(ctx)
	require.NoError(t, err)

	tmpDir := t.TempDir()
	carPath := filepath.Join(tmpDir, "snapshot.car")
	f, err := os.Create(carPath)
	require.NoError(t, err)
	require.NoError(t, m1.ExportCAR(ctx, f))
	require.NoError(t, f.Close())

	m2, err := mfs.New(ctx, nil, cid.Undef)
	require.NoError(t, err)

	rf, err := os.Open(carPath)
	require.NoError(t, err)
	defer rf.Close()

	root2, err := m2.ImportCAR(ctx, rf, nil)
	require.NoError(t, err)

	require.Equal(t, root1, root2)

	got, err := m2.ReadBytes(ctx, "/notes/today.md")
	require.NoError(t, err)
	require.Equal(t, []byte("day 1"), got)
}
