package api_impl

import (
	"fmt"

	"github.com/filecoin-project/go-filecoin/api"
)

type NodeId struct {
	api *API
}

func NewNodeId(api *API) *NodeId {
	return &NodeId{api: api}
}

// Details, returns detailed information about the underlying node.
func (a *NodeId) Details() (*api.IdDetails, error) {
	host := a.api.node.Host
	hostId := host.ID().Pretty()
	addrs := host.Addrs()

	details := api.IdDetails{
		Addresses: make([]string, len(addrs)),
		ID:        hostId,
		// TODO: fill out the other details
	}

	for i, addr := range addrs {
		details.Addresses[i] = fmt.Sprintf("%s/ipfs/%s", addr, hostId)
	}

	return &details, nil
}
