package network

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func ToMultiaddrs(addr []string) ([]multiaddr.Multiaddr, error) {
	maddrs := make([]multiaddr.Multiaddr, 0, len(addr))
	for _, a := range addr {
		ma, err := multiaddr.NewMultiaddr(a)
		if err != nil {
			return nil, err
		}
		maddrs = append(maddrs, ma)
	}
	return maddrs, nil

}

func ToAddrInfos(addrs []string) ([]peer.AddrInfo, error) {
	addrInfos := make([]peer.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		info, err := peer.AddrInfoFromString(addr)
		if err != nil {
			return nil, err
		}
		addrInfos = append(addrInfos, *info)
	}
	return addrInfos, nil
}
