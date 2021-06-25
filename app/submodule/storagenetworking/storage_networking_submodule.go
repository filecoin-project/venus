package storagenetworking

import (
	"context"
	"github.com/filecoin-project/venus/app/submodule/network"

	exchange "github.com/ipfs/go-ipfs-exchange-interface"
)

// StorageNetworkingSubmodule enhances the `Node` with data transfer capabilities.
type StorageNetworkingSubmodule struct { //nolint
	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface
}

// NewStorgeNetworkingSubmodule creates a new storage networking submodule.
func NewStorgeNetworkingSubmodule(ctx context.Context, network *network.NetworkSubmodule) (*StorageNetworkingSubmodule, error) {
	return &StorageNetworkingSubmodule{
		Exchange: network.Bitswap,
	}, nil
}

//API create a new storage implement
func (storageNetworking *StorageNetworkingSubmodule) API() IStorageNetworking {
	return &storageNetworkingAPI{storageNetworking: storageNetworking}
}
