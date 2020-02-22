package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	specst "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalClientNodeConnector(t *testing.T) {
	ctx := context.Background()

	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	cs := requireNewChainStoreWithBlock(ctx, t)
	rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))

	rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
	assert.NotNil(t, rcnc)
}

func TestRetrievalClientNodeConnector_GetOrCreatePaymentChannel(t *testing.T) {
	ctx := context.Background()

	bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t)
	paychAddr := specst.NewIDAddr(t, 101)

	t.Run("Errors if clientWallet get balance fails", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.BalanceErr = errors.New("boom")

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
		res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		assert.EqualError(t, err, "boom")
		assert.Equal(t, address.Undef, res)
		rmc.BalanceErr = nil
	})

	t.Run("if the payment channel does not exist", func(t *testing.T) {
		bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
		cs := requireNewChainStoreWithBlock(ctx, t)

		t.Run("creates a new payment channel registry entry and posts createChannel message", func(t *testing.T) {
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
				Owner: clientAddr,
				State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
			}

			rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)

			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
			require.NoError(t, err)
			assert.Equal(t, address.Undef, res)

			res, err = rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, types.NewAttoFILFromFIL(0))
			require.NoError(t, err)
			assert.NotEmpty(t, res.String())
			rmc.Verify()
		})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
				Owner: clientAddr,
				State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
			}
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)

			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, big.NewInt(2000))
			assert.EqualError(t, err, "not enough funds in wallet")
			assert.Equal(t, address.Undef, res)
		})

		t.Run("Errors if client or minerWallet addr is invalid", func(t *testing.T) {
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
				Owner: clientAddr,
				State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
			}
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
			_, err := rcnc.GetOrCreatePaymentChannel(ctx, address.Undef, minerAddr, channelAmount)
			assert.EqualError(t, err, "empty address")

			_, err = rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, address.Undef, channelAmount)
			assert.EqualError(t, err, "empty address")

		})
	})

	t.Run("if payment channel exists", func(t *testing.T) {
		t.Run("Retrieves existing payment channel address", func(t *testing.T) {
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
				Owner: clientAddr,
				State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
			}
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
			expectedChID, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
			require.NoError(t, err)
			assert.Equal(t, address.Undef, expectedChID)

			actualChID, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
			require.NoError(t, err)
			assert.NotEqual(t, "", actualChID.String())
			rmc.Verify()
		})
	})
}

func TestRetrievalClientNodeConnector_AllocateLane(t *testing.T) {
	ctx := context.Background()
	bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t)

	paychAddr := specst.NewIDAddr(t, 101)

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)

		addr, err := address.NewIDAddress(12345)
		require.NoError(t, err)
		res, err := rcnc.AllocateLane(addr)
		assert.EqualError(t, err, "payment channel does not exist: t012345")
		assert.Zero(t, res)
	})
	t.Run("Increments and returns lastLane val", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
			Owner: clientAddr,
			State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
		}

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
		_, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)
		require.Len(t, rmc.ActualPmtChans, 1)

		paychAddr, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, types.ZeroAttoFIL)
		require.NoError(t, err)
		require.NotEqual(t, paychAddr, address.Undef)

		lane, err := rcnc.AllocateLane(paychAddr)
		require.NoError(t, err)
		chinfo := rmc.ActualPmtChans[paychAddr]
		require.Len(t, chinfo.State.LaneStates, int(lane)+1)
		actualLs := chinfo.State.LaneStates[lane]
		expLs := paych.LaneState{
			ID:       lane,
			Redeemed: big.NewInt(0),
			Nonce:    lane + 1,
		}
		assert.Equal(t, &expLs, actualLs)
	})
}

func TestRetrievalClientNodeConnector_CreatePaymentVoucher(t *testing.T) {
	ctx := context.Background()
	bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t)
	paychAddr := specst.NewIDAddr(t, 101)

	t.Run("Returns a voucher with a signature", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
			Owner: clientAddr,
			State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
		}
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
		_, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)

		chid, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)
		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		expectedAmt := big.NewInt(100)

		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, expectedAmt, lane)
		require.NoError(t, err)
		assert.Equal(t, expectedAmt, voucher.Amount)
		assert.Equal(t, lane, voucher.Lane)
		assert.Equal(t, uint64(2), voucher.Nonce)
		assert.NotNil(t, voucher.Signature)
	})

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
		chid, err := address.NewIDAddress(rand.Uint64())
		require.NoError(t, err)
		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, big.NewInt(100), 1)
		assert.EqualError(t, err, "no such ChannelID")
		assert.Nil(t, voucher)
	})

	t.Run("Errors if can't sign bytes", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.SigErr = errors.New("signature failure")
		rmc.ExpectedPmtChans[paychAddr] = &paymentchannel.ChannelInfo{
			Owner: clientAddr,
			State: &paych.State{From: clientAddr, To: minerAddr, ToSend: abi.NewTokenAmount(0)},
		}
		rmc.StubSignature(errors.New("signature failure"))

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, rmc, rmc)
		_, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)
		chid, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, big.NewInt(0))
		require.NoError(t, err)
		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, big.NewInt(1), lane)
		assert.EqualError(t, err, "signature failure")
		assert.Nil(t, voucher)
	})
	t.Run("Errors if can't get block height/head tipset", func(t *testing.T) {
		_, _, _, localCs := requireNewEmptyChainStore(ctx, t)

		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rcnc := NewRetrievalClientConnector(bs, localCs, rmc, rmc, rmc)
		res, err := rcnc.CreatePaymentVoucher(ctx, paychAddr, abi.NewTokenAmount(1), 0)
		assert.EqualError(t, err, "Key not found in tipindex")
		var expRes *paych.SignedVoucher
		assert.Equal(t, expRes, res)
	})
}

func testSetup(ctx context.Context, t *testing.T) (bstore.Blockstore, *chain.Store, address.Address, address.Address, abi.TokenAmount) {
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	cs := requireNewChainStoreWithBlock(ctx, t)

	clientAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)
	minerAddr, err := address.NewIDAddress(rand.Uint64())
	require.NoError(t, err)

	channelAmount := big.NewInt(500)
	return bs, cs, clientAddr, minerAddr, channelAmount
}

func requireNewChainStoreWithBlock(ctx context.Context, t *testing.T) *chain.Store {
	root, builder, genTS, cs := requireNewEmptyChainStore(ctx, t)

	rootBlk := builder.AppendBlockOnBlocks()
	th.RequireNewTipSet(t, rootBlk)
	require.NoError(t, cs.SetHead(ctx, genTS))

	// add tipset and state to chainstore
	require.NoError(t, cs.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: root,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}))
	return cs
}

func requireNewEmptyChainStore(ctx context.Context, t *testing.T) (cid.Cid, *chain.Builder, block.TipSet, *chain.Store) {
	cst := cbor.NewMemCborStore()

	// Cribbed from chain/store_test
	st1 := state.NewTree(cst)
	root, err := st1.Flush(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()

	// setup chain store
	ds := r.Datastore()
	cs := chain.NewStore(ds, cst, state.NewTreeLoader(), chain.NewStatusReporter(), genTS.At(0).Cid())
	return root, builder, genTS, cs
}
