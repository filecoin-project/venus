package submodule

import (
	"context"

	exchange "github.com/ipfs/go-ipfs-exchange-interface"
)

// StorageNetworkingSubmodule enhances the `Node` with data transfer capabilities.
type StorageNetworkingSubmodule struct {
	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface
}

// NewStorgeNetworkingSubmodule creates a new storage networking submodule.
func NewStorgeNetworkingSubmodule(ctx context.Context, network *NetworkSubmodule) (StorageNetworkingSubmodule, error) {
	return StorageNetworkingSubmodule{
		Exchange: network.Bitswap,
	}, nil
}
