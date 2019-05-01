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

// sectorInfo combines sector Metadata from rust proofs with go-filecoin specific data
type sectorInfo struct {
	Metadata         *sectorbuilder.SealedSectorMetadata
	CommitMessageCid *cid.Cid
	Succeeded        bool
	ErrorMessage     string
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
	// Maps from sector id to information about sector seal.
	SealedSectors map[uint64]*sectorInfo

	onSuccess func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata)
	onFail    func(dealCid cid.Cid, message string)
}

func newDealsAwaitingSeal() *dealsAwaitingSeal {
	return &dealsAwaitingSeal{
		SectorsToDeals: make(map[uint64][]cid.Cid),
		SealedSectors:  make(map[uint64]*sectorInfo),
	}
}

func (dealsAwaitingSeal *dealsAwaitingSeal) add(sectorID uint64, dealCid cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	sector, ok := dealsAwaitingSeal.SealedSectors[sectorID]

	// if sector sealing hasn't succeed or failed yet, just add to SectorToDeals and exit
	if !ok {
		deals, ok := dealsAwaitingSeal.SectorsToDeals[sectorID]
		if ok {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = append(deals, dealCid)
		} else {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = []cid.Cid{dealCid}
		}
		return
	}

	// We have sealing information, so process deal with sector data immediately
	if sector.Succeeded {
		dealsAwaitingSeal.onSuccess(dealCid, sector.Metadata)
	} else {
		dealsAwaitingSeal.onFail(dealCid, sector.ErrorMessage)
	}

	// Don't keep references to sectors around forever. Assume that at most
	// one success-before-add call will happen (eg, in a test). Sector sealing
	// outside of tests is so slow that it shouldn't happen in practice.
	// So now that it has happened once, clean it up. If we wanted to keep
	// the state around for longer for some reason we need to limit how many
	// sectors we hang onto, eg keep a fixed-length slice of successes
	// and failures and shift the oldest off and the newest on.
	delete(dealsAwaitingSeal.SealedSectors, sectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) success(sector *sectorbuilder.SealedSectorMetadata, commitMessage *cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SealedSectors[sector.SectorID] = &sectorInfo{
		Succeeded:        true,
		Metadata:         sector,
		CommitMessageCid: commitMessage,
	}

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sector.SectorID] {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sector.SectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) fail(sectorID uint64, message string) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SealedSectors[sectorID] = &sectorInfo{
		Succeeded:    false,
		ErrorMessage: message,
	}

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sectorID] {
		dealsAwaitingSeal.onFail(dealCid, message)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) commitMessageCid(sectorID uint64) (*cid.Cid, bool) {
	sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
	if !ok {
		return nil, false
	}
	return sectorData.CommitMessageCid, sectorData.CommitMessageCid != nil
}
