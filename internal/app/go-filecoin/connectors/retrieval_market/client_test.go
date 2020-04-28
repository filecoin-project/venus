package retrievalmarketconnector_test

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	specs "github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	specst "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	paychtest "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel/testing"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalClientConnector(t *testing.T) {
	ctx := context.Background()

	bs, cs, _, _, _ := testSetup(ctx, t, abi.NewTokenAmount(1))

	rmc := NewRetrievalMarketClientFakeAPI(t)
	pchMgr := makePaychMgr(ctx, t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"),
		abi.NewTokenAmount(999))

	rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
	assert.NotNil(t, rcnc)
}

func TestRetrievalClientConnector_GetOrCreatePaymentChannel(t *testing.T) {
	ctx := context.Background()

	paychAddr := specst.NewActorAddr(t, "paych")
	balance := abi.NewTokenAmount(1000)
	channelAmt := abi.NewTokenAmount(101)

	t.Run("if the payment channel does not exist", func(t *testing.T) {
		t.Run("creates a new payment channel registry entry and posts createChannel message", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, balance)
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
			rmc := NewRetrievalMarketClientFakeAPI(t)

			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

			tok, err := encoding.Encode(genTs.Key())
			require.NoError(t, err)

			res, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
			require.NoError(t, err)
			assert.Equal(t, paychAddr, res)
			assertChannel(t, paychAddr, pchMgr, true)
			rmc.Verify()
		})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, balance)
			rmc := NewRetrievalMarketClientFakeAPI(t)
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

			tok, err := encoding.Encode(genTs.Key())
			require.NoError(t, err)

			res, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, big.NewInt(2000), tok)
			assert.EqualError(t, err, "not enough funds in wallet")
			assert.Equal(t, address.Undef, res)
			assertChannel(t, paychAddr, pchMgr, false)
		})

		t.Run("Errors if client or minerWallet addr is invalid", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, balance)
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
			rmc := NewRetrievalMarketClientFakeAPI(t)
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

			tok, err := encoding.Encode(genTs.Key())
			require.NoError(t, err)

			_, _, err = rcnc.GetOrCreatePaymentChannel(ctx, address.Undef, minerAddr, channelAmt, tok)
			assert.EqualError(t, err, "empty address")

			_, _, err = rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, address.Undef, channelAmt, tok)
			assert.EqualError(t, err, "empty address")
			assertChannel(t, paychAddr, pchMgr, false)
		})
	})

	t.Run("if payment channel exists, returns payment channel addr", func(t *testing.T) {
		bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, balance)
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

		tok, err := encoding.Encode(genTs.Key())
		require.NoError(t, err)

		expectedChID, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)
		assert.Equal(t, paychAddr, expectedChID)

		actualChID, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)
		assert.Equal(t, expectedChID, actualChID)
	})
}

func TestRetrievalClientConnector_AllocateLane(t *testing.T) {
	ctx := context.Background()
	bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, abi.NewTokenAmount(100))

	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	paychAddr := specst.NewIDAddr(t, 101)
	channelAmt := abi.NewTokenAmount(10)
	pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

		addr, err := address.NewIDAddress(12345)
		require.NoError(t, err)
		res, err := rcnc.AllocateLane(addr)
		assert.EqualError(t, err, "No state for /t012345")
		assert.Zero(t, res)
	})
	t.Run("Increments and returns lastLane val", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		_, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)

		lane, err := rcnc.AllocateLane(paychAddr)
		require.NoError(t, err)

		chinfo, err := pchMgr.GetPaymentChannelInfo(paychAddr)
		require.NoError(t, err)
		require.Equal(t, chinfo.NextLane-1, lane)
	})
}

