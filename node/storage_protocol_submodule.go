package node

import "github.com/filecoin-project/go-filecoin/protocol/storage"

// StorageProtocolSubmodule enhances the `Node` with "Storage" protocol capabilities.
type StorageProtocolSubmodule struct {
	StorageAPI *storage.API

	// Storage Market Interfaces
	StorageMiner *storage.Miner
}
