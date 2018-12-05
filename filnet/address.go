package filnet

import (
	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	ma "gx/ipfs/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
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
