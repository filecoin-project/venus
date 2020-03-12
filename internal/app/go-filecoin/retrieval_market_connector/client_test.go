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

	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	. "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

func TestNewRetrievalClientConnector(t *testing.T) {
	ctx := context.Background()

	bs, cs, _, _, _ := testSetup(ctx, t, abi.NewTokenAmount(1))

	rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
	pchMgr := makePaychMgr(ctx, t,
		specst.NewIDAddr(t, 99),
		specst.NewIDAddr(t, 100),
		specst.NewActorAddr(t, "foobar"))

	rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
	assert.NotNil(t, rcnc)
}

func TestRetrievalClientConnector_GetOrCreatePaymentChannel(t *testing.T) {
	ctx := context.Background()

	paychUniqueAddr := specst.NewActorAddr(t, "paych")

	t.Run("if the payment channel does not exist", func(t *testing.T) {
		t.Run("creates a new payment channel registry entry and posts createChannel message", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t, abi.NewTokenAmount(1000))
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychUniqueAddr)
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))

			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
			require.NoError(t, err)
			assert.Equal(t, paychUniqueAddr, res)
			assertChannel(t, paychUniqueAddr, pchMgr, true)
			rmc.Verify()
		})
		t.Run("Errors if there aren't enough funds in wallet", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, _ := testSetup(ctx, t, abi.NewTokenAmount(1000))
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychUniqueAddr)
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

			res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, big.NewInt(2000))
			assert.EqualError(t, err, "not enough funds in wallet")
			assert.Equal(t, address.Undef, res)
			assertChannel(t, paychUniqueAddr, pchMgr, false)
		})

		t.Run("Errors if client or minerWallet addr is invalid", func(t *testing.T) {
			bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t, abi.NewTokenAmount(1000))
			pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychUniqueAddr)
			rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
			rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
			_, err := rcnc.GetOrCreatePaymentChannel(ctx, address.Undef, minerAddr, channelAmount)
			assert.EqualError(t, err, "empty address")

			_, err = rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, address.Undef, channelAmount)
			assert.EqualError(t, err, "empty address")
			assertChannel(t, paychUniqueAddr, pchMgr, false)
		})
	})

	t.Run("if payment channel exists, returns payment channel addr", func(t *testing.T) {
		bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t, abi.NewTokenAmount(1000))
		pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychUniqueAddr)
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		expectedChID, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)
		assert.Equal(t, paychUniqueAddr, expectedChID)

		actualChID, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)
		assert.Equal(t, expectedChID, actualChID)
	})
}

