package paymentchannel_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
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

// TestCreatePaymentChannel tests that the RetrievalClient can call through to the InitActor and
// successfully cause a new payment channel to be created.
func TestCreatePaymentChannel(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	balance := abi.NewTokenAmount(1000000)
	chainBuilder, bs, genTs := testSetup2(ctx, t)
	ds := dss.MutexWrap(datastore.NewMapDatastore())
	fms := paychtest.NewFakeInitActorUtil(ctx, t, balance)
	rt := fms.Runtime
	root, err := chainBuilder.GetTipSetStateRoot(genTs.Key())
	require.NoError(t, err)

	channelAmt := abi.NewTokenAmount(101)
	_, client, miner, paychID, paych := fms.StubCtorSendResponse(channelAmt)
	fakeProvider := message.NewFakeProvider(t)
	fakeProvider.Builder = chainBuilder
	clientActor := actor.NewActor(builtin.AccountActorCodeID, balance, root)
	fakeProvider.SetHead(genTs.Key())
	fakeProvider.SetActor(client, clientActor)

	viewer := paychtest.NewFakeStateViewer(t)

	pchMgr := pch.NewManager(context.Background(), ds, fms, fms, viewer)

	viewer.GetFakeStateView().AddActorWithState(paych, client, miner, address.Undef)

	rmc := retrievalmarketconnector.NewRetrievalMarketClientFakeAPI(t)

	rcnc := retrievalmarketconnector.NewRetrievalClientConnector(bs, fakeProvider, rmc, pchMgr)
	assert.NotNil(t, rcnc)
	tok, err := encoding.Encode(genTs.Key())
	require.NoError(t, err)

	res, _, err := rcnc.GetOrCreatePaymentChannel(ctx, client, miner, channelAmt, tok)
	require.NoError(t, err)
	assert.Equal(t, paych, res)
	var st init_.State
	rt.GetState(&st)
	actualIDAddr, err := st.ResolveAddress(adt.AsStore(rt), paych)
	require.NoError(t, err)
	require.Equal(t, paychID, actualIDAddr)
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