func TestRetrievalClientConnector_CreatePaymentVoucher(t *testing.T) {
	ctx := context.Background()
	balance := abi.NewTokenAmount(1000)
	bs, cs, clientAddr, minerAddr, genTs := testSetup(ctx, t, balance)
	paychAddr := specst.NewIDAddr(t, 101)
	expVoucherAmt := big.NewInt(10)
	channelAmt := abi.NewTokenAmount(101)

	pchActor := actor.NewActor(shared_testutil.GenerateCids(1)[0], channelAmt, cid.Undef)
	cs.SetActor(paychAddr, pchActor)

	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	t.Run("Returns a voucher with a signature", func(t *testing.T) {
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)

		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, expVoucherAmt, lane, tok)
		require.NoError(t, err)
		assert.Equal(t, expVoucherAmt, voucher.Amount)
		assert.Equal(t, lane, voucher.Lane)
		assert.Equal(t, uint64(1), voucher.Nonce)
		assert.NotNil(t, voucher.Signature)
		chinfo, err := pchMgr.GetPaymentChannelInfo(paychAddr)
		require.NoError(t, err)
		// nil SecretPreimage gets stored as zero value.
		voucher.SecretPreimage = []byte{}
		assert.True(t, chinfo.HasVoucher(voucher))
	})

	t.Run("Each lane or voucher increases NextNonce", func(t *testing.T) {
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)

		expectedNonce := uint64(9) // 4 lanes * 3 vouchers each + 1
		for i := 0; i < 3; i++ {
			lane, err := rcnc.AllocateLane(chid)
			require.NoError(t, err)
			for j := 0; j < 2; j++ {
				amt := int64(i + j + 1)
				newAmt := big.NewInt(amt)
				_, err := rcnc.CreatePaymentVoucher(ctx, chid, newAmt, lane, tok)
				require.NoError(t, err)
			}
		}
		chinfo, err := pchMgr.GetPaymentChannelInfo(paychAddr)
		require.NoError(t, err)
		assert.Equal(t, expectedNonce, chinfo.NextNonce)
	})

	t.Run("Errors if can't get block height/head tipset", func(t *testing.T) {
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		_, _, _, localCs, _ := requireNewEmptyChainStore(ctx, t)
		messageStore := chain.NewMessageStore(bs)
		cs := cst.NewChainStateReadWriter(localCs, messageStore, bs, builtin.DefaultActors)

		rmc := NewRetrievalMarketClientFakeAPI(t)
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		res, err := rcnc.CreatePaymentVoucher(ctx, paychAddr, abi.NewTokenAmount(1), 0, tok)
		assert.EqualError(t, err, "Key not found in tipindex")
		var expRes *paych.SignedVoucher
		assert.Equal(t, expRes, res)
	})

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		badAddr := specst.NewIDAddr(t, 990)
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, badAddr, channelAmt)

		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		voucher, err := rcnc.CreatePaymentVoucher(ctx, badAddr, big.NewInt(100), 1, tok)
		assert.EqualError(t, err, "No such address t0990")
		assert.Nil(t, voucher)
	})

	t.Run("errors if not enough balance in payment channel", func(t *testing.T) {
		notEnough := abi.NewTokenAmount(1)
		poorPaych := specst.NewActorAddr(t, "poorpaych")
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, poorPaych, notEnough)

		poorActor := actor.NewActor(shared_testutil.GenerateCids(1)[0], notEnough, cid.Undef)
		cs.SetActor(poorPaych, poorActor)

		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, notEnough, tok)
		require.NoError(t, err)

		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		tooMuch := abi.NewTokenAmount(100)
		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, tooMuch, lane, tok)
		assert.EqualError(t, err, "insufficient funds for voucher amount")
		assert.Nil(t, voucher)
	})

	t.Run("errors if lane is invalid", func(t *testing.T) {
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)

		// check when no lanes allocated
		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, expVoucherAmt, 0, tok)
		require.Nil(t, voucher)
		assert.EqualError(t, err, "lane does not exist 0")
		require.Nil(t, voucher)

		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		// check when there is a lane allocated
		voucher, err = rcnc.CreatePaymentVoucher(ctx, chid, expVoucherAmt, lane+1, tok)
		require.Nil(t, voucher)
		assert.EqualError(t, err, "lane does not exist 1")
	})

	t.Run("Errors if can't sign bytes", func(t *testing.T) {
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr, channelAmt)
		rmc := NewRetrievalMarketClientFakeAPI(t)
		rmc.SigErr = errors.New("signature failure")
		rmc.StubSignature(errors.New("signature failure"))

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		_, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmt, tok)
		require.NoError(t, err)
		chid, _, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, big.NewInt(0), tok)
		require.NoError(t, err)
		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, big.NewInt(1), lane, tok)
		assert.EqualError(t, err, "signature failure")
		assert.Nil(t, voucher)
	})
}

func testSetup(ctx context.Context, t *testing.T, bal abi.TokenAmount) (bstore.Blockstore, *message.FakeProvider, address.Address, address.Address, block.TipSet) {
	_, builder, genTs, chainStore, st1 := requireNewEmptyChainStore(ctx, t)
	rootBlk := builder.AppendBlockOnBlocks()
	block.RequireNewTipSet(t, rootBlk)
	require.NoError(t, chainStore.SetHead(ctx, genTs))
	root, err := st1.Commit(ctx)
	require.NoError(t, err)

	// add tipset and state to chainstore
	require.NoError(t, chainStore.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          genTs,
		TipSetStateRoot: root,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}))

	ds := repo.NewInMemoryRepo().ChainDatastore()
	bs := bstore.NewBlockstore(ds)

	fakeProvider := message.NewFakeProvider(t)
	fakeProvider.Builder = builder
	clientAddr := specst.NewIDAddr(t, 102)
	clientActor := actor.NewActor(specs.AccountActorCodeID, bal, cid.Undef)
	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(clientAddr, clientActor)

	minerAddr := specst.NewIDAddr(t, 101)

	return bs, fakeProvider, clientAddr, minerAddr, genTs
}

func requireNewEmptyChainStore(ctx context.Context, t *testing.T) (cid.Cid, *chain.Builder, block.TipSet, *chain.Store, state.Tree) {
	store := cbor.NewMemCborStore()

	// Cribbed from chain/store_test
	st1 := state.NewState(store)
	root, err := st1.Commit(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()

	// setup chain store
	ds := r.Datastore()
	cs := chain.NewStore(ds, store, chain.NewStatusReporter(), genTS.At(0).Cid())
	return root, builder, genTS, cs, st1
}

func makePaychMgr(ctx context.Context, t *testing.T, client, miner, paych address.Address, channelAmt abi.TokenAmount) *pch.Manager {
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := paychtest.NewFakePaymentChannelAPI(ctx, t)
	viewer := paychtest.NewFakeStateViewer(t)
	pchMgr := pch.NewManager(context.Background(), ds, testAPI, testAPI, viewer)
	blockHeight := uint64(1234)

	testAPI.ExpectedMsgCid, testAPI.ExpectedResult = paychtest.GenCreatePaychActorMessage(t, client, miner, paych, channelAmt, exitcode.Ok, blockHeight)

	viewer.GetFakeStateView().AddActorWithState(paych, client, miner, address.Undef)
	return pchMgr
}

func assertChannel(t *testing.T, paych address.Address, pchMgr *pch.Manager, exists bool) {
	has, err := pchMgr.ChannelExists(paych)
	assert.NoError(t, err)
	assert.Equal(t, has, exists)
}
