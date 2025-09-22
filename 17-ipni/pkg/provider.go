package ipni

import (
	"github.com/ipni/go-indexer-core"
)

var (
	providerMetaPrefix = []byte{0xFF, 'M', 0x01}
)

type Provider struct {
	db indexer.Interface

	ID         string
	Addrs      []string // multiaddr or URLs
	Transports []Transport
	Region     string
	ASN        uint32
	Meta       map[string]string
}
