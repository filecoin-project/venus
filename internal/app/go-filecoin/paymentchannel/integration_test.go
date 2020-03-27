package paymentchannel_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	specmock "github.com/filecoin-project/specs-actors/support/mock"
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
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

//
func TestConstructInitActor(t *testing.T) {
	ctx := context.Background()
	receiver := spect.NewIDAddr(t, 1000)
	builder := specmock.NewBuilder(context.Background(), receiver).WithCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fms := pch.NewFakeActorInterface(t, ctx, builder)
	assert.NotNil(t, fms)
}

func TestCreatePaymentChannel(t *testing.T) {
	ctx := context.Background()

	balance := abi.NewTokenAmount(1000000)
	chainBuilder, bs, genTs := testSetup2(ctx, t)
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	builder := specmock.NewBuilder(context.Background(), builtin.InitActorAddr).
		WithBalance(balance, abi.NewTokenAmount(0))

	fms := pch.NewFakeActorInterface(t, ctx, builder)
	assert.NotNil(t, fms)
	rt := fms.Runtime

	channelAmt := abi.NewTokenAmount(101)
	_, client, miner, paychID, paych := fms.StubCtorSendResponse(channelAmt)
	fakeProvider := message.NewFakeProvider(t)
	fakeProvider.Builder = chainBuilder
	clientActor := actor.NewActor(builtin.AccountActorCodeID, balance)
	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(client, clientActor)

	viewer := pch.NewFakeStateViewer(t)

	pchMgr := pch.NewManager(context.Background(), ds, fms, fms, viewer)

	viewer.AddActorWithState(paych, client, miner, address.Undef)

	rmc := retrievalmarketconnector.NewRetrievalMarketClientFakeAPI(t)

	rcnc := retrievalmarketconnector.NewRetrievalClientConnector(bs, fakeProvider, rmc, pchMgr)
	assert.NotNil(t, rcnc)
	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	res, err := rcnc.GetOrCreatePaymentChannel(ctx, client, miner, channelAmt, tok)
	require.NoError(t, err)
	assert.Equal(t, paych, res)
	var st init_.State
	rt.GetState(&st)
	actualIdAddr, err := st.ResolveAddress(adt.AsStore(rt), paych)
	require.NoError(t, err)
	require.Equal(t, paychID, actualIdAddr)
}

func testSetup2(ctx context.Context, t *testing.T) (*chain.Builder, bstore.Blockstore, block.TipSet) {
	_, builder, genTs, cs, st1 := requireNewEmptyChainStore(ctx, t)
	rootBlk := builder.AppendBlockOnBlocks()
	th.RequireNewTipSet(t, rootBlk)
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

	// TODO: use a real provider?

	fakeProvider := message.NewFakeProvider(t)
	fakeProvider.Builder = builder
	clientAddr := spect.NewIDAddr(t, 102)
	clientActor := actor.NewActor(builtin.AccountActorCodeID, bal)
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
