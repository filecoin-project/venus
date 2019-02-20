package filnet

import (
	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
)

// TODO we're using the ipfs/ protocol for peer ids, we should be using p2p.

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
