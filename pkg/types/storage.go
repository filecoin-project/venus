package types

import (
	"github.com/ipfs/go-datastore"
)

// MetadataDS stores metadata. By default it's namespaced under /metadata in
// main repo datastore.
type MetadataDS datastore.Batching
