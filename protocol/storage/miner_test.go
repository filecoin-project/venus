package storage

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type minerTestPorcelain struct {
	config *cfg.Config
}

func newMinerTestPorcelain() *minerTestPorcelain {
	return &minerTestPorcelain{
		config: cfg.NewConfig(repo.NewInMemoryRepo()),
	}
}

func (mtp *minerTestPorcelain) MessageSendWithRetry(ctx context.Context, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasUnits, params ...interface{}) error {
	return nil
}

func (mtp *minerTestPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return [][]byte{}, nil, nil
}

func (mtp *minerTestPorcelain) ConfigGet(dottedPath string) (interface{}, error) {
	return mtp.config.Get(dottedPath)
}

func (mtp *minerTestPorcelain) ConfigSet(dottedKey string, jsonString string) error {
	return mtp.config.Set(dottedKey, jsonString)
}

func TestReceiveStorageProposal(t *testing.T) {
	t.Run("Accepts proposals with sufficient TotalPrice", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		accepted := false
		rejected := false

		porcelainAPI := newMinerTestPorcelain()
		miner := Miner{
			porcelainAPI: porcelainAPI,
			proposalAcceptor: func(ctx context.Context, m *Miner, p *DealProposal) (*DealResponse, error) {
				accepted = true
				return &DealResponse{}, nil
			},
			proposalRejector: func(ctx context.Context, m *Miner, p *DealProposal, reason string) (*DealResponse, error) {
				rejected = true
				return &DealResponse{Message: reason}, nil
			},
		}

		// configure storage price
		porcelainAPI.config.Set("mining.storagePrice", `"50"`)

		proposal := &DealProposal{
			TotalPrice: types.NewAttoFILFromFIL(75),
		}

		_, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.True(accepted, "Proposal has been accepted")
		assert.False(rejected, "Proposal has not been rejected")
	})

	t.Run("Rejects proposals with insufficient TotalPrice", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		accepted := false
		rejected := false

		porcelainAPI := newMinerTestPorcelain()
		miner := Miner{
			porcelainAPI: porcelainAPI,
			proposalAcceptor: func(ctx context.Context, m *Miner, p *DealProposal) (*DealResponse, error) {
				accepted = true
				return &DealResponse{}, nil
			},
			proposalRejector: func(ctx context.Context, m *Miner, p *DealProposal, reason string) (*DealResponse, error) {
				rejected = true
				return &DealResponse{Message: reason}, nil
			},
		}

		// configure storage price
		porcelainAPI.config.Set("mining.storagePrice", `"50"`)

		proposal := &DealProposal{
			TotalPrice: types.NewAttoFILFromFIL(25),
		}

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.False(accepted, "Proposal has not been accepted")
		assert.True(rejected, "Proposal has been rejected")

		assert.Equal("proposed price 25 is less that miner's current asking price: 50", res.Message)
	})
}

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
			dealsDs: repo.NewInMemoryRepo().DealsDatastore(),
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
