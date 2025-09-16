package unixfs

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
)

func (u *UnixFsWrapper) CarExport(ctx context.Context, roots []cid.Cid, w io.Writer) error {
	// car.WriteAsCarV1()

	// car.NewSelectiveWriter(ctx)
	return nil
}

func (u *UnixFsWrapper) CarImport(ctx context.Context, root cid.Cid) ([]cid.Cid, error) {

	return nil, nil
}
