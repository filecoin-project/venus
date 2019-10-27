package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
)

// StorageProtocolSubmodule enhances the `Node` with "Storage" protocol capabilities.
type StorageProtocolSubmodule struct {
	StorageAPI *storage.API

	// Storage Market Interfaces
	StorageMiner *storage.Miner
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(ctx context.Context) (StorageProtocolSubmodule, error) {
	return StorageProtocolSubmodule{
		// StorageAPI: nil,
		// StorageMiner: nil,
	}, nil
}
