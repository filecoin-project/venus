package vm2

import (
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal/storagemap"
)

// Re-exports

// Runtime is the vm runtime.
type Runtime = runtime.Runtime

// Storage is the vm storage.
type Storage = runtime.Storage

// StorageMap manages Storages.
type StorageMap = storagemap.StorageMap

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return storagemap.NewStorageMap(bs)
}
