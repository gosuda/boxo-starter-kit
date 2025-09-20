package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	unixfs "github.com/gosuda/boxo-starter-kit/06-unixfs-car/pkg"
)

func TestUnixFsBytes(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), 15*time.Second)
	defer timeout()

	ufs, err := unixfs.New(0, nil)
	require.NoError(t, err)

	input := []byte("hello unixfs")
	c, err := ufs.PutBytes(ctx, input)
	require.NoError(t, err)

	output, err := ufs.GetBytes(ctx, c)
	require.NoError(t, err)
	require.Equal(t, input, output, "output must match input")
}

func TestUnixFsFiles(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), 15*time.Second)
	defer timeout()

	ufs, err := unixfs.New(0, nil)
	require.NoError(t, err)

	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "sample.txt")
	srcData := []byte("file-path-roundtrip")
	err = os.WriteFile(srcPath, srcData, 0o644)
	require.NoError(t, err)

	c, err := ufs.PutPath(ctx, srcPath)
	require.NoError(t, err)

	output, err := ufs.GetBytes(ctx, c)
	require.NoError(t, err)

	require.Equal(t, srcData, output, "output must match input")

	dstPath := filepath.Join(tmp, "output.txt")
	err = ufs.GetPath(ctx, c, dstPath)
	require.NoError(t, err)

	gotData, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	require.Equal(t, srcData, gotData, "file content must match")
}

func TestUnixFsDirs(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), 15*time.Second)
	defer timeout()

	ufs, err := unixfs.New(0, nil)
	require.NoError(t, err)

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	err = os.Mkdir(srcDir, 0o755)
	require.NoError(t, err)

	files := map[string][]byte{
		"file1.txt": []byte("content of file 1"),
		"file2.txt": []byte("content of file 2"),
	}
	for name, data := range files {
		err = os.WriteFile(filepath.Join(srcDir, name), data, 0o644)
		require.NoError(t, err)
	}

	c, err := ufs.PutPath(ctx, srcDir)
	require.NoError(t, err)

	dstDir := filepath.Join(tmp, "dst")
	err = ufs.GetPath(ctx, c, dstDir)
	require.NoError(t, err)

	for name, data := range files {
		gotData, err := os.ReadFile(filepath.Join(dstDir, name))
		require.NoError(t, err)
		require.Equal(t, data, gotData, "file content must match for %s", name)
	}
}

func TestCar(t *testing.T) {
	ctx, timeout := context.WithTimeout(context.Background(), 15*time.Second)
	defer timeout()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "x.txt"), []byte("X"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "y.txt"), []byte("Y"), 0o644))

	fx, err := os.Open(filepath.Join(tmp, "x.txt"))
	require.NoError(t, err)
	defer fx.Close()

	fy, err := os.Open(filepath.Join(tmp, "y.txt"))
	require.NoError(t, err)
	defer fy.Close()

	ufs, err := unixfs.New(0, nil)
	require.NoError(t, err)

	rootX, err := ufs.Put(ctx, files.NewReaderFile(fx))
	require.NoError(t, err)

	rootY, err := ufs.Put(ctx, files.NewReaderFile(fy))
	require.NoError(t, err)

	carBytes, err := ufs.CarExportBytes(ctx, []cid.Cid{rootX, rootY})
	require.NoError(t, err)
	require.NotEmpty(t, carBytes)

	ufs2, err := unixfs.New(0, nil)
	require.NoError(t, err)
	imported, err := ufs2.CarImportBytes(ctx, carBytes)
	require.NoError(t, err)
	require.ElementsMatch(t, []cid.Cid{rootX, rootY}, imported)
}
