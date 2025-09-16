package mfs

import (
	"context"
	"path"
	"strings"

	"github.com/ipfs/go-cid"
)

func normPath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func dummypf(ctx context.Context, c cid.Cid) error {
	return nil
}
