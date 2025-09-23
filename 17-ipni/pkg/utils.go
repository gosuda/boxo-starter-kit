package ipni

import (
	"github.com/ipni/go-indexer-core"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-varint"
)

func MakeTopic(topic string) string {
	return "/indexer/ingest/" + topic
}

type TransportKind string

const (
	TUnknown   TransportKind = "unknown"
	TLocal     TransportKind = "local"
	THTTP      TransportKind = "http"
	TGraphSync TransportKind = "graphsync"
	TBitswap   TransportKind = "bitswap"
)

func ExportTransportKind(val indexer.Value) TransportKind {
	if len(val.MetadataBytes) == 0 {
		return TBitswap
	}
	code, _, err := varint.FromUvarint(val.MetadataBytes)
	if err != nil {
		return TUnknown
	}

	switch multicodec.Code(code) {
	case multicodec.TransportBitswap:
		return TBitswap
	case multicodec.TransportIpfsGatewayHttp:
		return THTTP
	case multicodec.TransportGraphsyncFilecoinv1:
		return TGraphSync
	default:
		return TUnknown
	}
}

// func NormalizeFromEngine(ctx context.Context, vals []indexer.Value) Providers {
// 	out := make([]ProviderView, 0, len(vals))
// 	for _, v := range vals {
// 		pv := ProviderView{
// 			ID:         v.ProviderID.String(),
// 			Info:       nil,
// 			Transports: []Transport{},
// 			Meta:       map[string]string{},
// 		}

// 		// ContextID (hex)
// 		if len(v.ContextID) > 0 {
// 			pv.Meta["context_id"] = hex.EncodeToString(v.ContextID)
// 		}

// 		// Parse metadata by multicodec prefix instead of assuming GraphSync
// 		if len(v.MetadataBytes) == 0 {
// 			pv.Transports = append(pv.Transports, Transport{Kind: TBitswap})
// 			out = append(out, pv)
// 			continue
// 		}

// 		code, off, err := varint.FromUvarint(v.MetadataBytes)
// 		if err != nil {
// 			pv.Meta["metadata_parse_error"] = err.Error()
// 			out = append(out, pv)
// 			continue
// 		}

// 		switch code {
// 		case uint64(multicodec.TransportBitswap):
// 			pv.Transports = append(pv.Transports, Transport{Kind: TBitswap})

// 		case uint64(multicodec.TransportIpfsGatewayHttp):
// 			pv.Transports = append(pv.Transports, Transport{Kind: THTTP})

// 		case uint64(multicodec.TransportGraphsyncFilecoinv1):
// 			pv.Transports = append(pv.Transports, Transport{Kind: TGraphSync})
// 			piece, verified, fast, err := decodeGraphsyncFilecoinV1(v.MetadataBytes[off:])
// 			if err != nil {
// 				pv.Meta["metadata_parse_error"] = err.Error()
// 			} else {
// 				if piece.Defined() {
// 					pv.Meta["piece_cid"] = piece.String()
// 				}
// 				pv.Meta["verified_deal"] = verified
// 				pv.Meta["fast_retrieval"] = fast
// 			}

// 		default:
// 			pv.Transports = append(pv.Transports, Transport{Kind: TGraphSync})
// 			pv.Meta["metadata_note"] = fmt.Sprintf("unknown multicodec: 0x%x", uint64(code))
// 		}

// 		out = append(out, pv)
// 	}
// 	return Providers{Items: out, Source: "engine"}
// }

// func decodeGraphsyncFilecoinV1(payload []byte) (piece cid.Cid, verified, fast bool, err error) {
// 	nb := basicnode.Prototype.Any.NewBuilder()
// 	if err = dagcbor.Decode(nb, bytes.NewReader(payload)); err != nil {
// 		return cid.Undef, false, false, fmt.Errorf("dagcbor decode: %w", err)
// 	}
// 	node := nb.Build()

// 	// PieceCID (link)
// 	ent, err := node.LookupByString("PieceCID")
// 	if err != nil {
// 		return cid.Undef, false, false, fmt.Errorf("PieceCID not found: %w", err)
// 	}

// 	lnk, e := ent.AsLink()
// 	if e != nil {
// 		return cid.Undef, false, false, fmt.Errorf("PieceCID not a link: %w", e)
// 	}
// 	if cl, ok := lnk.(cidlink.Link); ok {
// 		piece = cl.Cid
// 	}
// 	// VerifiedDeal
// 	ent, err = node.LookupByString("VerifiedDeal")
// 	if err != nil {
// 		return cid.Undef, false, false, fmt.Errorf("VerifiedDeal not found: %w", err)
// 	}

// 	if vb, e := ent.AsBool(); e == nil {
// 		verified = vb
// 	}
// 	// FastRetrieval
// 	ent, err = node.LookupByString("FastRetrieval")
// 	if err != nil {
// 		return cid.Undef, false, false, fmt.Errorf("FastRetrieval not found: %w", err)
// 	}
// 	if vb, e := ent.AsBool(); e == nil {
// 		fast = vb
// 	}
// 	return
// }
