package ipldprime

import (
	"fmt"
	"math"
	"reflect"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
)

func NodeToAny(n datamodel.Node) (any, error) {
	switch n.Kind() {
	case datamodel.Kind_Null:
		return nil, nil
	case datamodel.Kind_Bool:
		return n.AsBool()
	case datamodel.Kind_Int:
		return n.AsInt()
	case datamodel.Kind_Float:
		return n.AsFloat()
	case datamodel.Kind_String:
		return n.AsString()
	case datamodel.Kind_Bytes:
		return n.AsBytes()
	case datamodel.Kind_Link:
		lk, err := n.AsLink()
		if err != nil {
			return nil, err
		}
		if cl, ok := lk.(cidlink.Link); ok {
			return cl.Cid, nil
		}
		return nil, fmt.Errorf("unsupported link type %T", lk)
	case datamodel.Kind_List:
		itr := n.ListIterator()
		var out []any
		for !itr.Done() {
			_, v, _ := itr.Next()
			av, err := NodeToAny(v)
			if err != nil {
				return nil, err
			}
			out = append(out, av)
		}
		return out, nil
	case datamodel.Kind_Map:
		itr := n.MapIterator()
		m := make(map[string]any)
		for !itr.Done() {
			k, v, _ := itr.Next()
			ks, err := k.AsString()
			if err != nil {
				return nil, fmt.Errorf("map key is not string: %w", err)
			}
			av, err := NodeToAny(v)
			if err != nil {
				return nil, err
			}
			m[ks] = av
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unknown kind: %v", n.Kind())
	}
}

func AnyToNode(v any) (datamodel.Node, error) {
	if n, ok := v.(datamodel.Node); ok {
		return n, nil
	}
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := assignAny(nb, v); err != nil {
		return nil, err
	}
	return nb.Build(), nil
}

func assignAny(ass datamodel.NodeAssembler, v any) error {
	if v == nil {
		return ass.AssignNull()
	}

	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ass.AssignNull()
		}
		rv = rv.Elem()
		v = rv.Interface()
	}

	switch t := v.(type) {
	case string:
		return ass.AssignString(t)
	case bool:
		return ass.AssignBool(t)
	case int:
		return ass.AssignInt(int64(t))
	case int8:
		return ass.AssignInt(int64(t))
	case int16:
		return ass.AssignInt(int64(t))
	case int32:
		return ass.AssignInt(int64(t))
	case int64:
		return ass.AssignInt(t)
	case uint:
		if uint64(t) > math.MaxInt64 {
			return fmt.Errorf("unsigned int overflows int64: %d", t)
		}
		return ass.AssignInt(int64(t))
	case uint8:
		return ass.AssignInt(int64(t))
	case uint16:
		return ass.AssignInt(int64(t))
	case uint32:
		if uint64(t) > math.MaxInt64 {
			return fmt.Errorf("unsigned int overflows int64: %d", t)
		}
		return ass.AssignInt(int64(t))
	case uint64:
		if t > math.MaxInt64 {
			return fmt.Errorf("uint64 overflows int64: %d", t)
		}
		return ass.AssignInt(int64(t))
	case float32:
		f := float64(t)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("non-finite float not allowed in dag-cbor")
		}
		return ass.AssignFloat(f)
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return fmt.Errorf("non-finite float not allowed in dag-cbor")
		}
		return ass.AssignFloat(t)
	case []byte:
		return ass.AssignBytes(t)
	case datamodel.Node:
		return ass.AssignNode(t)
	case datamodel.Link:
		return ass.AssignLink(t)
	case cid.Cid:
		return ass.AssignLink(cidlink.Link{Cid: t})

	case map[string]any:
		n, err := BuildMap(t)
		if err != nil {
			return err
		}
		return ass.AssignNode(n)
	case []any:
		n, err := BuildList(t...)
		if err != nil {
			return err
		}
		return ass.AssignNode(n)
	}

	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		lb := basicnode.Prototype.List.NewBuilder()
		la, err := lb.BeginList(int64(rv.Len()))
		if err != nil {
			return err
		}
		for i := 0; i < rv.Len(); i++ {
			if err := assignAny(la.AssembleValue(), rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		if err := la.Finish(); err != nil {
			return err
		}
		return ass.AssignNode(lb.Build())
	}

	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		keys := rv.MapKeys()
		mb := basicnode.Prototype.Map.NewBuilder()
		ma, err := mb.BeginMap(int64(len(keys)))
		if err != nil {
			return err
		}
		for _, k := range keys {
			if err := ma.AssembleKey().AssignString(k.String()); err != nil {
				return err
			}
			if err := assignAny(ma.AssembleValue(), rv.MapIndex(k).Interface()); err != nil {
				return err
			}
		}
		if err := ma.Finish(); err != nil {
			return err
		}
		return ass.AssignNode(mb.Build())
	}

	return fmt.Errorf("unsupported type %T", v)
}

func BuildMap(kv map[string]any) (datamodel.Node, error) {
	mb := basicnode.Prototype.Map.NewBuilder()
	ma, err := mb.BeginMap(int64(len(kv)))
	if err != nil {
		return nil, err
	}
	for k, v := range kv {
		if err := ma.AssembleKey().AssignString(k); err != nil {
			return nil, err
		}
		if err := assignAny(ma.AssembleValue(), v); err != nil {
			return nil, err
		}
	}
	if err := ma.Finish(); err != nil {
		return nil, err
	}
	return mb.Build(), nil
}

func BuildList(items ...any) (datamodel.Node, error) {
	lb := basicnode.Prototype.List.NewBuilder()
	la, err := lb.BeginList(int64(len(items)))
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if err := assignAny(la.AssembleValue(), it); err != nil {
			return nil, err
		}
	}
	if err := la.Finish(); err != nil {
		return nil, err
	}
	return lb.Build(), nil
}

func lookupListIndex(n datamodel.Node, seg string) (datamodel.Node, error) {
	if n.Kind() != datamodel.Kind_List {
		return nil, fmt.Errorf("not a list")
	}
	var idx int
	_, err := fmt.Sscanf(seg, "%d", &idx)
	if err != nil {
		return nil, fmt.Errorf("invalid list index %q", seg)
	}
	itr := n.ListIterator()
	i := 0
	for !itr.Done() {
		_, v, _ := itr.Next()
		if i == idx {
			return v, nil
		}
		i++
	}
	return nil, fmt.Errorf("index out of range")
}

func ExtractChildCIDs(n datamodel.Node) []cid.Cid {
	var out []cid.Cid
	switch n.Kind() {
	case datamodel.Kind_Link:
		if lk, err := n.AsLink(); err == nil {
			if cl, ok := lk.(cidlink.Link); ok {
				out = append(out, cl.Cid)
			}
		}
	case datamodel.Kind_List:
		it := n.ListIterator()
		for !it.Done() {
			_, v, _ := it.Next()
			out = append(out, ExtractChildCIDs(v)...)
		}
	case datamodel.Kind_Map:
		it := n.MapIterator()
		for !it.Done() {
			_, v, _ := it.Next()
			out = append(out, ExtractChildCIDs(v)...)
		}
	}
	return out
}
