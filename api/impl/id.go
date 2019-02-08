package impl

import (
	"fmt"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/api"
)

type nodeID struct {
	api *nodeAPI
}

func newNodeID(api *nodeAPI) *nodeID {
	return &nodeID{api: api}
}

// Details, returns detailed information about the underlying node.
func (a *nodeID) Details() (*api.IDDetails, error) {
	host := a.api.node.Host()
	hostID := host.ID()
	addrs := host.Addrs()

	details := api.IDDetails{
		Addresses: make([]ma.Multiaddr, len(addrs)),
		ID:        hostID,
		// TODO: fill out the other details
	}

	for i, addr := range addrs {
		subaddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", hostID.Pretty()))
		if err != nil {
			return nil, err
		}
		details.Addresses[i] = addr.Encapsulate(subaddr)
	}

	return &details, nil
}
