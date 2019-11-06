package porcelain_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type testDealLsPlumbing struct {
	deals        []*storagedeal.Deal
	minerAddress address.Address
}

func (tdlp *testDealLsPlumbing) DealsLs(_ context.Context) (<-chan *porcelain.StorageDealLsResult, error) {
	dealCh := make(chan *porcelain.StorageDealLsResult)
	go func() {
		for _, deal := range tdlp.deals {
			dealCh <- &porcelain.StorageDealLsResult{
				Deal: *deal,
			}
		}
		close(dealCh)
	}()
	return dealCh, nil
}

func (tdlp *testDealLsPlumbing) ConfigGet(path string) (interface{}, error) {
	return tdlp.minerAddress, nil
}

func TestDealGet(t *testing.T) {
	tf.UnitTest(t)

	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	expectedDeal := &storagedeal.Deal{
		Response: &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				ProposalCid: dealCid,
			},
		},
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{expectedDeal},
	}

	resultDeal, err := porcelain.DealGet(context.Background(), plumbing, dealCid)
	assert.NoError(t, err)
	assert.NotNil(t, resultDeal)
	assert.Equal(t, expectedDeal, resultDeal)
}

func TestDealGetNotFound(t *testing.T) {
	tf.UnitTest(t)

	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	badCid := cidGetter()
	expectedDeal := &storagedeal.Deal{
		Response: &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				ProposalCid: dealCid,
			},
		},
	}

	plumbing := &testDealLsPlumbing{
		deals: []*storagedeal.Deal{expectedDeal},
	}

	resultDeal, err := porcelain.DealGet(context.Background(), plumbing, badCid)
	assert.Error(t, err, porcelain.ErrDealNotFound)
	assert.Nil(t, resultDeal)
}

type testRedeemPlumbing struct {
	t *testing.T

	blockHeight uint64
	dealCid     cid.Cid
	gasPrice    types.GasUnits
	messageCid  cid.Cid
	vouchers    []*types.PaymentVoucher

	ResultingFromAddr       address.Address
	ResultingActorAddr      address.Address
	ResultingMethod         types.MethodID
	ResultingVoucherPayer   address.Address
	ResultingVoucherChannel *types.ChannelID
	ResultingVoucherAmount  types.AttoFIL
	ResultingVoucherValidAt *types.BlockHeight
}

func (trp *testRedeemPlumbing) ChainHeadKey() block.TipSetKey {
	return block.NewTipSetKey()
}

func (trp *testRedeemPlumbing) ChainTipSet(_ block.TipSetKey) (block.TipSet, error) {
	return block.NewTipSet(&block.Block{Height: types.Uint64(trp.blockHeight)})
}

func (trp *testRedeemPlumbing) DealGet(_ context.Context, c cid.Cid) (*storagedeal.Deal, error) {
	require.Equal(trp.t, trp.dealCid, c)

	deal := &storagedeal.Deal{
		Proposal: &storagedeal.SignedProposal{
			Proposal: storagedeal.Proposal{
				Payment: storagedeal.PaymentInfo{
					Vouchers: trp.vouchers,
				},
			},
		},
	}

	return deal, nil
}

func (trp *testRedeemPlumbing) MessagePreview(_ context.Context, fromAddr address.Address, actorAddr address.Address, method types.MethodID, params ...interface{}) (types.GasUnits, error) {
	trp.ResultingFromAddr = fromAddr
	trp.ResultingActorAddr = actorAddr
	trp.ResultingMethod = method
	trp.ResultingVoucherPayer = params[0].(address.Address)
	trp.ResultingVoucherChannel = params[1].(*types.ChannelID)
	trp.ResultingVoucherAmount = params[2].(types.AttoFIL)
	trp.ResultingVoucherValidAt = params[3].(*types.BlockHeight)
	return trp.gasPrice, nil
}

