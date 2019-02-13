package storage

import (
	"context"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReceiveStorageProposal(t *testing.T) {
	t.Run("Accepts proposals with sufficient TotalPrice", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		accepted := false
		rejected := false
		var message string

		porcelainAPI := newMinerTestPorcelain()
		miner := Miner{
			porcelainAPI:   porcelainAPI,
			minerOwnerAddr: porcelainAPI.targetAddress,
			proposalAcceptor: func(ctx context.Context, m *Miner, p *DealProposal) (*DealResponse, error) {
				accepted = true
				return &DealResponse{State: Accepted}, nil
			},
			proposalRejector: func(ctx context.Context, m *Miner, p *DealProposal, reason string) (*DealResponse, error) {
				message = reason
				rejected = true
				return &DealResponse{State: Rejected, Message: reason}, nil
			},
		}

		proposal := testDealProposal(porcelainAPI, VoucherInterval, 1773, porcelainAPI.targetAddress)

		_, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.True(accepted, "Proposal has been accepted")
		assert.False(rejected, "Proposal has not been rejected")
		assert.Equal("", message)
	})

	t.Run("Rejects proposals with insufficient TotalPrice", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, proposal := newMinerTestSetup()

		// configure storage price
		porcelainAPI.config.Set("mining.storagePrice", `".0005"`)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Equal("proposed price (2500) is less than expected (5000) given asking price of 0.0005", res.Message)
	})

	t.Run("Rejects proposals with invalid payment channel", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, proposal := newMinerTestSetup()
		porcelainAPI.noChannels = true

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "could not find payment channel")
	})

	t.Run("Rejects proposals with wrong target", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, miner, proposal := newMinerTestSetup()
		miner.minerOwnerAddr = address.TestAddress

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "not target of payment channel")
	})

	t.Run("Rejects proposals with too short channel eol", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, proposal := newMinerTestSetup()
		porcelainAPI.channelEol = types.NewBlockHeight(1200)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "less than required eol")
	})

	t.Run("Rejects proposals with no payments", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, miner, proposal := newMinerTestSetup()
		proposal.Payment.Vouchers = []*paymentbroker.PaymentVoucher{}

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "contains no payment vouchers")
	})

	t.Run("Rejects proposals with vouchers with invalid signatures", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		_, miner, proposal := newMinerTestSetup()
		proposal.Payment.Vouchers[0].Signature = types.Signature([]byte{})

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "invalid signature in voucher")
	})

	t.Run("Rejects proposals with when payments start too late", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, _ := newMinerTestSetup()
		porcelainAPI.paymentStart = porcelainAPI.paymentStart.Add(types.NewBlockHeight(15))
		proposal := testDealProposal(porcelainAPI, VoucherInterval, 1773, porcelainAPI.targetAddress)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "payments start after deal start interval")
	})

	t.Run("Rejects proposals with vouchers with long intervals", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, _ := newMinerTestSetup()
		porcelainAPI.paymentStart = porcelainAPI.paymentStart.Sub(types.NewBlockHeight(15))
		proposal := testDealProposal(porcelainAPI, VoucherInterval+15, 1773, porcelainAPI.targetAddress)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "interval between vouchers")
	})

	t.Run("Rejects proposals with vouchers with insufficient amounts", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		porcelainAPI, miner, _ := newMinerTestSetup()
		proposal := testDealProposal(porcelainAPI, VoucherInterval, 1, porcelainAPI.targetAddress)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(err)

		assert.Equal(Rejected, res.State)
		assert.Contains(res.Message, "voucher amount")
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

type minerTestPorcelain struct {
	config        *cfg.Config
	payerAddress  address.Address
	targetAddress address.Address
	channelID     *types.ChannelID
	messageCid    *cid.Cid
	signer        types.MockSigner
	noChannels    bool
	blockHeight   *types.BlockHeight
	channelEol    *types.BlockHeight
	paymentStart  *types.BlockHeight
}

func newMinerTestPorcelain() *minerTestPorcelain {
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	payerAddr, err := ki[0].Address()
	if err != nil {
		panic("Could not create payer address")
	}

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()

	cid := cidGetter()

	config := cfg.NewConfig(repo.NewInMemoryRepo())
	config.Set("mining.storagePrice", `".00025"`)

	blockHeight := types.NewBlockHeight(773)
	return &minerTestPorcelain{
		config:        config,
		payerAddress:  payerAddr,
		targetAddress: addressGetter(),
		channelID:     types.NewChannelID(73),
		messageCid:    &cid,
		signer:        mockSigner,
		noChannels:    false,
		channelEol:    types.NewBlockHeight(13773),
		blockHeight:   blockHeight,
		paymentStart:  blockHeight,
	}
}

func (mtp *minerTestPorcelain) MessageSend(ctx context.Context, from, to address.Address, val *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return cid.Cid{}, nil
}

func (mtp *minerTestPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	channels := map[string]*paymentbroker.PaymentChannel{}

	if !mtp.noChannels {
		id := mtp.channelID.KeyString()
		channels[id] = &paymentbroker.PaymentChannel{
			Target:         mtp.targetAddress,
			Amount:         types.NewAttoFILFromFIL(100000),
			AmountRedeemed: types.NewAttoFILFromFIL(0),
			Eol:            mtp.channelEol,
		}
	}

	channelsBytes, err := actor.MarshalStorage(channels)
	if err != nil {
		panic(err)
	}
	return [][]byte{channelsBytes}, nil, nil
}

func (mtp *minerTestPorcelain) ConfigGet(dottedPath string) (interface{}, error) {
	return mtp.config.Get(dottedPath)
}

func (mtp *minerTestPorcelain) ChainBlockHeight(ctx context.Context) (*types.BlockHeight, error) {
	return mtp.blockHeight, nil
}

func (mtp *minerTestPorcelain) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return nil
}

