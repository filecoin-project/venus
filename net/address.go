package net

import (
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// PeerAddrsToPeerInfos converts a slice of string peer addresses
// (multiaddr + ipfs peerid) to PeerInfos.
func PeerAddrsToPeerInfos(addrs []string) ([]pstore.PeerInfo, error) {
	var pis []pstore.PeerInfo
	for _, addr := range addrs {
		a, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}

		pinfo, err := pstore.InfoFromP2pAddr(a)
		if err != nil {
			return nil, err
		}
		pis = append(pis, *pinfo)
	}
	return pis, nil
}
