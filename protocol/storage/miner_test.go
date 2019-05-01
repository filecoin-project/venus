package storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

var (
	defaultAmountInc = uint64(1773)
	defaultPieceSize = uint64(1000)

	minerPriceString = ".00025"
)

func TestReceiveStorageProposal(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Accepts proposals with sufficient TotalPrice", func(t *testing.T) {
		accepted := false
		rejected := false
		var message string

		porcelainAPI := newMinerTestPorcelain(t)
		miner := Miner{
			porcelainAPI:   porcelainAPI,
			minerOwnerAddr: porcelainAPI.targetAddress,
			proposalAcceptor: func(m *Miner, p *storagedeal.Proposal) (*storagedeal.Response, error) {
				accepted = true
				return &storagedeal.Response{State: storagedeal.Accepted}, nil
			},
			proposalRejector: func(m *Miner, p *storagedeal.Proposal, reason string) (*storagedeal.Response, error) {
				message = reason
				rejected = true
				return &storagedeal.Response{State: storagedeal.Rejected, Message: reason}, nil
			},
		}

		vouchers := testPaymentVouchers(porcelainAPI, VoucherInterval, defaultAmountInc)
		proposal := testSignedDealProposal(porcelainAPI, vouchers, defaultPieceSize)

		_, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.True(t, accepted, "Proposal has been accepted")
		assert.False(t, rejected, "Proposal has not been rejected")
		assert.Equal(t, "", message)
	})

	t.Run("Rejects proposals with insufficient TotalPrice", func(t *testing.T) {
		porcelainAPI, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		// configure storage price
		assert.NoError(t, porcelainAPI.config.Set("mining.storagePrice", `".0005"`))

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Equal(t, "proposed price (2500) is less than expected (5000) given asking price of 0.0005", res.Message)
	})

	t.Run("Rejects proposals with invalid payment channel", func(t *testing.T) {
		porcelainAPI, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		porcelainAPI.noChannels = true

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "could not find payment channel")
	})

	t.Run("Rejects proposals with wrong target", func(t *testing.T) {
		_, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		miner.minerOwnerAddr = address.TestAddress

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "not target of payment channel")
	})

	t.Run("Rejects proposals with too short channel eol", func(t *testing.T) {
		porcelainAPI, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)
		porcelainAPI.channelEol = types.NewBlockHeight(1200)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "less than required eol")
	})

	t.Run("Rejects proposals with no payments", func(t *testing.T) {
		porcelainAPI, miner, _ := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)
		proposal := testSignedDealProposal(porcelainAPI, []*types.PaymentVoucher{}, defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "contains no payment vouchers")
	})

	t.Run("Rejects proposals with vouchers with invalid signatures", func(t *testing.T) {
		porcelainAPI, miner, _ := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		invalidSigVouchers := testPaymentVouchers(porcelainAPI, VoucherInterval, defaultAmountInc)
		invalidSigVouchers[0].Signature = types.Signature([]byte{})
		proposal := testSignedDealProposal(porcelainAPI, invalidSigVouchers, defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "invalid signature in voucher")
	})

	t.Run("Rejects proposals with when payments start too late", func(t *testing.T) {
		porcelainAPI := newMinerTestPorcelain(t)
		porcelainAPI.paymentStart = porcelainAPI.paymentStart.Add(types.NewBlockHeight(15))

		miner, _ := newMinerTestSetup(porcelainAPI, VoucherInterval, defaultAmountInc)
		proposal := testSignedDealProposal(porcelainAPI, testPaymentVouchers(porcelainAPI, VoucherInterval, defaultAmountInc), defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "payments start after deal start interval")
	})

	t.Run("Rejects proposals with vouchers with long intervals", func(t *testing.T) {
		porcelainAPI, miner, _ := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		porcelainAPI.paymentStart = porcelainAPI.paymentStart.Sub(types.NewBlockHeight(15))
		proposal := testSignedDealProposal(porcelainAPI, testPaymentVouchers(porcelainAPI, VoucherInterval+15, defaultAmountInc), defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "interval between vouchers")
	})

	t.Run("Rejects proposals with vouchers with insufficient amounts", func(t *testing.T) {
		porcelainAPI, miner, _ := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

		proposal := testSignedDealProposal(porcelainAPI, testPaymentVouchers(porcelainAPI, VoucherInterval, 1), defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Contains(t, res.Message, "voucher amount")
	})

	t.Run("Rejects proposals with invalid signature", func(t *testing.T) {
		_, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)
		proposal.Signature = []byte{'0', '0', '0'}

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Equal(t, "invalid deal signature", res.Message)
	})

	t.Run("Rejects proposals piece larger than sector size", func(t *testing.T) {
		porcelainAPI := newMinerTestPorcelain(t)
		miner := Miner{
			porcelainAPI:   porcelainAPI,
			minerOwnerAddr: porcelainAPI.targetAddress,
			proposalAcceptor: func(m *Miner, p *storagedeal.Proposal) (*storagedeal.Response, error) {
				return &storagedeal.Response{State: storagedeal.Accepted}, nil
			},
			proposalRejector: func(m *Miner, p *storagedeal.Proposal, reason string) (*storagedeal.Response, error) {
				return &storagedeal.Response{State: storagedeal.Rejected, Message: reason}, nil
			},
		}

		vouchers := testPaymentVouchers(porcelainAPI, VoucherInterval, 2*defaultAmountInc)
		proposal := testSignedDealProposal(porcelainAPI, vouchers, 2*defaultPieceSize)

		res, err := miner.receiveStorageProposal(context.Background(), proposal)
		require.NoError(t, err)

		assert.Equal(t, storagedeal.Rejected, res.State)
		assert.Equal(t, "piece is 2000 bytes but sector size is 1016 bytes", res.Message)
	})
}

