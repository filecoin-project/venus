package storage

import (
	"context"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	offroute "gx/ipfs/QmVZ6cQXHoTQja4oo9GhhHZi7dThi4x98mRKgGtKnTy37u/go-ipfs-routing/offline"
	rhost "gx/ipfs/QmYxivS34F2M2n44WQQnRHGAKS8aoRUxwGpi9wk4Cdn4Jf/go-libp2p/p2p/host/routed"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/network"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	w "github.com/filecoin-project/go-filecoin/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type minerTestPorcelain struct {
	config  *cfg.Config
	network *network.Network
	wallet  *w.Wallet
}

func newminerTestPorcelain() *minerTestPorcelain {
	repo := repo.NewInMemoryRepo()

	router := offroute.NewOfflineRouter(repo.Datastore(), network.BlankValidator{})
	peerHost := rhost.Wrap(network.NoopLibP2PHost{}, router)
	network := network.NewNetwork(peerHost)

	walletBackend, _ := w.NewDSBackend(repo.WalletDatastore())

	return &minerTestPorcelain{
		config: cfg.NewConfig(repo),
		network: network,
		wallet: w.New(walletBackend),
	}
}

func (mtp *minerTestPorcelain) MessageSendWithRetry(ctx context.Context, numRetries uint, waitDuration time.Duration, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasUnits, params ...interface{}) error {
	return nil
}

func (mtp *minerTestPlumbing) MessagePreview(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	return types.NewGasUnits(0), nil
}

func (mtp *minerTestPlumbing) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return [][]byte{}, nil, nil
}

func (mtp *minerTestPlumbing) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error) {
	return nil, nil
}

func (mtp *minerTestPlumbing) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return cid.Cid{}, nil
}

func (mtp *minerTestPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return [][]byte{}, nil, nil
}

func (mtp *minerTestPorcelain) ConfigGet(dottedPath string) (interface{}, error) {
	return mtp.config.Get(dottedPath)
}

func (mtp *minerTestPlumbing) ConfigSet(dottedKey string, jsonString string) error {
	return mtp.config.Set(dottedKey, jsonString)
}

func (mtp *minerTestPlumbing) WalletAddresses() []address.Address {
	return mtp.wallet.Addresses()
}

func (mtp *minerTestPlumbing) WalletFind(address address.Address) (w.Backend, error) {
	return mtp.wallet.Find(address)
}

func TestReceiveStorageProposal(t *testing.T) {
	t.Run("Accepts proposals with sufficient TotalPrice", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		accepted := false
		rejected := false

		porcelainAPI := newminerTestPorcelain()
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

		porcelainAPI := newminerTestPorcelain()
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
