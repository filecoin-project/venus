package paymentchannel_test

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	paychtest "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel/testing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// TestAddFundsToChannel verifies that a call to GetOrCreatePaymentChannel sends
// funds to the actor if it  already exists
func TestPaymentChannel(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	chainBuilder, bs, genTs := testSetup2(ctx, t)
	ds := dss.MutexWrap(datastore.NewMapDatastore())

	balance := abi.NewTokenAmount(1000000)
	initActorUtil := paychtest.NewFakeInitActorUtil(ctx, t, balance)
	root, err := chainBuilder.GetTipSetStateRoot(genTs.Key())
	require.NoError(t, err)

	initialChannelAmt := abi.NewTokenAmount(1200)
	_, client, miner, paychID, paych := initActorUtil.StubCtorSendResponse(initialChannelAmt)
	fakeProvider := message.NewFakeProvider(t)
	fakeProvider.Builder = chainBuilder
	clientActor := actor.NewActor(builtin.AccountActorCodeID, balance, root)
	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(client, clientActor)

	viewer := paychtest.NewFakeStateViewer(t)

	pchMgr := pch.NewManager(context.Background(), ds, initActorUtil, initActorUtil, viewer)

	viewer.GetFakeStateView().AddActorWithState(paych, client, miner, address.Undef)

	rmc := retrievalmarketconnector.NewRetrievalMarketClientFakeAPI(t)

	connector := retrievalmarketconnector.NewRetrievalClientConnector(bs, fakeProvider, rmc, pchMgr)
	assert.NotNil(t, connector)
	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	addr, mcid, err := connector.GetOrCreatePaymentChannel(ctx, client, miner, initialChannelAmt, tok)
	require.NoError(t, err)
	assert.Equal(t, address.Undef, addr)

	// give a chance for the goroutine to finish creating channel info
	time.Sleep(100 * time.Millisecond)
	addr, err = connector.WaitForPaymentChannelCreation(mcid)
	require.NoError(t, err)
	assert.Equal(t, paych, addr)

	chinfo, err := pchMgr.GetPaymentChannelInfo(paych)
	require.NoError(t, err)
	require.Equal(t, paych, chinfo.UniqueAddr)

	paychActorUtil := paychtest.FakePaychActorUtil{
		PaychAddr:      paych,
		PaychIDAddr:    paychID,
		Client:         client,
		ClientID:       spect.NewIDAddr(t, 999),
		Miner:          miner,
		PcActorHarness: new(paychtest.PcActorHarness),
	}
	paychActorUtil.ConstructPaychActor(t, initialChannelAmt)

	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(client, clientActor)

	viewer.GetFakeStateView().AddActorWithState(paychActorUtil.PaychAddr, paychActorUtil.Client, paychActorUtil.Miner, address.Undef)
	assert.NotNil(t, connector)

	addVal := abi.NewTokenAmount(333)
	expCid := paychActorUtil.StubSendFundsResponse(paychActorUtil.Client, addVal, exitcode.Ok, 1)

	// set up sends and waits to go to the payment channel actor util / harness
	initActorUtil.DelegateSender(paychActorUtil.Send)
	initActorUtil.DelegateWaiter(paychActorUtil.Wait)

	addr, mcid, err = connector.GetOrCreatePaymentChannel(ctx, client, miner, addVal, tok)
	require.NoError(t, err)
	assert.Equal(t, paychActorUtil.PaychAddr, addr)
	assert.True(t, mcid.Equals(expCid))

	paychActorUtil.Runtime.Verify()

	err = connector.WaitForPaymentChannelAddFunds(mcid)
	require.NoError(t, err)
	assert.Equal(t, abi.NewTokenAmount(333), paychActorUtil.Runtime.ValueReceived())
}

func testSetup2(ctx context.Context, t *testing.T) (*chain.Builder, bstore.Blockstore, block.TipSet) {
	_, builder, genTs, cs, st1 := requireNewEmptyChainStore(ctx, t)
	rootBlk := builder.AppendBlockOnBlocks()
	block.RequireNewTipSet(t, rootBlk)
	require.NoError(t, cs.SetHead(ctx, genTs))
	root, err := st1.Commit(ctx)
	require.NoError(t, err)

	// add tipset and state to chainstore
	require.NoError(t, cs.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          genTs,
		TipSetStateRoot: root,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}))

	ds := repo.NewInMemoryRepo().ChainDatastore()
	bs := bstore.NewBlockstore(ds)
	return builder, bs, genTs
}

func TestPaychActorIFace(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	fai := paychtest.NewFakePaychActorUtil(ctx, t, abi.NewTokenAmount(1200))
	require.NotNil(t, fai)
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