func TestRetrievalClientConnector_AllocateLane(t *testing.T) {
	ctx := context.Background()
	bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t, abi.NewTokenAmount(100))

	paychAddr := specst.NewIDAddr(t, 101)
	pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr)

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

		addr, err := address.NewIDAddress(12345)
		require.NoError(t, err)
		res, err := rcnc.AllocateLane(addr)
		assert.EqualError(t, err, "No state for /t012345")
		assert.Zero(t, res)
	})
	t.Run("Increments and returns lastLane val", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		_, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
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
	bs, cs, clientAddr, minerAddr, channelAmount := testSetup(ctx, t, abi.NewTokenAmount(100))
	paychAddr := specst.NewIDAddr(t, 101)
	pchMgr := makePaychMgr(ctx, t, clientAddr, minerAddr, paychAddr)
	expectedAmt := big.NewInt(100)

	t.Run("Returns a voucher with a signature", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)

		lane, err := rcnc.AllocateLane(chid)
		require.NoError(t, err)

		voucher, err := rcnc.CreatePaymentVoucher(ctx, chid, expectedAmt, lane)
		require.NoError(t, err)
		assert.Equal(t, expectedAmt, voucher.Amount)
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
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		chid, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, minerAddr, channelAmount)
		require.NoError(t, err)

		expectedNonce := uint64(9) // 4 lanes * 3 vouchers each + 1
		for i := 0; i < 3; i++ {
			lane, err := rcnc.AllocateLane(chid)
			require.NoError(t, err)
			for j := 0; j < 2; j++ {
				amt := int64(i + j + 1)
				newAmt := big.NewInt(amt)
				_, err := rcnc.CreatePaymentVoucher(ctx, chid, newAmt, lane)
				require.NoError(t, err)
			}
		}
		chinfo, err := pchMgr.GetPaymentChannelInfo(paychAddr)
		require.NoError(t, err)
		assert.Equal(t, expectedNonce, chinfo.NextNonce)
	})

	t.Run("Errors if payment channel does not exist", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.StubSignature(nil)

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		paychAddr := specst.NewIDAddr(t, 990)
		voucher, err := rcnc.CreatePaymentVoucher(ctx, paychAddr, big.NewInt(100), 1)
		assert.EqualError(t, err, "No state for /t0990: datastore: key not found")
		assert.Nil(t, voucher)
	})

	t.Run("Errors if can't sign bytes", func(t *testing.T) {
		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rmc.SigErr = errors.New("signature failure")
		rmc.StubSignature(errors.New("signature failure"))

		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
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
		_, _, _, localCs, _ := requireNewEmptyChainStore(ctx, t)
		messageStore := chain.NewMessageStore(bs)
		cs := cst.NewChainStateReadWriter(localCs, messageStore, bs, builtin.DefaultActors)

		rmc := NewRetrievalMarketClientFakeAPI(t, big.NewInt(1000))
		rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)
		res, err := rcnc.CreatePaymentVoucher(ctx, paychAddr, abi.NewTokenAmount(1), 0)
		assert.EqualError(t, err, "Key not found in tipindex")
		var expRes *paych.SignedVoucher
		assert.Equal(t, expRes, res)
	})
}

func testSetup(ctx context.Context, t *testing.T, bal abi.TokenAmount) (bstore.Blockstore, ChainReaderAPI, address.Address, address.Address, abi.TokenAmount) {
	_, builder, genTs, chainStore, st1 := requireNewEmptyChainStore(ctx, t)
	rootBlk := builder.AppendBlockOnBlocks()
	th.RequireNewTipSet(t, rootBlk)
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
	clientActor := actor.NewActor(specs.AccountActorCodeID, bal)
	fakeProvider.SetHeadAndActor(t, genTs.Key(), clientAddr, clientActor)

	minerAddr := specst.NewIDAddr(t, 101)
	channelAmount := big.NewInt(10)
	return bs, fakeProvider, clientAddr, minerAddr, channelAmount
}

func requireNewEmptyChainStore(ctx context.Context, t *testing.T) (cid.Cid, *chain.Builder, block.TipSet, *chain.Store, state.Tree) {
	cst := cbor.NewMemCborStore()

	// Cribbed from chain/store_test
	st1 := state.NewState(cst)
	root, err := st1.Commit(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.NewGenesis()
	r := repo.NewInMemoryRepo()

	// setup chain store
	ds := r.Datastore()
	cs := chain.NewStore(ds, cst, chain.NewStatusReporter(), genTS.At(0).Cid())
	return root, builder, genTS, cs, st1
}

func makePaychMgr(ctx context.Context, t *testing.T, client, miner, paych address.Address) *pch.Manager {
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := pch.NewFakePaymentChannelAPI(ctx, t)
	root := shared_testutil.GenerateCids(1)[0]
	viewer := makeStateViewer(t, root, nil)
	pchMgr := pch.NewManager(context.Background(), ds, testAPI, testAPI, viewer, &cst.ChainStateReadWriter{})
	blockHeight := uint64(1234)

	testAPI.StubCreatePaychActorMessage(t, client, miner, paych, exitcode.Ok, blockHeight)

	return pchMgr
}

func makeStateViewer(t *testing.T, stateRoot cid.Cid, viewErr error) *pch.FakeStateViewer {
	return &pch.FakeStateViewer{
		Views: map[cid.Cid]*pch.FakeStateView{stateRoot: pch.NewFakeStateView(t, viewErr)}}
}

func assertChannel(t *testing.T, paych address.Address, pchMgr *pch.Manager, exists bool) {
	has, err := pchMgr.ChannelExists(paych)
	assert.NoError(t, err)
	assert.Equal(t, has, exists)
}
