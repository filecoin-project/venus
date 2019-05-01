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

	t.Run("add before success", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1)
		dealsAwaitingSeal.add(someOtherSectorID, cid2)
		dealsAwaitingSeal.success(wantSector, &commitSectorCid)

		assert.Len(t, gotCids, 2, "onSuccess should've been called twice")
	})

	t.Run("add after success", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.success(wantSector, &commitSectorCid)
		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1) // Shouldn't trigger a call, see add().
		dealsAwaitingSeal.add(someOtherSectorID, cid2)

		assert.Len(t, gotCids, 1, "onSuccess should've been called once")
	})

	t.Run("add before fail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1)
		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")

		assert.Len(t, gotCids, 2, "onFail should've been called twice")
	})

	t.Run("add after fail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")
		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1) // Shouldn't trigger a call, see add().

		assert.Len(t, gotCids, 1, "onFail should've been called once")
	})
}

func TestDealsAwaitingSealSuccess(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorId := uint64(42)
	sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorId}
	msgCid := newCid()

	t.Run("success calls onSuccess for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.True(t, unseenDealCids[dealCid])
			delete(unseenDealCids, dealCid)
		}

		dealsAwaitingSeal.success(sector, &msgCid)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("success clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)

		dealsAwaitingSeal.success(sector, &msgCid)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorId])
	})

	t.Run("success adds sector to successful sectors", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)

		dealsAwaitingSeal.success(sector, &msgCid)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorId]
		require.True(t, ok)
		assert.Equal(t, sector, sectorData.Metadata)
	})

	t.Run("success stores commit message cid with sector data", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)

		dealsAwaitingSeal.success(sector, &msgCid)

		actualMsgCid, ok := dealsAwaitingSeal.commitMessageCid(sectorId)
		require.True(t, ok)
		assert.Equal(t, msgCid, *actualMsgCid)
	})
}

func TestDealsAwaitingSealFail(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorId := uint64(42)
	errorMessage := "test error message"

	t.Run("fail calls onFail with correct message for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			require.True(t, unseenDealCids[dealCid])
			assert.Equal(t, errorMessage, message)
			delete(unseenDealCids, dealCid)
		}

		dealsAwaitingSeal.fail(sectorId, errorMessage)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("fail clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)

		dealsAwaitingSeal.fail(sectorId, errorMessage)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorId])
	})

	t.Run("fail adds error message to failure map", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorId, cid1, cid2)

		dealsAwaitingSeal.fail(sectorId, errorMessage)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorId]
		require.True(t, ok)
		assert.Equal(t, errorMessage, sectorData.ErrorMessage)
	})
}

func setupTestDealsAwaitingSeals(sectorId uint64, deals ...cid.Cid) *dealsAwaitingSeal {
	dealsAwaitingSeal := newDealsAwaitingSeal()
	dealsAwaitingSeal.SectorsToDeals[sectorId] = deals
	dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {}
	dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {}
	return dealsAwaitingSeal
}
