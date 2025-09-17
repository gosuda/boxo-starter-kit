package ipldprime

import (
	"context"

	"github.com/ipfs/go-cid"
)

type Fetcher interface {
	Fetch(ctx context.Context, wants []cid.Cid) error
}
