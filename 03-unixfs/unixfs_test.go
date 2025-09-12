package unixfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	unixfs "github.com/gosunuts/boxo-starter-kit/03-unixfs/pkg"
)

func TestUnixFsBytes(t *testing.T) {
	ctx := context.TODO()
	ufs, err := unixfs.New(0)
	require.NoError(t, err)

	input := []byte("hello unixfs")
	c, err := ufs.PutBytes(ctx, input)
	require.NoError(t, err)

	output, err := ufs.GetBytes(ctx, c)
	require.NoError(t, err)
	require.Equal(t, input, output, "output must match input")
}

func TestUnixFsFileDir(t *testing.T) {

}
