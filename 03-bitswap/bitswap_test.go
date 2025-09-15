package bitswap_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bitswap "github.com/gosuda/boxo-starter-kit/03-bitswap/pkg"
)

func TestBitswap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bswap1, err := bitswap.New(ctx, nil, nil)
	require.NoError(t, err)
	defer bswap1.Close()

	bswap2, err := bitswap.New(ctx, nil, nil)
	require.NoError(t, err)
	defer bswap2.Close()

	err = bswap1.HostWrapper.ConnectToPeer(ctx, bswap2.HostWrapper.GetFullAddresses()...)
	require.NoError(t, err)

	payload := []byte("Hello, Bitswap!")
	c, err := bswap1.PutBlockRaw(ctx, payload)
	require.NoError(t, err)

	receive, err := bswap2.GetBlockRaw(ctx, c)
	require.NoError(t, err)

	require.Equal(t, payload, receive)
}
