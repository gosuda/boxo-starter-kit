package dasl

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"

	ipldprime "github.com/gosuda/boxo-starter-kit/12-ipld-prime/pkg"
)

type DaslWrapper struct {
	ipld *ipldprime.IpldWrapper
	ts   *schema.TypeSystem

	tRoot schema.Type
	tUser schema.Type
	tPost schema.Type
}

func NewDaslWrapper(ipld *ipldprime.IpldWrapper) (*DaslWrapper, error) {
	var err error
	if ipld == nil {
		ipld, err = ipldprime.NewDefault(nil, nil)
		if err != nil {
			return nil, err
		}
	}
	file, err := schemadsl.ParseBytes([]byte(schemaDasl))
	if err != nil {
		return nil, fmt.Errorf("schema parse file: %w", err)
	}

	ts := schema.TypeSystem{}
	ts.Init()
	if err := schemadmt.Compile(&ts, file); err != nil {
		return nil, fmt.Errorf("schema parse: %w", err)
	}
	w := &DaslWrapper{
		ipld:  ipld,
		ts:    &ts,
		tRoot: ts.TypeByName("Root"),
		tUser: ts.TypeByName("User"),
		tPost: ts.TypeByName("Post"),
	}
	if w.tRoot == nil || w.tUser == nil || w.tPost == nil {
		return nil, fmt.Errorf("schema type missing (Root/User/Post)")
	}
	return w, nil
}

// ----- Prototypes -----

func (w *DaslWrapper) protoRoot() datamodel.NodePrototype {
	return bindnode.Prototype((*Root)(nil), w.tRoot)
}
func (w *DaslWrapper) protoUser() datamodel.NodePrototype {
	return bindnode.Prototype((*User)(nil), w.tUser)
}
func (w *DaslWrapper) protoPost() datamodel.NodePrototype {
	return bindnode.Prototype((*Post)(nil), w.tPost)
}

func (w *DaslWrapper) PutRoot(ctx context.Context, v *Root) (cid.Cid, error) {
	if v == nil {
		return cid.Undef, fmt.Errorf("PutRoot: nil value")
	}
	n := bindnode.Wrap(v, w.tRoot) // Go → Node
	return w.ipld.PutIPLD(ctx, n)
}

func (w *DaslWrapper) GetRoot(ctx context.Context, c cid.Cid) (*Root, error) {
	n, err := w.ipld.GetIPLDWith(ctx, c, w.protoRoot())
	if err != nil {
		return nil, err
	}
	val := bindnode.Unwrap(n) // Node → Go
	out, ok := val.(*Root)
	if !ok {
		return nil, fmt.Errorf("unwrap Root: type assertion to *Root failed")
	}
	return out, nil
}

func (w *DaslWrapper) PutUser(ctx context.Context, v *User) (cid.Cid, error) {
	if v == nil {
		return cid.Undef, fmt.Errorf("PutUser: nil value")
	}
	n := bindnode.Wrap(v, w.tUser)
	return w.ipld.PutIPLD(ctx, n)
}

func (w *DaslWrapper) GetUser(ctx context.Context, c cid.Cid) (*User, error) {
	n, err := w.ipld.GetIPLDWith(ctx, c, w.protoUser())
	if err != nil {
		return nil, err
	}
	val := bindnode.Unwrap(n)
	out, ok := val.(*User)
	if !ok {
		return nil, fmt.Errorf("unwrap User: type assertion to *User failed")
	}
	return out, nil
}

func (w *DaslWrapper) PutPost(ctx context.Context, v *Post) (cid.Cid, error) {
	if v == nil {
		return cid.Undef, fmt.Errorf("PutPost: nil value")
	}
	n := bindnode.Wrap(v, w.tPost)
	return w.ipld.PutIPLD(ctx, n)
}

func (w *DaslWrapper) GetPost(ctx context.Context, c cid.Cid) (*Post, error) {
	n, err := w.ipld.GetIPLDWith(ctx, c, w.protoPost())
	if err != nil {
		return nil, err
	}
	val := bindnode.Unwrap(n)
	out, ok := val.(*Post)
	if !ok {
		return nil, fmt.Errorf("unwrap Post: type assertion to *Post failed")
	}

	return out, nil
}
