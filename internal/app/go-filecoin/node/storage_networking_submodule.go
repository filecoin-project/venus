package node

import exchange "github.com/ipfs/go-ipfs-exchange-interface"

// StorageNetworkingSubmodule enhances the `Node` with data transfer capabilities.
type StorageNetworkingSubmodule struct {
	// Exchange is the interface for fetching data from other nodes.
	Exchange exchange.Interface
}
