package main

import (
	"context"
	"testing"
	"time"

	mc "github.com/multiformats/go-multicodec"
	"github.com/stretchr/testify/require"

	dasl "github.com/gosuda/boxo-starter-kit/13-dasl/pkg"
)

func TestDaslWrapperPutGet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dsl, err := dasl.NewDaslWrapper(nil)
	require.NoError(t, err)

	u1 := dasl.User{
		Id:     "u1",
		Name:   "Neo",
		Email:  "neo@matrix.io",
		Avatar: []byte("avatar-bytes"),
	}
	u1Cid, err := dsl.PutUser(ctx, &u1)
	require.NoError(t, err)
	require.True(t, u1Cid.Defined())

	p1 := dasl.Post{
		Id:        "p1",
		Author:    u1Cid,
		Title:     "Hello, IPLD",
		Body:      "content",
		Tags:      []string{"ipld", "bindnode"},
		CreatedAt: time.Now().Unix(),
	}
	p1Cid, err := dsl.PutPost(ctx, &p1)
	require.NoError(t, err)
	require.True(t, p1Cid.Defined())

	root := &dasl.Root{
		Users: u1,
		Posts: p1,
	}
	rootCid, err := dsl.PutRoot(ctx, root)
	require.NoError(t, err)
	require.True(t, rootCid.Defined())

	gotRoot, err := dsl.GetRoot(ctx, rootCid)
	require.NoError(t, err)
	require.NotNil(t, gotRoot)

	require.Equal(t, "Neo", gotRoot.Users.Name)
	require.Equal(t, "Hello, IPLD", gotRoot.Posts.Title)

	gotUser, err := dsl.GetUser(ctx, u1Cid)
	require.NoError(t, err)
	require.Equal(t, "neo@matrix.io", gotUser.Email)

	gotPost, err := dsl.GetPost(ctx, p1Cid)
	require.NoError(t, err)
	require.Equal(t, u1Cid, gotPost.Author)

	require.Equal(t, uint64(mc.DagCbor), rootCid.Prefix().Codec)
	require.Equal(t, uint64(mc.DagCbor), u1Cid.Prefix().Codec)
	require.Equal(t, uint64(mc.DagCbor), p1Cid.Prefix().Codec)
}
