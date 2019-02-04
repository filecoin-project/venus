package impl

import (
	"fmt"

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
		Addresses: make([]string, len(addrs)),
		ID:        hostID,
		// TODO: fill out the other details
	}

	for i, addr := range addrs {
		details.Addresses[i] = fmt.Sprintf("%s/ipfs/%s", addr, hostID.Pretty())
	}

	return &details, nil
}
