package storage

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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

	t.Run("attachDealToSector before onSealSuccess", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid0)
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid1)
		dealsAwaitingSeal.attachDealToSector(context.Background(), someOtherSectorID, cid2)
		dealsAwaitingSeal.onSealSuccess(context.Background(), wantSector, commitSectorCid)

		assert.Len(t, gotCids, 2, "onSuccess should've been called twice")
	})

	t.Run("attachDealToSector after onSealSuccess", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.onSealSuccess(context.Background(), wantSector, commitSectorCid)
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid0)
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid1) // Shouldn't trigger a call, see attachDealToSector().
		dealsAwaitingSeal.attachDealToSector(context.Background(), someOtherSectorID, cid2)

		assert.Len(t, gotCids, 1, "onSuccess should've been called once")
	})

	t.Run("attachDealToSector before onSealFail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.Fail(t, "onSuccess should not have been called")
		}
		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid0)
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid1)
		dealsAwaitingSeal.onSealFail(context.Background(), wantSectorID, wantMessage)
		dealsAwaitingSeal.onSealFail(context.Background(), someOtherSectorID, "some message")

		assert.Len(t, gotCids, 2, "onFail should've been called twice")
	})

	t.Run("attachDealToSector after onSealFail", func(t *testing.T) {
		dealsAwaitingSeal := newDealsAwaitingSeal()
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.Fail(t, "onSuccess should not have been called")
		}
		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			assert.Equal(t, message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.onSealFail(context.Background(), wantSectorID, wantMessage)
		dealsAwaitingSeal.onSealFail(context.Background(), someOtherSectorID, "some message")
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid0)
		dealsAwaitingSeal.attachDealToSector(context.Background(), wantSectorID, cid1) // Shouldn't trigger a call, see attachDealToSector().

		assert.Len(t, gotCids, 1, "onFail should've been called once")
	})
}

func TestDealsAwaitingSealSuccess(t *testing.T) {
	tf.UnitTest(t)

	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorID := uint64(42)
	sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorID}
	msgCid := newCid()

	t.Run("onSealSuccess calls onSuccess for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			require.True(t, unseenDealCids[dealCid])
			delete(unseenDealCids, dealCid)
		}
		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			require.Fail(t, "onFail should not have been called")
		}

		dealsAwaitingSeal.onSealSuccess(context.Background(), sector, msgCid)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("onSealSuccess clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.onSealSuccess(context.Background(), sector, msgCid)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorID])
	})

	t.Run("onSealSuccess adds sector to successful sectors", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.onSealSuccess(context.Background(), sector, msgCid)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
		require.True(t, ok)
		assert.Equal(t, sector, sectorData.Metadata)
	})

	t.Run("onSealSuccess stores commit message cid with sector data", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.onSealSuccess(context.Background(), sector, msgCid)

		actualMsgCid, ok := dealsAwaitingSeal.commitMessageCid(sectorID)
		require.True(t, ok)
		assert.Equal(t, msgCid, actualMsgCid)
	})
}

func TestDealsAwaitingSealFail(t *testing.T) {
	tf.UnitTest(t)

	newCid := types.NewCidForTestGetter()
	cid1 := newCid()
	cid2 := newCid()

	sectorID := uint64(42)
	errorMessage := "test error message"

	t.Run("onSealFail calls onFail with correct message for all deals", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)
		unseenDealCids := map[cid.Cid]bool{cid1: true, cid2: true}

		dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {
			require.True(t, unseenDealCids[dealCid])
			assert.Equal(t, errorMessage, message)
			delete(unseenDealCids, dealCid)
		}

		dealsAwaitingSeal.onSealFail(context.Background(), sectorID, errorMessage)

		// called onSuccess for all deals
		assert.Equal(t, 0, len(unseenDealCids))
	})

	t.Run("onSealFail clears sector from sector to deals cache", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.onSealFail(context.Background(), sectorID, errorMessage)

		// cleared SectorsToDeals for this sector
		assert.Nil(t, dealsAwaitingSeal.SectorsToDeals[sectorID])
	})

	t.Run("onSealFail adds error message to failure map", func(t *testing.T) {
		dealsAwaitingSeal := setupTestDealsAwaitingSeals(sectorID, cid1, cid2)

		dealsAwaitingSeal.onSealFail(context.Background(), sectorID, errorMessage)

		sectorData, ok := dealsAwaitingSeal.SealedSectors[sectorID]
		require.True(t, ok)
		assert.Equal(t, errorMessage, sectorData.ErrorMessage)
	})
}

func setupTestDealsAwaitingSeals(sectorID uint64, deals ...cid.Cid) *dealsAwaitingSeal {
	dealsAwaitingSeal := newDealsAwaitingSeal()
	dealsAwaitingSeal.SectorsToDeals[sectorID] = deals
	dealsAwaitingSeal.onSuccess = func(_ context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {}
	dealsAwaitingSeal.onFail = func(_ context.Context, dealCid cid.Cid, message string) {}
	return dealsAwaitingSeal
}