func (trp *testRedeemPlumbing) MessageSend(_ context.Context, fromAddr address.Address, actorAddr address.Address, _ types.AttoFIL, _ types.AttoFIL, _ types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error) {
	trp.ResultingFromAddr = fromAddr
	trp.ResultingActorAddr = actorAddr
	trp.ResultingMethod = method
	trp.ResultingVoucherPayer = params[0].(address.Address)
	trp.ResultingVoucherChannel = params[1].(*types.ChannelID)
	trp.ResultingVoucherAmount = params[2].(types.AttoFIL)
	trp.ResultingVoucherValidAt = params[3].(*types.BlockHeight)
	return trp.messageCid, nil, nil
}

func TestDealRedeem(t *testing.T) {
	tf.UnitTest(t)

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	messageCid := cidGetter()
	fromAddr := addressGetter()
	payerAddr := addressGetter()
	channelID := types.NewChannelID(0)
	tooSmallVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(1),
		ValidAt: *types.NewBlockHeight(10),
	}
	expectedVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(2),
		ValidAt: *types.NewBlockHeight(20),
	}
	notYetValidVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(3),
		ValidAt: *types.NewBlockHeight(30),
	}
	vouchers := []*types.PaymentVoucher{
		tooSmallVoucher,
		expectedVoucher,
		notYetValidVoucher,
	}
	plumbing := &testRedeemPlumbing{
		t:           t,
		blockHeight: 25,
		dealCid:     dealCid,
		messageCid:  messageCid,
		vouchers:    vouchers,
	}

	resultCid, err := porcelain.DealRedeem(context.Background(), plumbing, fromAddr, dealCid, types.NewAttoFILFromFIL(0), types.NewGasUnits(0))
	require.NoError(t, err)

	assert.Equal(t, fromAddr, plumbing.ResultingFromAddr)
	assert.Equal(t, address.PaymentBrokerAddress, plumbing.ResultingActorAddr)
	assert.Equal(t, paymentbroker.Redeem, plumbing.ResultingMethod)
	assert.Equal(t, payerAddr, plumbing.ResultingVoucherPayer)
	assert.Equal(t, channelID, plumbing.ResultingVoucherChannel)
	assert.Equal(t, types.NewAttoFILFromFIL(2), plumbing.ResultingVoucherAmount)
	assert.Equal(t, types.NewBlockHeight(20), plumbing.ResultingVoucherValidAt)

	assert.Equal(t, messageCid, resultCid)
}

func TestDealRedeemPreview(t *testing.T) {
	tf.UnitTest(t)

	addressGetter := address.NewForTestGetter()
	cidGetter := types.NewCidForTestGetter()
	dealCid := cidGetter()
	fromAddr := addressGetter()
	payerAddr := addressGetter()
	channelID := types.NewChannelID(0)
	tooSmallVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(1),
		ValidAt: *types.NewBlockHeight(10),
	}
	expectedVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(2),
		ValidAt: *types.NewBlockHeight(20),
	}
	notYetValidVoucher := &types.PaymentVoucher{
		Payer:   payerAddr,
		Channel: *channelID,
		Amount:  types.NewAttoFILFromFIL(3),
		ValidAt: *types.NewBlockHeight(30),
	}
	vouchers := []*types.PaymentVoucher{
		tooSmallVoucher,
		expectedVoucher,
		notYetValidVoucher,
	}
	plumbing := &testRedeemPlumbing{
		t:           t,
		blockHeight: 25,
		dealCid:     dealCid,
		gasPrice:    types.NewGasUnits(42),
		vouchers:    vouchers,
	}

	resultGasPrice, err := porcelain.DealRedeemPreview(context.Background(), plumbing, fromAddr, dealCid)
	require.NoError(t, err)

	assert.Equal(t, fromAddr, plumbing.ResultingFromAddr)
	assert.Equal(t, address.PaymentBrokerAddress, plumbing.ResultingActorAddr)
	assert.Equal(t, paymentbroker.Redeem, plumbing.ResultingMethod)
	assert.Equal(t, payerAddr, plumbing.ResultingVoucherPayer)
	assert.Equal(t, channelID, plumbing.ResultingVoucherChannel)
	assert.Equal(t, types.NewAttoFILFromFIL(2), plumbing.ResultingVoucherAmount)
	assert.Equal(t, types.NewBlockHeight(20), plumbing.ResultingVoucherValidAt)

	assert.Equal(t, types.NewGasUnits(42), resultGasPrice)
}
