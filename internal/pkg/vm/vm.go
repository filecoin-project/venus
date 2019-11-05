package vm

import (
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/vminternal/storagemap"
)

// Re-exports

// StorageMap manages Storages.
type StorageMap = storagemap.StorageMap

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return storagemap.NewStorageMap(bs)
}
