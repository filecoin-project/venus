package paymentchannel_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	specmock "github.com/filecoin-project/specs-actors/support/mock"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	pch "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

//
func TestConstructInitActor(t *testing.T) {
	ctx := context.Background()
	receiver := spect.NewIDAddr(t, 1000)
	builder := specmock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fms := NewFakeMessageSender(t, ctx, builder)
	assert.NotNil(t, fms)
	initAddr := builtin.InitActorAddr
	fms.ConstructActor(builtin.InitActorCodeID, initAddr, address.Undef, abi.NewTokenAmount(0))
}

// you do need to construct InitActor before you can call things on it
func TestCreatePaymentChannel(t *testing.T) {
	tf.FunctionalTest(t)
	ctx := context.Background()
	//store := cbor.NewMemCborStore()
	// Cribbed from chain/store_test
	//st1 := state.NewState(store)
	//root, err := st1.Commit(ctx)
	//require.NoError(t, err)
	//
	//// link testing state to test block
	//builder := chain.NewBuilder(t, address.Undef)
	//genTS := builder.NewGenesis()
	//ds := repo.NewInMemoryRepo().ChainDatastore()
	//bs := blockstore.NewBlockstore(ds)
	//cst := cborutil.NewIpldStore(bs)
	//// setup chain store
	//cs := chain.NewStore(ds, store, chain.NewStatusReporter(), genTS.At(0).Cid())
	//viewer := NewManagerStateViewer(cs, cst)
	//manager := NewManager(ctx, ds, nil, nil, viewer)

	clientAddr := spect.NewActorAddr(t, "clientActor")

	initAddr := builtin.InitActorAddr
	bal := abi.NewTokenAmount(1000000)
	receiver := spect.NewIDAddr(t, 1000)
	builder := specmock.NewBuilder(context.Background(), receiver).
		WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID).
		WithBalance(bal, abi.NewTokenAmount(0))

	fms := NewFakeMessageSender(t, ctx, builder)
	assert.NotNil(t, fms)
	rt := fms.Runtime()
	rt.SetReceived(bal)

	// construct InitActor
	fms.ConstructActor(builtin.InitActorCodeID, initAddr, address.Undef, abi.NewTokenAmount(0))

	paych := spect.NewActorAddr(t, "paych")
	rt.SetNewActorAddress(paych)
	expectedIdAddr1 := spect.NewIDAddr(t, 100)
	rt.ExpectCreateActor(builtin.PaymentChannelActorCodeID, expectedIdAddr1)

	var fakeParams = runtime.CBORBytes([]byte("foo"))
	rt.ExpectSend(expectedIdAddr1, builtin.MethodConstructor, fakeParams, bal, nil, exitcode.Ok)

	fms.ExecAndVerify(clientAddr, initAddr, builtin.PaymentChannelActorCodeID, fakeParams)

	var st init_.State
	rt.GetState(&st)
	actualIdAddr, err := st.ResolveAddress(adt.AsStore(rt), paych)
	require.NoError(t, err)
	require.Equal(t, expectedIdAddr1, actualIdAddr)

	balance := abi.NewTokenAmount(1000)
	channelAmt := abi.NewTokenAmount(101)
	bs, cs, client, miner, genTs := testSetup(ctx, t, balance)
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	testAPI := pch.NewFakePaymentChannelAPI(ctx, t)
	viewer := pch.NewFakeStateViewer(t)
	pchMgr := pch.NewManager(context.Background(), ds, testAPI, testAPI, viewer)
	blockHeight := uint64(1234)

	testAPI.StubCreatePaychActorMessage(t, client, miner, paych, channelAmt, exitcode.Ok, blockHeight)
	viewer.AddActorWithState(paych, client, miner, address.Undef)

	rmc := retrievalmarketconnector.NewRetrievalMarketClientFakeAPI(t)

	rcnc := NewRetrievalClientConnector(bs, cs, rmc, pchMgr)

	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	res, err := rcnc.GetOrCreatePaymentChannel(ctx, clientAddr, miner, channelAmt, tok)
	require.NoError(t, err)
}

func testSetup(ctx context.Context, t *testing.T, bal abi.TokenAmount) (bstore.Blockstore, *message.FakeProvider, address.Address, address.Address, block.TipSet) {
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
	clientAddr := spect.NewIDAddr(t, 102)
	clientActor := actor.NewActor(specs.AccountActorCodeID, bal)
	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(clientAddr, clientActor)

	minerAddr := spect.NewIDAddr(t, 101)

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
