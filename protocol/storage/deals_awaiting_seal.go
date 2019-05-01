package storage

import (
	"sync"

	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(dealsAwaitingSeal{})
}

// sectorData combines sector Metadata from rust proofs with go-filecoin specific data
type sectorData struct {
	Metadata      *sectorbuilder.SealedSectorMetadata
	CommitMessage *cid.Cid
}

// dealsAwaitingSeal is a container for keeping track of which sectors have
// pieces from which deals. We need it to accommodate a race condition where
// a sector commit message is added to chain before we can add the sector/deal
// book-keeping. It effectively caches success and failure results for sectors
// for tardy add() calls.
type dealsAwaitingSeal struct {
	l sync.Mutex
	// Maps from sector id to the deal cids with pieces in the sector.
	SectorsToDeals map[uint64][]cid.Cid
	// Maps from sector id to sector.
	SuccessfulSectors map[uint64]*sectorData
	// Maps from sector id to seal failure error string.
	FailedSectors map[uint64]string

	onSuccess func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata)
	onFail    func(dealCid cid.Cid, message string)
}

func newDealsAwaitingSeal() *dealsAwaitingSeal {
	return &dealsAwaitingSeal{
		SectorsToDeals:    make(map[uint64][]cid.Cid),
		SuccessfulSectors: make(map[uint64]*sectorData),
		FailedSectors:     make(map[uint64]string),
	}
}

func (dealsAwaitingSeal *dealsAwaitingSeal) add(sectorID uint64, dealCid cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	if sector, ok := dealsAwaitingSeal.SuccessfulSectors[sectorID]; ok {
		dealsAwaitingSeal.onSuccess(dealCid, sector.Metadata)
		// Don't keep references to sectors around forever. Assume that at most
		// one success-before-add call will happen (eg, in a test). Sector sealing
		// outside of tests is so slow that it shouldn't happen in practice.
		// So now that it has happened once, clean it up. If we wanted to keep
		// the state around for longer for some reason we need to limit how many
		// sectors we hang onto, eg keep a fixed-length slice of successes
		// and failures and shift the oldest off and the newest on.
		delete(dealsAwaitingSeal.SuccessfulSectors, sectorID)
	} else if message, ok := dealsAwaitingSeal.FailedSectors[sectorID]; ok {
		dealsAwaitingSeal.onFail(dealCid, message)
		// Same as above.
		delete(dealsAwaitingSeal.FailedSectors, sectorID)
	} else {
		deals, ok := dealsAwaitingSeal.SectorsToDeals[sectorID]
		if ok {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = append(deals, dealCid)
		} else {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = []cid.Cid{dealCid}
		}
	}
}

func (dealsAwaitingSeal *dealsAwaitingSeal) success(sector *sectorbuilder.SealedSectorMetadata, commitMessage *cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SuccessfulSectors[sector.SectorID] = &sectorData{
		Metadata:      sector,
		CommitMessage: commitMessage,
	}

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sector.SectorID] {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sector.SectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) fail(sectorID uint64, message string) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.FailedSectors[sectorID] = message

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sectorID] {
		dealsAwaitingSeal.onFail(dealCid, message)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) commitMessageCid(sectorID uint64) (*cid.Cid, bool) {
	sectorData, ok := dealsAwaitingSeal.SuccessfulSectors[sectorID]
	if !ok {
		return nil, false
	}
	return sectorData.CommitMessage, sectorData.CommitMessage != nil
}
