package storage

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
)

func TestDealsAwaitingSeal(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	cid0 := newCid()
	cid1 := newCid()
	cid2 := newCid()

	wantSectorID := uint64(42)
	wantSector := &sectorbuilder.SealedSectorMetadata{SectorID: wantSectorID}
	someOtherSectorID := uint64(100)

	wantMessage := "boom"

	t.Run("saveDealsAwaitingSeal saves, loadDealsAwaitingSeal loads", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		miner := &Miner{
			dealsAwaitingSeal: &dealsAwaitingSealStruct{
				SectorsToDeals:    make(map[uint64][]cid.Cid),
				SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
				FailedSectors:     make(map[uint64]string),
			},
			dealsAwaitingSealDs: repo.NewInMemoryRepo().DealsAwaitingSealDatastore(),
		}

		miner.dealsAwaitingSeal.add(wantSectorID, cid0)

		require.NoError(miner.saveDealsAwaitingSeal())
		miner.dealsAwaitingSeal = &dealsAwaitingSealStruct{}
		require.NoError(miner.loadDealsAwaitingSeal())

		assert.Equal(cid0, miner.dealsAwaitingSeal.SectorsToDeals[42][0])
	})

	t.Run("add before success", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1)
		dealsAwaitingSeal.add(someOtherSectorID, cid2)
		dealsAwaitingSeal.success(wantSector)

		assert.Len(gotCids, 2, "onSuccess should've been called twice")
	})

	t.Run("add after success", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.success(wantSector)
		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1) // Shouldn't trigger a call, see add().
		dealsAwaitingSeal.add(someOtherSectorID, cid2)

		assert.Len(gotCids, 1, "onSuccess should've been called once")
	})

	t.Run("add before fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1)
		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")

		assert.Len(gotCids, 2, "onFail should've been called twice")
	})

	t.Run("add after fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onFail = func(dealCid cid.Cid, message string) {
			assert.Equal(message, wantMessage)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.fail(wantSectorID, wantMessage)
		dealsAwaitingSeal.fail(someOtherSectorID, "some message")
		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1) // Shouldn't trigger a call, see add().

		assert.Len(gotCids, 1, "onFail should've been called once")
	})
}
