package api_impl

import (
	"fmt"

	"github.com/filecoin-project/go-filecoin/api"
)

type NodeID struct {
	api *NodeAPI
}

func NewNodeID(api *NodeAPI) *NodeID {
	return &NodeID{api: api}
}

// Details, returns detailed information about the underlying node.
func (a *NodeID) Details() (*api.IDDetails, error) {
	host := a.api.node.Host
	hostID := host.ID().Pretty()
	addrs := host.Addrs()

	details := api.IDDetails{
		Addresses: make([]string, len(addrs)),
		ID:        hostID,
		// TODO: fill out the other details
	}

	for i, addr := range addrs {
		details.Addresses[i] = fmt.Sprintf("%s/ipfs/%s", addr, hostID)
	}

	return &details, nil
}
