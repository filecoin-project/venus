package storage_mining_submodule

import (
	"sync"

	"github.com/filecoin-project/go-fil-markets/storedcounter"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fsm "github.com/filecoin-project/storage-fsm"
	"github.com/ipfs/go-datastore"
)

// persistedSectorIDCounter dispenses unique sector numbers using a
// monotonically increasing internal counter
type persistedSectorNumberCounter struct {
	inner   *storedcounter.StoredCounter
	innerLk sync.Mutex
}

func (s *persistedSectorNumberCounter) Next() (abi.SectorNumber, error) {
	s.innerLk.Lock()
	defer s.innerLk.Unlock()
	i, err := s.inner.Next()
	return abi.SectorNumber(i), err
}

func newPersistedSectorNumberCounter(ds datastore.Batching) fsm.SectorIDCounter {
	sc := storedcounter.New(ds, datastore.NewKey("/storage/nextid"))
	return &persistedSectorNumberCounter{inner: sc}
}