func TestDealsAwaitingSeal(t *testing.T) {
	tf.UnitTest(t)

	newCid := types.NewCidForTestGetter()
	cid0 := newCid()
	cid1 := newCid()
	cid2 := newCid()

	wantSectorID := uint64(42)
	wantSector := &sectorbuilder.SealedSectorMetadata{SectorID: wantSectorID}
	someOtherSectorID := uint64(100)

	wantMessage := "boom"

	t.Run("saveDealsAwaitingSeal saves, loadDealsAwaitingSeal loads", func(t *testing.T) {
		miner := &Miner{
			dealsAwaitingSeal: &dealsAwaitingSealStruct{
				SectorsToDeals:    make(map[uint64][]cid.Cid),
				SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
				FailedSectors:     make(map[uint64]string),
			},
			dealsAwaitingSealDs: repo.NewInMemoryRepo().DealsDatastore(),
		}

		miner.dealsAwaitingSeal.add(wantSectorID, cid0)

		require.NoError(t, miner.saveDealsAwaitingSeal())
		miner.dealsAwaitingSeal = &dealsAwaitingSealStruct{}
		require.NoError(t, miner.loadDealsAwaitingSeal())

		assert.Equal(t, cid0, miner.dealsAwaitingSeal.SectorsToDeals[42][0])
	})

	t.Run("add before success", func(t *testing.T) {
		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1)
		dealsAwaitingSeal.add(someOtherSectorID, cid2)
		dealsAwaitingSeal.success(wantSector)

		assert.Len(t, gotCids, 2, "onSuccess should've been called twice")
	})

	t.Run("add after success", func(t *testing.T) {
		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
		gotCids := []cid.Cid{}
		dealsAwaitingSeal.onSuccess = func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
			assert.Equal(t, sector, wantSector)
			gotCids = append(gotCids, dealCid)
		}

		dealsAwaitingSeal.success(wantSector)
		dealsAwaitingSeal.add(wantSectorID, cid0)
		dealsAwaitingSeal.add(wantSectorID, cid1) // Shouldn't trigger a call, see add().
		dealsAwaitingSeal.add(someOtherSectorID, cid2)

		assert.Len(t, gotCids, 1, "onSuccess should've been called once")
	})

	t.Run("add before fail", func(t *testing.T) {
		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
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
		dealsAwaitingSeal := &dealsAwaitingSealStruct{
			SectorsToDeals:    make(map[uint64][]cid.Cid),
			SuccessfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
			FailedSectors:     make(map[uint64]string),
		}
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

func TestOnCommitmentAddedToChain(t *testing.T) {
	tf.UnitTest(t)

	cidGetter := types.NewCidForTestGetter()
	proposalCid := cidGetter()
	sectorID := uint64(777)

	// Simulate successful sealing and posting a commitSector to chain
	msgCid := cidGetter()
	var commD types.CommD
	copy(commD[:], []byte{9, 9, 9, 9})
	pip := []byte{3, 3, 3, 3, 3}

	t.Run("On successful commitment", func(t *testing.T) {
		// create new miner with deal in the accepted state and mapped to a sector
		_, miner, proposal := minerWithAcceptedDealTestSetup(t, proposalCid, sectorID)

		piece := &sectorbuilder.PieceInfo{Ref: proposal.PieceRef, Size: 10999, InclusionProof: pip}
		sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorID, CommD: commD, Pieces: []*sectorbuilder.PieceInfo{piece}}

		miner.OnCommitmentSent(sector, msgCid, nil)

		// retrieve deal response
		dealResponse := miner.Query(proposal.Proposal.PieceRef)

		assert.Equal(t, storagedeal.Posted, dealResponse.State, "deal should be in posted state")
		require.NotNil(t, dealResponse.ProofInfo, "deal should have proof info")
		assert.Equal(t, sectorID, dealResponse.ProofInfo.SectorID, "sector id should match committed sector")
		assert.Equal(t, &msgCid, dealResponse.ProofInfo.CommitmentMessage, "CommitmentMessage should be cid of commitSector messsage")
		assert.Equal(t, pip, dealResponse.ProofInfo.PieceInclusionProof, "PieceInclusionProof should be proof generated after sealing")
	})

	t.Run("Committing doesn't fail when piece info is missing", func(t *testing.T) {
		// create new miner with deal in the accepted state and mapped to a sector
		_, miner, proposal := minerWithAcceptedDealTestSetup(t, proposalCid, sectorID)

		sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorID, CommD: commD, Pieces: []*sectorbuilder.PieceInfo{}}

		miner.OnCommitmentSent(sector, msgCid, nil)

		// retrieve deal response
		dealResponse := miner.Query(proposal.Proposal.PieceRef)

		assert.Equal(t, storagedeal.Posted, dealResponse.State, "deal should be in posted state")

		// expect proof to be nil because it wasn't provided
		assert.Nil(t, dealResponse.ProofInfo.PieceInclusionProof)
	})

	t.Run("Committing doesn't fail when deal isn't found", func(t *testing.T) {
		// create new miner with deal in the accepted state and mapped to a sector
		_, miner, proposal := minerWithAcceptedDealTestSetup(t, proposalCid, sectorID)

		sector := &sectorbuilder.SealedSectorMetadata{SectorID: sectorID, CommD: commD, Pieces: []*sectorbuilder.PieceInfo{}}

		miner.OnCommitmentSent(sector, msgCid, nil)

		// retrieve deal response
		dealResponse := miner.Query(proposal.Proposal.PieceRef)

		assert.Equal(t, storagedeal.Posted, dealResponse.State, "deal should be in posted state")

		// expect proof to be nil because it wasn't provided
		assert.Nil(t, dealResponse.ProofInfo.PieceInclusionProof)
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
	deals         map[cid.Cid]*storagedeal.Deal

	testing *testing.T
}

func (mtp *minerTestPorcelain) MinerGetSectorSize(ctx context.Context, minerAddr address.Address) (*types.BytesAmount, error) {
	return types.OneKiBSectorSize, nil
}

func (mtp *minerTestPorcelain) ChainSampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	bytes := make([]byte, 42)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	return bytes, nil
}

func newMinerTestPorcelain(t *testing.T) *minerTestPorcelain {
	mockSigner, ki := types.NewMockSignersAndKeyInfo(1)
	payerAddr, err := ki[0].Address()
	require.NoError(t, err, "Could not create payer address")

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()

	messageCid := cidGetter()

	config := cfg.NewConfig(repo.NewInMemoryRepo())
	require.NoError(t, config.Set("mining.storagePrice", fmt.Sprintf("%q", minerPriceString)))

	blockHeight := types.NewBlockHeight(773)
	return &minerTestPorcelain{
		config:        config,
		payerAddress:  payerAddr,
		targetAddress: addressGetter(),
		channelID:     types.NewChannelID(73),
		messageCid:    &messageCid,
		signer:        mockSigner,
		noChannels:    false,
		channelEol:    types.NewBlockHeight(13773),
		blockHeight:   blockHeight,
		paymentStart:  blockHeight,
		testing:       t,
		deals:         make(map[cid.Cid]*storagedeal.Deal),
	}
}

func (mtp *minerTestPorcelain) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	return nil, nil
}