func newTestMiner(api *minerTestPorcelain) *Miner {
	return &Miner{
		porcelainAPI:   api,
		minerOwnerAddr: api.targetAddress,
		proposalAcceptor: func(ctx context.Context, m *Miner, p *DealProposal) (*DealResponse, error) {
			return &DealResponse{State: Accepted}, nil
		},
		proposalRejector: func(ctx context.Context, m *Miner, p *DealProposal, reason string) (*DealResponse, error) {
			return &DealResponse{State: Rejected, Message: reason}, nil
		},
	}
}

func newMinerTestSetup() (*minerTestPorcelain, *Miner, *DealProposal) {
	porcelainAPI := newMinerTestPorcelain()
	return porcelainAPI, newTestMiner(porcelainAPI), testDealProposal(porcelainAPI, VoucherInterval, 1773, porcelainAPI.targetAddress)
}

func testDealProposal(porcelainAPI *minerTestPorcelain, voucherInterval int, amountInc uint64, addr address.Address) *DealProposal {
	vouchers := make([]*paymentbroker.PaymentVoucher, 10)

	for i := 0; i < 10; i++ {
		validAt := porcelainAPI.paymentStart.Add(types.NewBlockHeight(uint64((i + 1) * voucherInterval)))
		amount := types.NewAttoFILFromFIL(uint64(i+1) * amountInc)
		signature, err := paymentbroker.SignVoucher(porcelainAPI.channelID, amount, validAt, porcelainAPI.payerAddress, porcelainAPI.signer)
		if err != nil {
			panic("Could not sign valid proposal")
		}
		vouchers[i] = &paymentbroker.PaymentVoucher{
			Channel:   *porcelainAPI.channelID,
			Payer:     porcelainAPI.payerAddress,
			Target:    porcelainAPI.targetAddress,
			Amount:    *amount,
			ValidAt:   *validAt,
			Signature: signature,
		}
	}

	return &DealProposal{
		TotalPrice: types.NewAttoFILFromFIL(2500),
		Size:       types.NewBytesAmount(1000),
		Duration:   10000,
		Payment: PaymentInfo{
			Payer:         porcelainAPI.payerAddress,
			PayChActor:    address.PaymentBrokerAddress,
			Channel:       porcelainAPI.channelID,
			ChannelMsgCid: porcelainAPI.messageCid,
			Vouchers:      vouchers,
		},
	}
}
