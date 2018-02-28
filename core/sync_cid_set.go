package core

import (
	"sync"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// SyncCidSet wraps a lock around cid.Set operations.
type SyncCidSet struct {
	lk  sync.Mutex
	set *cid.Set
}

// Add adds a cid to the set.
func (scs *SyncCidSet) Add(c *cid.Cid) {
	scs.lk.Lock()
	defer scs.lk.Unlock()
	scs.set.Add(c)
}

// Has returns if the passed in cid is in this set.
func (scs *SyncCidSet) Has(c *cid.Cid) bool {
	scs.lk.Lock()
	defer scs.lk.Unlock()
	return scs.set.Has(c)
}