func (mtp *minerTestPorcelain) MessageSend(ctx context.Context, from, to address.Address, val *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return cid.Cid{}, nil
}

func (mtp *minerTestPorcelain) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	if method == "getProofsMode" {
		return messageQueryGetProofsMode()
	}
	return mtp.messageQueryPaymentBrokerLs()
}

func messageQueryGetProofsMode() ([][]byte, error) {
	return [][]byte{{byte(types.TestProofsMode)}}, nil
}

func (mtp *minerTestPorcelain) messageQueryPaymentBrokerLs() ([][]byte, error) {
	channels := map[string]*paymentbroker.PaymentChannel{}

	if !mtp.noChannels {
		id := mtp.channelID.KeyString()
		channels[id] = &paymentbroker.PaymentChannel{
			Target:         mtp.targetAddress,
			Amount:         types.NewAttoFILFromFIL(100000),
			AmountRedeemed: types.NewAttoFILFromFIL(0),
			AgreedEol:      mtp.channelEol,
			Eol:            mtp.channelEol,
		}
	}

	channelsBytes, err := actor.MarshalStorage(channels)
	require.NoError(mtp.testing, err)
	return [][]byte{channelsBytes}, nil
}

func (mtp *minerTestPorcelain) ConfigGet(dottedPath string) (interface{}, error) {
	return mtp.config.Get(dottedPath)
}

