package sectors

import (
	"sync"

	fsm "github.com/filecoin-project/go-filecoin/vendors/storage-sealing"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/ipfs/go-datastore"
)

// PersistedSectorNumberCounter dispenses unique sector numbers using a
// monotonically increasing internal counter
type PersistedSectorNumberCounter struct {
	inner   *storedcounter.StoredCounter
	innerLk sync.Mutex
}

var _ fsm.SectorIDCounter = new(PersistedSectorNumberCounter)

func (s *PersistedSectorNumberCounter) Next() (abi.SectorNumber, error) {
	s.innerLk.Lock()
	defer s.innerLk.Unlock()
	i, err := s.inner.Next()
	return abi.SectorNumber(i), err
}

func NewPersistedSectorNumberCounter(ds datastore.Batching) fsm.SectorIDCounter {
	sc := storedcounter.New(ds, datastore.NewKey("/storage/nextid"))
	return &PersistedSectorNumberCounter{inner: sc}
}
