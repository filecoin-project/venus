package chain

import (
	"sync"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type SyncCidSet struct {
	lk  sync.Mutex
	set *cid.Set
}

func (scs *SyncCidSet) Add(c *cid.Cid) {
	scs.lk.Lock()
	defer scs.lk.Unlock()
	scs.set.Add(c)
}

func (scs *SyncCidSet) Has(c *cid.Cid) bool {
	scs.lk.Lock()
	defer scs.lk.Unlock()
	return scs.set.Has(c)
}
