package net

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// PeerAddrsToAddrInfo converts a slice of string peer addresses
// (multiaddr + ipfs peerid) to PeerInfos.
func PeerAddrsToAddrInfo(addrs []string) ([]peer.AddrInfo, error) {
	var pis []peer.AddrInfo
	for _, addr := range addrs {
		a, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}

		pinfo, err := peer.AddrInfoFromP2pAddr(a)
		if err != nil {
			return nil, err
		}
		pis = append(pis, *pinfo)
	}
	return pis, nil
}