func (mtp *minerTestPorcelain) ChainBlockHeight() (*types.BlockHeight, error) {
	return mtp.blockHeight, nil
}

func (mtp *minerTestPorcelain) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return nil
}

func newTestMiner(api *minerTestPorcelain) *Miner {
	return &Miner{
		porcelainAPI:   api,
		minerOwnerAddr: api.targetAddress,
		proposalAcceptor: func(m *Miner, p *storagedeal.Proposal) (*storagedeal.Response, error) {
			return &storagedeal.Response{State: storagedeal.Accepted}, nil
		},
		proposalRejector: func(m *Miner, p *storagedeal.Proposal, reason string) (*storagedeal.Response, error) {
			return &storagedeal.Response{State: storagedeal.Rejected, Message: reason}, nil
		},
	}
}

func defaultMinerTestSetup(t *testing.T, voucherInverval int, amountInc uint64) (*minerTestPorcelain, *Miner, *storagedeal.SignedDealProposal) {
	papi := newMinerTestPorcelain(t)
	miner, sdp := newMinerTestSetup(papi, voucherInverval, amountInc)
	return papi, miner, sdp
}

// simulates a miner in the state where a proposal has been sent and the miner has accepted
func minerWithAcceptedDealTestSetup(t *testing.T, proposalCid cid.Cid, sectorID uint64) (*minerTestPorcelain, *Miner, *storagedeal.SignedDealProposal) {
	// start with miner and signed proposal
	porcelainAPI, miner, proposal := defaultMinerTestSetup(t, VoucherInterval, defaultAmountInc)

	// give the miner some place to store the deal
	miner.dealsAwaitingSealDs = repo.NewInMemoryRepo().DealsDs

	// create the dealsAwaitingSealStruct to manage the deal prior to sealing
	require.NoError(t, miner.loadDealsAwaitingSeal())

	// wire dealsAwaitingSeal with the actual commit success functionality
	miner.dealsAwaitingSeal.onSuccess = miner.onCommitSuccess

	// create the response and the deal
	resp := &storagedeal.Response{
		State:       storagedeal.Accepted,
		ProposalCid: proposalCid,
		Signature:   proposal.Signature,
	}

	storageDeal := &storagedeal.Deal{
		Miner:    miner.minerAddr,
		Proposal: &proposal.Proposal,
		Response: resp,
	}

	// Simulates miner.acceptProposal without going to the network to fetch the data by storing the deal.
	// Mapping the proposalCid to a sectorId simulates staging the sector.
	require.NoError(t, porcelainAPI.DealPut(storageDeal))
	miner.dealsAwaitingSeal.add(sectorID, proposalCid)

	return porcelainAPI, miner, proposal
}

