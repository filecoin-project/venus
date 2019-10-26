package storage

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/ipfs/go-cid"
)

func init() {
	encoding.RegisterIpldCborType(dealsAwaitingSeal{})
}

// sectorInfo combines sector Metadata from rust proofs with go-filecoin specific data
type sectorInfo struct {
	// Metadata contains information about the sealed sector needed to verify the seal
	Metadata *sectorbuilder.SealedSectorMetadata

	// CommitMessageCid is the cid of the commitSector message sent for sealed sector. It allows the client to coordinate on timing.
	CommitMessageCid cid.Cid

	// Succeeded indicates whether sealing was and committing was successful
	Succeeded bool

	// ErrorMessage indicate what went wrong if sealing or committing was not successful
	ErrorMessage string
}

// dealsAwaitingSeal is a container for keeping track of which sectors have
// pieces from which deals. We need it to accommodate a race condition where
// a sector commit message is added to chain before we can process the sector/deal
// book-keeping. It effectively caches success and failure results for sectors
// for tardy attachDealToSector() calls.
type dealsAwaitingSeal struct {
	l sync.Mutex
	// Maps from sector id to the deal cids with pieces in the sector.
	SectorsToDeals map[uint64][]cid.Cid
	// Maps from sector id to information about sector seal.
	SealedSectors map[uint64]*sectorInfo

	// onSuccess will be called only after the sector has been successfully sealed
	onSuccess func(ctx context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata)

	// onFail will be called if an error occurs during sealing or commitment
	onFail func(ctx context.Context, dealCid cid.Cid, message string)
}

func newDealsAwaitingSeal() *dealsAwaitingSeal {
	return &dealsAwaitingSeal{
		SectorsToDeals: make(map[uint64][]cid.Cid),
		SealedSectors:  make(map[uint64]*sectorInfo),
	}
}

// attachDealToSector checks the list of sealed sectors to see if a sector has been sealed. If sealing of this sector is done,
// onSuccess or onFailure will be called immediately, otherwise, add it to SectorsToDeals so we can respond
// when sealing completes.
func (dealsAwaitingSeal *dealsAwaitingSeal) attachDealToSector(ctx context.Context, sectorID uint64, dealCid cid.Cid) {
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
		dealsAwaitingSeal.onSuccess(ctx, dealCid, sector.Metadata)
	} else {
		dealsAwaitingSeal.onFail(ctx, dealCid, sector.ErrorMessage)
	}

	// Don't keep references to sectors around forever. Assume that at most
	// one onSealSuccess-before-attachDealToSector call will happen (eg, in a test). Sector sealing
	// outside of tests is so slow that it shouldn't happen in practice.
	// So now that it has happened once, clean it up. If we wanted to keep
	// the state around for longer for some reason we need to limit how many
	// sectors we hang onto, eg keep a fixed-length slice of successes
	// and failures and shift the oldest off and the newest on.
	delete(dealsAwaitingSeal.SealedSectors, sectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) onSealSuccess(ctx context.Context, sector *sectorbuilder.SealedSectorMetadata, commitMessageCID cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SealedSectors[sector.SectorID] = &sectorInfo{
		Succeeded:        true,
		Metadata:         sector,
		CommitMessageCid: commitMessageCID,
	}

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sector.SectorID] {
		dealsAwaitingSeal.onSuccess(ctx, dealCid, sector)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sector.SectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) onSealFail(ctx context.Context, sectorID uint64, message string) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SealedSectors[sectorID] = &sectorInfo{
		Succeeded:    false,
		ErrorMessage: message,
	}

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sectorID] {
		dealsAwaitingSeal.onFail(ctx, dealCid, message)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSeal) commitMessageCid(sectorID uint64) (cid.Cid, bool) {
	sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
	if !ok {
		return cid.Undef, false
	}
	return sectorData.CommitMessageCid, sectorData.CommitMessageCid != cid.Undef
}

// MarhsalJSON ensures that we have a lock while we marshal this structure to avoid concurrent
// iteration / writes.
func (dealsAwaitingSeal *dealsAwaitingSeal) MarshalJSON() ([]byte, error) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	return json.Marshal(struct {
		SectorsToDeals map[uint64][]cid.Cid
		SealedSectors  map[uint64]*sectorInfo
	}{
		SectorsToDeals: dealsAwaitingSeal.SectorsToDeals,
		SealedSectors:  dealsAwaitingSeal.SealedSectors,
	})
}
