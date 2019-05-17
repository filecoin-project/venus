package storage

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDealsAwaitingSealAdd(t *testing.T) {
	tf.UnitTest(t)

	newCid := types.NewCidForTestGetter()
	cid0 := newCid()
	cid1 := newCid()
	cid2 := newCid()
	commitSectorCid := newCid()

	wantSectorID := uint64(42)
	wantSector := &sectorbuilder.SealedSectorMetadata{SectorID: wantSectorID}
	someOtherSectorID := uint64(100)

	wantMessage := "boom"

	t.Run("process before success", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.process(wantSectorID, cid0)
		dealsAwaitingSeal.process(wantSectorID, cid1)
		dealsAwaitingSeal.process(someOtherSectorID, cid2)
		dealsAwaitingSeal.success(wantSector, commitSectorCid)

		assert.Len(t, gotCids, 2, "onSuccess should've been called twice")
	})

	t.Run("process after success", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.success(wantSector, commitSectorCid)
		dealsAwaitingSeal.process(wantSectorID, cid0)
		dealsAwaitingSeal.process(wantSectorID, cid1) // Shouldn't trigger a call, see process().
		dealsAwaitingSeal.process(someOtherSectorID, cid2)

		assert.Len(t, gotCids, 1, "onSuccess should've been called once")
	})

	t.Run("process before fail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.Fail(t, "onSuccess should not have been called")
		}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.process(wantSectorID, cid0)
		dealsAwaitingSeal.process(wantSectorID, cid1)
		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")

		assert.Len(t, gotCids, 2, "onFail should've been called twice")
	})

	t.Run("process after fail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.Fail(t, "onSuccess should not have been called")
		}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")
		dealsAwaitingSeal.process(wantSectorID, cid0)
		dealsAwaitingSeal.process(wantSectorID, cid1) // Shouldn't trigger a call, see process().

		assert.Len(t, gotCids, 1, "onFail should've been called once")
	})
}

func TestDealsAwaitingSealSuccess(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorID := uint64(42)
	sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorID}
	msgCid := newCid()

	t.Run("success calls onSuccess for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.True(t, unseenDealCids[dealCid])
			delete(unseenDealCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.success(sector, msgCid)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("success clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.success(sector, msgCid)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorID])
	})

	t.Run("success adds sector to successful sectors", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.success(sector, msgCid)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
		require.True(t, ok)
		assert.Equal(t, sector, sectorData.Metadata)
	})

	t.Run("success stores commit message cid with sector data", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.success(sector, msgCid)

		actualMsgCid, ok := dealsAwaitingSeal.commitMessageCid(sectorID)
		require.True(t, ok)
		assert.Equal(t, msgCid, actualMsgCid)
	})
}

func TestDealsAwaitingSealFail(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorID := uint64(42)
	errorMessage := "test error message"

	t.Run("fail calls onFail with correct message for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			require.True(t, unseenDealCids[dealCid])
			assert.Equal(t, errorMessage, message)
			delete(unseenDealCids, dealCid)
		}

		dealsAwaitingSeal.fail(sectorID, errorMessage)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("fail clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.fail(sectorID, errorMessage)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorID])
	})

	t.Run("fail adds error message to failure map", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.fail(sectorID, errorMessage)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
		require.True(t, ok)
		assert.Equal(t, errorMessage, sectorData.ErrorMessage)
	})
}

func setupTestDealsAwaitingSeals(sectorID uint64, deals ...cid.Cid) *dealsAwaitingSeal {
	dealsAwaitingSeal := newDealsAwaitingSeal()
	dealsAwaitingSeal.SectorsToDeals[sectorID] = deals
	dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {}
	dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {}
	return dealsAwaitingSeal
}