func newMinerTestSetup(porcelainAPI *minerTestPorcelain, voucherInterval int, amountInc uint64) (*Miner, *storagedeal.SignedDealProposal) {
	vouchers := testPaymentVouchers(porcelainAPI, voucherInterval, amountInc)
	return newTestMiner(porcelainAPI), testSignedDealProposal(porcelainAPI, vouchers, 1000)
}

func testPaymentVouchers(porcelainAPI *minerTestPorcelain, voucherInterval int, amountInc uint64) []*types.PaymentVoucher {
	vouchers := make([]*types.PaymentVoucher, 10)

	for i := 0; i < 10; i++ {
		validAt := porcelainAPI.paymentStart.Add(types.NewBlockHeight(uint64((i + 1) * voucherInterval)))
		amount := types.NewAttoFILFromFIL(uint64(i+1) * amountInc)
		signature, err := paymentbroker.SignVoucher(porcelainAPI.channelID, amount, validAt, porcelainAPI.payerAddress, nil, porcelainAPI.signer)
		require.NoError(porcelainAPI.testing, err, "could not sign valid proposal")

		vouchers[i] = &types.PaymentVoucher{
			Channel:   *porcelainAPI.channelID,
			Payer:     porcelainAPI.payerAddress,
			Target:    porcelainAPI.targetAddress,
			Amount:    *amount,
			ValidAt:   *validAt,
			Signature: signature,
		}
	}
	return vouchers

}

func testSignedDealProposal(porcelainAPI *minerTestPorcelain, vouchers []*types.PaymentVoucher, size uint64) *storagedeal.SignedDealProposal {
	duration := uint64(10000)
	minerPrice, _ := types.NewAttoFILFromFILString(minerPriceString)
	totalPrice := minerPrice.MulBigInt(big.NewInt(int64(size * duration)))

	proposal := &storagedeal.Proposal{
		MinerAddress: porcelainAPI.targetAddress,
		PieceRef:     types.NewCidForTestGetter()(),
		TotalPrice:   totalPrice,
		Size:         types.NewBytesAmount(size),
		Duration:     duration,
		Payment: storagedeal.PaymentInfo{
			Payer:         porcelainAPI.payerAddress,
			PayChActor:    address.PaymentBrokerAddress,
			Channel:       porcelainAPI.channelID,
			ChannelMsgCid: porcelainAPI.messageCid,
			Vouchers:      vouchers,
		},
	}

	signedProposal, err := proposal.NewSignedProposal(porcelainAPI.payerAddress, porcelainAPI.signer)
	require.NoError(porcelainAPI.testing, err)
	return signedProposal
}

func (mtp *minerTestPorcelain) DealsLs() ([]*storagedeal.Deal, error) {
	var results []*storagedeal.Deal

	for _, storageDeal := range mtp.deals {
		results = append(results, storageDeal)
	}
	return results, nil
}

func (mtp *minerTestPorcelain) DealGet(dealCid cid.Cid) *storagedeal.Deal {
	storageDeal, ok := mtp.deals[dealCid]
	if !ok {
		return nil
	}
	return storageDeal
}

func (mtp *minerTestPorcelain) DealPut(storageDeal *storagedeal.Deal) error {
	mtp.deals[storageDeal.Response.ProposalCid] = storageDeal
	return nil
}
